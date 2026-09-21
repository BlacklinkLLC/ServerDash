// Package automation runs ServerDash's built-in maintenance jobs
// (auto-restarting unhealthy containers, pruning, scheduled backups) and
// the cron schedule for user-defined scripts.
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"serverdash/internal/containers"
	"serverdash/internal/scripts"
	"serverdash/internal/store"
	"serverdash/internal/workflow"
)

type Runner struct {
	store    *store.Store
	docker   *containers.Client
	executor *workflow.Executor

	mu         sync.Mutex
	cron       *cron.Cron
	unhealthyC chan struct{}
}

func NewRunner(st *store.Store, docker *containers.Client, composeCmd []string) *Runner {
	return &Runner{
		store:    st,
		docker:   docker,
		executor: &workflow.Executor{Docker: docker, ComposeCmd: composeCmd},
	}
}

// Start begins running the unhealthy-restart ticker and loads the cron
// schedule from the database. Call Reload after any change to automation
// rules or scripts so the running schedule picks it up without a restart.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	if r.unhealthyC == nil {
		r.unhealthyC = make(chan struct{})
		go r.runUnhealthyLoop(ctx, r.unhealthyC)
	}
	r.mu.Unlock()

	if err := r.Reload(ctx); err != nil {
		log.Printf("automation: initial schedule load failed: %v", err)
	}
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unhealthyC != nil {
		close(r.unhealthyC)
		r.unhealthyC = nil
	}
	if r.cron != nil {
		r.cron.Stop()
		r.cron = nil
	}
}

// Reload rebuilds the cron schedule from the current automation_rules and
// scripts tables. It's deliberately a full rebuild rather than incremental
// add/remove bookkeeping — simpler to get right, and this only runs on
// admin edits, not in a hot path.
func (r *Runner) Reload(ctx context.Context) error {
	newCron := cron.New()

	prune, err := r.store.GetAutomationRule(ctx, store.RulePrune)
	if err != nil {
		return err
	}
	if prune.Enabled && prune.Schedule != "" {
		if _, err := newCron.AddFunc(prune.Schedule, func() { r.runPrune(context.Background()) }); err != nil {
			log.Printf("automation: bad prune schedule %q: %v", prune.Schedule, err)
		}
	}

	backup, err := r.store.GetAutomationRule(ctx, store.RuleBackup)
	if err != nil {
		return err
	}
	if backup.Enabled && backup.Schedule != "" {
		var cfg backupConfig
		if err := json.Unmarshal([]byte(backup.Config), &cfg); err != nil {
			log.Printf("automation: bad backup config: %v", err)
		} else if _, err := newCron.AddFunc(backup.Schedule, func() { r.runBackup(context.Background(), cfg) }); err != nil {
			log.Printf("automation: bad backup schedule %q: %v", backup.Schedule, err)
		}
	}

	scriptList, err := r.store.ListScripts(ctx)
	if err != nil {
		return err
	}
	for _, sc := range scriptList {
		if !sc.Enabled || sc.Schedule == "" {
			continue
		}
		sc := sc
		if _, err := newCron.AddFunc(sc.Schedule, func() {
			if err := scripts.RunAndRecord(context.Background(), r.store, sc, "schedule"); err != nil {
				log.Printf("automation: script %q run failed: %v", sc.Name, err)
			}
		}); err != nil {
			log.Printf("automation: bad schedule %q for script %q: %v", sc.Schedule, sc.Name, err)
		}
	}

	workflows, err := r.store.ListWorkflows(ctx)
	if err != nil {
		return err
	}
	for _, wf := range workflows {
		if !wf.Enabled {
			continue
		}
		spec, err := cronSpecFor(wf)
		if err != nil {
			log.Printf("automation: workflow %q has an invalid trigger: %v", wf.Name, err)
			continue
		}
		wf := wf
		if _, err := newCron.AddFunc(spec, func() { r.RunWorkflow(context.Background(), wf, "schedule") }); err != nil {
			log.Printf("automation: bad schedule %q for workflow %q: %v", spec, wf.Name, err)
		}
	}

	r.mu.Lock()
	old := r.cron
	r.cron = newCron
	r.mu.Unlock()

	newCron.Start()
	if old != nil {
		old.Stop()
	}
	return nil
}

// cronSpecFor turns a workflow's "HH:MM in this IANA timezone" trigger into
// a robfig/cron spec. The CRON_TZ= prefix is what lets each workflow run in
// its own timezone rather than the process's — e.g. "at midnight America/
// Chicago" keeps firing at local midnight across a DST transition, which a
// UTC-only cron spec computed once wouldn't.
func cronSpecFor(wf store.Workflow) (string, error) {
	parts := strings.SplitN(wf.TriggerTime, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("trigger time %q must be HH:MM", wf.TriggerTime)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return "", fmt.Errorf("invalid hour in %q", wf.TriggerTime)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return "", fmt.Errorf("invalid minute in %q", wf.TriggerTime)
	}
	tz := wf.TriggerTZ
	if tz == "" {
		tz = "UTC"
	}
	return fmt.Sprintf("CRON_TZ=%s %d %d * * *", tz, minute, hour), nil
}

// RunWorkflow executes a workflow's blocks and records the result, shared
// between the cron schedule above and the admin panel's "run now" button.
func (r *Runner) RunWorkflow(ctx context.Context, wf store.Workflow, triggeredBy string) {
	runID, err := r.store.StartWorkflowRun(ctx, wf.ID, triggeredBy)
	if err != nil {
		log.Printf("automation: record workflow run start for %q: %v", wf.Name, err)
		return
	}

	blocks, err := workflow.ParseBlocks(wf.Blocks)
	if err != nil {
		_ = r.store.FinishWorkflowRun(ctx, runID, false, err.Error())
		return
	}

	logText, ok := r.executor.Run(ctx, blocks)
	if err := r.store.FinishWorkflowRun(ctx, runID, ok, logText); err != nil {
		log.Printf("automation: record workflow run finish for %q: %v", wf.Name, err)
	}
}

func (r *Runner) runUnhealthyLoop(ctx context.Context, stop chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			rule, err := r.store.GetAutomationRule(ctx, store.RuleAutoRestartUnhealthy)
			if err != nil || !rule.Enabled {
				continue
			}
			r.restartUnhealthy(ctx)
		}
	}
}

func (r *Runner) restartUnhealthy(ctx context.Context) {
	list, err := r.docker.List(ctx)
	if err != nil {
		log.Printf("automation: list containers for health check failed: %v", err)
		return
	}
	for _, c := range list {
		if !c.Unhealthy() {
			continue
		}
		log.Printf("automation: restarting unhealthy container %s (%s)", c.Name, c.ID[:12])
		if err := r.docker.Restart(ctx, c.ID); err != nil {
			log.Printf("automation: restart of %s failed: %v", c.Name, err)
		}
	}
}

func (r *Runner) runPrune(ctx context.Context) {
	report, err := r.docker.Prune(ctx)
	if err != nil {
		log.Printf("automation: prune failed: %v", err)
		return
	}
	log.Printf("automation: prune reclaimed %d bytes (%d containers, %d images, %d volumes)",
		report.SpaceReclaimed, len(report.ContainersDeleted), report.ImagesDeleted, len(report.VolumesDeleted))
}

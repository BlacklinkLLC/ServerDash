// Package automation runs ServerDash's built-in maintenance jobs
// (auto-restarting unhealthy containers, pruning, scheduled backups) and
// the cron schedule for user-defined scripts.
package automation

import (
	"context"
	"encoding/json"
	"log"
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
	hostRoot string

	mu         sync.Mutex
	cron       *cron.Cron
	unhealthyC chan struct{}

	// running guards against overlapping runs of one workflow (a slow
	// deploy on a 1-minute interval, or an event that keeps firing).
	runMu   sync.Mutex
	running map[int64]bool

	events *eventWatcher
}

func NewRunner(st *store.Store, docker *containers.Client, composeCmd []string, hostRoot string) *Runner {
	return &Runner{
		store:    st,
		docker:   docker,
		hostRoot: hostRoot,
		executor: &workflow.Executor{Docker: docker, ComposeCmd: composeCmd, Store: st, HostRoot: hostRoot},
		running:  map[int64]bool{},
		events:   newEventWatcher(),
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
		go r.runEventLoop(ctx, r.unhealthyC)
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
		spec, err := workflow.ParseOptions(wf.Options, wf.TriggerTime, wf.TriggerTZ).Trigger.CronSpec()
		if err != nil {
			log.Printf("automation: workflow %q has an invalid trigger: %v", wf.Name, err)
			continue
		}
		if spec == "" {
			continue // event, webhook, or manual trigger: not on the cron schedule
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

// RunWorkflow executes a workflow's blocks and records the result, shared
// between the cron schedule above and the "run now" button.
func (r *Runner) RunWorkflow(ctx context.Context, wf store.Workflow, triggeredBy string) {
	r.RunWorkflowWith(ctx, wf, workflow.Context{Trigger: triggeredBy})
}

// RunWorkflowWith is RunWorkflow with event details (what fired it, a
// webhook payload) available to the blocks as {{variables}}. It returns
// false without running if this workflow is already mid-run.
func (r *Runner) RunWorkflowWith(ctx context.Context, wf store.Workflow, rc workflow.Context) bool {
	r.runMu.Lock()
	if r.running[wf.ID] {
		r.runMu.Unlock()
		log.Printf("automation: skipping %s run of %q: the previous run is still going", rc.Trigger, wf.Name)
		return false
	}
	r.running[wf.ID] = true
	r.runMu.Unlock()
	defer func() {
		r.runMu.Lock()
		delete(r.running, wf.ID)
		r.runMu.Unlock()
	}()

	rc.Workflow = wf.Name
	triggeredBy := rc.Trigger
	runID, err := r.store.StartWorkflowRun(ctx, wf.ID, triggeredBy)
	if err != nil {
		log.Printf("automation: record workflow run start for %q: %v", wf.Name, err)
		return true
	}

	blocks, err := workflow.ParseBlocks(wf.Blocks)
	if err != nil {
		_ = r.store.FinishWorkflowRun(ctx, runID, false, err.Error())
		return true
	}

	logText, ok := r.executor.Run(ctx, blocks, rc)
	if err := r.store.FinishWorkflowRun(ctx, runID, ok, logText); err != nil {
		log.Printf("automation: record workflow run finish for %q: %v", wf.Name, err)
	}
	if !ok {
		opts := workflow.ParseOptions(wf.Options, wf.TriggerTime, wf.TriggerTZ)
		if opts.OnFailure.URL != "" {
			r.executor.SendAlert(ctx, opts.OnFailure, wf.Name, rc, logText)
		}
	}
	return true
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

package automation

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"serverdash/internal/store"
	"serverdash/internal/system"
	"serverdash/internal/systemd"
	"serverdash/internal/workflow"
)

const eventInterval = 20 * time.Second

// eventWatcher holds per-workflow state for event triggers between ticks:
// when a metric first crossed its threshold, which containers/services
// were already in a bad state (so only transitions fire), and when each
// workflow last fired (for the cooldown).
type eventWatcher struct {
	mu        sync.Mutex
	since     map[int64]time.Time
	bad       map[int64]map[string]bool
	lastFired map[int64]time.Time
	primed    map[int64]bool
	// excursion is true once a metric trigger fired for the current spike;
	// it re-arms only after the metric drops back under the threshold.
	excursion map[int64]bool
}

func newEventWatcher() *eventWatcher {
	return &eventWatcher{
		since:     map[int64]time.Time{},
		bad:       map[int64]map[string]bool{},
		lastFired: map[int64]time.Time{},
		primed:    map[int64]bool{},
		excursion: map[int64]bool{},
	}
}

func (r *Runner) runEventLoop(ctx context.Context, stop chan struct{}) {
	t := time.NewTicker(eventInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-t.C:
			r.checkEvents(ctx)
		}
	}
}

// snapshotCache lazily collects each data source at most once per tick,
// however many workflows need it.
type snapshotCache struct {
	r         *Runner
	ctx       context.Context
	snap      *system.Snapshot
	ctrs      map[string]string // name -> "running" | "stopped" | "unhealthy"
	ctrsErr   error
	units     map[string]string // name -> active state
	unitsDone bool
}

func (c *snapshotCache) system() (*system.Snapshot, error) {
	if c.snap == nil {
		s, err := system.Collect(c.r.hostRoot)
		if err != nil {
			return nil, err
		}
		c.snap = &s
	}
	return c.snap, nil
}

func (c *snapshotCache) containers() (map[string]string, error) {
	if c.ctrs == nil && c.ctrsErr == nil {
		list, err := c.r.docker.List(c.ctx)
		if err != nil {
			c.ctrsErr = err
		} else {
			c.ctrs = map[string]string{}
			for _, x := range list {
				state := "stopped"
				if x.State == "running" {
					state = "running"
				}
				if x.Unhealthy() {
					state = "unhealthy"
				}
				c.ctrs[strings.TrimPrefix(x.Name, "/")] = state
			}
		}
	}
	return c.ctrs, c.ctrsErr
}

func (c *snapshotCache) services() map[string]string {
	if !c.unitsDone {
		c.unitsDone = true
		c.units = map[string]string{}
		if systemd.Available() {
			if list, err := systemd.List(c.ctx); err == nil {
				for _, u := range list {
					c.units[strings.TrimSuffix(u.Name, ".service")] = u.Active
				}
			}
		}
	}
	return c.units
}

func (r *Runner) checkEvents(ctx context.Context) {
	wfs, err := r.store.ListWorkflows(ctx)
	if err != nil {
		return
	}
	cache := &snapshotCache{r: r, ctx: ctx}
	w := r.events
	live := map[int64]bool{}

	for _, wf := range wfs {
		opts := workflow.ParseOptions(wf.Options, wf.TriggerTime, wf.TriggerTZ)
		t := opts.Trigger
		if !wf.Enabled || !t.IsEvent() {
			continue
		}
		live[wf.ID] = true
		detail := r.evaluate(wf, t, cache)
		if detail == "" {
			continue
		}
		w.mu.Lock()
		last := w.lastFired[wf.ID]
		if !last.IsZero() && time.Since(last) < t.Cooldown() {
			w.mu.Unlock()
			continue
		}
		w.lastFired[wf.ID] = time.Now()
		w.mu.Unlock()

		log.Printf("automation: event trigger for %q: %s", wf.Name, detail)
		wf := wf
		go r.RunWorkflowWith(context.Background(), wf, workflow.Context{Trigger: "event", Event: detail})
	}

	// Forget state for workflows that were deleted, disabled, or changed
	// to a non-event trigger, so re-enabling starts fresh.
	w.mu.Lock()
	for id := range w.primed {
		if !live[id] {
			delete(w.primed, id)
			delete(w.since, id)
			delete(w.bad, id)
			delete(w.excursion, id)
		}
	}
	w.mu.Unlock()
}

// evaluate returns a description of what happened if the trigger should
// fire now, or "" if not.
func (r *Runner) evaluate(wf store.Workflow, t workflow.Trigger, c *snapshotCache) string {
	w := r.events
	switch t.Type {
	case workflow.TriggerMetric:
		snap, err := c.system()
		if err != nil {
			return ""
		}
		var v float64
		label, unit := t.Metric, "%"
		switch t.Metric {
		case "cpu":
			v, label = snap.CPUPercent, "CPU"
		case "mem":
			v, label = snap.MemUsedPercent, "Memory"
		case "disk":
			for _, d := range snap.Disks {
				if d.UsedPercent > v {
					v = d.UsedPercent
				}
			}
			label = "Disk"
		case "load":
			v, label, unit = snap.LoadAvg1, "Load", ""
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		w.primed[wf.ID] = true
		if v <= t.Threshold {
			delete(w.since, wf.ID)
			delete(w.excursion, wf.ID)
			return ""
		}
		start, ok := w.since[wf.ID]
		if !ok {
			start = time.Now()
			w.since[wf.ID] = start
		}
		if w.excursion[wf.ID] || time.Since(start) < time.Duration(t.ForMinutes)*time.Minute {
			return ""
		}
		w.excursion[wf.ID] = true
		dur := ""
		if t.ForMinutes > 0 {
			dur = fmt.Sprintf(" for %dm", t.ForMinutes)
		}
		return fmt.Sprintf("%s at %.1f%s (threshold %g%s)%s", label, v, unit, t.Threshold, unit, dur)

	case workflow.TriggerContainer:
		ctrs, err := c.containers()
		if err != nil {
			return ""
		}
		return w.transitions(wf.ID, ctrs, t.Pattern, func(state string) bool {
			switch t.Event {
			case "stopped":
				return state == "stopped"
			case "unhealthy":
				return state == "unhealthy"
			}
			return state != "running"
		}, "container")

	case workflow.TriggerService:
		return w.transitions(wf.ID, c.services(), t.Pattern, func(state string) bool { return state == "failed" }, "service")
	}
	return ""
}

// transitions reports items matching pattern that newly entered a bad
// state since the last tick. The first tick after (re)start only records
// the current state, so already-stopped containers don't all fire at boot.
func (w *eventWatcher) transitions(id int64, states map[string]string, pattern string, isBad func(string) bool, kind string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	prev := w.bad[id]
	now := map[string]bool{}
	var fired []string
	for name, state := range states {
		if ok, _ := path.Match(pattern, name); !ok {
			continue
		}
		if isBad(state) {
			now[name] = true
			if w.primed[id] && !prev[name] {
				fired = append(fired, fmt.Sprintf("%s %s is %s", kind, name, state))
			}
		}
	}
	w.bad[id] = now
	w.primed[id] = true
	return strings.Join(fired, "; ")
}

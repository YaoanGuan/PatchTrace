package dispatch

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"
)

// lastReason is the per-project snapshot recorded after a reason task is
// planned. Comparing the current graph counts against it implements the
// "new situation" trigger definition.
type lastReason struct {
	FactCount int
	HintCount int
	HadOpen   bool
}

// Dispatcher owns scheduling state for one graph server. It is intentionally
// single-loop: Run is expected to be the only goroutine calling tick, so the
// maps below need no mutex (task runners added later must respect that).
type Dispatcher struct {
	cfg    Config
	client graphserver.Client

	registry *worker.Registry
	running  map[string]int

	lastReason map[string]lastReason

	// randIntn breaks ties between otherwise equivalent workers. It is a field
	// so tests can inject a deterministic choice.
	randIntn func(int) int
}

// New builds a dispatcher around the graph client and worker drivers.
func New(cfg Config, client graphserver.Client, drivers []worker.Driver) *Dispatcher {
	return &Dispatcher{
		cfg:        cfg,
		client:     client,
		registry:   worker.NewRegistry(worker.DefaultUnhealthyCooldown, drivers...),
		running:    make(map[string]int),
		lastReason: make(map[string]lastReason),
		randIntn:   rand.Intn,
	}
}

// Run ticks the scheduler every Config.Interval until ctx is cancelled. A
// transient graph error is logged and retried on the next tick rather than
// stopping the dispatcher.
//
// Tasks are planned and their worker selected here; executing them is the job
// of the dispatch/tasks runners (Tasks 7-9), which consume Decision values.
func (d *Dispatcher) Run(ctx context.Context) error {
	if d.cfg.Interval <= 0 {
		return fmt.Errorf("dispatch: runtime.interval must be positive")
	}
	ticker := time.NewTicker(d.cfg.Interval)
	defer ticker.Stop()

	for {
		if _, err := d.tick(ctx); err != nil && ctx.Err() == nil {
			misc.Warn("dispatch", "scheduler tick failed: "+err.Error(), nil)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// tick performs one scheduling pass and returns the decisions it would
// dispatch. Decisions carry the selected WorkerName.
func (d *Dispatcher) tick(ctx context.Context) ([]Decision, error) {
	projects, err := d.client.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("dispatch: list projects: %w", err)
	}
	d.pruneLastReason(projects)

	var (
		decisions []Decision
		firstErr  error
	)
	for _, p := range projects {
		dec, stats, ok, err := d.planProject(ctx, p)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		plannedReason := false
		if ok {
			if driver, selected := d.selectWorker(dec.TaskType); selected {
				dec.WorkerName = driver.Name()
				if dec.TaskType == worker.TaskReason {
					// Advance the fact/hint baseline only when a worker can
					// actually take the reason task; otherwise a later tick can
					// still observe the new situation.
					d.lastReason[p.ID] = lastReason{
						FactCount: stats.FactCount,
						HintCount: stats.HintCount,
						HadOpen:   stats.OpenIntents > 0,
					}
					plannedReason = true
				}
				decisions = append(decisions, dec)
			}
		}

		// Track the open-intent transition on every observation. Without this
		// refresh the "open intents drained" trigger could never fire, because
		// reason only runs when the open count is already zero.
		if !plannedReason {
			if prev, exists := d.lastReason[p.ID]; exists {
				prev.HadOpen = stats.OpenIntents > 0
				d.lastReason[p.ID] = prev
			}
		}
	}
	return decisions, firstErr
}

// pruneLastReason drops state for projects the server no longer lists (deleted
// projects, or projects filtered out of the active list).
func (d *Dispatcher) pruneLastReason(projects []graph.Project) {
	if len(d.lastReason) == 0 {
		return
	}
	keep := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		keep[p.ID] = struct{}{}
	}
	for id := range d.lastReason {
		if _, ok := keep[id]; !ok {
			delete(d.lastReason, id)
		}
	}
}

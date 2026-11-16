package dispatch

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"
	"AIxVuln/patchintel"
)

// maxBootstrapAttempts bounds how many consecutive bootstrap starts a project
// may make while it stays in its initial state. A failing bootstrap keeps the
// count; a project that leaves the initial state resets it.
const maxBootstrapAttempts = 3

// lastReason is the per-project snapshot recorded after a successful reason
// task. Comparing the current graph counts against it implements the "new
// situation" trigger definition.
type lastReason struct {
	FactCount int
	HintCount int
	HadOpen   bool
}

// Dispatcher owns scheduling state for one graph server. It is intentionally
// single-loop: Run is expected to be the only goroutine calling tick, so the
// scheduler maps need no mutex. The reason baseline is the exception: the
// executor's success hook runs in a task goroutine, so stateMu guards it.
type Dispatcher struct {
	cfg    Config
	client graphserver.Client

	registry *worker.Registry
	running  map[string]int
	executor *Executor
	// store and ruleBase are the lane's stores. StartLane always records them
	// so embedders can mount the HTTP API; ownsStore/ownsRuleBase decide
	// whether Close releases them.
	store        *graph.Store
	ownsStore    bool
	ruleBase     *patchintel.RuleBase
	ownsRuleBase bool

	stateMu    sync.Mutex
	lastReason map[string]lastReason

	// bootstrapAttempts counts consecutive bootstrap starts per project.
	// bootstrapCappedWarned makes the retry-cap warning fire once per attempt
	// streak; both are tick-owned and reset when the project leaves its
	// initial state.
	bootstrapAttempts     map[string]int
	bootstrapCappedWarned map[string]bool

	// randIntn breaks ties between otherwise equivalent workers. It is a field
	// so tests can inject a deterministic choice.
	randIntn func(int) int
}

// New builds a dispatcher around the graph client and worker drivers.
func New(cfg Config, client graphserver.Client, drivers []worker.Driver) *Dispatcher {
	return &Dispatcher{
		cfg:                   cfg,
		client:                client,
		registry:              worker.NewRegistry(worker.DefaultUnhealthyCooldown, drivers...),
		running:               make(map[string]int),
		lastReason:            make(map[string]lastReason),
		bootstrapAttempts:     make(map[string]int),
		bootstrapCappedWarned: make(map[string]bool),
		randIntn:              rand.Intn,
	}
}

// Store returns the graph store the lane works against, or nil for a
// dispatcher built directly with New. It lets main.go mount graphserver's HTTP
// routes on the same store the lane uses.
func (d *Dispatcher) Store() *graph.Store {
	if d == nil {
		return nil
	}
	return d.store
}

// Wait blocks until every executor goroutine started so far has returned. Call
// it after Run has stopped (the context is cancelled) and before closing stores
// the executor may still be using.
func (d *Dispatcher) Wait() {
	if d == nil {
		return
	}
	d.executor.Wait()
}

// Close releases the graph store and rule base opened by StartLane. A
// dispatcher built with New, or one using caller-supplied stores, has nothing
// to close and returns nil.
func (d *Dispatcher) Close() error {
	if d == nil {
		return nil
	}
	var errs []error
	if d.ownsStore && d.store != nil {
		errs = append(errs, d.store.Close())
	}
	if d.ownsRuleBase && d.ruleBase != nil {
		errs = append(errs, d.ruleBase.Close())
	}
	return errors.Join(errs...)
}

// SetExecutor wires the execution layer. A nil executor restores plan-only
// behaviour: tick returns decisions without running them. When the executor
// has no success hook yet, the dispatcher installs its baseline recorder.
func (d *Dispatcher) SetExecutor(e *Executor) {
	d.executor = e
	if e == nil {
		return
	}
	if e.deps.onReasonSuccess == nil {
		e.deps.onReasonSuccess = d.recordReasonSuccess
		return
	}
	previous := e.deps.onReasonSuccess
	e.deps.onReasonSuccess = func(p graph.Project, stats graphStats) {
		previous(p, stats)
		d.recordReasonSuccess(p, stats)
	}
}

// recordReasonSuccess advances the "new situation" baseline. It is called only
// after a reason runner returns without error, so failures and lease-held skips
// leave the trigger armed for the next tick.
func (d *Dispatcher) recordReasonSuccess(p graph.Project, stats graphStats) {
	d.stateMu.Lock()
	d.lastReason[p.ID] = lastReason{
		FactCount: stats.FactCount,
		HintCount: stats.HintCount,
		HadOpen:   stats.OpenIntents > 0,
	}
	d.stateMu.Unlock()
}

// Run ticks the scheduler every Config.Interval until ctx is cancelled. A
// transient graph error is logged and retried on the next tick rather than
// stopping the dispatcher.
//
// Tasks are planned and their worker selected here. When an executor is wired,
// each selected decision is started through it; otherwise the decisions are
// only returned (plan-only mode used by tests and embedders).
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

// tick performs one scheduling pass and returns the decisions it dispatched.
// Decisions carry the selected WorkerName. A reason baseline advances only when
// the executor reports a successful run, never at plan time.
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

		// Leaving the initial state means bootstrap succeeded (or never
		// applied): the retry budget and its warning are no longer relevant.
		if !stats.Initial {
			d.resetBootstrapAttempts(p.ID)
		}

		// Bounded bootstrap retry: stop starting new attempts once the streak
		// reaches the cap, and warn once rather than every tick.
		if ok && dec.TaskType == worker.TaskBootstrap && d.bootstrapAttempts[p.ID] >= maxBootstrapAttempts {
			d.warnBootstrapCapOnce(p.ID)
			ok = false
		}

		plannedReason := false
		if ok {
			if driver, selected := d.selectWorker(dec.TaskType); selected {
				dec.WorkerName = driver.Name()
				dec.stats = stats

				started := true
				if d.executor != nil {
					started = d.executor.Start(ctx, dec, p)
				}
				if started {
					decisions = append(decisions, dec)
					if d.executor != nil && dec.TaskType == worker.TaskBootstrap {
						d.bootstrapAttempts[p.ID]++
					}
					if dec.TaskType == worker.TaskReason {
						plannedReason = true
					}
				}
			}
		}

		// Track the open-intent transition on every observation. Without this
		// refresh the "open intents drained" trigger could never fire, because
		// reason only runs when the open count is already zero.
		if !plannedReason {
			d.stateMu.Lock()
			if prev, exists := d.lastReason[p.ID]; exists {
				prev.HadOpen = stats.OpenIntents > 0
				d.lastReason[p.ID] = prev
			}
			d.stateMu.Unlock()
		}
	}
	return decisions, firstErr
}

func (d *Dispatcher) warnBootstrapCapOnce(projectID string) {
	if d.bootstrapCappedWarned == nil {
		d.bootstrapCappedWarned = make(map[string]bool)
	}
	if d.bootstrapCappedWarned[projectID] {
		return
	}
	d.bootstrapCappedWarned[projectID] = true
	misc.Warn("dispatch", fmt.Sprintf(
		"bootstrap retry cap reached for project %s after %d consecutive starts; skipping until the graph leaves its initial state",
		projectID, maxBootstrapAttempts), nil)
}

func (d *Dispatcher) resetBootstrapAttempts(projectID string) {
	delete(d.bootstrapAttempts, projectID)
	delete(d.bootstrapCappedWarned, projectID)
}

// pruneLastReason drops state for projects the server no longer lists (deleted
// projects, or projects filtered out of the active list).
func (d *Dispatcher) pruneLastReason(projects []graph.Project) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()

	keep := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		keep[p.ID] = struct{}{}
	}
	for id := range d.lastReason {
		if _, ok := keep[id]; !ok {
			delete(d.lastReason, id)
		}
	}
	for id := range d.bootstrapAttempts {
		if _, ok := keep[id]; !ok {
			delete(d.bootstrapAttempts, id)
		}
	}
	for id := range d.bootstrapCappedWarned {
		if _, ok := keep[id]; !ok {
			delete(d.bootstrapCappedWarned, id)
		}
	}
}

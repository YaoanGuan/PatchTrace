package dispatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/tasks"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"
)

// executorDeps holds the task runners and the success hook. It is a struct (not
// package variables) so tests can drive every decision with fakes.
type executorDeps struct {
	bootstrap func(context.Context, graphserver.Client, worker.Driver, graph.Project) error
	reason    func(context.Context, graphserver.Client, worker.Driver, graph.Project) error
	// explore is the plain runner signature; exploreWithOptions, when set, is
	// preferred so the configured conclude budget reaches RunExploreWithOptions.
	explore            func(context.Context, graphserver.Client, worker.Driver, runtime.SandboxRuntime, graph.Project, string) error
	exploreWithOptions func(context.Context, graphserver.Client, worker.Driver, runtime.SandboxRuntime, graph.Project, string, tasks.ExploreOptions) error
	// onReasonSuccess runs after a reason task returns without error. The
	// dispatcher records the new situation baseline through it, so a skipped or
	// failed reason never advances the trigger.
	onReasonSuccess func(graph.Project, graphStats)
}

func defaultExecutorDeps() executorDeps {
	return executorDeps{
		bootstrap:          tasks.RunBootstrap,
		reason:             tasks.RunReason,
		explore:            tasks.RunExplore,
		exploreWithOptions: tasks.RunExploreWithOptions,
	}
}

// Executor is the bounded execution layer for Dispatcher decisions. It owns
// in-flight counts and runs each decision in its own goroutine, with per-type
// deadlines and the dispatcher's concurrency caps checked atomically in Start.
type Executor struct {
	cfg      Config
	client   graphserver.Client
	registry *worker.Registry
	rt       runtime.SandboxRuntime
	deps     executorDeps

	mu          sync.Mutex
	inFlight    int
	projectRuns map[string]int
	workerRuns  map[string]int
	wg          sync.WaitGroup
}

// NewExecutor builds the production executor: every decision runs through the
// tasks.Run* entry points.
func NewExecutor(cfg Config, client graphserver.Client, registry *worker.Registry, rt runtime.SandboxRuntime) *Executor {
	return newExecutor(cfg, client, registry, rt, executorDeps{})
}

// newExecutor is the injectable form used by tests. Missing runner fields fall
// back to the tasks.Run* functions; an explicitly injected explore runner wins
// over the default options-aware one.
func newExecutor(cfg Config, client graphserver.Client, registry *worker.Registry, rt runtime.SandboxRuntime, deps executorDeps) *Executor {
	defaults := defaultExecutorDeps()
	if deps.bootstrap == nil {
		deps.bootstrap = defaults.bootstrap
	}
	if deps.reason == nil {
		deps.reason = defaults.reason
	}
	if deps.explore == nil && deps.exploreWithOptions == nil {
		deps.explore = defaults.explore
		deps.exploreWithOptions = defaults.exploreWithOptions
	}
	return &Executor{
		cfg:         cfg,
		client:      client,
		registry:    registry,
		rt:          rt,
		deps:        deps,
		projectRuns: make(map[string]int),
		workerRuns:  make(map[string]int),
	}
}

// Start atomically checks the concurrency caps and, when they allow it, runs
// dec in a new goroutine. It returns false when a cap forbids starting; the
// next scheduler tick re-plans the decision.
func (e *Executor) Start(ctx context.Context, dec Decision, p graph.Project) bool {
	if e == nil {
		return false
	}
	projectID := decisionProjectID(dec, p)

	e.mu.Lock()
	if e.cfg.MaxWorkers > 0 && e.inFlight >= e.cfg.MaxWorkers {
		e.mu.Unlock()
		return false
	}
	if e.cfg.MaxProjectWorkers > 0 && e.projectRuns[projectID] >= e.cfg.MaxProjectWorkers {
		e.mu.Unlock()
		return false
	}
	if e.cfg.MaxRunningProjects > 0 && e.projectRuns[projectID] == 0 && len(e.projectRuns) >= e.cfg.MaxRunningProjects {
		e.mu.Unlock()
		return false
	}
	e.inFlight++
	e.projectRuns[projectID]++
	e.workerRuns[dec.WorkerName]++
	e.wg.Add(1)
	e.mu.Unlock()

	go e.run(ctx, dec, p)
	return true
}

// Running reports the in-flight task count for a project.
func (e *Executor) Running(projectID string) int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.projectRuns[projectID]
}

// RunningByWorker reports the in-flight task count for a worker name.
func (e *Executor) RunningByWorker(name string) int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.workerRuns[name]
}

// InFlight reports the total in-flight task count.
func (e *Executor) InFlight() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.inFlight
}

// Wait blocks until every goroutine started so far has returned. It is meant
// for tests and clean shutdown; call it only after Run stopped starting work.
func (e *Executor) Wait() {
	if e == nil {
		return
	}
	e.wg.Wait()
}

func (e *Executor) run(ctx context.Context, dec Decision, p graph.Project) {
	// Defers run LIFO: release the counts before Done so Wait returning means
	// the executor is fully quiescent.
	defer e.wg.Done()
	defer e.release(dec, p)

	if e.registry == nil {
		misc.Warn("dispatch", fmt.Sprintf("executor: no worker registry; dropping %s task for project %s", dec.TaskType, decisionProjectID(dec, p)), nil)
		return
	}
	driver, ok := e.registry.Find(dec.WorkerName)
	if !ok {
		misc.Warn("dispatch", fmt.Sprintf("executor: worker %q is no longer registered; dropping %s task for project %s",
			dec.WorkerName, dec.TaskType, decisionProjectID(dec, p)), nil)
		return
	}
	if e.registry.IsUnhealthy(dec.WorkerName) {
		misc.Warn("dispatch", fmt.Sprintf("executor: worker %q is unhealthy; dropping %s task for project %s",
			dec.WorkerName, dec.TaskType, decisionProjectID(dec, p)), nil)
		return
	}

	runCtx := ctx
	if timeout := e.timeoutFor(dec.TaskType); timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	err := e.invoke(runCtx, dec, p, driver)
	if err != nil {
		if errors.Is(err, tasks.ErrReasonLeaseHeld) {
			// Another worker owns the reason lease. This is not a failure and
			// must not advance the baseline.
			return
		}
		misc.Warn("dispatch", fmt.Sprintf("executor: %s task for project %s failed: %v",
			dec.TaskType, decisionProjectID(dec, p), err), nil)
		return
	}
	if dec.TaskType == worker.TaskReason && e.deps.onReasonSuccess != nil {
		e.deps.onReasonSuccess(p, dec.stats)
	}
}

func (e *Executor) invoke(ctx context.Context, dec Decision, p graph.Project, driver worker.Driver) error {
	switch dec.TaskType {
	case worker.TaskBootstrap:
		return e.deps.bootstrap(ctx, e.client, driver, p)
	case worker.TaskReason:
		return e.deps.reason(ctx, e.client, driver, p)
	case worker.TaskExplore:
		opts := tasks.ExploreOptions{ConcludeTimeout: e.cfg.ExploreConcludeTimeout}
		if e.deps.exploreWithOptions != nil {
			return e.deps.exploreWithOptions(ctx, e.client, driver, e.rt, p, dec.IntentID, opts)
		}
		return e.deps.explore(ctx, e.client, driver, e.rt, p, dec.IntentID)
	default:
		return fmt.Errorf("executor: unknown task type %q", dec.TaskType)
	}
}

func (e *Executor) timeoutFor(tt worker.TaskType) time.Duration {
	switch tt {
	case worker.TaskBootstrap:
		return e.cfg.BootstrapTimeout
	case worker.TaskReason:
		return e.cfg.ReasonTimeout
	case worker.TaskExplore:
		return e.cfg.ExploreTimeout
	default:
		return 0
	}
}

func (e *Executor) release(dec Decision, p graph.Project) {
	projectID := decisionProjectID(dec, p)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inFlight > 0 {
		e.inFlight--
	}
	if e.projectRuns[projectID] <= 1 {
		delete(e.projectRuns, projectID)
	} else {
		e.projectRuns[projectID]--
	}
	if e.workerRuns[dec.WorkerName] <= 1 {
		delete(e.workerRuns, dec.WorkerName)
	} else {
		e.workerRuns[dec.WorkerName]--
	}
}

func decisionProjectID(dec Decision, p graph.Project) string {
	if dec.ProjectID != "" {
		return dec.ProjectID
	}
	return p.ID
}

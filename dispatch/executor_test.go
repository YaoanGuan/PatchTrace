package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/tasks"
	"AIxVuln/dispatch/worker"
	"AIxVuln/dockerManager"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
)

// --- helpers ---------------------------------------------------------------

func executorConfig(maxWorkers, maxProjects, maxProjectWorkers int) Config {
	return Config{
		MaxWorkers:         maxWorkers,
		MaxRunningProjects: maxProjects,
		MaxProjectWorkers:  maxProjectWorkers,
	}
}

func executorTestRegistry(t *testing.T, drivers ...worker.Driver) *worker.Registry {
	t.Helper()
	return worker.NewRegistry(time.Minute, drivers...)
}

// blockingRunner reports when it started and then blocks until release or the
// run context expires. It is the shape the cap tests need: a run that stays
// in-flight until the test says otherwise.
func blockingRunner(started chan<- struct{}, release <-chan struct{}) func(context.Context) error {
	return func(ctx context.Context) error {
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func startDecision(t *testing.T, e *Executor, dec Decision, p graph.Project) bool {
	t.Helper()
	return e.Start(context.Background(), dec, p)
}

// setRegistryClock pins the registry's unhealthy cooldown clock so tests can
// cross the retry window without sleeping.
func setRegistryClock(r *worker.Registry, now *time.Time) {
	r.SetClock(func() time.Time { return *now })
}

// --- executor behavior -----------------------------------------------------

func TestExecutorRunsReasonAndAdvancesBaselineOnlyOnSuccess(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{
		projects: []graph.Project{activeProject("p1", false)},
		exports:  map[string]string{"p1": projectExport(facts, 0, nil)},
	}
	cfg := executorConfig(4, 4, 2)
	cfg.ReasonTimeout = time.Minute

	d := New(cfg, client, []worker.Driver{newFakeDriver("agent", worker.TaskReason)})
	var attempts atomic.Int32
	deps := executorDeps{
		reason: func(context.Context, graphserver.Client, worker.Driver, graph.Project) error {
			if attempts.Add(1) == 1 {
				return errors.New("reason exploded")
			}
			return nil
		},
	}
	e := newExecutor(cfg, client, d.registry, nil, deps)
	d.SetExecutor(e)
	now := time.Now()
	setRegistryClock(d.registry, &now)

	decisions, err := d.tick(context.Background())
	if err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if len(decisions) != 1 || decisions[0].TaskType != worker.TaskReason {
		t.Fatalf("first tick decisions = %+v, want one reason decision", decisions)
	}
	e.Wait()
	d.stateMu.Lock()
	_, baselineSet := d.lastReason["p1"]
	d.stateMu.Unlock()
	if baselineSet {
		t.Fatal("a failed reason run must not advance the baseline")
	}
	if !d.registry.IsUnhealthy("agent") {
		t.Fatal("a failed reason run must mark the worker unhealthy")
	}
	// The next tick must skip the worker until the cooldown expires.
	decisions, err = d.tick(context.Background())
	if err != nil {
		t.Fatalf("unhealthy tick: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("unhealthy tick decisions = %+v, want none while the worker cools down", decisions)
	}
	now = now.Add(2 * time.Minute)

	decisions, err = d.tick(context.Background())
	if err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if len(decisions) != 1 || decisions[0].TaskType != worker.TaskReason {
		t.Fatalf("second tick decisions = %+v, want reason replanned after the failure", decisions)
	}
	e.Wait()
	if d.registry.IsUnhealthy("agent") {
		t.Fatal("a successful reason run must clear the unhealthy window")
	}

	decisions, err = d.tick(context.Background())
	if err != nil {
		t.Fatalf("third tick: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("third tick decisions = %+v, want no reason after the successful baseline", decisions)
	}
	d.stateMu.Lock()
	baseline := d.lastReason["p1"]
	d.stateMu.Unlock()
	if baseline.FactCount != 3 || baseline.HintCount != 0 || baseline.HadOpen {
		t.Fatalf("baseline = %+v, want the plan-time graph snapshot", baseline)
	}
	if attempts.Load() != 2 {
		t.Fatalf("reason runs = %d, want 2", attempts.Load())
	}
}

func TestExecutorSkipsWhenReasonLeaseHeld(t *testing.T) {
	cfg := executorConfig(2, 2, 1)
	registry := executorTestRegistry(t, newFakeDriver("agent", worker.TaskReason))
	var successes atomic.Int32
	deps := executorDeps{
		reason: func(context.Context, graphserver.Client, worker.Driver, graph.Project) error {
			return fmt.Errorf("%w: project p1", tasks.ErrReasonLeaseHeld)
		},
		onReasonSuccess: func(graph.Project, graphStats) { successes.Add(1) },
	}
	e := newExecutor(cfg, nil, registry, nil, deps)
	dec := Decision{ProjectID: "p1", TaskType: worker.TaskReason, WorkerName: "agent"}

	if !startDecision(t, e, dec, graph.Project{ID: "p1"}) {
		t.Fatal("the first reason start must be accepted")
	}
	e.Wait()

	if successes.Load() != 0 {
		t.Fatalf("lease-held skip advanced the baseline %d times", successes.Load())
	}
	if registry.IsUnhealthy("agent") {
		t.Fatal("a lease-held skip is not a worker failure and must not enter the cooldown")
	}
	if got := e.InFlight(); got != 0 {
		t.Fatalf("in-flight after skip = %d, want 0", got)
	}
	if got := e.Running("p1"); got != 0 {
		t.Fatalf("running for p1 after skip = %d, want 0", got)
	}
	if got := e.RunningByWorker("agent"); got != 0 {
		t.Fatalf("running for agent after skip = %d, want 0", got)
	}
	// The skip must not poison later work: another start is accepted.
	if !startDecision(t, e, dec, graph.Project{ID: "p1"}) {
		t.Fatal("a lease-held skip must not consume the executor")
	}
	e.Wait()
}

func TestExecutorEnforcesProjectCap(t *testing.T) {
	cfg := executorConfig(8, 4, 1)
	registry := executorTestRegistry(t, newFakeDriver("agent", worker.TaskReason))
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	deps := executorDeps{
		reason: func(ctx context.Context, _ graphserver.Client, _ worker.Driver, _ graph.Project) error {
			return blockingRunner(started, release)(ctx)
		},
	}
	e := newExecutor(cfg, nil, registry, nil, deps)
	proj := graph.Project{ID: "p1"}
	dec := Decision{ProjectID: "p1", TaskType: worker.TaskReason, WorkerName: "agent"}

	if !startDecision(t, e, dec, proj) {
		t.Fatal("the first start for the project must be accepted")
	}
	<-started
	if got := e.Running("p1"); got != 1 {
		t.Fatalf("running for p1 = %d, want 1", got)
	}
	if startDecision(t, e, dec, proj) {
		t.Fatal("a second start must be rejected while the project cap is reached")
	}

	close(release)
	e.Wait()
	if got := e.Running("p1"); got != 0 {
		t.Fatalf("running for p1 after Wait = %d, want 0", got)
	}
	if got := e.InFlight(); got != 0 {
		t.Fatalf("in-flight after Wait = %d, want 0", got)
	}
}

func TestExecutorEnforcesTotalAndProjectCountCaps(t *testing.T) {
	t.Run("total in-flight cap", func(t *testing.T) {
		cfg := executorConfig(1, 4, 4)
		registry := executorTestRegistry(t, newFakeDriver("agent", worker.TaskReason))
		started := make(chan struct{}, 2)
		release := make(chan struct{})
		deps := executorDeps{
			reason: func(ctx context.Context, _ graphserver.Client, _ worker.Driver, _ graph.Project) error {
				return blockingRunner(started, release)(ctx)
			},
		}
		e := newExecutor(cfg, nil, registry, nil, deps)

		if !startDecision(t, e, Decision{ProjectID: "p1", TaskType: worker.TaskReason, WorkerName: "agent"}, graph.Project{ID: "p1"}) {
			t.Fatal("the first start must be accepted")
		}
		<-started
		if got := e.InFlight(); got != 1 {
			t.Fatalf("in-flight = %d, want 1", got)
		}
		if startDecision(t, e, Decision{ProjectID: "p2", TaskType: worker.TaskReason, WorkerName: "agent"}, graph.Project{ID: "p2"}) {
			t.Fatal("the total MaxWorkers cap must reject the second project")
		}
		close(release)
		e.Wait()
		if got := e.InFlight(); got != 0 {
			t.Fatalf("in-flight after Wait = %d, want 0", got)
		}
	})

	t.Run("distinct projects cap", func(t *testing.T) {
		cfg := executorConfig(10, 1, 4)
		registry := executorTestRegistry(t, newFakeDriver("agent", worker.TaskReason))
		started := make(chan struct{}, 3)
		release := make(chan struct{})
		deps := executorDeps{
			reason: func(ctx context.Context, _ graphserver.Client, _ worker.Driver, _ graph.Project) error {
				return blockingRunner(started, release)(ctx)
			},
		}
		e := newExecutor(cfg, nil, registry, nil, deps)

		if !startDecision(t, e, Decision{ProjectID: "p1", TaskType: worker.TaskReason, WorkerName: "agent"}, graph.Project{ID: "p1"}) {
			t.Fatal("the first project start must be accepted")
		}
		<-started
		if startDecision(t, e, Decision{ProjectID: "p2", TaskType: worker.TaskReason, WorkerName: "agent"}, graph.Project{ID: "p2"}) {
			t.Fatal("a second distinct project must be rejected while MaxRunningProjects is reached")
		}
		if !startDecision(t, e, Decision{ProjectID: "p1", TaskType: worker.TaskReason, WorkerName: "agent"}, graph.Project{ID: "p1"}) {
			t.Fatal("another start for an already-running project must still be allowed")
		}
		<-started
		close(release)
		e.Wait()
		if got := e.InFlight(); got != 0 {
			t.Fatalf("in-flight after Wait = %d, want 0", got)
		}
		if got := e.Running("p1"); got != 0 {
			t.Fatalf("running for p1 after Wait = %d, want 0", got)
		}
	})
}

func TestExecutorAppliesPerTypeTimeouts(t *testing.T) {
	cfg := executorConfig(8, 8, 8)
	cfg.BootstrapTimeout = 40 * time.Millisecond
	cfg.ReasonTimeout = 60 * time.Millisecond
	cfg.ExploreTimeout = 80 * time.Millisecond
	cfg.ExploreConcludeTimeout = 2 * time.Minute
	registry := executorTestRegistry(t, newFakeDriver("agent", worker.TaskBootstrap, worker.TaskReason, worker.TaskExplore))

	deadlines := make(chan time.Duration, 3)
	observedConclude := make(chan time.Duration, 1)
	blockUntilDeadline := func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			deadlines <- -1
			return nil
		}
		deadlines <- time.Until(deadline)
		<-ctx.Done()
		return nil
	}
	deps := executorDeps{
		bootstrap: func(ctx context.Context, _ graphserver.Client, _ worker.Driver, _ graph.Project) error {
			return blockUntilDeadline(ctx)
		},
		reason: func(ctx context.Context, _ graphserver.Client, _ worker.Driver, _ graph.Project) error {
			return blockUntilDeadline(ctx)
		},
		exploreWithOptions: func(ctx context.Context, _ graphserver.Client, _ worker.Driver, _ runtime.SandboxRuntime, _ graph.Project, _ string, opts tasks.ExploreOptions) error {
			observedConclude <- opts.ConcludeTimeout
			return blockUntilDeadline(ctx)
		},
	}
	e := newExecutor(cfg, nil, registry, nil, deps)

	check := func(tt worker.TaskType, want time.Duration) {
		t.Helper()
		projectID := "p-" + string(tt)
		if !startDecision(t, e, Decision{ProjectID: projectID, TaskType: tt, WorkerName: "agent"}, graph.Project{ID: projectID}) {
			t.Fatalf("%s start was rejected", tt)
		}
		got := <-deadlines
		if got <= 0 {
			t.Fatalf("%s runner received no deadline", tt)
		}
		if got > want || got < want-25*time.Millisecond {
			t.Fatalf("%s deadline = %v, want approximately %v", tt, got, want)
		}
		e.Wait()
	}
	check(worker.TaskBootstrap, cfg.BootstrapTimeout)
	check(worker.TaskReason, cfg.ReasonTimeout)
	check(worker.TaskExplore, cfg.ExploreTimeout)

	select {
	case got := <-observedConclude:
		if got != cfg.ExploreConcludeTimeout {
			t.Fatalf("explore conclude timeout = %v, want %v", got, cfg.ExploreConcludeTimeout)
		}
	default:
		t.Fatal("explore run did not receive ExploreOptions")
	}
}

func TestExecutorBoundsBootstrapRetries(t *testing.T) {
	client := &fakeClient{
		projects: []graph.Project{activeProject("p1", true)},
		exports:  map[string]string{"p1": projectExport(originGoalFacts(), 0, nil)},
	}
	cfg := executorConfig(4, 4, 2)
	cfg.BootstrapTimeout = time.Minute

	d := New(cfg, client, []worker.Driver{newFakeDriver("agent",
		worker.TaskBootstrap, worker.TaskReason, worker.TaskExplore)})
	deps := executorDeps{
		bootstrap: func(context.Context, graphserver.Client, worker.Driver, graph.Project) error {
			return errors.New("bootstrap failed")
		},
		reason: func(context.Context, graphserver.Client, worker.Driver, graph.Project) error {
			return nil
		},
		exploreWithOptions: func(context.Context, graphserver.Client, worker.Driver, runtime.SandboxRuntime, graph.Project, string, tasks.ExploreOptions) error {
			return nil
		},
	}
	e := newExecutor(cfg, client, d.registry, nil, deps)
	d.SetExecutor(e)
	now := time.Now()
	setRegistryClock(d.registry, &now)

	for i := 0; i < maxBootstrapAttempts; i++ {
		decisions, err := d.tick(context.Background())
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if len(decisions) != 1 || decisions[0].TaskType != worker.TaskBootstrap {
			t.Fatalf("tick %d decisions = %+v, want one bootstrap decision", i, decisions)
		}
		e.Wait()
		// Each failed bootstrap enters the unhealthy cooldown; cross it before
		// the next retry tick.
		now = now.Add(2 * time.Minute)
	}
	if got := d.bootstrapAttempts["p1"]; got != maxBootstrapAttempts {
		t.Fatalf("bootstrap attempts = %d, want %d", got, maxBootstrapAttempts)
	}

	decisions, err := d.tick(context.Background())
	if err != nil {
		t.Fatalf("capped tick: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("capped tick decisions = %+v, want none", decisions)
	}
	if !d.bootstrapCappedWarned["p1"] {
		t.Fatal("the retry cap must be logged once")
	}
	decisions, err = d.tick(context.Background())
	if err != nil {
		t.Fatalf("second capped tick: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("second capped tick decisions = %+v, want none", decisions)
	}

	// Leaving the initial state (a concluded bootstrap) releases the cap.
	client.exports["p1"] = projectExport(
		append(originGoalFacts(), factFixture{"f001", "patch_profile"}), 0, nil)
	if _, err := d.tick(context.Background()); err != nil {
		t.Fatalf("post-bootstrap tick: %v", err)
	}
	e.Wait()
	if got := d.bootstrapAttempts["p1"]; got != 0 {
		t.Fatalf("bootstrap attempts after leaving the initial state = %d, want 0", got)
	}
	if d.bootstrapCappedWarned["p1"] {
		t.Fatal("the cap warning must reset with the attempts")
	}
}

// --- production seams ------------------------------------------------------

func TestTaskManagerResolverMapsProjectToContainer(t *testing.T) {
	ctx := context.Background()
	resolve := containerResolver(func(projectID string) (*dockerManager.Sandbox, error) {
		if projectID != "p1" {
			return nil, errors.New("project " + projectID + " not found")
		}
		return &dockerManager.Sandbox{ContainerId: "container-123"}, nil
	})

	id, err := resolve(ctx, "p1")
	if err != nil {
		t.Fatalf("resolve p1: %v", err)
	}
	if id != "container-123" {
		t.Fatalf("container id = %q, want container-123", id)
	}

	if _, err := resolve(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing sandbox error = %v, want a project-scoped error", err)
	}

	noID := containerResolver(func(string) (*dockerManager.Sandbox, error) {
		return &dockerManager.Sandbox{}, nil
	})
	if id, err := noID(ctx, "p1"); err == nil || id != "" {
		t.Fatalf("empty sandbox container resolved to %q/%v, want a clear error", id, err)
	}

	nilSandbox := containerResolver(func(string) (*dockerManager.Sandbox, error) {
		return nil, nil
	})
	if id, err := nilSandbox(ctx, "p1"); err == nil || id != "" {
		t.Fatalf("nil sandbox resolved to %q/%v, want a clear error", id, err)
	}

	noLookup := containerResolver(nil)
	if id, err := noLookup(ctx, "p1"); err == nil || id != "" {
		t.Fatalf("nil lookup resolved to %q/%v, want a clear error", id, err)
	}
}

func TestNativeContextRendersConcludePlaceholders(t *testing.T) {
	export := projectExport(
		append(originGoalFacts(), factFixture{"f001", "patch_profile"}),
		0,
		[]intentFixture{{
			id:      "i001",
			state:   "claimed",
			content: "验证输入能否到达 eval 入口",
			creator: "reason",
			from:    []string{"f001"},
		}},
	)
	client := &fakeClient{exports: map[string]string{"p1": export}}
	template := "graph={graph_yaml}\nintent={intent_id}\ndescription={intent_description}"

	assertNoPlaceholders := func(t *testing.T, values map[string]string) {
		t.Helper()
		rendered := worker.RenderPrompt(template, values)
		for _, name := range []string{"graph_yaml", "intent_id", "intent_description"} {
			if strings.Contains(rendered, "{"+name+"}") {
				t.Fatalf("rendered context still contains {%s}:\n%s", name, rendered)
			}
		}
	}

	t.Run("known intent", func(t *testing.T) {
		values := NativeContext(client)(worker.Task{ProjectID: "p1", IntentID: "i001"})
		if values["intent_id"] != "i001" {
			t.Fatalf("intent_id = %q, want i001", values["intent_id"])
		}
		if values["intent_description"] != "验证输入能否到达 eval 入口" {
			t.Fatalf("intent_description = %q", values["intent_description"])
		}
		if !strings.Contains(values["graph_yaml"], "facts:") {
			t.Fatalf("graph_yaml = %q, want the project export", values["graph_yaml"])
		}
		assertNoPlaceholders(t, values)
	})

	t.Run("unknown intent", func(t *testing.T) {
		values := NativeContext(client)(worker.Task{ProjectID: "p1", IntentID: "i999"})
		if values["intent_id"] != "i999" {
			t.Fatalf("intent_id = %q, want i999", values["intent_id"])
		}
		if values["intent_description"] != "" {
			t.Fatalf("unknown intent description = %q, want empty", values["intent_description"])
		}
		assertNoPlaceholders(t, values)
	})

	t.Run("export failure", func(t *testing.T) {
		values := NativeContext(&fakeClient{exportErr: errors.New("graph down")})(
			worker.Task{ProjectID: "p1", IntentID: "i001"})
		if values["intent_id"] != "i001" {
			t.Fatalf("intent_id = %q, want the requested id", values["intent_id"])
		}
		if values["intent_description"] != "" || values["graph_yaml"] != "" {
			t.Fatalf("export failure values = %+v, want empty context", values)
		}
		assertNoPlaceholders(t, values)
	})

	t.Run("empty intent id", func(t *testing.T) {
		values := NativeContext(client)(worker.Task{ProjectID: "p1"})
		if values["intent_id"] == "" {
			t.Fatal("intent_id must never be empty, or the placeholder would survive")
		}
		assertNoPlaceholders(t, values)
	})
}

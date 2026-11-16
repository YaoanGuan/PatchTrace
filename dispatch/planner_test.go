package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
)

// --- fakes -----------------------------------------------------------------

// fakeClient embeds graphserver.Client so that only the methods exercised by
// the planner need an implementation. Calling an unimplemented method panics,
// which is the desired loud failure for an unexpected protocol call.
type fakeClient struct {
	graphserver.Client
	exports   map[string]string
	exportErr error
	projects  []graph.Project
	listErr   error
}

func (f *fakeClient) ExportYAML(ctx context.Context, projectID string) (string, error) {
	if f.exportErr != nil {
		return "", f.exportErr
	}
	body, ok := f.exports[projectID]
	if !ok {
		return "", fmt.Errorf("%w: project %s", graph.ErrNotFound, projectID)
	}
	return body, nil
}

func (f *fakeClient) ListProjects(ctx context.Context) ([]graph.Project, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.projects, nil
}

type fakeDriver struct {
	name      string
	supported map[worker.TaskType]bool
}

func (d *fakeDriver) Name() string                    { return d.name }
func (d *fakeDriver) Supports(t worker.TaskType) bool { return d.supported[t] }
func (d *fakeDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *fakeDriver) Execute(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{}, nil
}
func (d *fakeDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{}, nil
}
func (d *fakeDriver) SupportsConclude() bool { return false }

func newFakeDriver(name string, types ...worker.TaskType) *fakeDriver {
	supported := make(map[worker.TaskType]bool, len(types))
	for _, tt := range types {
		supported[tt] = true
	}
	return &fakeDriver{name: name, supported: supported}
}

// priorityDriver and maxRunningDriver exercise the optional attribute
// interfaces. Drivers that implement neither use the documented defaults.
type priorityDriver struct {
	*fakeDriver
	priority int
}

func (d *priorityDriver) Priority() int { return d.priority }

type maxRunningDriver struct {
	*fakeDriver
	max int
}

func (d *maxRunningDriver) MaxRunning() int { return d.max }

type rankedDriver struct {
	*fakeDriver
	priority int
	max      int
}

func (d *rankedDriver) Priority() int   { return d.priority }
func (d *rankedDriver) MaxRunning() int { return d.max }

func testDispatcher(t *testing.T, client graphserver.Client, drivers ...worker.Driver) *Dispatcher {
	t.Helper()
	return New(Config{}, client, drivers)
}

// --- export fixtures -------------------------------------------------------

type factFixture struct {
	id  string
	typ string
}

type intentFixture struct {
	id      string
	state   string
	content string
	creator string
	from    []string
}

func originGoalFacts() []factFixture {
	return []factFixture{{"origin", "origin"}, {"goal", "goal"}}
}

func projectExport(facts []factFixture, hintCount int, intents []intentFixture) string {
	var b strings.Builder
	b.WriteString("project:\n")
	b.WriteString("  title: test project\n")
	b.WriteString("  origin: /tmp/patch.diff\n")
	b.WriteString("  goal: verify the fix\n")
	b.WriteString("  bootstrap_enabled: true\n")

	if hintCount == 0 {
		b.WriteString("hints: []\n")
	} else {
		b.WriteString("hints:\n")
		for i := 0; i < hintCount; i++ {
			fmt.Fprintf(&b, "- id: h%03d\n  content: hint\n  creator: human\n  created_at: \"2026-01-01 00:00:00\"\n", i+1)
		}
	}

	if len(facts) == 0 {
		b.WriteString("facts: []\n")
	} else {
		b.WriteString("facts:\n")
		for _, f := range facts {
			fmt.Fprintf(&b, "- id: %s\n  type: %s\n  content: %s content\n  created_at: \"2026-01-01 00:00:00\"\n", f.id, f.typ, f.id)
		}
	}

	if len(intents) == 0 {
		b.WriteString("intents: []\n")
	} else {
		b.WriteString("intents:\n")
		for _, in := range intents {
			fmt.Fprintf(&b, "- id: %s\n  type: inversion\n  content: %s\n  from:\n", in.id, in.content)
			for _, from := range in.from {
				fmt.Fprintf(&b, "  - %s\n", from)
			}
			fmt.Fprintf(&b, "  to: null\n  state: %s\n  worker: \"\"\n  creator: %s\n  created_at: \"2026-01-01 00:00:00\"\n  concluded_at: null\n", in.state, in.creator)
		}
	}
	return b.String()
}

func activeProject(id string, bootstrap bool) graph.Project {
	return graph.Project{
		ID:               id,
		Title:            "test project",
		Origin:           "/tmp/patch.diff",
		Goal:             "verify the fix",
		Status:           "active",
		BootstrapEnabled: bootstrap,
	}
}

func mustPlan(t *testing.T, d *Dispatcher, p graph.Project) (Decision, bool) {
	t.Helper()
	dec, ok, err := d.PlanProject(context.Background(), p)
	if err != nil {
		t.Fatalf("PlanProject returned error: %v", err)
	}
	return dec, ok
}

// --- decision rules --------------------------------------------------------

func TestPlanPrefersBootstrapOnInitialProject(t *testing.T) {
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(originGoalFacts(), 0, nil),
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if !ok {
		t.Fatal("expected a bootstrap decision for an initial project")
	}
	if dec.TaskType != worker.TaskBootstrap {
		t.Fatalf("expected bootstrap, got %q", dec.TaskType)
	}
	if dec.ProjectID != "p1" {
		t.Fatalf("expected project p1, got %q", dec.ProjectID)
	}
}

func TestPlanBootstrapFromIDOnlyExport(t *testing.T) {
	// A minimal export fixture may omit fact types; origin/goal ids are enough.
	client := &fakeClient{exports: map[string]string{
		"p1": "facts:\n- id: origin\n- id: goal\nintents: []\n",
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if !ok || dec.TaskType != worker.TaskBootstrap {
		t.Fatalf("expected bootstrap from a minimal export, got ok=%v decision=%+v", ok, dec)
	}
}

func TestPlanBootstrapReusesReservedIntent(t *testing.T) {
	facts := originGoalFacts()
	intents := []intentFixture{{
		id:      "b001",
		state:   "open",
		content: "bootstrap",
		creator: "dispatcher.bootstrap",
		from:    []string{"origin"},
	}}
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, intents),
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if !ok || dec.TaskType != worker.TaskBootstrap {
		t.Fatalf("expected bootstrap, got ok=%v decision=%+v", ok, dec)
	}
	if dec.IntentID != "b001" {
		t.Fatalf("expected reserved bootstrap intent b001, got %q", dec.IntentID)
	}
}

func TestPlanSkipsWhileBootstrapIsClaimed(t *testing.T) {
	facts := originGoalFacts()
	intents := []intentFixture{{
		id:      "b001",
		state:   "claimed",
		content: "bootstrap",
		creator: "dispatcher.bootstrap",
		from:    []string{"origin"},
	}}
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, intents),
	}}
	d := testDispatcher(t, client)

	if _, ok := mustPlan(t, d, activeProject("p1", true)); ok {
		t.Fatal("a project with a claimed bootstrap intent must not be planned")
	}
}

func TestPlanTreatsReservedIdeaIntentAsReserved(t *testing.T) {
	// Any dispatcher.bootstrap intent is reserved, not just the one whose
	// content is "bootstrap". The leftover exploit_idea intent must neither
	// count as an ordinary intent nor suppress the initial bootstrap.
	intents := []intentFixture{{
		id:      "i900",
		state:   "open",
		content: "exploit_idea",
		creator: "dispatcher.bootstrap",
		from:    []string{"origin"},
	}}
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(originGoalFacts(), 0, intents),
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if !ok || dec.TaskType != worker.TaskBootstrap {
		t.Fatalf("expected bootstrap despite the reserved idea intent, got ok=%v decision=%+v", ok, dec)
	}
	if dec.IntentID != "" {
		t.Fatalf("a reserved non-bootstrap intent must not be reused as the bootstrap intent, got %q", dec.IntentID)
	}
}

func TestPlanSkipsBootstrapAfterReservedIntentConcluded(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	intents := []intentFixture{{
		id:      "b001",
		state:   "resolved",
		content: "bootstrap",
		creator: "dispatcher.bootstrap",
		from:    []string{"origin"},
	}}
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, intents),
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if ok && dec.TaskType == worker.TaskBootstrap {
		t.Fatalf("a concluded reserved intent must suppress bootstrap, got %+v", dec)
	}
}

func TestPlanToleratesHintsInExport(t *testing.T) {
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(originGoalFacts(), 1, nil),
	}}
	d := testDispatcher(t, client)

	dec, ok, err := d.PlanProject(context.Background(), activeProject("p1", true))
	if err != nil {
		t.Fatalf("a mapping-style hint must not break parsing: %v", err)
	}
	if !ok || dec.TaskType != worker.TaskBootstrap {
		t.Fatalf("expected bootstrap with one hint, got ok=%v decision=%+v", ok, dec)
	}
}

func TestPlanReasonOnNewFacts(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, nil),
	}}
	d := testDispatcher(t, client)
	d.lastReason["p1"] = lastReason{FactCount: 2, HintCount: 0, HadOpen: false}

	dec, ok := mustPlan(t, d, activeProject("p1", false))
	if !ok {
		t.Fatal("expected a reason decision after the fact count grew")
	}
	if dec.TaskType != worker.TaskReason {
		t.Fatalf("expected reason, got %q", dec.TaskType)
	}
	if dec.Trigger != triggerNewFacts {
		t.Fatalf("expected trigger %q, got %q", triggerNewFacts, dec.Trigger)
	}
}

func TestPlanReasonOnNewHints(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 1, nil),
	}}
	d := testDispatcher(t, client)
	d.lastReason["p1"] = lastReason{FactCount: 3, HintCount: 0, HadOpen: false}

	dec, ok := mustPlan(t, d, activeProject("p1", false))
	if !ok || dec.TaskType != worker.TaskReason {
		t.Fatalf("expected reason, got ok=%v decision=%+v", ok, dec)
	}
	if dec.Trigger != triggerNewHints {
		t.Fatalf("expected trigger %q, got %q", triggerNewHints, dec.Trigger)
	}
}

func TestPlanReasonFirstTimeWhenNoOpenIntent(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, nil),
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", false))
	if !ok || dec.TaskType != worker.TaskReason {
		t.Fatalf("expected reason, got ok=%v decision=%+v", ok, dec)
	}
	if dec.Trigger != triggerInitial {
		t.Fatalf("expected trigger %q, got %q", triggerInitial, dec.Trigger)
	}
}

func TestPlanReasonWhenOpenIntentsDrained(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, nil),
	}}
	d := testDispatcher(t, client)
	d.lastReason["p1"] = lastReason{FactCount: 3, HintCount: 0, HadOpen: true}

	dec, ok := mustPlan(t, d, activeProject("p1", false))
	if !ok || dec.TaskType != worker.TaskReason {
		t.Fatalf("expected reason, got ok=%v decision=%+v", ok, dec)
	}
	if dec.Trigger != triggerOpenDrained {
		t.Fatalf("expected trigger %q, got %q", triggerOpenDrained, dec.Trigger)
	}
}

func TestPlanExploreWhenOpenIntentExists(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	intents := []intentFixture{{
		id:      "i001",
		state:   "open",
		content: "verify unauthenticated code execution",
		creator: "reason",
		from:    []string{"f001"},
	}}
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, intents),
	}}
	d := testDispatcher(t, client)

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if !ok {
		t.Fatal("expected an explore decision when an open intent exists")
	}
	if dec.TaskType != worker.TaskExplore {
		t.Fatalf("expected explore, got %q", dec.TaskType)
	}
	if dec.IntentID != "i001" {
		t.Fatalf("expected intent i001, got %q", dec.IntentID)
	}
}

func TestPlanDoesNotExploreClaimedIntent(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	intents := []intentFixture{{
		id:      "i001",
		state:   "claimed",
		content: "being explored",
		creator: "reason",
		from:    []string{"f001"},
	}}
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, intents),
	}}
	d := testDispatcher(t, client)
	d.lastReason["p1"] = lastReason{FactCount: 3, HintCount: 0, HadOpen: true}

	dec, ok := mustPlan(t, d, activeProject("p1", true))
	if ok && dec.TaskType == worker.TaskExplore {
		t.Fatalf("a claimed intent is not claimable and must not be explored: %+v", dec)
	}
}

func TestReasonOnlyOncePerProject(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"}, factFixture{"f002", "observation"})
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, nil),
	}}
	d := testDispatcher(t, client)
	d.lastReason["p1"] = lastReason{FactCount: 2, HintCount: 0, HadOpen: false}

	p := activeProject("p1", false)
	p.Reason = &graph.ReasonLease{
		Worker:          "reason-worker",
		Trigger:         triggerNewFacts,
		StartedAt:       time.Now(),
		LastHeartbeatAt: time.Now(),
	}

	_, ok := mustPlan(t, d, p)
	if ok {
		t.Fatal("a project that already holds the reason lease must not be planned for reason")
	}
}

func TestPlanNoopWhenNothingNew(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(facts, 0, nil),
	}}
	d := testDispatcher(t, client)
	d.lastReason["p1"] = lastReason{FactCount: 3, HintCount: 0, HadOpen: false}

	_, ok := mustPlan(t, d, activeProject("p1", false))
	if ok {
		t.Fatal("expected no decision when the graph situation is unchanged")
	}
}

func TestPlanSkipsInactiveProject(t *testing.T) {
	client := &fakeClient{exports: map[string]string{
		"p1": projectExport(originGoalFacts(), 0, nil),
	}}
	d := testDispatcher(t, client)

	p := activeProject("p1", true)
	p.Status = "completed"

	_, ok := mustPlan(t, d, p)
	if ok {
		t.Fatal("an inactive project must never be planned")
	}
}

func TestPlanPropagatesExportError(t *testing.T) {
	client := &fakeClient{exportErr: errors.New("graph down")}
	d := testDispatcher(t, client)

	if _, _, err := d.PlanProject(context.Background(), activeProject("p1", true)); err == nil {
		t.Fatal("expected the export error to be propagated")
	}
}

func TestPruneLastReasonRemovesMissingProjects(t *testing.T) {
	d := testDispatcher(t, &fakeClient{})
	d.lastReason["kept"] = lastReason{FactCount: 1}
	d.lastReason["gone"] = lastReason{FactCount: 2}

	d.pruneLastReason([]graph.Project{{ID: "kept"}})

	if _, ok := d.lastReason["kept"]; !ok {
		t.Fatal("expected the listed project to be kept")
	}
	if _, ok := d.lastReason["gone"]; ok {
		t.Fatal("expected the unlisted project to be pruned")
	}
}

// --- loop integration ------------------------------------------------------

func TestTickPlansSelectsWorkerAndPrunes(t *testing.T) {
	client := &fakeClient{
		projects: []graph.Project{activeProject("p1", true)},
		exports: map[string]string{
			"p1": projectExport(originGoalFacts(), 0, nil),
		},
	}
	d := testDispatcher(t, client, newFakeDriver("boot", worker.TaskBootstrap))
	d.lastReason["gone"] = lastReason{FactCount: 9}

	decisions, err := d.tick(context.Background())
	if err != nil {
		t.Fatalf("tick returned error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].TaskType != worker.TaskBootstrap || decisions[0].WorkerName != "boot" {
		t.Fatalf("unexpected decision: %+v", decisions[0])
	}
	if _, ok := d.lastReason["gone"]; ok {
		t.Fatal("tick must prune state for projects the server no longer lists")
	}
}

func TestTickTracksOpenIntentDrain(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	openIntent := []intentFixture{{
		id:      "i001",
		state:   "open",
		content: "verify",
		creator: "reason",
		from:    []string{"f001"},
	}}
	client := &fakeClient{
		projects: []graph.Project{activeProject("p1", true)},
		exports: map[string]string{
			"p1": projectExport(facts, 0, openIntent),
		},
	}
	d := testDispatcher(t, client,
		newFakeDriver("agent", worker.TaskReason, worker.TaskExplore))
	d.lastReason["p1"] = lastReason{FactCount: 3, HintCount: 0, HadOpen: false}

	decisions, err := d.tick(context.Background())
	if err != nil {
		t.Fatalf("first tick returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].TaskType != worker.TaskExplore {
		t.Fatalf("expected an explore decision on the first tick, got %+v", decisions)
	}
	if !d.lastReason["p1"].HadOpen {
		t.Fatal("tick must remember that the project had an open intent")
	}

	client.exports["p1"] = projectExport(facts, 0, nil)
	decisions, err = d.tick(context.Background())
	if err != nil {
		t.Fatalf("second tick returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].TaskType != worker.TaskReason {
		t.Fatalf("expected a reason decision after the intents drained, got %+v", decisions)
	}
	if decisions[0].Trigger != triggerOpenDrained {
		t.Fatalf("expected trigger %q, got %q", triggerOpenDrained, decisions[0].Trigger)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	d := New(Config{Interval: time.Millisecond}, &fakeClient{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestRunRejectsZeroInterval(t *testing.T) {
	d := New(Config{}, &fakeClient{}, nil)
	if err := d.Run(context.Background()); err == nil {
		t.Fatal("expected Run to reject a non-positive interval")
	}
}

// --- worker selection ------------------------------------------------------

func TestSelectWorkerFiltersByTaskType(t *testing.T) {
	d := testDispatcher(t, &fakeClient{},
		newFakeDriver("bootstrap-only", worker.TaskBootstrap))

	if _, ok := d.selectWorker(worker.TaskExplore); ok {
		t.Fatal("a worker that does not support the task type must not be selected")
	}
	if drv, ok := d.selectWorker(worker.TaskBootstrap); !ok || drv.Name() != "bootstrap-only" {
		t.Fatalf("expected bootstrap-only worker, got ok=%v drv=%v", ok, drv)
	}
}

func TestSelectWorkerHonorsMaxRunning(t *testing.T) {
	busy := &maxRunningDriver{fakeDriver: newFakeDriver("busy", worker.TaskExplore), max: 1}
	idle := &maxRunningDriver{fakeDriver: newFakeDriver("idle", worker.TaskExplore), max: 1}
	d := testDispatcher(t, &fakeClient{}, busy, idle)
	d.running["busy"] = 1

	drv, ok := d.selectWorker(worker.TaskExplore)
	if !ok || drv.Name() != "idle" {
		t.Fatalf("expected idle worker, got ok=%v drv=%v", ok, drv)
	}
}

func TestSelectWorkerUnlimitedByDefault(t *testing.T) {
	drv := newFakeDriver("unlimited", worker.TaskExplore)
	d := testDispatcher(t, &fakeClient{}, drv)
	d.running["unlimited"] = 100

	if got, ok := d.selectWorker(worker.TaskExplore); !ok || got.Name() != "unlimited" {
		t.Fatalf("a driver without MaxRunning must be unlimited, got ok=%v drv=%v", ok, got)
	}
}

func TestSelectWorkerSkipsUnhealthy(t *testing.T) {
	sick := newFakeDriver("sick", worker.TaskExplore)
	healthy := newFakeDriver("healthy", worker.TaskExplore)
	d := testDispatcher(t, &fakeClient{}, sick, healthy)
	d.registry.MarkUnhealthy("sick")

	drv, ok := d.selectWorker(worker.TaskExplore)
	if !ok || drv.Name() != "healthy" {
		t.Fatalf("expected healthy worker, got ok=%v drv=%v", ok, drv)
	}
}

func TestSelectWorkerPrefersLowerPriority(t *testing.T) {
	low := &priorityDriver{fakeDriver: newFakeDriver("low", worker.TaskExplore), priority: 1}
	high := &priorityDriver{fakeDriver: newFakeDriver("high", worker.TaskExplore), priority: 0}
	d := testDispatcher(t, &fakeClient{}, low, high)

	drv, ok := d.selectWorker(worker.TaskExplore)
	if !ok || drv.Name() != "high" {
		t.Fatalf("expected high-priority worker, got ok=%v drv=%v", ok, drv)
	}
}

func TestSelectWorkerPrefersFewerRunning(t *testing.T) {
	busy := &priorityDriver{fakeDriver: newFakeDriver("busy", worker.TaskExplore), priority: 0}
	idle := &priorityDriver{fakeDriver: newFakeDriver("idle", worker.TaskExplore), priority: 0}
	d := testDispatcher(t, &fakeClient{}, busy, idle)
	d.running["busy"] = 2
	d.running["idle"] = 0

	drv, ok := d.selectWorker(worker.TaskExplore)
	if !ok || drv.Name() != "idle" {
		t.Fatalf("expected idle worker, got ok=%v drv=%v", ok, drv)
	}
}

func TestSelectWorkerRandomTieBreakIsInjectable(t *testing.T) {
	a := &rankedDriver{fakeDriver: newFakeDriver("a", worker.TaskExplore), priority: 0, max: 2}
	b := &rankedDriver{fakeDriver: newFakeDriver("b", worker.TaskExplore), priority: 0, max: 2}
	d := testDispatcher(t, &fakeClient{}, a, b)

	var sawN int
	d.randIntn = func(n int) int {
		sawN = n
		return 1
	}

	drv, ok := d.selectWorker(worker.TaskExplore)
	if !ok {
		t.Fatal("expected a worker to be selected")
	}
	if sawN != 2 {
		t.Fatalf("expected random choice over 2 tied workers, got n=%d", sawN)
	}
	if drv.Name() != "b" {
		t.Fatalf("expected injected random index to pick b, got %q", drv.Name())
	}
}

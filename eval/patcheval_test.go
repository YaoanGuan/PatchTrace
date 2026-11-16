package eval

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"AIxVuln/dispatch"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"
	"AIxVuln/patchintel"
)

const testSeedPattern = "langflow validate code executes user input"

func TestPatchevalRunsDualConditionAndReportsMetrics(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		N:            3,
		GraphDBPath:  filepath.Join(dir, "graph.db"),
		RuleBasePath: filepath.Join(dir, "rules.db"),
		SeedRules: []patchintel.Rule{{
			CVEID:      "CVE-2025-3248",
			Pattern:    testSeedPattern,
			Confidence: 0.8,
		}},
		RunOnce: func(_ context.Context, in RunInput) (Observation, error) {
			rules, err := in.RuleBase.Search("langflow validate code", 5)
			if err != nil {
				return Observation{}, err
			}
			switch in.Condition {
			case ConditionCold:
				if len(rules) != 0 {
					return Observation{}, errors.New("cold condition rule base is not empty")
				}
			case ConditionSeeded:
				if len(rules) == 0 {
					return Observation{}, errors.New("seeded condition rule base is empty")
				}
			default:
				return Observation{}, errors.New("unexpected condition " + in.Condition)
			}
			ok := in.Condition == ConditionSeeded
			return Observation{
				InversionOK: ok,
				TrialRounds: in.Run,
				// 400 bytes -> the default estimate would be 100 tokens; the
				// injected estimator below returns 400 to prove it is used.
				Results: []worker.Result{{Raw: strings.Repeat("x", 400)}},
			}, nil
		},
		Tokens: func(r worker.Result) int64 { return int64(len(r.Raw)) },
	}

	report, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Runs) != 6 {
		t.Fatalf("Runs = %d, want 6 (2 conditions x N=3)", len(report.Runs))
	}

	coldSuccess, seededSuccess := 0, 0
	for _, run := range report.Runs {
		if run.Tokens != 400 {
			t.Fatalf("%s run %d Tokens = %d, want 400 (injected estimator)", run.Condition, run.Run, run.Tokens)
		}
		switch run.Condition {
		case ConditionCold:
			if run.InversionOK {
				t.Fatalf("cold run %d unexpectedly inverted", run.Run)
			}
			if run.Run < 1 || run.Run > 3 {
				t.Fatalf("cold run number = %d, want 1..3", run.Run)
			}
			coldSuccess++
		case ConditionSeeded:
			if !run.InversionOK {
				t.Fatalf("seeded run %d did not invert", run.Run)
			}
			seededSuccess++
		default:
			t.Fatalf("unexpected condition %q", run.Condition)
		}
	}
	if coldSuccess != 3 || seededSuccess != 3 {
		t.Fatalf("cold=%d seeded=%d, want 3/3", coldSuccess, seededSuccess)
	}

	m := report.Metrics
	if m.InversionSuccessRate != 0.5 {
		t.Fatalf("InversionSuccessRate = %v, want 0.5", m.InversionSuccessRate)
	}
	if m.AvgTrialRounds == 0 {
		t.Fatal("AvgTrialRounds must be non-zero")
	}
	if m.AvgTrialRounds != 2 {
		t.Fatalf("AvgTrialRounds = %v, want 2", m.AvgTrialRounds)
	}
	// 6 runs x 400 tokens = 2400; 3 successful inversions -> 800.
	if m.TokensPerFinding != 800 {
		t.Fatalf("TokensPerFinding = %v, want 800", m.TokensPerFinding)
	}

	raw, err := report.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 7 {
		t.Fatalf("JSON lines = %d, want 6 runs + 1 summary; output:\n%s", len(lines), raw)
	}
	wantFirst := `{"condition":"cold","run":1,"inversion_ok":false,"trial_rounds":1,"tokens":400}`
	if lines[0] != wantFirst {
		t.Fatalf("first JSON line = %s, want %s", lines[0], wantFirst)
	}
	wantSecond := `{"condition":"cold","run":2,"inversion_ok":false,"trial_rounds":2,"tokens":400}`
	if lines[1] != wantSecond {
		t.Fatalf("second JSON line = %s, want %s", lines[1], wantSecond)
	}
	var summary struct {
		Metrics Metrics `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &summary); err != nil {
		t.Fatalf("decode summary line %q: %v", lines[len(lines)-1], err)
	}
	if summary.Metrics != m {
		t.Fatalf("summary metrics = %+v, want %+v", summary.Metrics, m)
	}
}

func TestPatchevalRequiresN(t *testing.T) {
	for _, n := range []int{0, 1, 2} {
		_, err := Run(context.Background(), Options{N: n})
		if err == nil {
			t.Fatalf("N=%d: Run returned nil error, want N>=3 rejection", n)
		}
		if !strings.Contains(err.Error(), "N") || !strings.Contains(err.Error(), "3") {
			t.Fatalf("N=%d: error %q should explain the N>=3 requirement", n, err)
		}
	}
}

func TestDefaultTokenEstimate(t *testing.T) {
	if got := defaultTokens(worker.Result{Raw: strings.Repeat("x", 400)}); got != 100 {
		t.Fatalf("defaultTokens = %d, want len(Raw)/4 = 100", got)
	}
}

// evalFakeDriver is a worker that returns a fixed result. It lets the
// measuring wrapper be tested without Docker or an LLM.
type evalFakeDriver struct {
	name string
}

func (d *evalFakeDriver) Name() string                      { return d.name }
func (d *evalFakeDriver) Supports(worker.TaskType) bool     { return true }
func (d *evalFakeDriver) HealthCheck(context.Context) error { return nil }
func (d *evalFakeDriver) SupportsConclude() bool            { return true }
func (d *evalFakeDriver) Execute(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{Accepted: true, Raw: "execute"}, nil
}
func (d *evalFakeDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{Accepted: true, Raw: "conclude"}, nil
}

// TestMeasuringDriverCountsTrialOncePerExploreTask pins the trial-round metric:
// one explore task that executes and then falls back to Conclude is still one
// trial. Conclude output still counts toward the token estimate.
func TestMeasuringDriverCountsTrialOncePerExploreTask(t *testing.T) {
	d := &measuringDriver{inner: &evalFakeDriver{name: "fake"}}
	task := worker.Task{Type: string(worker.TaskExplore)}
	ctx := context.Background()
	if _, err := d.Execute(ctx, task); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := d.Conclude(ctx, task); err != nil {
		t.Fatalf("Conclude: %v", err)
	}
	results, trials := d.snapshot()
	if trials != 1 {
		t.Fatalf("trial rounds = %d, want 1 (conclude is part of the same trial)", trials)
	}
	if len(results) != 2 {
		t.Fatalf("recorded results = %d, want 2 (execute + conclude token cost)", len(results))
	}
}

// TestPatchevalClearsDerivedRunFiles proves repeated Run calls do not reuse the
// derived per-run SQLite files: every cold repetition starts empty even after an
// earlier Run left facts and rules behind.
func TestPatchevalClearsDerivedRunFiles(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		N:            3,
		GraphDBPath:  filepath.Join(dir, "graph.db"),
		RuleBasePath: filepath.Join(dir, "rules.db"),
		SeedRules:    []patchintel.Rule{{Pattern: testSeedPattern, Confidence: 0.8}},
		RunOnce: func(ctx context.Context, in RunInput) (Observation, error) {
			projects, err := in.Store.ListProjects(ctx)
			if err != nil {
				return Observation{}, err
			}
			rules, err := in.RuleBase.Search("", 10)
			if err != nil {
				return Observation{}, err
			}
			if in.Condition == ConditionCold {
				if len(projects) != 0 {
					return Observation{}, errors.New("cold run reused a stale graph store")
				}
				if len(rules) != 0 {
					return Observation{}, errors.New("cold run reused a stale rule base")
				}
			}
			// Leave state behind so the next Run would see it without cleanup.
			if _, err := in.Store.CreateProject(ctx, graph.Project{
				ID: "leak", Title: "leak", Origin: "/tmp/patch.diff", Goal: "leak",
			}); err != nil {
				return Observation{}, err
			}
			if _, err := in.RuleBase.Promote(patchintel.Rule{Pattern: "leaked rule"}); err != nil {
				return Observation{}, err
			}
			return Observation{Results: []worker.Result{{Raw: "x"}}}, nil
		},
	}
	for run := 0; run < 2; run++ {
		if _, err := Run(context.Background(), opts); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}
}

func evalTestConfig() dispatch.Config {
	return dispatch.Config{
		ServerURL:                "http://127.0.0.1:0",
		MaxWorkers:               2,
		MaxRunningProjects:       1,
		MaxProjectWorkers:        1,
		Interval:                 10 * time.Millisecond,
		HealthcheckTimeout:       time.Second,
		BootstrapTimeout:         time.Minute,
		BootstrapConcludeTimeout: time.Minute,
		ReasonTimeout:            time.Minute,
		ExploreTimeout:           time.Minute,
		ExploreConcludeTimeout:   time.Minute,
		PromptGroup:              "default",
	}
}

// blockingReasonDriver blocks its reason execution past the lane deadline so
// the harness can prove it waits for in-flight executor goroutines.
type blockingReasonDriver struct {
	delay   time.Duration
	started chan<- time.Time
}

func (d *blockingReasonDriver) Name() string { return "blocking" }
func (d *blockingReasonDriver) Supports(tt worker.TaskType) bool {
	return tt == worker.TaskReason || tt == worker.TaskExplore
}
func (d *blockingReasonDriver) HealthCheck(context.Context) error { return nil }
func (d *blockingReasonDriver) Execute(context.Context, worker.Task) (worker.Result, error) {
	if d.started != nil {
		d.started <- time.Now()
	}
	time.Sleep(d.delay)
	return worker.Result{Accepted: true}, nil
}
func (d *blockingReasonDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{Accepted: true}, nil
}
func (d *blockingReasonDriver) SupportsConclude() bool { return true }

// TestPatchevalDefaultLaneWaitsAndLeavesCallerStoresOpen drives the default
// (RunOnce == nil) lane with a fake driver. The lane must wait for its executor
// goroutine before returning, and it must not close the store or rule base the
// caller opened.
func TestPatchevalDefaultLaneWaitsAndLeavesCallerStoresOpen(t *testing.T) {
	dir := t.TempDir()
	store, err := graph.Open(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	defer store.Close()
	ruleBase, err := patchintel.OpenRuleBase(filepath.Join(dir, "rules.db"))
	if err != nil {
		t.Fatalf("OpenRuleBase: %v", err)
	}
	defer ruleBase.Close()

	// The deadline only has to fire after the first reason task starts; it is
	// generous so the race detector cannot make planning miss it.
	const executeDelay = 900 * time.Millisecond
	started := make(chan time.Time, 4)
	opts := Options{
		N:           3,
		Config:      evalTestConfig(),
		Project:     graph.Project{ID: "p1", Title: "demo", Origin: "/tmp/patch.diff", Goal: "确认漏洞可利用"},
		MaxDuration: 500 * time.Millisecond,
		NewDriver: func(context.Context, RunInput) (worker.Driver, error) {
			return &blockingReasonDriver{delay: executeDelay, started: started}, nil
		},
	}
	in := RunInput{
		Condition: ConditionCold,
		Run:       1,
		Store:     store,
		Client:    graphserver.NewLocalClient(store),
		RuleBase:  ruleBase,
		Config:    opts.Config,
	}

	start := time.Now()
	if _, err := defaultRunOnce(opts)(context.Background(), in); err != nil {
		t.Fatalf("defaultRunOnce: %v", err)
	}
	elapsed := time.Since(start)
	select {
	case at := <-started:
		if elapsed < executeDelay {
			t.Fatalf("default lane returned after %v (execute started at %v, delay %v): Executor.Wait is missing",
				elapsed, at.Sub(start), executeDelay)
		}
	default:
		t.Fatalf("default lane never executed a task, so the wait assertion is not meaningful (elapsed %v)", elapsed)
	}
	if _, err := store.ListProjects(context.Background()); err != nil {
		t.Fatalf("lane closed the caller-supplied graph store: %v", err)
	}
	if _, err := ruleBase.Search("", 5); err != nil {
		t.Fatalf("lane closed the caller-supplied rule base: %v", err)
	}
}

func TestGraphLaneFlagDefaultsOff(t *testing.T) {
	t.Setenv("AIXVULN_DATA_DIR", t.TempDir())

	if misc.GraphLaneEnabled() {
		t.Fatal("graph lane must default to disabled")
	}
	for _, value := range []string{"true", "1", "yes", "on", "TRUE"} {
		if err := misc.SetConfigValue("misc", "graph_lane_enabled", value); err != nil {
			t.Fatalf("SetConfigValue(%q): %v", value, err)
		}
		if !misc.GraphLaneEnabled() {
			t.Fatalf("graph_lane_enabled=%q should enable the lane", value)
		}
	}
	for _, value := range []string{"false", "0", "no", "off", "garbage", ""} {
		if err := misc.SetConfigValue("misc", "graph_lane_enabled", value); err != nil {
			t.Fatalf("SetConfigValue(%q): %v", value, err)
		}
		if misc.GraphLaneEnabled() {
			t.Fatalf("graph_lane_enabled=%q must not enable the lane", value)
		}
	}
}

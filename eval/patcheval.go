// Package eval implements the patcheval dual-condition comparison for the
// graph lane (R14).
//
// The same target is evaluated twice: with an empty rule base ("cold") and
// with a pre-populated rule base ("seeded"). Each condition runs N>=3 times
// and every repetition contributes one JSON line with its inversion outcome,
// finding flag, trial rounds and token cost. The summary reports the three
// comparison metrics: inversion success rate, average trial rounds to a PoC,
// and tokens per finding.
//
// Inversion and finding are distinct: an inversion succeeds when the graph
// holds BOTH a patch_profile fact and an exploit_idea fact (R7). A finding uses
// the controller's runtime proxy: the project is completed, the export contains
// an exploit_idea fact, and the graph holds at least one verdict fact with a
// support edge. The two cost metrics only average over runs that reached a
// finding; with no findings they stay zero rather than implying a measured PoC.
//
// Tests inject RunOnce so no Docker daemon, LLM or network is required. The
// default RunOnce drives the real dispatcher lane through dispatch.StartLane.
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"AIxVuln/dispatch"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/patchintel"
)

// The two patcheval conditions required by R14.
const (
	// ConditionCold evaluates against an empty rule base.
	ConditionCold = "cold"
	// ConditionSeeded evaluates against a rule base pre-populated with
	// Options.SeedRules.
	ConditionSeeded = "seeded"
)

const (
	defaultMaxDuration = 30 * time.Second
	pollInterval       = 50 * time.Millisecond
)

// Metrics is the three-metric summary required by R14.
type Metrics struct {
	// InversionSuccessRate is successful inversions / total runs.
	InversionSuccessRate float64 `json:"inversion_success_rate"`
	// AvgTrialRounds is the mean explore trial count over runs that reached a
	// finding (runs without a finding do not dilute it; zero findings -> 0).
	AvgTrialRounds float64 `json:"avg_trial_rounds"`
	// TokensPerFinding is total estimated tokens / runs that reached a finding
	// (zero findings -> 0).
	TokensPerFinding float64 `json:"tokens_per_finding"`
}

// RunResult is one repetition in one condition. Its JSON shape is the
// line-oriented contract:
//
//	{"condition":"cold","run":1,"inversion_ok":true,"finding":false,"trial_rounds":2,"tokens":12345}
type RunResult struct {
	Condition   string `json:"condition"`
	Run         int    `json:"run"`
	InversionOK bool   `json:"inversion_ok"`
	// Finding is the controller's runtime PoC proxy: completed project plus an
	// exploit_idea fact plus a verdict fact with a support edge.
	Finding     bool  `json:"finding"`
	TrialRounds int   `json:"trial_rounds"`
	Tokens      int64 `json:"tokens"`
}

// Report is the full evaluation: every repetition plus the summary metrics.
type Report struct {
	Runs    []RunResult `json:"runs"`
	Metrics Metrics     `json:"metrics"`
}

// JSON renders the report as newline-delimited JSON: one RunResult per line
// followed by a final summary line carrying the metrics.
func (r Report) JSON() ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, run := range r.Runs {
		if err := enc.Encode(run); err != nil {
			return nil, fmt.Errorf("eval: encode run: %w", err)
		}
	}
	if err := enc.Encode(struct {
		Metrics Metrics `json:"metrics"`
	}{Metrics: r.Metrics}); err != nil {
		return nil, fmt.Errorf("eval: encode summary: %w", err)
	}
	return []byte(b.String()), nil
}

// Observation is the measured outcome of one repetition.
type Observation struct {
	// InversionOK reports whether the patch was inverted into the required
	// patch_profile/exploit_idea facts.
	InversionOK bool
	// Finding reports whether the repetition reached the controller's runtime
	// PoC proxy (completed project, exploit_idea fact, support verdict).
	Finding bool
	// TrialRounds is the number of explore attempts spent reaching the PoC.
	TrialRounds int
	// Results are the worker outputs produced by the repetition. Tokens are
	// estimated from them because worker.Result carries no token count.
	Results []worker.Result
}

// RunInput is everything one repetition may need.
type RunInput struct {
	Condition string
	Run       int
	Store     *graph.Store
	Client    graphserver.Client
	RuleBase  *patchintel.RuleBase
	Config    dispatch.Config
}

// RunOnceFunc executes one repetition. Tests supply a fake hook so the whole
// evaluation runs offline.
type RunOnceFunc func(ctx context.Context, in RunInput) (Observation, error)

// Options configures Run.
//
// The condition-specific SQLite files are derived from GraphDBPath and
// RuleBasePath by appending "-<condition>-<run>" before the extension, so each
// repetition starts from a clean store without sharing state.
type Options struct {
	// N is the repetitions per condition. It must be >= 3.
	N int

	// Config is the dispatcher configuration used by the default lane.
	Config dispatch.Config

	// Project is created in each repetition's graph store before the lane
	// runs. Title, Origin (a local diff path or http(s) URL) and Goal are
	// required.
	Project graph.Project

	// GraphDBPath and RuleBasePath are base SQLite paths. Both are required.
	GraphDBPath  string
	RuleBasePath string

	// SeedRules populate the seeded condition's rule base. Required so that
	// "seeded" is genuinely pre-populated.
	SeedRules []patchintel.Rule

	// RunOnce overrides the repetition body. When nil, the default dispatcher
	// lane is used.
	RunOnce RunOnceFunc

	// NewDriver overrides the default native driver. Ignored when RunOnce is
	// set.
	NewDriver func(ctx context.Context, in RunInput) (worker.Driver, error)

	// Complete is the native driver completion. When nil and NewDriver is nil,
	// Run resolves dispatch.NativeComplete("main_setting").
	Complete func(ctx context.Context, prompt string) (string, error)

	// Tokens estimates the token cost of one worker result. When nil it is
	// len(Result.Raw)/4, the documented patcheval estimate.
	Tokens func(worker.Result) int64

	// MaxDuration bounds one default-lane repetition. Zero means 30s.
	MaxDuration time.Duration
}

// defaultTokens is the documented token estimate: one token per four bytes of
// raw model output. worker.Result carries no usage count.
func defaultTokens(r worker.Result) int64 {
	return int64(len(r.Raw) / 4)
}

// Run executes both conditions N times each and returns the runs and metrics.
func Run(ctx context.Context, opts Options) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.N < 3 {
		return Report{}, fmt.Errorf("eval: N must be at least 3, got %d", opts.N)
	}
	if strings.TrimSpace(opts.GraphDBPath) == "" {
		return Report{}, errors.New("eval: GraphDBPath is required")
	}
	if strings.TrimSpace(opts.RuleBasePath) == "" {
		return Report{}, errors.New("eval: RuleBasePath is required")
	}
	if len(opts.SeedRules) == 0 {
		return Report{}, errors.New("eval: SeedRules must be non-empty so the seeded condition is pre-populated")
	}

	runOnce := opts.RunOnce
	if runOnce == nil {
		runOnce = defaultRunOnce(opts)
	}
	tokens := opts.Tokens
	if tokens == nil {
		tokens = defaultTokens
	}

	runs := make([]RunResult, 0, 2*opts.N)
	for _, condition := range []string{ConditionCold, ConditionSeeded} {
		for run := 1; run <= opts.N; run++ {
			if err := ctx.Err(); err != nil {
				return Report{}, fmt.Errorf("eval: %s run %d: %w", condition, run, err)
			}
			result, err := runOne(ctx, opts, condition, run, runOnce, tokens)
			if err != nil {
				return Report{}, fmt.Errorf("eval: %s run %d: %w", condition, run, err)
			}
			runs = append(runs, result)
		}
	}
	return Report{Runs: runs, Metrics: computeMetrics(runs)}, nil
}

func runOne(ctx context.Context, opts Options, condition string, run int, runOnce RunOnceFunc, tokens func(worker.Result) int64) (RunResult, error) {
	graphPath := conditionPath(opts.GraphDBPath, condition, run)
	if err := clearConditionFile(graphPath); err != nil {
		return RunResult{}, fmt.Errorf("clear graph store: %w", err)
	}
	store, err := graph.Open(graphPath)
	if err != nil {
		return RunResult{}, fmt.Errorf("open graph store: %w", err)
	}
	defer store.Close()

	rulePath := conditionPath(opts.RuleBasePath, condition, run)
	if err := clearConditionFile(rulePath); err != nil {
		return RunResult{}, fmt.Errorf("clear rule base: %w", err)
	}
	ruleBase, err := patchintel.OpenRuleBase(rulePath)
	if err != nil {
		return RunResult{}, fmt.Errorf("open rule base: %w", err)
	}
	defer ruleBase.Close()

	if condition == ConditionSeeded {
		for i, rule := range opts.SeedRules {
			if _, err := ruleBase.Promote(rule); err != nil {
				return RunResult{}, fmt.Errorf("seed rule %d: %w", i, err)
			}
		}
	}

	observation, err := runOnce(ctx, RunInput{
		Condition: condition,
		Run:       run,
		Store:     store,
		Client:    graphserver.NewLocalClient(store),
		RuleBase:  ruleBase,
		Config:    opts.Config,
	})
	if err != nil {
		return RunResult{}, err
	}

	var total int64
	for _, result := range observation.Results {
		total += tokens(result)
	}
	return RunResult{
		Condition:   condition,
		Run:         run,
		InversionOK: observation.InversionOK,
		Finding:     observation.Finding,
		TrialRounds: observation.TrialRounds,
		Tokens:      total,
	}, nil
}

func conditionPath(base, condition string, run int) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return fmt.Sprintf("%s-%s-%d%s", stem, condition, run, ext)
}

// clearConditionFile removes the derived SQLite artifacts (including WAL and
// shared-memory files) so a repeated Run starts from a clean store. Only the
// derived -<condition>-<run> path is touched; the base paths are never removed.
func clearConditionFile(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func computeMetrics(runs []RunResult) Metrics {
	if len(runs) == 0 {
		return Metrics{}
	}
	inversions := 0
	findings := 0
	rounds := 0
	var tokens int64
	for _, run := range runs {
		tokens += run.Tokens
		if run.InversionOK {
			inversions++
		}
		if !run.Finding {
			continue
		}
		findings++
		rounds += run.TrialRounds
	}
	metrics := Metrics{InversionSuccessRate: float64(inversions) / float64(len(runs))}
	if findings > 0 {
		metrics.AvgTrialRounds = float64(rounds) / float64(findings)
		metrics.TokensPerFinding = float64(tokens) / float64(findings)
	}
	return metrics
}

// inversionOK reports whether the export carries the two facts R7 requires for
// an inverted patch: a patch_profile and an exploit_idea.
func inversionOK(export string) bool {
	return strings.Contains(export, "type: patch_profile") &&
		strings.Contains(export, "type: exploit_idea")
}

// detectFinding implements the controller's runtime finding proxy for R14:
// the project is completed, the export has an exploit_idea fact, and at least
// one verdict fact carries a support edge. A refute-only verdict does not count.
func detectFinding(ctx context.Context, store *graph.Store, projectID, export string) (bool, error) {
	project, err := store.GetProject(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("eval: read project for finding: %w", err)
	}
	if project.Status != "completed" || !strings.Contains(export, "type: exploit_idea") {
		return false, nil
	}
	nodes, err := store.Nodes(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("eval: read nodes for finding: %w", err)
	}
	verdicts := make(map[string]struct{})
	for _, n := range nodes {
		if n.Kind == "fact" && n.Type == "verdict" {
			verdicts[n.ID] = struct{}{}
		}
	}
	if len(verdicts) == 0 {
		return false, nil
	}
	edges, err := store.Edges(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("eval: read edges for finding: %w", err)
	}
	for _, edge := range edges {
		if edge.Kind != "support" {
			continue
		}
		if _, ok := verdicts[edge.From]; ok {
			return true, nil
		}
	}
	return false, nil
}

// defaultRunOnce drives the real dispatcher lane for one repetition.
func defaultRunOnce(opts Options) RunOnceFunc {
	return func(ctx context.Context, in RunInput) (Observation, error) {
		inner, err := defaultDriver(ctx, opts, in)
		if err != nil {
			return Observation{}, err
		}
		measured := &measuringDriver{inner: inner}
		lane, err := dispatch.StartLane(ctx, dispatch.LaneOptions{
			Config:   in.Config,
			Store:    in.Store,
			RuleBase: in.RuleBase,
			Drivers:  []worker.Driver{measured},
		})
		if err != nil {
			return Observation{}, fmt.Errorf("start lane: %w", err)
		}

		project, err := in.Client.CreateProject(ctx, opts.Project)
		if err != nil {
			return Observation{}, fmt.Errorf("create project: %w", err)
		}

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- lane.Run(runCtx) }()

		maxDuration := opts.MaxDuration
		if maxDuration <= 0 {
			maxDuration = defaultMaxDuration
		}
		deadline := time.NewTimer(maxDuration)
		defer deadline.Stop()
		poll := time.NewTicker(pollInterval)
		defer poll.Stop()

		var (
			runErr  error
			runDone bool
		)
	wait:
		for {
			select {
			case <-ctx.Done():
				runErr = ctx.Err()
				break wait
			case err := <-done:
				runDone = true
				runErr = err
				break wait
			case <-deadline.C:
				break wait
			case <-poll.C:
				p, err := in.Client.GetProject(ctx, project.ID)
				if err == nil && p.Status == "completed" {
					break wait
				}
			}
		}

		cancel()
		if !runDone {
			<-done
		}
		// Run stopped starting work, but executor goroutines may still be
		// finishing graph writes. Wait for them before returning so runOne can
		// close the caller-supplied stores safely.
		lane.Wait()
		if err := lane.Close(); err != nil {
			return Observation{}, fmt.Errorf("close lane: %w", err)
		}
		if runErr != nil {
			return Observation{}, runErr
		}

		results, trialRounds := measured.snapshot()
		exportCtx := context.Background()
		export, err := in.Client.ExportYAML(exportCtx, project.ID)
		if err != nil {
			return Observation{}, fmt.Errorf("export graph: %w", err)
		}
		if in.Store == nil {
			return Observation{}, errors.New("eval: default lane requires a graph store")
		}
		finding, err := detectFinding(exportCtx, in.Store, project.ID, export)
		if err != nil {
			return Observation{}, err
		}
		return Observation{
			InversionOK: inversionOK(export),
			Finding:     finding,
			TrialRounds: trialRounds,
			Results:     results,
		}, nil
	}
}

func defaultDriver(ctx context.Context, opts Options, in RunInput) (worker.Driver, error) {
	if opts.NewDriver != nil {
		driver, err := opts.NewDriver(ctx, in)
		if err != nil {
			return nil, err
		}
		if driver == nil {
			return nil, errors.New("eval: NewDriver returned a nil driver")
		}
		return driver, nil
	}
	complete := opts.Complete
	if complete == nil {
		var err error
		complete, err = dispatch.NativeComplete("main_setting")
		if err != nil {
			return nil, err
		}
	}
	return worker.NewNative(worker.NativeConfig{
		Context:  dispatch.NativeContext(in.Client),
		Complete: complete,
	}), nil
}

// measuringDriver records every tool result so the harness can estimate token
// cost and count explore rounds without changing the driver contract.
type measuringDriver struct {
	inner worker.Driver

	mu      sync.Mutex
	results []worker.Result
	explore int
}

func (d *measuringDriver) Name() string { return d.inner.Name() }

func (d *measuringDriver) Supports(tt worker.TaskType) bool { return d.inner.Supports(tt) }

func (d *measuringDriver) HealthCheck(ctx context.Context) error {
	return d.inner.HealthCheck(ctx)
}

func (d *measuringDriver) Execute(ctx context.Context, t worker.Task) (worker.Result, error) {
	result, err := d.inner.Execute(ctx, t)
	d.record(t, result, true)
	return result, err
}

func (d *measuringDriver) Conclude(ctx context.Context, t worker.Task) (worker.Result, error) {
	result, err := d.inner.Conclude(ctx, t)
	// Conclude is the fallback half of the same trial; only Execute starts a
	// trial round. Its result still contributes to the token estimate.
	d.record(t, result, false)
	return result, err
}

func (d *measuringDriver) SupportsConclude() bool { return d.inner.SupportsConclude() }

func (d *measuringDriver) record(t worker.Task, result worker.Result, countTrial bool) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.results = append(d.results, result)
	if countTrial && worker.TaskType(strings.TrimSpace(t.Type)) == worker.TaskExplore {
		d.explore++
	}
}

func (d *measuringDriver) snapshot() ([]worker.Result, int) {
	if d == nil {
		return nil, 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]worker.Result(nil), d.results...), d.explore
}

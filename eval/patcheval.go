// Package eval implements the patcheval dual-condition comparison for the
// graph lane (R14).
//
// The same target is evaluated twice: with an empty rule base ("cold") and
// with a pre-populated rule base ("seeded"). Each condition runs N>=3 times
// and every repetition contributes one JSON line with its inversion outcome,
// trial rounds and token cost. The summary reports the three comparison
// metrics: inversion success rate, average trial rounds to a PoC, and tokens
// per finding.
//
// Tests inject RunOnce so no Docker daemon, LLM or network is required. The
// default RunOnce drives the real dispatcher lane through dispatch.StartLane.
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	// AvgTrialRounds is the mean explore trial count over successful
	// inversions (runs that never reach a PoC do not dilute it).
	AvgTrialRounds float64 `json:"avg_trial_rounds"`
	// TokensPerFinding is total estimated tokens / successful inversions.
	TokensPerFinding float64 `json:"tokens_per_finding"`
}

// RunResult is one repetition in one condition. Its JSON shape is the
// line-oriented contract:
//
//	{"condition":"cold","run":1,"inversion_ok":true,"trial_rounds":2,"tokens":12345}
type RunResult struct {
	Condition   string `json:"condition"`
	Run         int    `json:"run"`
	InversionOK bool   `json:"inversion_ok"`
	TrialRounds int    `json:"trial_rounds"`
	Tokens      int64  `json:"tokens"`
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
	store, err := graph.Open(conditionPath(opts.GraphDBPath, condition, run))
	if err != nil {
		return RunResult{}, fmt.Errorf("open graph store: %w", err)
	}
	defer store.Close()

	ruleBase, err := patchintel.OpenRuleBase(conditionPath(opts.RuleBasePath, condition, run))
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
		TrialRounds: observation.TrialRounds,
		Tokens:      total,
	}, nil
}

func conditionPath(base, condition string, run int) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return fmt.Sprintf("%s-%s-%d%s", stem, condition, run, ext)
}

func computeMetrics(runs []RunResult) Metrics {
	if len(runs) == 0 {
		return Metrics{}
	}
	successes := 0
	rounds := 0
	var tokens int64
	for _, run := range runs {
		tokens += run.Tokens
		if !run.InversionOK {
			continue
		}
		successes++
		rounds += run.TrialRounds
	}
	metrics := Metrics{InversionSuccessRate: float64(successes) / float64(len(runs))}
	if successes > 0 {
		metrics.AvgTrialRounds = float64(rounds) / float64(successes)
		metrics.TokensPerFinding = float64(tokens) / float64(successes)
	}
	return metrics
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
			Config:  in.Config,
			Store:   in.Store,
			Drivers: []worker.Driver{measured},
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

	wait:
		for {
			select {
			case <-ctx.Done():
				cancel()
				<-done
				return Observation{}, ctx.Err()
			case err := <-done:
				if err != nil {
					return Observation{}, err
				}
				break wait
			case <-deadline.C:
				cancel()
				<-done
				break wait
			case <-poll.C:
				p, err := in.Client.GetProject(ctx, project.ID)
				if err == nil && p.Status == "completed" {
					cancel()
					<-done
					break wait
				}
			}
		}

		results, trialRounds := measured.snapshot()
		inversionOK := false
		if export, err := in.Client.ExportYAML(context.Background(), project.ID); err == nil {
			inversionOK = strings.Contains(export, "type: patch_profile")
		}
		return Observation{
			InversionOK: inversionOK,
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
	d.record(t, result)
	return result, err
}

func (d *measuringDriver) Conclude(ctx context.Context, t worker.Task) (worker.Result, error) {
	result, err := d.inner.Conclude(ctx, t)
	d.record(t, result)
	return result, err
}

func (d *measuringDriver) SupportsConclude() bool { return d.inner.SupportsConclude() }

func (d *measuringDriver) record(t worker.Task, result worker.Result) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.results = append(d.results, result)
	if worker.TaskType(strings.TrimSpace(t.Type)) == worker.TaskExplore {
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

package eval

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"AIxVuln/dispatch/worker"
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

package toolCalling

import (
	"AIxVuln/patchintel"
	"AIxVuln/taskManager"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type promoteRuleToolResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Result  struct {
		Mode   string          `json:"mode"`
		Rule   patchintel.Rule `json:"rule"`
		Result string          `json:"result"`
	} `json:"result"`
}

func executePromoteRuleTool(t *testing.T, tool *PromoteRuleTool, args map[string]interface{}) promoteRuleToolResponse {
	t.Helper()
	raw := tool.Execute(args)
	var resp promoteRuleToolResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal tool response %q: %v", raw, err)
	}
	if !resp.Success {
		t.Fatalf("tool failed: %s", resp.Error)
	}
	return resp
}

// TestPromoteRuleToolDistinguishesConfirmedAndRefutedOutcomes pins the shared
// contract: a confirmed outcome opens a new rule version, while a refuted
// outcome lowers the confidence of the existing row in place.
func TestPromoteRuleToolDistinguishesConfirmedAndRefutedOutcomes(t *testing.T) {
	ruleBase, err := patchintel.OpenRuleBase(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("OpenRuleBase: %v", err)
	}
	t.Cleanup(func() { _ = ruleBase.Close() })

	tool := &PromoteRuleTool{task: &taskManager.Task{}, ruleBase: ruleBase}
	pattern := "rule feedback tool confirmed and refuted branches"

	promoted := executePromoteRuleTool(t, tool, map[string]interface{}{
		"pattern":    pattern,
		"confidence": 0.5,
	})
	if promoted.Result.Mode != "promote" || promoted.Result.Rule.Version != 1 {
		t.Fatalf("promote result = %+v, want mode=promote version=1", promoted.Result)
	}
	if promoted.Result.Rule.ConfirmCount != 0 {
		t.Fatalf("new rule confirm_count = %d, want 0", promoted.Result.Rule.ConfirmCount)
	}

	confirmed := executePromoteRuleTool(t, tool, map[string]interface{}{
		"ruleId":            promoted.Result.Rule.ID,
		"outcome":           "confirmed",
		"verificationNotes": "sandbox confirmed",
	})
	if confirmed.Result.Mode != "outcome" {
		t.Fatalf("mode = %q, want outcome", confirmed.Result.Mode)
	}
	if confirmed.Result.Rule.ID == promoted.Result.Rule.ID {
		t.Fatalf("confirmed outcome must create a new version, got the same row %d", confirmed.Result.Rule.ID)
	}
	if confirmed.Result.Rule.Version != 2 {
		t.Fatalf("confirmed version = %d, want 2", confirmed.Result.Rule.Version)
	}
	if confirmed.Result.Rule.ConfirmCount != 1 {
		t.Fatalf("confirmed confirm_count = %d, want 1", confirmed.Result.Rule.ConfirmCount)
	}
	if confirmed.Result.Rule.Confidence <= promoted.Result.Rule.Confidence {
		t.Fatalf("confirmed confidence = %v, want > %v", confirmed.Result.Rule.Confidence, promoted.Result.Rule.Confidence)
	}
	if !strings.Contains(confirmed.Result.Result, "new rule version") {
		t.Fatalf("confirmed result = %q, want it to mention a new rule version", confirmed.Result.Result)
	}

	refuted := executePromoteRuleTool(t, tool, map[string]interface{}{
		"ruleId":            confirmed.Result.Rule.ID,
		"outcome":           "refuted",
		"verificationNotes": "false positive",
	})
	if refuted.Result.Mode != "outcome" {
		t.Fatalf("mode = %q, want outcome", refuted.Result.Mode)
	}
	if refuted.Result.Rule.ID != confirmed.Result.Rule.ID {
		t.Fatalf("refuted outcome must adjust rule %d in place, got id %d", confirmed.Result.Rule.ID, refuted.Result.Rule.ID)
	}
	if refuted.Result.Rule.Version != confirmed.Result.Rule.Version {
		t.Fatalf("refuted version = %d, want unchanged %d", refuted.Result.Rule.Version, confirmed.Result.Rule.Version)
	}
	if refuted.Result.Rule.ConfirmCount != confirmed.Result.Rule.ConfirmCount {
		t.Fatalf("refuted confirm_count = %d, want unchanged %d", refuted.Result.Rule.ConfirmCount, confirmed.Result.Rule.ConfirmCount)
	}
	if refuted.Result.Rule.Confidence >= confirmed.Result.Rule.Confidence {
		t.Fatalf("refuted confidence = %v, want < %v", refuted.Result.Rule.Confidence, confirmed.Result.Rule.Confidence)
	}
	if !strings.Contains(refuted.Result.Result, "in-place") || !strings.Contains(refuted.Result.Result, "no new version") {
		t.Fatalf("refuted result = %q, want an in-place adjustment without a new version", refuted.Result.Result)
	}
}

package patchintel

import (
	"path/filepath"
	"testing"
)

func openTestRuleBase(t *testing.T) *RuleBase {
	t.Helper()
	base, err := OpenRuleBase(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("OpenRuleBase: %v", err)
	}
	t.Cleanup(func() { _ = base.Close() })
	return base
}

func TestPromoteCreatesNewVersionsWithoutOverwriting(t *testing.T) {
	base := openTestRuleBase(t)
	first, err := base.Promote(Rule{
		CVEID:             "CVE-2025-3248",
		Pattern:           "unauthenticated code-validation endpoint executes user Python",
		TriggerFeatures:   "POST /api/v1/validate/code body.code",
		VerificationNotes: "refusal text is not success",
		Confidence:        0.6,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if first.Version != 1 {
		t.Fatalf("expected version 1, got %d", first.Version)
	}
	second, err := base.Promote(Rule{
		CVEID:             "CVE-2025-3248",
		Pattern:           first.Pattern,
		TriggerFeatures:   first.TriggerFeatures,
		VerificationNotes: first.VerificationNotes,
		Confidence:        0.7,
	})
	if err != nil {
		t.Fatalf("Promote second: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}
	old, err := base.Get(first.ID)
	if err != nil {
		t.Fatalf("Get old: %v", err)
	}
	if old.Version != 1 || old.Confidence != 0.6 {
		t.Fatalf("old version must be preserved, got %+v", old)
	}
}

func TestPromotePersistsConfirmCount(t *testing.T) {
	base := openTestRuleBase(t)
	rule, err := base.Promote(Rule{
		Pattern:      "confirmed at promotion time",
		Confidence:   0.6,
		ConfirmCount: 1,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if rule.ConfirmCount != 1 {
		t.Fatalf("confirm_count = %d, want 1", rule.ConfirmCount)
	}
}

func TestUpdateOutcomeConfirmedCreatesNewVersion(t *testing.T) {
	base := openTestRuleBase(t)
	rule, err := base.Promote(Rule{
		Pattern:    "path traversal in static file handler",
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	confirmed, err := base.UpdateOutcome(rule.ID, true, "sandbox confirmed traversal")
	if err != nil {
		t.Fatalf("UpdateOutcome: %v", err)
	}
	if confirmed.Version != 2 {
		t.Fatalf("expected version 2, got %d", confirmed.Version)
	}
	if confirmed.Confidence <= 0.5 {
		t.Fatalf("expected raised confidence, got %v", confirmed.Confidence)
	}
	if confirmed.ConfirmCount != 1 {
		t.Fatalf("expected confirm_count 1, got %d", confirmed.ConfirmCount)
	}
	if confirmed.VerificationNotes != "sandbox confirmed traversal" {
		t.Fatalf("expected appended note, got %q", confirmed.VerificationNotes)
	}
	// The confirmed version is a new row and the original stays unchanged.
	original, err := base.Get(rule.ID)
	if err != nil {
		t.Fatalf("Get original: %v", err)
	}
	if original.Version != 1 || original.ConfirmCount != 0 || original.Confidence != 0.5 {
		t.Fatalf("original version must be retained, got %+v", original)
	}
}

func TestUpdateOutcomeRefutedAdjustsConfidenceInPlace(t *testing.T) {
	base := openTestRuleBase(t)
	rule, err := base.Promote(Rule{
		Pattern:    "path traversal in static file handler",
		Confidence: 0.6,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	refuted, err := base.UpdateOutcome(rule.ID, false, "false positive")
	if err != nil {
		t.Fatalf("UpdateOutcome refuted: %v", err)
	}
	if refuted.ID != rule.ID {
		t.Fatalf("refuted outcome must revise rule %d in place, got id %d", rule.ID, refuted.ID)
	}
	if refuted.Version != rule.Version {
		t.Fatalf("refuted outcome must not bump the version: got %d, want %d", refuted.Version, rule.Version)
	}
	if refuted.ConfirmCount != rule.ConfirmCount {
		t.Fatalf("refuted outcome must not change confirm_count: got %d, want %d", refuted.ConfirmCount, rule.ConfirmCount)
	}
	if refuted.Confidence >= rule.Confidence {
		t.Fatalf("expected lowered confidence, got %v", refuted.Confidence)
	}
	if refuted.VerificationNotes != "false positive" {
		t.Fatalf("expected appended note, got %q", refuted.VerificationNotes)
	}
	latest, err := base.FindLatestByPattern(rule.Pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern: %v", err)
	}
	if latest.ID != rule.ID || latest.Version != 1 {
		t.Fatalf("expected the single in-place row, got %+v", latest)
	}
}

func TestSearchReturnsLatestVersionRankedByRelevance(t *testing.T) {
	base := openTestRuleBase(t)
	langflow, err := base.Promote(Rule{
		CVEID:             "CVE-2025-3248",
		Pattern:           "langflow validate code endpoint executes user Python",
		TriggerFeatures:   "POST /api/v1/validate/code",
		VerificationNotes: "check response includes computed value",
		Confidence:        0.6,
	})
	if err != nil {
		t.Fatalf("Promote langflow: %v", err)
	}
	if _, err := base.Promote(Rule{Pattern: "wordpress plugin options update privilege escalation", Confidence: 0.9}); err != nil {
		t.Fatalf("Promote wordpress: %v", err)
	}
	if _, err := base.UpdateOutcome(langflow.ID, true, "confirmed"); err != nil {
		t.Fatalf("UpdateOutcome: %v", err)
	}
	rules, err := base.Search("langflow validate code", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected search hits")
	}
	if rules[0].Pattern != langflow.Pattern {
		t.Fatalf("expected langflow rule first, got %+v", rules[0])
	}
	if rules[0].Version != 2 {
		t.Fatalf("expected latest version 2, got %d", rules[0].Version)
	}
	if err := base.RecordHits([]int64{rules[0].ID}); err != nil {
		t.Fatalf("RecordHits: %v", err)
	}
	hits, err := base.Get(rules[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if hits.HitCount != 1 {
		t.Fatalf("expected hit_count 1, got %d", hits.HitCount)
	}
}

func TestLinkIdeaOutcomeCreatesNewVersion(t *testing.T) {
	base := openTestRuleBase(t)
	rule, err := base.Promote(Rule{
		CVEID:      "CVE-2025-3248",
		Pattern:    "unauthenticated code validation executes user input",
		Confidence: 0.6,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if linked, err := base.FindLatestByPattern(rule.Pattern); err != nil || linked == nil || linked.ID != rule.ID {
		t.Fatalf("FindLatestByPattern: got %+v, err %v", linked, err)
	}
	if missing, err := base.FindLatestByPattern("no such pattern"); err != nil || missing != nil {
		t.Fatalf("expected nil for unknown pattern, got %+v, err %v", missing, err)
	}
	if err := base.LinkIdea("PO-patch-1", rule.ID); err != nil {
		t.Fatalf("LinkIdea: %v", err)
	}
	updated, err := base.RecordIdeaOutcome("PO-patch-1", true, "sandbox confirmed")
	if err != nil {
		t.Fatalf("RecordIdeaOutcome: %v", err)
	}
	if updated == nil || updated.Version != 2 || updated.ConfirmCount != 1 {
		t.Fatalf("unexpected updated rule: %+v", updated)
	}
	// A second outcome must build on the newest version, not the linked one.
	again, err := base.RecordIdeaOutcome("PO-patch-1", true, "re-confirmed")
	if err != nil {
		t.Fatalf("RecordIdeaOutcome second: %v", err)
	}
	if again == nil || again.Version != 3 || again.ConfirmCount != 2 {
		t.Fatalf("expected version 3 / confirm 2, got %+v", again)
	}
	if ignored, err := base.RecordIdeaOutcome("E.1", true, "ordinary idea"); err != nil || ignored != nil {
		t.Fatalf("unlinked idea must be ignored, got %+v, err %v", ignored, err)
	}
}

func TestLatestPatternsFiltersByCVE(t *testing.T) {
	base := openTestRuleBase(t)
	if _, err := base.Promote(Rule{CVEID: "CVE-1", Pattern: "pattern one"}); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if _, err := base.Promote(Rule{CVEID: "CVE-2", Pattern: "pattern two"}); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	rules, err := base.LatestPatterns("CVE-2", 10)
	if err != nil {
		t.Fatalf("LatestPatterns: %v", err)
	}
	if len(rules) != 1 || rules[0].CVEID != "CVE-2" {
		t.Fatalf("unexpected filtered rules: %+v", rules)
	}
}

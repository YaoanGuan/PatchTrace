package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"AIxVuln/graph"
	"AIxVuln/patchintel"
)

// --- helpers ---------------------------------------------------------------

func newOutcomeStores(t *testing.T) (*graph.Store, *patchintel.RuleBase) {
	t.Helper()
	s, err := graph.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	rb, err := patchintel.OpenRuleBase(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("patchintel.OpenRuleBase: %v", err)
	}
	t.Cleanup(func() { _ = rb.Close() })
	return s, rb
}

func newOutcomeProject(t *testing.T, s *graph.Store) graph.Project {
	t.Helper()
	p, err := s.CreateProject(context.Background(), graph.Project{
		ID:               "p1",
		Title:            "outcome project",
		Origin:           "a patch was applied",
		Goal:             "the vulnerability is understood",
		BootstrapEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p
}

func ruleHitFacts(t *testing.T, s *graph.Store, projectID string) []graph.Node {
	t.Helper()
	nodes, err := s.Nodes(context.Background(), projectID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	var hits []graph.Node
	for _, n := range nodes {
		if n.Kind == "fact" && n.Type == "rule_hit" {
			hits = append(hits, n)
		}
	}
	return hits
}

type ruleHitPayload struct {
	RuleID       int64   `json:"rule_id"`
	Version      int     `json:"version"`
	ConfirmCount int     `json:"confirm_count"`
	Confidence   float64 `json:"confidence"`
	Pattern      string  `json:"pattern"`
	Confirmed    bool    `json:"confirmed"`
}

func decodeRuleHit(t *testing.T, n graph.Node) ruleHitPayload {
	t.Helper()
	var payload ruleHitPayload
	if err := json.Unmarshal([]byte(n.Content), &payload); err != nil {
		t.Fatalf("rule_hit content %q is not valid JSON: %v", n.Content, err)
	}
	return payload
}

func hasProduceEdge(t *testing.T, s *graph.Store, projectID, from, to string) bool {
	t.Helper()
	edges, err := s.Edges(context.Background(), projectID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	for _, e := range edges {
		if e.Kind == "produce" && e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// --- required tests --------------------------------------------------------

func TestConfirmedVerdictIncrementsConfirmCount(t *testing.T) {
	s, rb := newOutcomeStores(t)
	p := newOutcomeProject(t, s)

	rule, err := rb.Promote(patchintel.Rule{
		Pattern:    "langflow validate code executes user input",
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}

	verdict, err := s.AddVerdict(context.Background(), p.ID, "origin", "support",
		"sandbox confirmed code execution", "sandbox: command=id exit=0")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}

	note := "pattern: langflow validate code executes user input\nsandbox execution exited 0"
	if err := ApplyOutcome(context.Background(), rb, s, p.ID, verdict.ID, true, note); err != nil {
		t.Fatalf("ApplyOutcome: %v", err)
	}

	latest, err := rb.FindLatestByPattern(rule.Pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern: %v", err)
	}
	if latest == nil {
		t.Fatal("rule disappeared")
	}
	if latest.Version != 2 {
		t.Fatalf("version = %d, want 2 (confirmed outcome bumps version)", latest.Version)
	}
	if latest.ConfirmCount != 1 {
		t.Fatalf("confirm_count = %d, want 1", latest.ConfirmCount)
	}
	if latest.Confidence <= 0.5 {
		t.Fatalf("confidence = %v, want > 0.5", latest.Confidence)
	}

	old, err := rb.Get(rule.ID)
	if err != nil {
		t.Fatalf("Get old version: %v", err)
	}
	if old.Version != 1 || old.Confidence != 0.5 {
		t.Fatalf("old version must be retained, got %+v", old)
	}

	hits := ruleHitFacts(t, s, p.ID)
	if len(hits) != 1 {
		t.Fatalf("rule_hit facts = %d, want 1", len(hits))
	}
	if len(hits[0].From) != 1 || hits[0].From[0] != verdict.ID {
		t.Fatalf("rule_hit from = %v, want [%s]", hits[0].From, verdict.ID)
	}
	if !hasProduceEdge(t, s, p.ID, verdict.ID, hits[0].ID) {
		t.Fatalf("missing produce edge %s -> %s", verdict.ID, hits[0].ID)
	}
	payload := decodeRuleHit(t, hits[0])
	if payload.RuleID != latest.ID || payload.Version != 2 || payload.ConfirmCount != 1 {
		t.Fatalf("rule_hit payload = %+v, want rule=%d version=2 confirm=1", payload, latest.ID)
	}
	if payload.Confidence != latest.Confidence {
		t.Fatalf("rule_hit confidence = %v, want %v", payload.Confidence, latest.Confidence)
	}
	if !payload.Confirmed {
		t.Fatal("rule_hit confirmed = false, want true")
	}
	if payload.Pattern != rule.Pattern {
		t.Fatalf("rule_hit pattern = %q, want %q", payload.Pattern, rule.Pattern)
	}
}

func TestFailedVerdictLowersConfidence(t *testing.T) {
	s, rb := newOutcomeStores(t)
	p := newOutcomeProject(t, s)

	rule, err := rb.Promote(patchintel.Rule{
		Pattern:    "path traversal in static file handler",
		Confidence: 0.6,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}

	verdict, err := s.AddVerdict(context.Background(), p.ID, "origin", "refute",
		"sandbox did not reproduce traversal", "sandbox: command=curl exit=1")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}

	note := "pattern: path traversal in static file handler\nsandbox refuted traversal"
	if err := ApplyOutcome(context.Background(), rb, s, p.ID, verdict.ID, false, note); err != nil {
		t.Fatalf("ApplyOutcome: %v", err)
	}

	latest, err := rb.FindLatestByPattern(rule.Pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern: %v", err)
	}
	if latest == nil {
		t.Fatal("rule disappeared")
	}
	if latest.Version != 1 {
		t.Fatalf("version = %d, want 1 (failed outcome must not bump version)", latest.Version)
	}
	if latest.ConfirmCount != 0 {
		t.Fatalf("confirm_count = %d, want 0", latest.ConfirmCount)
	}
	if latest.Confidence >= 0.6 {
		t.Fatalf("confidence = %v, want < 0.6", latest.Confidence)
	}

	// The failure is recorded in place: the lowered row is the original one.
	updated, err := rb.Get(rule.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.Version != 1 || updated.Confidence != latest.Confidence {
		t.Fatalf("failure must update the existing row in place, got %+v", updated)
	}

	hits := ruleHitFacts(t, s, p.ID)
	if len(hits) != 1 {
		t.Fatalf("rule_hit facts = %d, want 1", len(hits))
	}
	payload := decodeRuleHit(t, hits[0])
	if payload.RuleID != rule.ID || payload.Version != 1 || payload.ConfirmCount != 0 {
		t.Fatalf("rule_hit payload = %+v, want rule=%d version=1 confirm=0", payload, rule.ID)
	}
	if payload.Confirmed {
		t.Fatal("rule_hit confirmed = true, want false")
	}
	if payload.Confidence != latest.Confidence {
		t.Fatalf("rule_hit confidence = %v, want %v", payload.Confidence, latest.Confidence)
	}
}

func TestNewPatternPromotedAsNewVersion(t *testing.T) {
	s, rb := newOutcomeStores(t)
	p := newOutcomeProject(t, s)

	pattern := "server-side template injection in preview endpoint"

	first, err := s.AddVerdict(context.Background(), p.ID, "origin", "support",
		"sandbox confirmed template evaluation", "")
	if err != nil {
		t.Fatalf("AddVerdict first: %v", err)
	}
	if err := ApplyOutcome(context.Background(), rb, s, p.ID, first.ID, true, "pattern: "+pattern); err != nil {
		t.Fatalf("ApplyOutcome first: %v", err)
	}
	v1, err := rb.FindLatestByPattern(pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern v1: %v", err)
	}
	if v1 == nil || v1.Version != 1 {
		t.Fatalf("first outcome must promote version 1, got %+v", v1)
	}
	if v1.ConfirmCount != 0 {
		t.Fatalf("promoted rule confirm_count = %d, want 0 (Promote has no confirm field)", v1.ConfirmCount)
	}

	second, err := s.AddVerdict(context.Background(), p.ID, "origin", "support",
		"sandbox re-confirmed template evaluation", "")
	if err != nil {
		t.Fatalf("AddVerdict second: %v", err)
	}
	if err := ApplyOutcome(context.Background(), rb, s, p.ID, second.ID, true, "pattern: "+pattern); err != nil {
		t.Fatalf("ApplyOutcome second: %v", err)
	}
	v2, err := rb.FindLatestByPattern(pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern v2: %v", err)
	}
	if v2.Version != 2 {
		t.Fatalf("second outcome version = %d, want 2", v2.Version)
	}
	if v2.ConfirmCount != 1 {
		t.Fatalf("second outcome confirm_count = %d, want 1", v2.ConfirmCount)
	}
	if v2.Confidence <= v1.Confidence {
		t.Fatalf("confidence = %v, want > %v", v2.Confidence, v1.Confidence)
	}

	// The version-1 row stays behind for auditability.
	retained, err := rb.Get(v1.ID)
	if err != nil {
		t.Fatalf("Get v1: %v", err)
	}
	if retained.Version != 1 || retained.ConfirmCount != 0 || retained.Confidence != v1.Confidence {
		t.Fatalf("version 1 must be retained unchanged, got %+v", retained)
	}

	if hits := ruleHitFacts(t, s, p.ID); len(hits) != 2 {
		t.Fatalf("rule_hit facts = %d, want 2", len(hits))
	}
}

// --- focused tests ---------------------------------------------------------

func TestApplyOutcomeRejectsInvalidInputWithoutWrites(t *testing.T) {
	s, rb := newOutcomeStores(t)
	p := newOutcomeProject(t, s)

	if _, err := rb.Promote(patchintel.Rule{Pattern: "existing pattern", Confidence: 0.5}); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	intent, err := s.DeclareIntent(context.Background(), p.ID, graph.Node{
		Type: "gap", Content: "open question", Creator: "tester", From: []string{"origin"},
	})
	if err != nil {
		t.Fatalf("DeclareIntent: %v", err)
	}
	observation, err := s.AddFact(context.Background(), p.ID, graph.Node{
		Type: "observation", Content: "an observation", Creator: "tester", From: []string{"origin"},
	})
	if err != nil {
		t.Fatalf("AddFact: %v", err)
	}
	verdict, err := s.AddVerdict(context.Background(), p.ID, "origin", "support", "valid verdict", "")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}

	nodesBefore, err := s.Nodes(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Nodes before: %v", err)
	}

	cases := []struct {
		name      string
		rb        *patchintel.RuleBase
		s         *graph.Store
		projectID string
		verdictID string
	}{
		{"nil rule base", nil, s, p.ID, verdict.ID},
		{"nil store", rb, nil, p.ID, verdict.ID},
		{"empty project", rb, s, "", verdict.ID},
		{"empty verdict", rb, s, p.ID, ""},
		{"unknown verdict", rb, s, p.ID, "does-not-exist"},
		{"intent is not a verdict", rb, s, p.ID, intent.ID},
		{"plain fact is not a verdict", rb, s, p.ID, observation.ID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ApplyOutcome(context.Background(), tc.rb, tc.s, tc.projectID, tc.verdictID, true, "")
			if !errors.Is(err, graph.ErrInvalid) {
				t.Fatalf("error = %v, want graph.ErrInvalid", err)
			}
		})
	}

	nodesAfter, err := s.Nodes(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Nodes after: %v", err)
	}
	if len(nodesAfter) != len(nodesBefore) {
		t.Fatalf("invalid input wrote nodes: %d -> %d", len(nodesBefore), len(nodesAfter))
	}
	if hits := ruleHitFacts(t, s, p.ID); len(hits) != 0 {
		t.Fatalf("invalid input wrote rule_hit facts: %+v", hits)
	}
	rules, err := rb.LatestPatterns("", 0)
	if err != nil {
		t.Fatalf("LatestPatterns: %v", err)
	}
	if len(rules) != 1 || rules[0].Pattern != "existing pattern" {
		t.Fatalf("invalid input touched the rule base: %+v", rules)
	}
}

func TestApplyOutcomeUsesLinkedIdeaBeforePattern(t *testing.T) {
	s, rb := newOutcomeStores(t)
	p := newOutcomeProject(t, s)

	rule, err := rb.Promote(patchintel.Rule{
		Pattern:    "unauthenticated code validation endpoint",
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	idea, err := s.AddFact(context.Background(), p.ID, graph.Node{
		Type: "exploit_idea", Content: "idea derived from the patch", Creator: "tester", From: []string{"origin"},
	})
	if err != nil {
		t.Fatalf("AddFact idea: %v", err)
	}
	if err := rb.LinkIdea(idea.ID, rule.ID); err != nil {
		t.Fatalf("LinkIdea: %v", err)
	}

	verdict, err := s.AddVerdict(context.Background(), p.ID, idea.ID, "support",
		"sandbox confirmed linked idea", "")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}
	if err := ApplyOutcome(context.Background(), rb, s, p.ID, verdict.ID, true, "sandbox confirmed"); err != nil {
		t.Fatalf("ApplyOutcome: %v", err)
	}

	latest, err := rb.FindLatestByPattern(rule.Pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern: %v", err)
	}
	if latest.Version != 2 || latest.ConfirmCount != 1 {
		t.Fatalf("linked idea rule = %+v, want version 2 / confirm 1", latest)
	}
	// The verdict description must not have been promoted as a separate rule.
	if extra, err := rb.FindLatestByPattern("sandbox confirmed linked idea"); err != nil || extra != nil {
		t.Fatalf("verdict description was promoted separately: %+v, err %v", extra, err)
	}
}

func TestApplyOutcomeDerivesPatternFromVerdictContent(t *testing.T) {
	s, rb := newOutcomeStores(t)
	p := newOutcomeProject(t, s)

	pattern := "unbounded file read in export handler"
	rule, err := rb.Promote(patchintel.Rule{Pattern: pattern, Confidence: 0.5})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	verdict, err := s.AddVerdict(context.Background(), p.ID, "origin", "support",
		pattern, "sandbox: command=curl exit=0")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}
	// No marker: the pattern comes from the verdict description with the
	// evidence reference suffix stripped.
	if err := ApplyOutcome(context.Background(), rb, s, p.ID, verdict.ID, true, "no marker here"); err != nil {
		t.Fatalf("ApplyOutcome: %v", err)
	}

	latest, err := rb.FindLatestByPattern(rule.Pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern: %v", err)
	}
	if latest.Version != 2 || latest.ConfirmCount != 1 {
		t.Fatalf("derived-pattern rule = %+v, want version 2 / confirm 1", latest)
	}
}

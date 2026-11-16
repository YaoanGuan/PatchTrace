package graph

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// mustObservation creates the sandbox observation fact a verdict must rest on.
func mustObservation(t *testing.T, s *Store, projectID string) Node {
	t.Helper()
	n, err := s.AddFact(ctx, projectID, Node{
		Type:    "observation",
		Content: "sandbox: command=`id` exit=0",
		Creator: "tester",
		From:    []string{"origin"},
	})
	if err != nil {
		t.Fatalf("AddFact(observation): %v", err)
	}
	return n
}

func TestVerdictRequiresSupportOrRefuteEdge(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)
	source := mustObservation(t, s, p.ID)

	for _, verdict := range []string{"support", "refute"} {
		t.Run(verdict, func(t *testing.T) {
			node, err := s.AddVerdict(ctx, p.ID, target.ID, verdict,
				"沙箱观测确认该方向", source.ID, "file:///tmp/evidence.log")
			if err != nil {
				t.Fatalf("AddVerdict(%s): %v", verdict, err)
			}
			if node.Kind != "fact" {
				t.Fatalf("verdict node kind = %q, want fact", node.Kind)
			}
			if node.Type != "verdict" {
				t.Fatalf("verdict node type = %q, want verdict", node.Type)
			}
			if node.State != "resolved" {
				t.Fatalf("verdict node state = %q, want resolved", node.State)
			}
			if !strings.Contains(node.Content, "沙箱观测确认该方向") {
				t.Fatalf("verdict content = %q, want the description", node.Content)
			}
			if !strings.Contains(node.Content, "file:///tmp/evidence.log") {
				t.Fatalf("verdict content = %q, want the evidence ref", node.Content)
			}

			edges, err := s.Edges(ctx, p.ID)
			if err != nil {
				t.Fatalf("Edges: %v", err)
			}
			foundJudgement := false
			foundProduce := false
			for _, e := range edges {
				if e.Kind == verdict && e.From == node.ID && e.To == target.ID {
					foundJudgement = true
				}
				if e.Kind == "produce" && e.From == source.ID && e.To == node.ID {
					foundProduce = true
				}
			}
			if !foundJudgement {
				t.Fatalf("missing %s edge %s -> %s in %+v", verdict, node.ID, target.ID, edges)
			}
			if !foundProduce {
				t.Fatalf("missing produce edge %s -> %s in %+v", source.ID, node.ID, edges)
			}
		})
	}
}

// TestAddVerdictRejectsMissingEvidenceSource inverts the old behaviour that
// allowed a verdict with a blank evidence reference. A verdict must cite the
// sandbox observation it rests on.
func TestAddVerdictRejectsMissingEvidenceSource(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)

	nodesBefore, err := s.Nodes(ctx, p.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	edgesBefore, err := s.Edges(ctx, p.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}

	_, err = s.AddVerdict(ctx, p.ID, target.ID, "support", "有证据但未给来源", "  ", "")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddVerdict err = %v, want ErrInvalid", err)
	}

	nodesAfter, err := s.Nodes(ctx, p.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	edgesAfter, err := s.Edges(ctx, p.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	if len(nodesAfter) != len(nodesBefore) || len(edgesAfter) != len(edgesBefore) {
		t.Fatalf("rejected verdict wrote state: nodes %d->%d edges %d->%d",
			len(nodesBefore), len(nodesAfter), len(edgesBefore), len(edgesAfter))
	}
}

func TestAddVerdictRejectsInvalidInputWithNoWrites(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)
	source := mustObservation(t, s, p.ID)
	nonObservation, err := s.AddFact(ctx, p.ID, Node{
		Type:    "exploit_idea",
		Content: "not an observation",
		Creator: "tester",
		From:    []string{"origin"},
	})
	if err != nil {
		t.Fatalf("AddFact: %v", err)
	}

	nodesBefore, err := s.Nodes(ctx, p.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	edgesBefore, err := s.Edges(ctx, p.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}

	cases := []struct {
		name             string
		targetID         string
		verdict          string
		description      string
		evidenceSourceID string
		want             error
	}{
		{name: "unknown verdict", targetID: target.ID, verdict: "maybe", description: "x", evidenceSourceID: source.ID, want: ErrInvalid},
		{name: "empty description", targetID: target.ID, verdict: "support", description: "  ", evidenceSourceID: source.ID, want: ErrInvalid},
		{name: "empty target", targetID: "  ", verdict: "support", description: "x", evidenceSourceID: source.ID, want: ErrInvalid},
		{name: "unknown target", targetID: "i999", verdict: "support", description: "x", evidenceSourceID: source.ID, want: ErrNotFound},
		{name: "empty source", targetID: target.ID, verdict: "support", description: "x", evidenceSourceID: "  ", want: ErrInvalid},
		{name: "unknown source", targetID: target.ID, verdict: "support", description: "x", evidenceSourceID: "f999", want: ErrInvalid},
		{name: "non-observation source", targetID: target.ID, verdict: "support", description: "x", evidenceSourceID: nonObservation.ID, want: ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.AddVerdict(ctx, p.ID, tc.targetID, tc.verdict, tc.description, tc.evidenceSourceID, ""); !errors.Is(err, tc.want) {
				t.Fatalf("AddVerdict err = %v, want %v", err, tc.want)
			}
		})
	}

	nodesAfter, err := s.Nodes(ctx, p.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	edgesAfter, err := s.Edges(ctx, p.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	if len(nodesAfter) != len(nodesBefore) || len(edgesAfter) != len(edgesBefore) {
		t.Fatalf("invalid verdict wrote state: nodes %d->%d edges %d->%d",
			len(nodesBefore), len(nodesAfter), len(edgesBefore), len(edgesAfter))
	}
}

// TestAddFactRejectsVerdictType closes the alternate fact-minting path: a
// verdict must never exist without AddVerdict's edges and evidence source.
func TestAddFactRejectsVerdictType(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	_, err := s.AddFact(ctx, p.ID, Node{
		Type:    "verdict",
		Content: "forged verdict",
		Creator: "tester",
		From:    []string{"origin"},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddFact(type=verdict) err = %v, want ErrInvalid", err)
	}
	if _, err := s.GetNode(ctx, p.ID, "f001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a rejected verdict fact must not be written, got %v", err)
	}
}

func TestAddVerdictRequiresActiveProject(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)
	source := mustObservation(t, s, p.ID)
	if _, err := s.SetProjectStatus(ctx, p.ID, "stopped"); err != nil {
		t.Fatalf("SetProjectStatus: %v", err)
	}

	_, err := s.AddVerdict(ctx, p.ID, target.ID, "support", "x", source.ID, "")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AddVerdict err = %v, want ErrForbidden", err)
	}
}

func TestAddVerdictAllowsFactTarget(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	it := mustIntent(t, s, p.ID)
	if _, err := s.ClaimIntent(ctx, p.ID, it.ID, "w1", time.Minute); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	fact, _, err := s.ConcludeIntent(ctx, p.ID, it.ID, "w1", "观测结论", "observation")
	if err != nil {
		t.Fatalf("ConcludeIntent: %v", err)
	}

	node, err := s.AddVerdict(ctx, p.ID, fact.ID, "refute", "反证", fact.ID, "")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}
	edges, err := s.Edges(ctx, p.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	found := false
	for _, e := range edges {
		if e.Kind == "refute" && e.From == node.ID && e.To == fact.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing refute edge %s -> %s", node.ID, fact.ID)
	}
}

// TestVerdictProvenanceAndCompletion is I1's regression guard: the evidence
// source's produce edge gives the verdict a causal chain, so Provenance finds
// it and Complete accepts the verdict id in `from`.
func TestVerdictProvenanceAndCompletion(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	it := mustIntent(t, s, p.ID)
	if _, err := s.ClaimIntent(ctx, p.ID, it.ID, "w1", time.Minute); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	observation, _, err := s.ConcludeIntent(ctx, p.ID, it.ID, "w1", "沙箱观测结论", "observation")
	if err != nil {
		t.Fatalf("ConcludeIntent: %v", err)
	}
	verdict, err := s.AddVerdict(ctx, p.ID, observation.ID, "support", "证据支持", observation.ID, "sandbox: exit=0")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}

	chain, err := s.Provenance(ctx, p.ID, verdict.ID)
	if err != nil {
		t.Fatalf("Provenance(verdict): %v", err)
	}
	if len(chain) < 3 || chain[0].ID != "origin" || chain[len(chain)-2].ID != observation.ID || chain[len(chain)-1].ID != verdict.ID {
		t.Fatalf("provenance chain = %+v, want origin -> ... -> %s -> %s", chain, observation.ID, verdict.ID)
	}

	completed, err := s.Complete(ctx, p.ID, []string{verdict.ID}, "finding confirmed", "w1")
	if err != nil {
		t.Fatalf("Complete(verdict from): %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("status = %q, want completed", completed.Status)
	}
}

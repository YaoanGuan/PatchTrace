package graph

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVerdictRequiresSupportOrRefuteEdge(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)

	for _, verdict := range []string{"support", "refute"} {
		t.Run(verdict, func(t *testing.T) {
			node, err := s.AddVerdict(ctx, p.ID, target.ID, verdict,
				"沙箱观测确认该方向", "file:///tmp/evidence.log")
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
			found := false
			for _, e := range edges {
				if e.Kind == verdict && e.From == node.ID && e.To == target.ID {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s edge %s -> %s in %+v", verdict, node.ID, target.ID, edges)
			}
		})
	}
}

func TestAddVerdictContentOmitsEmptyEvidenceRef(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)

	node, err := s.AddVerdict(ctx, p.ID, target.ID, "support", "有证据但未给引用", "  ")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}
	if node.Content != "有证据但未给引用" {
		t.Fatalf("content = %q, want the bare description", node.Content)
	}
}

func TestAddVerdictRejectsInvalidInputWithNoWrites(t *testing.T) {
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

	cases := []struct {
		name        string
		targetID    string
		verdict     string
		description string
		want        error
	}{
		{name: "unknown verdict", targetID: target.ID, verdict: "maybe", description: "x", want: ErrInvalid},
		{name: "empty description", targetID: target.ID, verdict: "support", description: "  ", want: ErrInvalid},
		{name: "empty target", targetID: "  ", verdict: "support", description: "x", want: ErrInvalid},
		{name: "unknown target", targetID: "i999", verdict: "support", description: "x", want: ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.AddVerdict(ctx, p.ID, tc.targetID, tc.verdict, tc.description, ""); !errors.Is(err, tc.want) {
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

func TestAddVerdictRequiresActiveProject(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	target := mustIntent(t, s, p.ID)
	if _, err := s.SetProjectStatus(ctx, p.ID, "stopped"); err != nil {
		t.Fatalf("SetProjectStatus: %v", err)
	}

	_, err := s.AddVerdict(ctx, p.ID, target.ID, "support", "x", "")
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

	node, err := s.AddVerdict(ctx, p.ID, fact.ID, "refute", "反证", "")
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

package graph

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var ctx = context.Background()

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s
}

func mustProject(t *testing.T, s *Store) Project {
	t.Helper()
	p, err := s.CreateProject(ctx, Project{
		ID:               "p1",
		Title:            "test project",
		Origin:           "a patch was applied",
		Goal:             "the vulnerability is understood",
		BootstrapEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Status != "active" {
		t.Fatalf("new project status = %q, want active", p.Status)
	}
	for _, id := range []string{"origin", "goal"} {
		n, err := s.GetNode(ctx, p.ID, id)
		if err != nil {
			t.Fatalf("GetNode(%s): %v", id, err)
		}
		if n.Kind != "fact" {
			t.Fatalf("node %s kind = %q, want fact", id, n.Kind)
		}
	}
	return p
}

func mustIntent(t *testing.T, s *Store, projectID string) Node {
	t.Helper()
	n, err := s.DeclareIntent(ctx, projectID, Node{
		Type:    "gap",
		Content: "we do not know whether the sink is reachable",
		Creator: "tester",
		From:    []string{"origin"},
	})
	if err != nil {
		t.Fatalf("DeclareIntent: %v", err)
	}
	if n.Kind != "intent" {
		t.Fatalf("kind = %q, want intent", n.Kind)
	}
	if n.State != "open" {
		t.Fatalf("state = %q, want open", n.State)
	}
	if n.Worker != "" {
		t.Fatalf("worker = %q, want empty", n.Worker)
	}
	if n.To != nil {
		t.Fatalf("to = %v, want nil", n.To)
	}
	return n
}

func TestClaimIntentIsAtomic(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	it := mustIntent(t, s, p.ID)

	var ok int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.ClaimIntent(ctx, p.ID, it.ID, fmt.Sprintf("w%d", i), time.Minute); err == nil {
				atomic.AddInt32(&ok, 1)
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("want exactly 1 claim, got %d", ok)
	}
}

func TestLeaseExpiryReopensIntent(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	it := mustIntent(t, s, p.ID)

	if _, err := s.ClaimIntent(ctx, p.ID, it.ID, "w1", -time.Minute); err != nil {
		t.Fatalf("ClaimIntent with expired lease: %v", err)
	}

	got, err := s.GetNode(ctx, p.ID, it.ID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.State != "open" {
		t.Fatalf("state = %q, want open after lease expiry", got.State)
	}
	if got.Worker != "" {
		t.Fatalf("worker = %q, want empty after lease expiry", got.Worker)
	}
	if got.LeaseUntil != nil {
		t.Fatalf("lease_until = %v, want nil after lease expiry", got.LeaseUntil)
	}
}

func TestBlockEdgeCycleRejected(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	h, err := s.AddHint(ctx, p.ID, "a hint", "tester")
	if err != nil {
		t.Fatalf("AddHint: %v", err)
	}

	if err := s.AddEdge(ctx, p.ID, "origin", "goal", "block"); err != nil {
		t.Fatalf("AddEdge origin->goal: %v", err)
	}
	if err := s.AddEdge(ctx, p.ID, "goal", h.ID, "block"); err != nil {
		t.Fatalf("AddEdge goal->hint: %v", err)
	}
	if err := s.AddEdge(ctx, p.ID, h.ID, "origin", "block"); !errors.Is(err, ErrCycle) {
		t.Fatalf("AddEdge cycle err = %v, want ErrCycle", err)
	}
	if err := s.AddEdge(ctx, p.ID, "origin", "origin", "block"); !errors.Is(err, ErrCycle) {
		t.Fatalf("AddEdge self-loop err = %v, want ErrCycle", err)
	}
}

func TestConcludeCreatesFactAndProduceEdge(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)
	it := mustIntent(t, s, p.ID)

	if _, err := s.ClaimIntent(ctx, p.ID, it.ID, "w1", time.Minute); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	fact, concluded, err := s.ConcludeIntent(ctx, p.ID, it.ID, "w1", "the sink is reachable", "observation")
	if err != nil {
		t.Fatalf("ConcludeIntent: %v", err)
	}
	if fact.Kind != "fact" {
		t.Fatalf("fact kind = %q, want fact", fact.Kind)
	}
	if fact.Type != "observation" {
		t.Fatalf("fact type = %q, want observation", fact.Type)
	}
	if fact.CreatedAt == nil {
		t.Fatal("fact created_at is nil")
	}
	if concluded.To == nil || *concluded.To != fact.ID {
		t.Fatalf("concluded.To = %v, want %s", concluded.To, fact.ID)
	}
	if concluded.ConcludedAt == nil {
		t.Fatal("concluded_at is nil")
	}

	edges, err := s.Edges(ctx, p.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	found := false
	for _, e := range edges {
		if e.Kind == "produce" && e.From == "origin" && e.To == fact.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing produce edge origin->%s in %+v", fact.ID, edges)
	}
}

func TestCompleteRejectsUnknownFact(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	_, err := s.Complete(ctx, p.ID, []string{"f999"}, "goal reached", "w1")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Complete err = %v, want ErrInvalid", err)
	}
	got, err := s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("status = %q, want active after rejected complete", got.Status)
	}
}

func TestReopenCreatesCorrectionFact(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	completed, err := s.Complete(ctx, p.ID, []string{"origin"}, "goal reached", "w1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("status = %q, want completed", completed.Status)
	}

	reopened, err := s.Reopen(ctx, p.ID, "reviewer", "the goal was misjudged")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if reopened.Status != "active" {
		t.Fatalf("status = %q, want active", reopened.Status)
	}

	nodes, err := s.Nodes(ctx, p.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	var feedback, correction *Node
	for i := range nodes {
		n := nodes[i]
		if n.Kind == "intent" && n.Content == "external_feedback" {
			feedback = &n
		}
		if n.Kind == "fact" && n.Content == "the goal was misjudged" {
			correction = &n
		}
	}
	if feedback == nil {
		t.Fatal("missing external_feedback intent")
	}
	if correction == nil {
		t.Fatal("missing correction fact")
	}
	if feedback.To == nil || *feedback.To != correction.ID {
		t.Fatalf("feedback.To = %v, want %s", feedback.To, correction.ID)
	}
	if feedback.Creator != "reviewer" || feedback.Worker != "reviewer" {
		t.Fatalf("feedback creator/worker = %q/%q, want reviewer", feedback.Creator, feedback.Worker)
	}
	if len(feedback.From) != 1 || feedback.From[0] != "origin" {
		t.Fatalf("feedback.From = %v, want [origin]", feedback.From)
	}
}

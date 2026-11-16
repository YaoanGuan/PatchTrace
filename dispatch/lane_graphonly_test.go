package dispatch

import (
	"context"
	"path/filepath"
	"testing"

	"AIxVuln/graph"
)

// TestStartLaneGraphOnly proves GraphOnly opens the stores and returns a
// dispatcher with Store() set, no drivers and no executor, and that Wait/Close
// are safe and leave the store usable until Close.
func TestStartLaneGraphOnly(t *testing.T) {
	dir := t.TempDir()
	d, err := StartLane(context.Background(), LaneOptions{
		Config:       laneTestConfig(),
		GraphDBPath:  filepath.Join(dir, "graph.db"),
		RuleBasePath: filepath.Join(dir, "rules.db"),
		GraphOnly:    true,
	})
	if err != nil {
		t.Fatalf("StartLane(GraphOnly): %v", err)
	}
	if d == nil {
		t.Fatal("StartLane(GraphOnly) returned nil dispatcher")
	}
	t.Cleanup(func() { _ = d.Close() })

	store := d.Store()
	if store == nil {
		t.Fatal("GraphOnly dispatcher has no graph store")
	}
	if got := len(d.registry.Drivers()); got != 0 {
		t.Fatalf("GraphOnly registry drivers = %d, want 0", got)
	}
	if d.executor != nil {
		t.Fatal("GraphOnly dispatcher has an executor")
	}

	// The caller must not call Run, but Wait must still be safe.
	d.Wait()

	p, err := store.CreateProject(context.Background(), graph.Project{
		Title: "graph-only", Origin: "diff.patch", Goal: "read-only panel",
	})
	if err != nil {
		t.Fatalf("store operation after StartLane(GraphOnly): %v", err)
	}
	if p.ID == "" {
		t.Fatal("store CreateProject returned an empty id")
	}
}

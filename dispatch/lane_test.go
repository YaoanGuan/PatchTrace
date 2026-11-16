package dispatch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func laneTestConfig() Config {
	return Config{
		ServerURL:                "http://127.0.0.1:0",
		MaxWorkers:               2,
		MaxRunningProjects:       1,
		MaxProjectWorkers:        1,
		Interval:                 time.Second,
		HealthcheckTimeout:       time.Second,
		BootstrapTimeout:         time.Minute,
		BootstrapConcludeTimeout: time.Minute,
		ReasonTimeout:            time.Minute,
		ExploreTimeout:           time.Minute,
		ExploreConcludeTimeout:   time.Minute,
		PromptGroup:              "default",
	}
}

func fakeComplete(_ context.Context, _ string) (string, error) {
	return "{}", nil
}

func TestStartLaneReturnsDispatcherWithExecutor(t *testing.T) {
	dir := t.TempDir()
	d, err := StartLane(context.Background(), LaneOptions{
		Config:      laneTestConfig(),
		GraphDBPath: filepath.Join(dir, "graph.db"),
		Complete:    fakeComplete,
	})
	if err != nil {
		t.Fatalf("StartLane: %v", err)
	}
	if d == nil {
		t.Fatal("StartLane returned nil dispatcher")
	}
	if d.executor == nil {
		t.Fatal("StartLane did not wire the executor")
	}
	if len(d.registry.Drivers()) != 1 {
		t.Fatalf("registry drivers = %d, want 1 native driver", len(d.registry.Drivers()))
	}
	if d.registry.Drivers()[0].Name() != "native" {
		t.Fatalf("driver name = %q, want native", d.registry.Drivers()[0].Name())
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestStartLaneRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d, err := StartLane(ctx, LaneOptions{
		Config:      laneTestConfig(),
		GraphDBPath: filepath.Join(t.TempDir(), "graph.db"),
		Complete:    fakeComplete,
	})
	if d != nil {
		t.Fatalf("StartLane returned a dispatcher for a cancelled context: %+v", d)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StartLane error = %v, want context.Canceled", err)
	}
}

func TestStartLaneRequiresCompletion(t *testing.T) {
	_, err := StartLane(context.Background(), LaneOptions{
		Config:      laneTestConfig(),
		GraphDBPath: filepath.Join(t.TempDir(), "graph.db"),
	})
	if err == nil {
		t.Fatal("StartLane accepted a lane without a completion function")
	}
}

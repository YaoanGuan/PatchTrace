package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"AIxVuln/dispatch"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
)

func lifecycleTestConfig() dispatch.Config {
	return dispatch.Config{
		MaxWorkers:         1,
		MaxRunningProjects: 1,
		MaxProjectWorkers:  1,
		Interval:           time.Hour, // no second tick during the test
	}
}

// blockingListClient blocks ListProjects until released, even after the
// dispatcher context is cancelled. It lets a test observe that shutdown waits
// for Run to return instead of closing stores underneath it.
type blockingListClient struct {
	graphserver.Client // embedded nil: only ListProjects is used by the tick

	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingListClient) ListProjects(ctx context.Context) ([]graph.Project, error) {
	c.once.Do(func() { close(c.started) })
	<-c.release
	return nil, ctx.Err()
}

// TestGraphLaneShutdownWaitsForRun proves shutdown does not return (and
// therefore does not call Close) before the Run goroutine has exited.
func TestGraphLaneShutdownWaitsForRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &blockingListClient{started: make(chan struct{}), release: make(chan struct{})}
	dispatcher := dispatch.New(lifecycleTestConfig(), client, nil)

	runDone := make(chan struct{})
	go func() {
		_ = dispatcher.Run(ctx)
		close(runDone)
	}()
	<-client.started

	lane := &graphLane{dispatcher: dispatcher, done: runDone}
	shutdownDone := make(chan struct{})
	go func() {
		lane.shutdown(cancel)
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned before Run exited")
	case <-time.After(100 * time.Millisecond):
	}

	close(client.release)
	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not return after Run exited")
	}
}

// blockingDriver blocks inside Execute until released, so the executor has an
// in-flight goroutine when shutdown starts.
type blockingDriver struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (d *blockingDriver) Name() string                      { return "blocking" }
func (d *blockingDriver) Supports(worker.TaskType) bool     { return true }
func (d *blockingDriver) HealthCheck(context.Context) error { return nil }
func (d *blockingDriver) SupportsConclude() bool            { return false }
func (d *blockingDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{}, nil
}

func (d *blockingDriver) Execute(context.Context, worker.Task) (worker.Result, error) {
	d.once.Do(func() { close(d.started) })
	<-d.release
	return worker.Result{Accepted: false, Raw: "{}"}, nil
}

// TestGraphLaneShutdownWaitsForExecutor proves shutdown drains in-flight
// executor work (dispatcher.Wait) before returning/Close.
func TestGraphLaneShutdownWaitsForExecutor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, err := graph.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("open graph store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateProject(ctx, graph.Project{
		ID: "p", Title: "t", Origin: "diff", Goal: "g", BootstrapEnabled: true,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	driver := &blockingDriver{started: make(chan struct{}), release: make(chan struct{})}
	client := graphserver.NewLocalClient(store)
	cfg := lifecycleTestConfig()
	dispatcher := dispatch.New(cfg, client, []worker.Driver{driver})
	executor := dispatch.NewExecutor(cfg, client, worker.NewRegistry(time.Second, driver), nil)
	dispatcher.SetExecutor(executor)

	dec := dispatch.Decision{ProjectID: "p", TaskType: worker.TaskReason, WorkerName: driver.Name()}
	if !executor.Start(ctx, dec, graph.Project{ID: "p"}) {
		t.Fatal("executor refused to start the test task")
	}
	select {
	case <-driver.started:
	case <-time.After(5 * time.Second):
		t.Fatal("blocking driver did not start")
	}

	lane := &graphLane{dispatcher: dispatcher}
	shutdownDone := make(chan struct{})
	go func() {
		lane.shutdown(cancel)
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned while an executor task was still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(driver.release)
	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not return after the executor task finished")
	}
	if inFlight := executor.InFlight(); inFlight != 0 {
		t.Fatalf("in-flight tasks after shutdown = %d, want 0", inFlight)
	}
}

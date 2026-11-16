package dispatch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/worker"
	"AIxVuln/dockerManager"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"
)

// LaneOptions configures the production graph lane.
//
// The lane is an opt-in alternative to the existing hub-and-spoke closed loop:
// graph.Store -> graphserver.Client -> worker.Driver -> SandboxRuntime ->
// Executor. Callers run the returned Dispatcher themselves.
type LaneOptions struct {
	Config Config

	// GraphDBPath is the SQLite graph database. Empty defaults to
	// <misc.DATA_DIR>/graph.db. It is ignored when Store is supplied.
	GraphDBPath string

	// Store lets an embedder reuse an already-open graph store (e.g. the eval
	// harness). Ownership stays with the caller; Dispatcher.Close will not
	// close it.
	Store *graph.Store

	// Drivers overrides the native driver. When empty, Complete is required
	// and the lane builds worker.NewNative.
	Drivers []worker.Driver

	// Native driver settings; ignored when Drivers is non-empty.
	WorkerName string
	Section    string
	PromptDir  string
	Complete   func(ctx context.Context, prompt string) (string, error)
}

// StartLane wires the production graph lane: graph store, in-process graph
// protocol client, worker driver, Docker-backed sandbox runtime and bounded
// executor. It does not start the scheduler loop; the caller runs
// Dispatcher.Run with the desired context.
//
// A cancelled context is reported before any store is opened. StartLane never
// touches the Docker daemon or the LLM; those happen only when tasks execute.
func StartLane(ctx context.Context, opts LaneOptions) (*Dispatcher, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("dispatch: start lane: %w", err)
	}

	store := opts.Store
	ownsStore := false
	if store == nil {
		path := strings.TrimSpace(opts.GraphDBPath)
		if path == "" {
			path = filepath.Join(misc.GetDataDir(), "graph.db")
		}
		opened, err := graph.Open(path)
		if err != nil {
			return nil, fmt.Errorf("dispatch: open graph store: %w", err)
		}
		store = opened
		ownsStore = true
	}

	if err := ctx.Err(); err != nil {
		if ownsStore {
			_ = store.Close()
		}
		return nil, fmt.Errorf("dispatch: start lane: %w", err)
	}

	client := graphserver.NewLocalClient(store)

	drivers := append([]worker.Driver(nil), opts.Drivers...)
	if len(drivers) == 0 {
		if opts.Complete == nil {
			if ownsStore {
				_ = store.Close()
			}
			return nil, errors.New("dispatch: start lane requires a completion function or pre-built drivers")
		}
		drivers = append(drivers, worker.NewNative(worker.NativeConfig{
			Name:      opts.WorkerName,
			Section:   opts.Section,
			PromptDir: opts.PromptDir,
			Context:   NativeContext(client),
			Complete:  opts.Complete,
		}))
	}

	rt := runtime.NewDockerSandboxRuntime(dockerManager.NewDockerManager().ExecCapture, NewTaskManagerResolver())
	d := New(opts.Config, client, drivers)
	d.SetExecutor(NewExecutor(opts.Config, client, d.registry, rt))
	if ownsStore {
		d.store = store
	}
	return d, nil
}

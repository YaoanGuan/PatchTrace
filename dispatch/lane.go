package dispatch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/tasks"
	"AIxVuln/dispatch/worker"
	"AIxVuln/dockerManager"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"
	"AIxVuln/patchintel"
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

	// RuleBasePath is the SQLite PatchOracle rule base that explore verdicts
	// feed through CE4. Empty defaults to <misc.DATA_DIR>/patchintel/rules.db,
	// the same file patchintel.DefaultRuleBase uses, so feedback is visible to
	// bootstrap's rule search. Ignored when RuleBase is supplied.
	RuleBasePath string

	// RuleBase lets an embedder reuse an already-open rule base (e.g. the eval
	// harness). Ownership stays with the caller; Dispatcher.Close will not
	// close it.
	RuleBase *patchintel.RuleBase

	// Drivers overrides the native driver. When empty, Complete is required
	// and the lane builds worker.NewNative.
	Drivers []worker.Driver

	// Runtime overrides the Docker-backed sandbox runtime. Tests inject a fake
	// so explore verification never touches a Docker daemon.
	Runtime runtime.SandboxRuntime

	// Native driver settings; ignored when Drivers is non-empty.
	WorkerName string
	Section    string
	PromptDir  string
	Complete   func(ctx context.Context, prompt string) (string, error)
}

// defaultRuleBasePath mirrors patchintel.DefaultRuleBase. Keeping the lane on
// the same file is what lets CE4 feedback influence later bootstrap prompts.
func defaultRuleBasePath() string {
	return filepath.Join(misc.GetDataDir(), "patchintel", "rules.db")
}

// StartLane wires the production graph lane: graph store, in-process graph
// protocol client, worker driver, Docker-backed sandbox runtime and bounded
// executor. It does not start the scheduler loop; the caller runs
// Dispatcher.Run with the desired context.
//
// Explore verdicts are fed back into the rule base (CE4) through the executor's
// OnVerdict hook, so an accepted verdict grows the rule base and writes a
// rule_hit fact. The hook derives the confirmation from the support/refute edge
// AddVerdict wrote.
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
	cleanupStore := func() {
		if ownsStore {
			_ = store.Close()
		}
	}

	ruleBase := opts.RuleBase
	ownsRuleBase := false
	if ruleBase == nil {
		path := strings.TrimSpace(opts.RuleBasePath)
		if path == "" {
			path = defaultRuleBasePath()
		}
		opened, err := patchintel.OpenRuleBase(path)
		if err != nil {
			cleanupStore()
			return nil, fmt.Errorf("dispatch: open rule base: %w", err)
		}
		ruleBase = opened
		ownsRuleBase = true
	}
	cleanupStores := func() {
		if ownsRuleBase {
			_ = ruleBase.Close()
		}
		cleanupStore()
	}

	if err := ctx.Err(); err != nil {
		cleanupStores()
		return nil, fmt.Errorf("dispatch: start lane: %w", err)
	}

	client := graphserver.NewLocalClient(store)

	drivers := append([]worker.Driver(nil), opts.Drivers...)
	if len(drivers) == 0 {
		if opts.Complete == nil {
			cleanupStores()
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

	rt := opts.Runtime
	if rt == nil {
		rt = runtime.NewDockerSandboxRuntime(dockerManager.NewDockerManager().ExecCapture, NewTaskManagerResolver())
	}
	d := New(opts.Config, client, drivers)
	executor := NewExecutor(opts.Config, client, d.registry, rt)
	executor.SetExploreOptions(tasks.ExploreOptions{
		OnVerdict: func(ctx context.Context, projectID, verdictFactID string) error {
			return applyVerdictOutcome(ctx, store, ruleBase, projectID, verdictFactID)
		},
	})
	// Bootstrap must search and record hits in the same rule base CE4 writes
	// into, otherwise injected rules and grown rules live in different SQLite
	// files (R11).
	executor.SetBootstrapOptions(tasks.BootstrapOptions{
		SearchRules: func(query string) []patchintel.Rule {
			rules, err := ruleBase.Search(query, tasks.BootstrapRuleTopK)
			if err != nil {
				misc.Warn("dispatch", fmt.Sprintf("lane: search rule base: %v", err), nil)
				return nil
			}
			return rules
		},
		RecordHits: ruleBase.RecordHits,
	})
	d.SetExecutor(executor)
	d.store = store
	d.ownsStore = ownsStore
	d.ruleBase = ruleBase
	d.ownsRuleBase = ownsRuleBase
	return d, nil
}

// applyVerdictOutcome turns one written verdict fact into CE4 rule feedback.
// AddVerdict links the verdict to its target with a support or refute edge, so
// the edge kind is the authoritative confirmation signal.
func applyVerdictOutcome(ctx context.Context, store *graph.Store, rb *patchintel.RuleBase, projectID, verdictID string) error {
	confirmed, err := verdictConfirmed(ctx, store, projectID, verdictID)
	if err != nil {
		return err
	}
	return tasks.ApplyOutcome(ctx, rb, store, projectID, verdictID, confirmed, "")
}

// verdictConfirmed reads the verdict's support/refute edge. A verdict fact
// without either edge is invalid: CE4 must not guess.
func verdictConfirmed(ctx context.Context, store *graph.Store, projectID, verdictID string) (bool, error) {
	edges, err := store.Edges(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("dispatch: read verdict edges: %w", err)
	}
	for _, e := range edges {
		if e.From != verdictID {
			continue
		}
		switch e.Kind {
		case "support":
			return true, nil
		case "refute":
			return false, nil
		}
	}
	return false, fmt.Errorf("%w: verdict %s has no support/refute edge", graph.ErrInvalid, verdictID)
}

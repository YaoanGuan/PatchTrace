package dispatch

import (
	"context"
	"fmt"
	"strings"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"

	yaml "github.com/goccy/go-yaml"
)

// Reason trigger values recorded on the project reason lease. They describe
// which "new situation" rule fired; the set is intentionally small.
const (
	triggerInitial     = "initial"
	triggerNewFacts    = "new_facts"
	triggerNewHints    = "new_hints"
	triggerOpenDrained = "open_intents_drained"
)

// reservedBootstrapIntentContent is the fixed description of the single
// bootstrap intent the dispatcher may reserve on an initial project.
const reservedBootstrapIntentContent = "bootstrap"

// reservedBootstrapIntentCreator is the fixed creator of the reserved bootstrap
// intent. The pair (content, creator, from=[origin]) identifies it.
const reservedBootstrapIntentCreator = "dispatcher.bootstrap"

// Decision is the planner's verdict for one project. WorkerName is filled by
// the dispatcher loop after worker selection; PlanProject only decides the
// task type, intent and trigger.
type Decision struct {
	ProjectID  string
	TaskType   worker.TaskType
	IntentID   string
	WorkerName string
	Trigger    string
}

// PlanProject decides what the project needs next. It is a pure read of the
// graph and dispatcher-local state: it never claims a lease or writes the
// protocol. ok is false when no task is currently warranted.
func (d *Dispatcher) PlanProject(ctx context.Context, p graph.Project) (Decision, bool, error) {
	dec, _, ok, err := d.planProject(ctx, p)
	return dec, ok, err
}

// planProject is the internal form that also returns the parsed graph counts so
// the loop can record the reason snapshot without exporting the graph twice.
func (d *Dispatcher) planProject(ctx context.Context, p graph.Project) (Decision, graphStats, bool, error) {
	if p.Status != "" && p.Status != "active" {
		return Decision{}, graphStats{}, false, nil
	}

	export, err := d.client.ExportYAML(ctx, p.ID)
	if err != nil {
		return Decision{}, graphStats{}, false, fmt.Errorf("dispatch: plan project %s: %w", p.ID, err)
	}
	stats, err := parseGraphStats(export)
	if err != nil {
		return Decision{}, graphStats{}, false, fmt.Errorf("dispatch: plan project %s: %w", p.ID, err)
	}

	// A claimed reserved bootstrap intent means a bootstrap task is already
	// running for this project; it owns the project until it concludes.
	if stats.ReservedClaimed {
		return Decision{}, stats, false, nil
	}

	// 1. Initial state: only the origin/goal facts and no ordinary intents.
	// An existing open reserved bootstrap intent is reused.
	if p.BootstrapEnabled && stats.Initial && stats.OrdinaryIntents == 0 {
		return Decision{
			ProjectID: p.ID,
			TaskType:  worker.TaskBootstrap,
			IntentID:  stats.ReservedOpenIntentID,
		}, stats, true, nil
	}

	// 2. New situation: reason runs only when no intent is claimable and the
	// project-level reason lease is free.
	if stats.OpenIntents == 0 && !reasonLeaseHeld(p) {
		prev, seen := d.lastReason[p.ID]
		if trigger, ok := newSituationTrigger(prev, stats, seen); ok {
			return Decision{
				ProjectID: p.ID,
				TaskType:  worker.TaskReason,
				Trigger:   trigger,
			}, stats, true, nil
		}
	}

	// 3. Explore a claimable (open) ordinary intent.
	if stats.OpenIntentID != "" {
		return Decision{
			ProjectID: p.ID,
			TaskType:  worker.TaskExplore,
			IntentID:  stats.OpenIntentID,
		}, stats, true, nil
	}

	return Decision{}, stats, false, nil
}

// graphStats is the small projection of the YAML export used by the decision
// rules. Keeping it behind parseGraphStats lets a future protocol Stats method
// replace the YAML round-trip without touching the rules.
type graphStats struct {
	FactCount int
	HintCount int

	// OpenIntents counts claimable ordinary intents (state=open). Reserved
	// bootstrap intents are excluded: they are handled by the bootstrap task.
	OpenIntents int
	// OpenIntentID is the first open ordinary intent, in export order.
	OpenIntentID string

	// OrdinaryIntents counts every non-reserved intent regardless of state.
	OrdinaryIntents int
	// Initial is true when the only facts are origin and goal.
	Initial bool
	// ReservedOpenIntentID is the open reserved bootstrap intent, if any.
	ReservedOpenIntentID string
	// ReservedClaimed is true when the reserved bootstrap intent is claimed,
	// which means a bootstrap task is already running for this project.
	ReservedClaimed bool
}

type exportSnapshot struct {
	Facts   []exportFact   `yaml:"facts"`
	Hints   []struct{}     `yaml:"hints"`
	Intents []exportIntent `yaml:"intents"`
}

type exportFact struct {
	ID   string `yaml:"id"`
	Type string `yaml:"type"`
}

type exportIntent struct {
	ID      string   `yaml:"id"`
	Content string   `yaml:"content"`
	From    []string `yaml:"from"`
	State   string   `yaml:"state"`
	Creator string   `yaml:"creator"`
}

// parseGraphStats derives the decision counts from the fixed
// project/hints/facts/intents YAML export produced by graph.ExportYAML.
func parseGraphStats(export string) (graphStats, error) {
	var snapshot exportSnapshot
	if err := yaml.Unmarshal([]byte(export), &snapshot); err != nil {
		return graphStats{}, fmt.Errorf("parse graph export: %w", err)
	}

	stats := graphStats{
		FactCount: len(snapshot.Facts),
		HintCount: len(snapshot.Hints),
	}
	// The store seeds origin/goal with both those ids and those fact types.
	// Accept either marker so a minimal export fixture without types still
	// identifies the initial state.
	factIDs := make(map[string]struct{}, len(snapshot.Facts))
	factTypes := make(map[string]struct{}, len(snapshot.Facts))
	for _, f := range snapshot.Facts {
		factIDs[strings.TrimSpace(f.ID)] = struct{}{}
		factTypes[strings.TrimSpace(f.Type)] = struct{}{}
	}
	if len(snapshot.Facts) == 2 {
		stats.Initial = hasBoth(factIDs, "origin", "goal") || hasBoth(factTypes, "origin", "goal")
	}

	for _, in := range snapshot.Intents {
		if isReservedBootstrapIntent(in) {
			switch in.State {
			case "open":
				if stats.ReservedOpenIntentID == "" {
					stats.ReservedOpenIntentID = in.ID
				}
			case "claimed":
				stats.ReservedClaimed = true
			}
			continue
		}
		stats.OrdinaryIntents++
		if in.State == "open" {
			stats.OpenIntents++
			if stats.OpenIntentID == "" {
				stats.OpenIntentID = in.ID
			}
		}
	}
	return stats, nil
}

func hasBoth(set map[string]struct{}, a, b string) bool {
	_, hasA := set[a]
	_, hasB := set[b]
	return hasA && hasB
}

// isReservedBootstrapIntent reports whether the intent matches the fixed
// dispatcher.bootstrap convention (content, creator, from=[origin]).
func isReservedBootstrapIntent(in exportIntent) bool {
	if strings.TrimSpace(in.Content) != reservedBootstrapIntentContent {
		return false
	}
	if strings.TrimSpace(in.Creator) != reservedBootstrapIntentCreator {
		return false
	}
	return len(in.From) == 1 && strings.TrimSpace(in.From[0]) == "origin"
}

// reasonLeaseHeld reports whether another worker currently owns the project
// reason lease. Expired leases are reclaimed by the graph server on every read
// path (ListProjects/GetProject), so a non-nil Reason already means "live".
func reasonLeaseHeld(p graph.Project) bool {
	return p.Reason != nil
}

// newSituationTrigger applies the "new situation" definition. seen is false
// the first time the planner observes a project.
func newSituationTrigger(prev lastReason, cur graphStats, seen bool) (string, bool) {
	if !seen {
		return triggerInitial, true
	}
	switch {
	case cur.FactCount > prev.FactCount:
		return triggerNewFacts, true
	case cur.HintCount > prev.HintCount:
		return triggerNewHints, true
	case prev.HadOpen && cur.OpenIntents == 0:
		return triggerOpenDrained, true
	default:
		return "", false
	}
}

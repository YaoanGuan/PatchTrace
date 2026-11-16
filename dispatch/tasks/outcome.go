package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"AIxVuln/graph"
	"AIxVuln/patchintel"
)

// CE4 rule feedback defaults.
const (
	// outcomeCreator marks rule_hit facts written by this feedback path.
	outcomeCreator  = "dispatcher.outcome"
	outcomeFactType = "rule_hit"

	// New patterns are promoted with a confidence derived from the rule base's
	// default 0.5, adjusted by the same deltas UpdateOutcome applies (+0.1 for a
	// confirmation, -0.15 for a refutation). A confirmation is also carried into
	// the new row's confirm_count (1), while a refutation starts at 0; hit_count
	// always starts at 0 because promotion is not a prompt hit.
	outcomeInitialConfidence   = 0.5
	outcomeConfirmedConfidence = outcomeInitialConfidence + 0.1
	outcomeFailedConfidence    = outcomeInitialConfidence - 0.15

	// evidenceRefMarker is the suffix AddVerdict appends to a verdict's
	// description. It is stripped before deriving a pattern from the fact.
	evidenceRefMarker = "\n证据引用："
)

// outcomePatternMarkerRE extracts an explicit pattern key from a note:
// `pattern: ...` or `rule: ...` (case-insensitive, ASCII or full-width colon).
var outcomePatternMarkerRE = regexp.MustCompile(`(?i)(?:^|[\s,;])(?:pattern|rule)\s*[:：]\s*([^\n]+)`)

// ApplyOutcome feeds an explore verdict back into the PatchOracle rule base and
// records the feedback as a `rule_hit` fact in the provenance graph (CE4, R11).
//
// Rule resolution is deterministic and runs in this order:
//
//  1. Idea link. The verdict's judged target is found through the support/
//     refute edge AddVerdict wrote. Candidates are the target itself, then (for
//     an intent) its from facts nearest-first, or (for a fact) the target's
//     provenance chain nearest-first. The first candidate linked through
//     RuleBase's idea links wins and is updated with RecordIdeaOutcome.
//  2. Pattern. The pattern key is taken from a `pattern:`/`rule:` marker in
//     note when present; otherwise it is the verdict description with the
//     evidence-reference suffix removed and all whitespace collapsed to single
//     spaces. FindLatestByPattern resolves an exact match, which is updated with
//     UpdateOutcome.
//  3. New pattern. When no rule matches, Promote creates a version-1 rule with
//     that pattern (confidence 0.6 and confirm_count 1 for a confirmation; 0.35
//     and confirm_count 0 for a refutation, mirroring UpdateOutcome's deltas).
//
// A confirmed outcome raises confidence, increments confirm_count and opens a
// new version while the old row stays. A failed outcome lowers the confidence
// of the matching row in place without a version bump.
//
// The feedback is then published as a `rule_hit` fact whose from fact is the
// verdict. There is no cross-store transaction between SQLite files: the rule
// base is written first and the graph fact second, so if the graph write fails
// the rule-base update has already landed. In that case the returned error joins
// the graph failure with a note that the rule changed, making the divergence
// visible instead of silently dropping the feedback.
//
// Validation happens before any write. A nil rule base or store, an empty
// project or verdict id, or a node that is not a fact of type verdict yields a
// graph.ErrInvalid error and leaves both stores untouched.
func ApplyOutcome(ctx context.Context, rb *patchintel.RuleBase, s *graph.Store, projectID, verdictID string, confirmed bool, note string) error {
	projectID = strings.TrimSpace(projectID)
	verdictID = strings.TrimSpace(verdictID)
	if rb == nil || s == nil || projectID == "" || verdictID == "" {
		return fmt.Errorf("%w: rule base, graph store, project and verdict are required", graph.ErrInvalid)
	}

	verdict, err := s.GetNode(ctx, projectID, verdictID)
	if err != nil {
		if errors.Is(err, graph.ErrNotFound) {
			return fmt.Errorf("%w: verdict %s not found", graph.ErrInvalid, verdictID)
		}
		return err
	}
	if verdict.Kind != "fact" || verdict.Type != "verdict" {
		return fmt.Errorf("%w: node %s is not a verdict fact", graph.ErrInvalid, verdictID)
	}

	rule, err := resolveOutcomeRule(ctx, rb, s, projectID, verdict, confirmed, note)
	if err != nil {
		return err
	}

	content, err := marshalRuleHit(rule, confirmed)
	if err != nil {
		return err
	}
	if _, err := s.AddFact(ctx, projectID, graph.Node{
		Type:    outcomeFactType,
		Content: content,
		Creator: outcomeCreator,
		From:    []string{verdict.ID},
	}); err != nil {
		return errors.Join(
			fmt.Errorf("outcome: rule %d was updated to version %d but the rule_hit fact write failed", rule.ID, rule.Version),
			err,
		)
	}
	return nil
}

// resolveOutcomeRule applies the resolution order documented on ApplyOutcome
// and returns the resulting (newest) rule row.
func resolveOutcomeRule(ctx context.Context, rb *patchintel.RuleBase, s *graph.Store, projectID string, verdict graph.Node, confirmed bool, note string) (*patchintel.Rule, error) {
	linked, err := outcomeFromLinkedIdea(ctx, rb, s, projectID, verdict.ID, confirmed, note)
	if err != nil {
		return nil, err
	}
	if linked != nil {
		return linked, nil
	}

	pattern := outcomePattern(note, verdict.Content)
	existing, err := rb.FindLatestByPattern(pattern)
	if err != nil {
		return nil, fmt.Errorf("outcome: find rule by pattern: %w", err)
	}
	if existing != nil {
		updated, err := rb.UpdateOutcome(existing.ID, confirmed, note)
		if err != nil {
			return nil, fmt.Errorf("outcome: update rule %d: %w", existing.ID, err)
		}
		return updated, nil
	}

	confidence := outcomeConfirmedConfidence
	confirmCount := 1
	if !confirmed {
		confidence = outcomeFailedConfidence
		confirmCount = 0
	}
	promoted, err := rb.Promote(patchintel.Rule{
		Pattern:           pattern,
		VerificationNotes: note,
		Confidence:        confidence,
		ConfirmCount:      confirmCount,
	})
	if err != nil {
		return nil, fmt.Errorf("outcome: promote rule: %w", err)
	}
	return promoted, nil
}

// outcomeFromLinkedIdea tries the verdict's provenance candidates as PatchOracle
// idea ids in nearest-first order. RecordIdeaOutcome is itself the link lookup:
// an unlinked candidate is a no-op, so the first candidate that returns a rule
// is the attributed idea.
func outcomeFromLinkedIdea(ctx context.Context, rb *patchintel.RuleBase, s *graph.Store, projectID, verdictID string, confirmed bool, note string) (*patchintel.Rule, error) {
	candidates, err := outcomeIdeaCandidates(ctx, s, projectID, verdictID)
	if err != nil {
		return nil, err
	}
	for _, ideaID := range candidates {
		rule, err := rb.RecordIdeaOutcome(ideaID, confirmed, note)
		if err != nil {
			return nil, fmt.Errorf("outcome: record idea %s outcome: %w", ideaID, err)
		}
		if rule != nil {
			return rule, nil
		}
	}
	return nil, nil
}

// outcomeIdeaCandidates lists the ids whose idea links may explain the verdict,
// nearest-first: the judged target, then the intent's from facts (newest
// declaration first) or the target fact's provenance chain (closest to the
// target first). Duplicate ids are dropped.
func outcomeIdeaCandidates(ctx context.Context, s *graph.Store, projectID, verdictID string) ([]string, error) {
	edges, err := s.Edges(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("outcome: read graph edges: %w", err)
	}
	var targets []string
	for _, e := range edges {
		if e.From == verdictID && (e.Kind == "support" || e.Kind == "refute") {
			targets = append(targets, e.To)
		}
	}
	sort.Strings(targets)

	seen := make(map[string]struct{}, len(targets))
	var candidates []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		candidates = append(candidates, id)
	}

	for _, targetID := range targets {
		add(targetID)
		target, err := s.GetNode(ctx, projectID, targetID)
		if err != nil {
			if errors.Is(err, graph.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("outcome: read verdict target %s: %w", targetID, err)
		}
		if target.Kind == "intent" {
			for i := len(target.From) - 1; i >= 0; i-- {
				add(target.From[i])
			}
			continue
		}
		chain, err := s.Provenance(ctx, projectID, targetID)
		if err != nil {
			if errors.Is(err, graph.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("outcome: read provenance for %s: %w", targetID, err)
		}
		for i := len(chain) - 1; i >= 0; i-- {
			add(chain[i].ID)
		}
	}
	return candidates, nil
}

// outcomePattern derives the stable rule key. An explicit `pattern:`/`rule:`
// marker in note wins; otherwise the verdict description is used with the
// evidence-reference suffix removed and whitespace collapsed so the same
// observation yields the same key.
func outcomePattern(note, verdictContent string) string {
	if match := outcomePatternMarkerRE.FindStringSubmatch(note); match != nil {
		if pattern := strings.TrimSpace(match[1]); pattern != "" {
			return pattern
		}
	}
	content := verdictContent
	if i := strings.Index(content, evidenceRefMarker); i >= 0 {
		content = content[:i]
	}
	return strings.Join(strings.Fields(content), " ")
}

// ruleHitContent is the JSON payload carried by a rule_hit fact.
type ruleHitContent struct {
	RuleID       int64   `json:"rule_id"`
	Version      int     `json:"version"`
	ConfirmCount int     `json:"confirm_count"`
	Confidence   float64 `json:"confidence"`
	Pattern      string  `json:"pattern"`
	Confirmed    bool    `json:"confirmed"`
}

func marshalRuleHit(rule *patchintel.Rule, confirmed bool) (string, error) {
	data, err := json.Marshal(ruleHitContent{
		RuleID:       rule.ID,
		Version:      rule.Version,
		ConfirmCount: rule.ConfirmCount,
		Confidence:   rule.Confidence,
		Pattern:      rule.Pattern,
		Confirmed:    confirmed,
	})
	if err != nil {
		return "", fmt.Errorf("outcome: encode rule_hit: %w", err)
	}
	return string(data), nil
}

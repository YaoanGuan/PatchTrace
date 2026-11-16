package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"AIxVuln/graph"
	"AIxVuln/patchintel"

	yaml "github.com/goccy/go-yaml"
)

// Reserved intent convention. It must match dispatch/planner.go so the
// planner and the runner agree on which intents are reserved. The creator marks
// both the bootstrap task and the exploit_idea fact it writes.
const (
	reservedBootstrapContent = "bootstrap"
	exploitIdeaContent       = "exploit_idea"
	reservedBootstrapCreator = "dispatcher.bootstrap"
	reservedOriginFact       = "origin"
	goalFactID               = "goal"
)

// Bootstrap runtime defaults. The runner signature carries no dispatcher
// config, so these mirror the graph protocol / native driver defaults.
const (
	bootstrapPromptDir   = "prompts/graph/default"
	bootstrapWorkerName  = "bootstrap"
	bootstrapIntentLease = 5 * time.Minute
	bootstrapRuleTopK    = 5
	maxPromptDiffRunes   = 20000
	patchProfileFactType = "patch_profile"
	exploitIdeaFactType  = "exploit_idea"
)

// searchDefaultRules reads the process-wide rule base. Callers treat it as
// best-effort: a cold or unavailable rule base must not block a bootstrap.
func searchDefaultRules(query string) []patchintel.Rule {
	base, err := patchintel.DefaultRuleBase()
	if err != nil {
		return nil
	}
	rules, err := base.Search(query, bootstrapRuleTopK)
	if err != nil {
		return nil
	}
	return rules
}

// promptGraphSnapshot is the minimal project snapshot rendered into the
// bootstrap prompt. It mirrors graph.ExportYAML for the initial state (only the
// origin and goal facts exist) and is marshalled locally so the task package
// never reaches into graph's internal export structs.
type promptGraphSnapshot struct {
	Project promptGraphProject `yaml:"project"`
	// Hints is deliberately opaque: the real export emits mapping nodes
	// (id/content/creator/...), not scalar strings. The runner only needs the
	// intents, but unmarshalling must not fail on a project with hints.
	Hints   []any               `yaml:"hints"`
	Facts   []promptGraphFact   `yaml:"facts"`
	Intents []promptGraphIntent `yaml:"intents"`
}

type promptGraphProject struct {
	Title            string `yaml:"title"`
	Origin           string `yaml:"origin"`
	Goal             string `yaml:"goal"`
	BootstrapEnabled bool   `yaml:"bootstrap_enabled"`
}

type promptGraphFact struct {
	ID      string `yaml:"id"`
	Type    string `yaml:"type"`
	Content string `yaml:"content"`
}

type promptGraphIntent struct {
	ID      string   `yaml:"id"`
	Type    string   `yaml:"type,omitempty"`
	Content string   `yaml:"content"`
	From    []string `yaml:"from"`
	State   string   `yaml:"state,omitempty"`
	Creator string   `yaml:"creator,omitempty"`
	To      *string  `yaml:"to,omitempty"`
}

// reservedBootstrapState is the projection of the graph export that the runner
// needs before claiming the reserved bootstrap intent.
type reservedBootstrapState struct {
	OpenID    string
	Claimed   bool
	Concluded bool
}

func findReservedBootstrapIntent(export string) (reservedBootstrapState, error) {
	var snapshot promptGraphSnapshot
	if strings.TrimSpace(export) != "" {
		if err := yaml.Unmarshal([]byte(export), &snapshot); err != nil {
			return reservedBootstrapState{}, fmt.Errorf("bootstrap: parse graph export: %w", err)
		}
	}
	var state reservedBootstrapState
	for _, intent := range snapshot.Intents {
		if !isReservedIntent(intent) || !isBootstrapIntent(intent) {
			continue
		}
		switch strings.TrimSpace(intent.State) {
		case "open":
			if state.OpenID == "" {
				state.OpenID = strings.TrimSpace(intent.ID)
			}
		case "claimed":
			state.Claimed = true
		default:
			if intent.To != nil {
				state.Concluded = true
			}
		}
	}
	return state, nil
}

// isReservedIntent reports whether the intent was written by the bootstrap
// runner. It is intentionally creator-only so a leftover exploit_idea intent is
// never mistaken for ordinary work.
func isReservedIntent(intent promptGraphIntent) bool {
	return strings.TrimSpace(intent.Creator) == reservedBootstrapCreator
}

// isBootstrapIntent narrows a reserved intent to the bootstrap task itself.
func isBootstrapIntent(intent promptGraphIntent) bool {
	return strings.TrimSpace(intent.Content) == reservedBootstrapContent
}

// loadPatchProfile resolves the project origin to a patch profile. A readable
// local diff file wins over an http(s) URL; anything else is a hard error so
// the caller can stop before touching the graph.
func loadPatchProfile(ctx context.Context, origin string) (*patchintel.PatchProfile, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return nil, errors.New("bootstrap: project origin is empty; want a local patch diff path or http(s) URL")
	}
	if info, err := os.Stat(origin); err == nil && info.Mode().IsRegular() {
		return patchintel.IngestLocal(origin)
	}
	if isHTTPURL(origin) {
		return patchintel.IngestURL(ctx, origin)
	}
	return nil, fmt.Errorf("bootstrap: origin %q is neither a readable local patch diff nor an http(s) URL", origin)
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func bootstrapRuleQuery(profile *patchintel.PatchProfile) string {
	if profile == nil {
		return ""
	}
	parts := []string{
		profile.CVE,
		profile.GHSA,
		profile.Scene,
		strings.Join(profile.Files, " "),
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// minimalGraphYAML renders the initial project snapshot. The prompt-visible
// graph starts with exactly the origin and goal facts.
func minimalGraphYAML(p graph.Project) string {
	snapshot := promptGraphSnapshot{
		Project: promptGraphProject{
			Title:            p.Title,
			Origin:           p.Origin,
			Goal:             p.Goal,
			BootstrapEnabled: p.BootstrapEnabled,
		},
		Hints: []any{},
		Facts: []promptGraphFact{
			{ID: reservedOriginFact, Type: reservedOriginFact, Content: p.Origin},
			{ID: goalFactID, Type: goalFactID, Content: p.Goal},
		},
		Intents: []promptGraphIntent{},
	}
	data, err := yaml.Marshal(snapshot)
	if err != nil {
		return fmt.Sprintf("project:\n  origin: %q\n  goal: %q\n", p.Origin, p.Goal)
	}
	return strings.TrimRight(string(data), "\n")
}

func bootstrapFactIDsJSON() string {
	data, err := json.Marshal([]string{reservedOriginFact, goalFactID})
	if err != nil {
		return "[]"
	}
	return string(data)
}

func formatPatchProfile(profile *patchintel.PatchProfile) string {
	if profile == nil {
		return "（无补丁档案）"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- CVE: %s\n", patchintel.MetadataOrUnknown(profile.CVE))
	fmt.Fprintf(&b, "- GHSA: %s\n", patchintel.MetadataOrUnknown(profile.GHSA))
	fmt.Fprintf(&b, "- 受影响版本: %s\n", patchintel.MetadataOrUnknown(profile.AffectedVersions))
	fmt.Fprintf(&b, "- 修复 commit: %s\n", patchintel.MetadataOrUnknown(profile.FixCommit))
	fmt.Fprintf(&b, "- 场景: %s\n", patchintel.MetadataOrUnknown(profile.Scene))
	fmt.Fprintf(&b, "- 涉及文件: %s\n", patchintel.MetadataOrUnknown(strings.Join(profile.Files, ", ")))
	fmt.Fprintf(&b, "- 来源: %s\n", patchintel.MetadataOrUnknown(profile.SourceURL))
	b.WriteString("\n### 补丁 diff\n\n```diff\n")
	b.WriteString(patchintel.TruncateRunes(profile.RawDiff, maxPromptDiffRunes))
	b.WriteString("\n```")
	return strings.TrimRight(b.String(), "\n")
}

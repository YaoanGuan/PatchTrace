package graph

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	yaml "github.com/goccy/go-yaml"
)

func declareOpenIntent(t *testing.T, s *Store, projectID string, from []string, typ, content string) Node {
	t.Helper()
	n, err := s.DeclareIntent(ctx, projectID, Node{
		Type:    typ,
		Content: content,
		Creator: "tester",
		From:    from,
	})
	if err != nil {
		t.Fatalf("DeclareIntent(%s): %v", content, err)
	}
	return n
}

func concludeNewFact(t *testing.T, s *Store, projectID string, from []string, factType, intentContent, conclusion string) Node {
	t.Helper()
	it := declareOpenIntent(t, s, projectID, from, "gap", intentContent)
	fact, concluded, err := s.ConcludeIntent(ctx, projectID, it.ID, "tester", conclusion, factType)
	if err != nil {
		t.Fatalf("ConcludeIntent(%s): %v", intentContent, err)
	}
	if concluded.To == nil || *concluded.To != fact.ID {
		t.Fatalf("concluded.To = %v, want %s", concluded.To, fact.ID)
	}
	return fact
}

func ids(nodes []Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

func assertOrder(t *testing.T, text string, needles ...string) {
	t.Helper()
	pos := -1
	for _, needle := range needles {
		i := strings.Index(text, needle)
		if i < 0 {
			t.Fatalf("missing %q in:\n%s", needle, text)
		}
		if i <= pos {
			t.Fatalf("%q out of order in:\n%s", needle, text)
		}
		pos = i
	}
}

func TestProvenanceReturnsPatchToFindingPath(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	f1 := concludeNewFact(t, s, p.ID, []string{"origin"}, "patch_profile", "profile the patch", "the patch adds a bounds check")
	f2 := concludeNewFact(t, s, p.ID, []string{f1.ID}, "exploit_idea", "invert the check", "the check is missing on one path")
	f3 := concludeNewFact(t, s, p.ID, []string{f2.ID}, "verdict", "verify the idea", "the sink is reachable")

	got, err := s.Provenance(ctx, p.ID, f3.ID)
	if err != nil {
		t.Fatalf("Provenance: %v", err)
	}
	want := []string{"origin", f1.ID, f2.ID, f3.ID}
	if !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("Provenance ids = %v, want %v", ids(got), want)
	}
	if got[len(got)-1].ID != f3.ID {
		t.Fatalf("Provenance ends at %q, want target %q", got[len(got)-1].ID, f3.ID)
	}
}

func TestProvenanceSkipsCycles(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	f1 := concludeNewFact(t, s, p.ID, []string{"origin"}, "patch_profile", "profile the patch", "patch profile")
	f2 := concludeNewFact(t, s, p.ID, []string{f1.ID}, "exploit_idea", "invert the check", "exploit idea")
	// Close a produce cycle: f2 -> f1 alongside the existing f1 -> f2 edge.
	if err := s.AddEdge(ctx, p.ID, f2.ID, f1.ID, "produce"); err != nil {
		t.Fatalf("AddEdge back edge: %v", err)
	}

	got, err := s.Provenance(ctx, p.ID, f2.ID)
	if err != nil {
		t.Fatalf("Provenance: %v", err)
	}
	want := []string{"origin", f1.ID, f2.ID}
	if !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("Provenance ids = %v, want %v (cycle must not repeat nodes)", ids(got), want)
	}
}

func TestProvenanceReturnsNotFoundWhenNoPath(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	hint, err := s.AddHint(ctx, p.ID, "an out-of-band observation", "analyst")
	if err != nil {
		t.Fatalf("AddHint: %v", err)
	}

	if _, err := s.Provenance(ctx, p.ID, hint.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Provenance(hint) err = %v, want ErrNotFound", err)
	}
	if _, err := s.Provenance(ctx, p.ID, "f999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Provenance(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestGapsExcludesBlockedIntents(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	i1 := declareOpenIntent(t, s, p.ID, []string{"origin"}, "gap", "first gap")
	i2 := declareOpenIntent(t, s, p.ID, []string{"origin"}, "gap", "second gap")
	if err := s.AddEdge(ctx, p.ID, i1.ID, i2.ID, "block"); err != nil {
		t.Fatalf("AddEdge block: %v", err)
	}

	gaps, err := s.Gaps(ctx, p.ID)
	if err != nil {
		t.Fatalf("Gaps: %v", err)
	}
	if want := []string{i1.ID}; !reflect.DeepEqual(ids(gaps), want) {
		t.Fatalf("Gaps ids = %v, want %v (blocked intent must be excluded)", ids(gaps), want)
	}

	if _, _, err := s.ConcludeIntent(ctx, p.ID, i1.ID, "tester", "first gap resolved", "observation"); err != nil {
		t.Fatalf("ConcludeIntent: %v", err)
	}
	gaps, err = s.Gaps(ctx, p.ID)
	if err != nil {
		t.Fatalf("Gaps after unblock: %v", err)
	}
	if want := []string{i2.ID}; !reflect.DeepEqual(ids(gaps), want) {
		t.Fatalf("Gaps after unblock ids = %v, want %v", ids(gaps), want)
	}
}

func TestGapsExcludesClaimedBlocker(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	// Declare A already claimed (state=claimed), then B blocked by A.
	blocker, err := s.DeclareIntent(ctx, p.ID, Node{
		Type:    "gap",
		Content: "blocker in flight",
		Creator: "tester",
		Worker:  "tester",
		From:    []string{"origin"},
	})
	if err != nil {
		t.Fatalf("DeclareIntent(blocker): %v", err)
	}
	if blocker.State != "claimed" {
		t.Fatalf("blocker state = %q, want claimed", blocker.State)
	}
	dependent := declareOpenIntent(t, s, p.ID, []string{"origin"}, "gap", "dependent gap")
	if err := s.AddEdge(ctx, p.ID, blocker.ID, dependent.ID, "block"); err != nil {
		t.Fatalf("AddEdge block: %v", err)
	}

	gaps, err := s.Gaps(ctx, p.ID)
	if err != nil {
		t.Fatalf("Gaps: %v", err)
	}
	if len(ids(gaps)) != 0 {
		t.Fatalf("Gaps ids = %v, want empty (claimed blocker is not settled)", ids(gaps))
	}

	if _, _, err := s.ConcludeIntent(ctx, p.ID, blocker.ID, "tester", "blocker resolved", "observation"); err != nil {
		t.Fatalf("ConcludeIntent(blocker): %v", err)
	}
	gaps, err = s.Gaps(ctx, p.ID)
	if err != nil {
		t.Fatalf("Gaps after resolve: %v", err)
	}
	if want := []string{dependent.ID}; !reflect.DeepEqual(ids(gaps), want) {
		t.Fatalf("Gaps after resolve ids = %v, want %v", ids(gaps), want)
	}
}

func TestGapsSortsByCreatedAt(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	i1 := declareOpenIntent(t, s, p.ID, []string{"origin"}, "gap", "first gap")
	i2 := declareOpenIntent(t, s, p.ID, []string{"origin"}, "gap", "second gap")

	gaps, err := s.Gaps(ctx, p.ID)
	if err != nil {
		t.Fatalf("Gaps: %v", err)
	}
	if want := []string{i1.ID, i2.ID}; !reflect.DeepEqual(ids(gaps), want) {
		t.Fatalf("Gaps ids = %v, want %v (created_at ascending)", ids(gaps), want)
	}
}

func TestExportYAMLMatchesProtocolShape(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	hint, err := s.AddHint(ctx, p.ID, "the sink may be reachable", "analyst")
	if err != nil {
		t.Fatalf("AddHint: %v", err)
	}
	fact := concludeNewFact(t, s, p.ID, []string{"origin"}, "patch_profile", "profile the patch", "the patch adds a bounds check")
	open := declareOpenIntent(t, s, p.ID, []string{fact.ID}, "gap", "is the sink still reachable")

	out, err := s.ExportYAML(ctx, p.ID)
	if err != nil {
		t.Fatalf("ExportYAML: %v", err)
	}

	assertOrder(t, out, "project:", "hints:", "facts:", "intents:")
	for _, banned := range []string{"reason", "lease", "last_heartbeat_at"} {
		if strings.Contains(out, banned) {
			t.Fatalf("export must not contain %q:\n%s", banned, out)
		}
	}

	var doc struct {
		Project struct {
			Title            string `yaml:"title"`
			Origin           string `yaml:"origin"`
			Goal             string `yaml:"goal"`
			BootstrapEnabled bool   `yaml:"bootstrap_enabled"`
		} `yaml:"project"`
		Hints []struct {
			ID        string `yaml:"id"`
			Content   string `yaml:"content"`
			Creator   string `yaml:"creator"`
			CreatedAt string `yaml:"created_at"`
		} `yaml:"hints"`
		Facts []struct {
			ID        string `yaml:"id"`
			Type      string `yaml:"type"`
			Content   string `yaml:"content"`
			CreatedAt string `yaml:"created_at"`
		} `yaml:"facts"`
		Intents []struct {
			ID          string   `yaml:"id"`
			From        []string `yaml:"from"`
			To          *string  `yaml:"to"`
			State       string   `yaml:"state"`
			Worker      string   `yaml:"worker"`
			CreatedAt   string   `yaml:"created_at"`
			ConcludedAt *string  `yaml:"concluded_at"`
		} `yaml:"intents"`
	}
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("export is not valid yaml: %v\n%s", err, out)
	}

	if doc.Project.Title != "test project" || doc.Project.Origin != "a patch was applied" || doc.Project.Goal != "the vulnerability is understood" {
		t.Fatalf("project section = %+v", doc.Project)
	}
	if !doc.Project.BootstrapEnabled {
		t.Fatal("project.bootstrap_enabled = false, want true")
	}

	if len(doc.Hints) != 1 || doc.Hints[0].ID != hint.ID || doc.Hints[0].Content != "the sink may be reachable" || doc.Hints[0].Creator != "analyst" {
		t.Fatalf("hints section = %+v", doc.Hints)
	}

	seenFacts := map[string]bool{}
	for _, f := range doc.Facts {
		seenFacts[f.ID] = true
	}
	for _, want := range []string{"origin", "goal", fact.ID} {
		if !seenFacts[want] {
			t.Fatalf("facts section missing %q: %+v", want, doc.Facts)
		}
	}
	if len(doc.Facts) != 3 {
		t.Fatalf("facts section has %d entries, want 3", len(doc.Facts))
	}

	if len(doc.Intents) != 2 {
		t.Fatalf("intents section has %d entries, want 2: %+v", len(doc.Intents), doc.Intents)
	}
	var foundOpen bool
	for _, it := range doc.Intents {
		if it.ID != open.ID {
			continue
		}
		foundOpen = true
		if it.To != nil {
			t.Fatalf("open intent to = %v, want null", *it.To)
		}
		if it.State != "open" || it.Worker != "" {
			t.Fatalf("open intent state/worker = %q/%q, want open/\"\"", it.State, it.Worker)
		}
		if it.ConcludedAt != nil {
			t.Fatalf("open intent concluded_at = %v, want null", *it.ConcludedAt)
		}
		if len(it.From) != 1 || it.From[0] != fact.ID {
			t.Fatalf("open intent from = %v, want [%s]", it.From, fact.ID)
		}
	}
	if !foundOpen {
		t.Fatalf("open intent %s missing from export: %+v", open.ID, doc.Intents)
	}

	timeRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)
	for _, f := range doc.Facts {
		if !timeRe.MatchString(f.CreatedAt) {
			t.Fatalf("fact %s created_at = %q, want YYYY-MM-DD HH:mm:ss", f.ID, f.CreatedAt)
		}
	}
	if !timeRe.MatchString(doc.Hints[0].CreatedAt) {
		t.Fatalf("hint created_at = %q, want YYYY-MM-DD HH:mm:ss", doc.Hints[0].CreatedAt)
	}
}

func TestExportYAMLUnknownProject(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ExportYAML(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ExportYAML err = %v, want ErrNotFound", err)
	}
}

func TestTimelineIsChronological(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	if _, err := s.AddHint(ctx, p.ID, "the sink may be reachable", "analyst"); err != nil {
		t.Fatalf("AddHint: %v", err)
	}
	concludeNewFact(t, s, p.ID, []string{"origin"}, "patch_profile", "profile the patch", "the patch adds a bounds check")

	out, err := s.ExportTimeline(ctx, p.ID)
	if err != nil {
		t.Fatalf("ExportTimeline: %v", err)
	}
	assertOrder(t, out, "PROJECT CREATED", "HINT", "INTENT DECLARED", "INTENT CONCLUDED")

	lineRe := regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \S`)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if !lineRe.MatchString(line) {
			t.Fatalf("timeline line %q lacks a YYYY-MM-DD HH:mm:ss timestamp", line)
		}
	}
}

func TestTimelineDropsCompletedAfterReopen(t *testing.T) {
	s := newTestStore(t)
	p := mustProject(t, s)

	if _, err := s.Complete(ctx, p.ID, []string{"origin"}, "goal reached", "w1"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	out, err := s.ExportTimeline(ctx, p.ID)
	if err != nil {
		t.Fatalf("ExportTimeline after complete: %v", err)
	}
	if !strings.Contains(out, "PROJECT COMPLETED") {
		t.Fatalf("timeline missing PROJECT COMPLETED:\n%s", out)
	}

	if _, err := s.Reopen(ctx, p.ID, "reviewer", "the goal was misjudged"); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	out, err = s.ExportTimeline(ctx, p.ID)
	if err != nil {
		t.Fatalf("ExportTimeline after reopen: %v", err)
	}
	if strings.Contains(out, "PROJECT COMPLETED") {
		t.Fatalf("timeline retains stale PROJECT COMPLETED after reopen:\n%s", out)
	}
}

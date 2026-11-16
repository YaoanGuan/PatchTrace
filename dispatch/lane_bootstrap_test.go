package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/patchintel"
)

// laneBootstrapDriver captures the bootstrap prompt and accepts the task.
type laneBootstrapDriver struct {
	prompt string
}

func (d *laneBootstrapDriver) Name() string                  { return "eval-driver" }
func (d *laneBootstrapDriver) Supports(worker.TaskType) bool { return true }
func (d *laneBootstrapDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *laneBootstrapDriver) SupportsConclude() bool { return true }
func (d *laneBootstrapDriver) Execute(_ context.Context, t worker.Task) (worker.Result, error) {
	d.prompt = t.Prompt
	return worker.Result{Accepted: true}, nil
}
func (d *laneBootstrapDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{Accepted: true}, nil
}

type laneNoopRuntime struct{}

func (laneNoopRuntime) Exec(context.Context, string, runtime.ExecRequest) (runtime.ExecResult, error) {
	return runtime.ExecResult{}, nil
}

// TestLaneBootstrapUsesLaneRuleBase closes I3: the lane's bootstrap must search
// the same RuleBase that CE4 writes into, record injection hits there, and make
// the injected rule visible to the worker prompt.
func TestLaneBootstrapUsesLaneRuleBase(t *testing.T) {
	dir := t.TempDir()
	promptDir := filepath.Join(dir, "prompts", "graph", "default")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "bootstrap.md"), []byte("# {graph_yaml}\n\n{fact_ids}\n"), 0o644); err != nil {
		t.Fatalf("write bootstrap prompt: %v", err)
	}
	patchPath := filepath.Join(dir, "patch.diff")
	const diff = "diff --git a/app.py b/app.py\n--- a/app.py\n+++ b/app.py\n@@ -1 +1 @@\n-x = eval(input())\n+x = int(input())\n"
	if err := os.WriteFile(patchPath, []byte(diff), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}
	t.Chdir(dir)

	ctx := context.Background()
	store, err := graph.Open(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	defer store.Close()
	ruleBase, err := patchintel.OpenRuleBase(filepath.Join(dir, "rules.db"))
	if err != nil {
		t.Fatalf("OpenRuleBase: %v", err)
	}
	defer ruleBase.Close()

	rule, err := ruleBase.Promote(patchintel.Rule{
		Pattern: "langflow validate code executes user input", Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	project, err := store.CreateProject(ctx, graph.Project{
		ID: "p1", Title: "lane rule base", Origin: patchPath,
		Goal: "确认补丁修复的漏洞可利用", BootstrapEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Pre-conclude the reserved bootstrap and exploit_idea intents so the
	// runner resumes instead of calling the LLM inversion (offline test).
	bootstrapIntent, err := store.DeclareIntent(ctx, project.ID, graph.Node{
		Content: "bootstrap", From: []string{"origin"}, Creator: "dispatcher.bootstrap",
	})
	if err != nil {
		t.Fatalf("DeclareIntent(bootstrap): %v", err)
	}
	if _, err := store.ClaimIntent(ctx, project.ID, bootstrapIntent.ID, "fixture", time.Minute); err != nil {
		t.Fatalf("ClaimIntent(bootstrap): %v", err)
	}
	patchFact, _, err := store.ConcludeIntent(ctx, project.ID, bootstrapIntent.ID, "fixture", "patch profile", "patch_profile")
	if err != nil {
		t.Fatalf("ConcludeIntent(bootstrap): %v", err)
	}
	ideaIntent, err := store.DeclareIntent(ctx, project.ID, graph.Node{
		Content: "exploit_idea", From: []string{patchFact.ID}, Creator: "dispatcher.bootstrap",
	})
	if err != nil {
		t.Fatalf("DeclareIntent(idea): %v", err)
	}
	if _, err := store.ClaimIntent(ctx, project.ID, ideaIntent.ID, "fixture", time.Minute); err != nil {
		t.Fatalf("ClaimIntent(idea): %v", err)
	}
	ideaFact, _, err := store.ConcludeIntent(ctx, project.ID, ideaIntent.ID, "fixture", "exploit idea", "exploit_idea")
	if err != nil {
		t.Fatalf("ConcludeIntent(idea): %v", err)
	}
	if err := ruleBase.LinkIdea(ideaFact.ID, rule.ID); err != nil {
		t.Fatalf("LinkIdea: %v", err)
	}

	driver := &laneBootstrapDriver{}
	lane, err := StartLane(ctx, LaneOptions{
		Config:   Config{PromptGroup: "default"},
		Store:    store,
		RuleBase: ruleBase,
		Drivers:  []worker.Driver{driver},
		Runtime:  laneNoopRuntime{},
	})
	if err != nil {
		t.Fatalf("StartLane: %v", err)
	}
	defer lane.Close()

	if lane.executor == nil {
		t.Fatal("StartLane did not install an executor")
	}
	if err := lane.executor.invoke(ctx, Decision{
		TaskType: worker.TaskBootstrap, WorkerName: driver.Name(), ProjectID: project.ID,
	}, project, driver); err != nil {
		t.Fatalf("bootstrap invoke: %v", err)
	}
	if !strings.Contains(driver.prompt, rule.Pattern) {
		t.Fatalf("bootstrap prompt does not contain the lane rule base rule %q:\n%s", rule.Pattern, driver.prompt)
	}
	hit, err := ruleBase.Get(rule.ID)
	if err != nil {
		t.Fatalf("Get rule: %v", err)
	}
	if hit.HitCount != 1 {
		t.Fatalf("rule hit_count = %d, want 1 (injection recorded in the lane rule base)", hit.HitCount)
	}

	// CE4 must write into the same base: the lane's OnVerdict hook uses the
	// same RuleBase the bootstrap search just hit.
	observation, err := store.AddFact(ctx, project.ID, graph.Node{
		Type: "observation", Content: "sandbox exit=0", Creator: "eval-driver", From: []string{"origin"},
	})
	if err != nil {
		t.Fatalf("AddFact(observation): %v", err)
	}
	verdict, err := store.AddVerdict(ctx, project.ID, ideaFact.ID, "support",
		rule.Pattern, observation.ID, "sandbox: exit=0")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}
	if lane.executor.exploreOpts.OnVerdict == nil {
		t.Fatal("StartLane did not install the CE4 OnVerdict hook")
	}
	if err := lane.executor.exploreOpts.OnVerdict(ctx, project.ID, verdict.ID); err != nil {
		t.Fatalf("OnVerdict: %v", err)
	}
	latest, err := ruleBase.FindLatestByPattern(rule.Pattern)
	if err != nil {
		t.Fatalf("FindLatestByPattern: %v", err)
	}
	if latest == nil || latest.Version != 2 || latest.ConfirmCount != 1 {
		t.Fatalf("CE4 did not grow the lane rule base: %+v", latest)
	}
}

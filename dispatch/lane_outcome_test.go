package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/patchintel"
)

const laneExploreDescription = "未认证 eval 入口复现了代码执行"

// laneExploreDriver is an offline worker that always returns an accepted
// explore result carrying a support verdict and a sandbox command.
type laneExploreDriver struct {
	name string
	raw  string
}

func (d *laneExploreDriver) Name() string                  { return d.name }
func (d *laneExploreDriver) Supports(worker.TaskType) bool { return true }
func (d *laneExploreDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *laneExploreDriver) Execute(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{
		Accepted:    true,
		Description: laneExploreDescription,
		Raw:         d.raw,
	}, nil
}
func (d *laneExploreDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{}, nil
}
func (d *laneExploreDriver) SupportsConclude() bool { return true }

// laneFakeRuntime records that the explore verification ran without touching
// Docker.
type laneFakeRuntime struct {
	calls  int
	result runtime.ExecResult
}

func (r *laneFakeRuntime) Exec(context.Context, string, runtime.ExecRequest) (runtime.ExecResult, error) {
	r.calls++
	return r.result, nil
}

// lanePromptWorkdir creates the shipped prompt layout in a temporary cwd. The
// explore runner loads prompts/graph/default/explore.md relative to it.
func lanePromptWorkdir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	promptDir := filepath.Join(dir, "prompts", "graph", "default")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	contract := "# 任务\n\n```yaml\n{graph_yaml}\n```\n\n意图编号：{intent_id}\n\n意图描述：{intent_description}\n"
	if err := os.WriteFile(filepath.Join(promptDir, "explore.md"), []byte(contract), 0o644); err != nil {
		t.Fatalf("write explore prompt: %v", err)
	}
	t.Chdir(dir)
}

// TestStartLaneExploreVerdictFeedsRuleBase is the offline CE4/R15 integration
// proof: an accepted explore verdict written through the real lane wiring must
// produce a graph rule_hit fact and grow the real SQLite rule base.
func TestStartLaneExploreVerdictFeedsRuleBase(t *testing.T) {
	lanePromptWorkdir(t)
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.db")
	rulePath := filepath.Join(dir, "rules.db")

	driver := &laneExploreDriver{
		name: "fake-explore",
		raw: `{"accepted":true,"data":{"description":"` + laneExploreDescription +
			`","verdict":"support","command":"echo pwn","workdir":"/tmp"}}`,
	}
	rt := &laneFakeRuntime{result: runtime.ExecResult{ExitCode: 0, Stdout: "pwn"}}

	d, err := StartLane(context.Background(), LaneOptions{
		Config:       laneTestConfig(),
		GraphDBPath:  graphPath,
		RuleBasePath: rulePath,
		Drivers:      []worker.Driver{driver},
		Runtime:      rt,
	})
	if err != nil {
		t.Fatalf("StartLane: %v", err)
	}
	store := d.Store()
	if store == nil {
		t.Fatal("StartLane did not expose its graph store")
	}
	client := graphserver.NewLocalClient(store)
	ctx := context.Background()
	project, err := client.CreateProject(ctx, graph.Project{
		ID: "p1", Title: "demo", Origin: "/tmp/patch.diff", Goal: "确认漏洞可利用", BootstrapEnabled: false,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	intent, err := client.DeclareIntent(ctx, project.ID, graph.Node{
		Type: "gap", Content: "验证未认证 eval 入口", Creator: "tester", From: []string{"origin"},
	})
	if err != nil {
		t.Fatalf("DeclareIntent: %v", err)
	}

	dec := Decision{ProjectID: project.ID, TaskType: worker.TaskExplore, IntentID: intent.ID, WorkerName: driver.Name()}
	if err := d.executor.invoke(ctx, dec, project, driver); err != nil {
		t.Fatalf("invoke explore: %v", err)
	}
	if rt.calls != 1 {
		t.Fatalf("sandbox runtime calls = %d, want 1", rt.calls)
	}

	nodes, err := store.Nodes(ctx, project.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	var verdictID string
	ruleHits := 0
	for _, n := range nodes {
		switch n.Type {
		case "verdict":
			verdictID = n.ID
		case "rule_hit":
			ruleHits++
		}
	}
	if verdictID == "" {
		t.Fatal("explore did not write a verdict fact")
	}
	if ruleHits != 1 {
		t.Fatalf("rule_hit facts = %d, want 1 (CE4 feedback must be wired)", ruleHits)
	}
	edges, err := store.Edges(ctx, project.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	support := false
	for _, e := range edges {
		if e.From == verdictID && e.Kind == "support" {
			support = true
		}
	}
	if !support {
		t.Fatalf("no support edge from verdict %s", verdictID)
	}

	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ruleBase, err := patchintel.OpenRuleBase(rulePath)
	if err != nil {
		t.Fatalf("open rule base: %v", err)
	}
	defer ruleBase.Close()
	rules, err := ruleBase.Search("", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("rule base gained no rule from the explore verdict")
	}
	rule := rules[0]
	if !strings.Contains(rule.Pattern, laneExploreDescription) {
		t.Fatalf("rule pattern = %q, want it to contain %q", rule.Pattern, laneExploreDescription)
	}
	if rule.ConfirmCount != 1 {
		t.Fatalf("rule confirm_count = %d, want 1 for a support verdict", rule.ConfirmCount)
	}
}

// TestStartLaneCloseLeavesCallerStoresOpen pins the ownership contract: a lane
// that did not open the store or rule base must not close them.
func TestStartLaneCloseLeavesCallerStoresOpen(t *testing.T) {
	dir := t.TempDir()
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

	d, err := StartLane(context.Background(), LaneOptions{
		Config:   laneTestConfig(),
		Store:    store,
		RuleBase: ruleBase,
		Drivers:  []worker.Driver{&laneExploreDriver{name: "fake-explore"}},
		Runtime:  &laneFakeRuntime{},
	})
	if err != nil {
		t.Fatalf("StartLane: %v", err)
	}
	if d.Store() != store {
		t.Fatal("StartLane did not expose the caller-supplied store")
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := store.ListProjects(context.Background()); err != nil {
		t.Fatalf("lane closed the caller-supplied graph store: %v", err)
	}
	if _, err := ruleBase.Search("", 5); err != nil {
		t.Fatalf("lane closed the caller-supplied rule base: %v", err)
	}
}

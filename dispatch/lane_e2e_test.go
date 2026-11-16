package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/tasks"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/patchintel"
)

// e2eScriptDriver runs the bootstrap -> reason -> explore -> reason(complete)
// sequence against the real store and runners with no Docker or LLM.
type e2eScriptDriver struct {
	name    string
	execute func(context.Context, worker.Task) (worker.Result, error)
}

func (d *e2eScriptDriver) Name() string                  { return d.name }
func (d *e2eScriptDriver) Supports(worker.TaskType) bool { return true }
func (d *e2eScriptDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *e2eScriptDriver) SupportsConclude() bool { return true }
func (d *e2eScriptDriver) Execute(ctx context.Context, t worker.Task) (worker.Result, error) {
	return d.execute(ctx, t)
}
func (d *e2eScriptDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{Accepted: true, Description: "conclude fallback"}, nil
}

func e2eStringPtr(s string) *string { return &s }

// TestLaneOfflineEndToEnd is the G2/R10 regression guard: bootstrap, explore
// and reason run through the real dispatcher/executor/store, the sandbox
// evidence verdict completes the project with a provenance path to origin, and
// CE4 grows the rule base. No verdict may exist without an observation source.
func TestLaneOfflineEndToEnd(t *testing.T) {
	dir := t.TempDir()
	promptDir := filepath.Join(dir, "prompts", "graph", "default")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	prompts := map[string]string{
		"bootstrap.md": "# {graph_yaml}\n\n{fact_ids}\n",
		"reason.md":    "# {graph_yaml}\n\n{fact_ids}\n{open_intents}\n",
		"explore.md":   "# {graph_yaml}\n\n{intent_id}\n{intent_description}\n",
	}
	for name, body := range prompts {
		if err := os.WriteFile(filepath.Join(promptDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
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
	client := graphserver.NewLocalClient(store)

	project, err := store.CreateProject(ctx, graph.Project{
		ID: "p-e2e", Title: "offline e2e", Origin: patchPath,
		Goal: "确认补丁修复的漏洞可利用", BootstrapEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	driver := &e2eScriptDriver{name: "e2e-driver"}
	driver.execute = func(ctx context.Context, task worker.Task) (worker.Result, error) {
		switch worker.TaskType(task.Type) {
		case worker.TaskBootstrap:
			return worker.Result{
				Accepted: true,
				Fact:     e2eStringPtr("补丁档案：eval 改为 int"),
				Intents:  []worker.IntentSpec{{Description: "验证输入能否到达 eval 入口"}},
			}, nil
		case worker.TaskExplore:
			return worker.Result{
				Accepted:    true,
				Description: "沙箱确认输入可到达 eval 入口",
				Raw: `{"accepted":true,"data":{"description":"沙箱确认输入可到达 eval 入口",` +
					`"command":"true","verdict":"support","evidence_ref":"file:///tmp/e2e.log"}}`,
			}, nil
		case worker.TaskReason:
			nodes, err := store.Nodes(ctx, project.ID)
			if err != nil {
				return worker.Result{}, err
			}
			for _, n := range nodes {
				if n.Kind == "fact" && n.Type == "verdict" {
					return worker.Result{
						Accepted:     true,
						Complete:     e2eStringPtr("沙箱证据确认漏洞可利用"),
						CompleteFrom: []string{n.ID},
					}, nil
				}
			}
			return worker.Result{Accepted: true}, nil
		default:
			return worker.Result{Accepted: true}, nil
		}
	}

	cfg := Config{
		MaxWorkers: 4, MaxRunningProjects: 4, MaxProjectWorkers: 4,
		Interval:               time.Hour,
		BootstrapTimeout:       time.Minute,
		ReasonTimeout:          time.Minute,
		ExploreTimeout:         time.Minute,
		ExploreConcludeTimeout: time.Minute,
	}
	dispatcher := New(cfg, client, []worker.Driver{driver})
	executor := NewExecutor(cfg, client, dispatcher.registry, e2eRuntime{})
	executor.SetBootstrapOptions(tasks.BootstrapOptions{
		SearchRules: func(string) []patchintel.Rule { return nil },
		Invert: func(_ context.Context, profile *patchintel.PatchProfile, _ []patchintel.Rule, _ string) (*patchintel.InversionOutput, error) {
			idea, _, err := patchintel.BuildIdeaFromResponse(`{
				"harm": "攻击者可执行任意代码",
				"condition": "目标暴露 eval 入口",
				"vuln_type": "RCE",
				"exploit_point": {"title": "eval 未授权代码执行", "type": "RCE", "confidence": "high"}
			}`, profile)
			if err != nil {
				return nil, err
			}
			return &patchintel.InversionOutput{Idea: idea}, nil
		},
	})
	executor.SetExploreOptions(tasks.ExploreOptions{
		OnVerdict: func(ctx context.Context, projectID, verdictFactID string) error {
			return applyVerdictOutcome(ctx, store, ruleBase, projectID, verdictFactID)
		},
	})
	dispatcher.SetExecutor(executor)

	wantTypes := []worker.TaskType{worker.TaskBootstrap, worker.TaskExplore, worker.TaskReason}
	for i, want := range wantTypes {
		decisions, err := dispatcher.tick(ctx)
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if len(decisions) != 1 || decisions[0].TaskType != want {
			t.Fatalf("tick %d decisions = %+v, want one %s", i, decisions, want)
		}
		executor.Wait()
	}

	completed, err := store.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("project status = %q, want completed", completed.Status)
	}

	nodes, err := store.Nodes(ctx, project.ID)
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}
	edges, err := store.Edges(ctx, project.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	var verdicts []graph.Node
	for _, n := range nodes {
		if n.Kind == "fact" && n.Type == "verdict" {
			verdicts = append(verdicts, n)
		}
	}
	if len(verdicts) != 1 {
		t.Fatalf("verdict facts = %d, want 1", len(verdicts))
	}
	verdict := verdicts[0]
	provenance, err := store.Provenance(ctx, project.ID, verdict.ID)
	if err != nil {
		t.Fatalf("Provenance(verdict): %v", err)
	}
	if len(provenance) < 3 || provenance[0].ID != "origin" || provenance[len(provenance)-1].ID != verdict.ID {
		t.Fatalf("provenance = %+v, want origin -> ... -> verdict %s", provenance, verdict.ID)
	}
	observationSource := false
	for _, edge := range edges {
		if edge.Kind == "produce" && edge.To == verdict.ID && edge.From == provenance[len(provenance)-2].ID {
			observationSource = true
		}
	}
	if !observationSource {
		t.Fatalf("verdict %s has no observation produce edge", verdict.ID)
	}

	rules, err := ruleBase.LatestPatterns("", 0)
	if err != nil {
		t.Fatalf("LatestPatterns: %v", err)
	}
	if len(rules) != 1 || rules[0].ConfirmCount != 1 {
		t.Fatalf("CE4 rules = %+v, want one confirmed rule", rules)
	}
	if !strings.Contains(rules[0].Pattern, "沙箱确认输入可到达 eval 入口") {
		t.Fatalf("CE4 rule pattern = %q, want the verdict description", rules[0].Pattern)
	}
}

type e2eRuntime struct{}

func (e2eRuntime) Exec(context.Context, string, runtime.ExecRequest) (runtime.ExecResult, error) {
	return runtime.ExecResult{Stdout: "ok", ExitCode: 0}, nil
}

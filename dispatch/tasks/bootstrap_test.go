package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/patchintel"
)

// --- fakes -----------------------------------------------------------------

// fakeGraphClient embeds graphserver.Client so that any protocol call the task
// is not expected to make panics loudly. Only the methods bootstrap needs are
// implemented, and each records what it was asked to write.
type fakeGraphClient struct {
	graphserver.Client

	export    string
	exportErr error

	declared  []graph.Node
	claimed   []string
	released  []string
	concluded []concludeCall
}

type concludeCall struct {
	IntentID    string
	Worker      string
	Description string
	FactType    string
}

func (c *fakeGraphClient) ExportYAML(context.Context, string) (string, error) {
	if c.exportErr != nil {
		return "", c.exportErr
	}
	return c.export, nil
}

func (c *fakeGraphClient) DeclareIntent(_ context.Context, _ string, n graph.Node) (graph.Node, error) {
	c.declared = append(c.declared, n)
	return graph.Node{
		ID:      fmt.Sprintf("i%d", len(c.declared)),
		Kind:    "intent",
		Content: n.Content,
		From:    n.From,
		Creator: n.Creator,
		State:   "open",
	}, nil
}

func (c *fakeGraphClient) ClaimIntent(_ context.Context, _, intentID, workerName string, _ time.Duration) (graph.Node, error) {
	c.claimed = append(c.claimed, intentID)
	return graph.Node{ID: intentID, Kind: "intent", State: "claimed", Worker: workerName}, nil
}

func (c *fakeGraphClient) ReleaseIntent(_ context.Context, _, intentID, workerName string) (graph.Node, error) {
	c.released = append(c.released, intentID)
	return graph.Node{ID: intentID, Kind: "intent", State: "open"}, nil
}

func (c *fakeGraphClient) ConcludeIntent(_ context.Context, _, intentID, workerName, description, factType string) (graph.Node, graph.Node, error) {
	c.concluded = append(c.concluded, concludeCall{
		IntentID:    intentID,
		Worker:      workerName,
		Description: description,
		FactType:    factType,
	})
	fact := graph.Node{ID: "f001", Kind: "fact", Type: factType, Content: description, State: "resolved"}
	intent := graph.Node{ID: intentID, Kind: "intent", State: "resolved"}
	return fact, intent, nil
}

type fakeDriver struct {
	name   string
	result worker.Result
	err    error
	tasks  []worker.Task
}

func (d *fakeDriver) Name() string                  { return d.name }
func (d *fakeDriver) Supports(worker.TaskType) bool { return true }
func (d *fakeDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *fakeDriver) Execute(_ context.Context, t worker.Task) (worker.Result, error) {
	d.tasks = append(d.tasks, t)
	return d.result, d.err
}
func (d *fakeDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{}, errors.New("bootstrap must not call driver Conclude")
}
func (d *fakeDriver) SupportsConclude() bool { return false }

// --- fixtures --------------------------------------------------------------

const bootstrapSampleDiff = `diff --git a/app.py b/app.py
--- a/app.py
+++ b/app.py
@@ -1,2 +1,2 @@
-x = eval(input())
+x = int(input())
`

const bootstrapInitialExport = "facts:\n- id: origin\n- id: goal\nintents: []\n"

// bootstrapWorkdir creates a temporary cwd that contains the shipped prompt
// directory layout and changes into it for the duration of the test.
func bootstrapWorkdir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	promptDir := filepath.Join(dir, "prompts", "graph", "default")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	contract := "# 任务\n\n{graph_yaml}\n\n{fact_ids}\n"
	if err := os.WriteFile(filepath.Join(promptDir, "bootstrap.md"), []byte(contract), 0o644); err != nil {
		t.Fatalf("write bootstrap prompt: %v", err)
	}
	t.Chdir(dir)
	return dir
}

func bootstrapPatchFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "patch.diff")
	if err := os.WriteFile(path, []byte(bootstrapSampleDiff), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}
	return path
}

// stubBootstrapDeps removes the rule base and scene template lookups so the
// tests stay offline and never initialise the process-wide config database.
// The prompt loader still runs so the placeholder contract is exercised.
func stubBootstrapDeps() bootstrapDeps {
	return bootstrapDeps{
		searchRules: func(string) []patchintel.Rule { return nil },
		loadScene:   func(string) (string, string) { return "SCENE-TEMPLATE", "test" },
		loadPrompt:  worker.LoadPrompt,
	}
}

func bootstrapProject(patchPath string) graph.Project {
	return graph.Project{
		ID:               "p1",
		Title:            "demo",
		Origin:           patchPath,
		Goal:             "确认补丁修复的漏洞可利用",
		BootstrapEnabled: true,
	}
}

// --- tests -----------------------------------------------------------------

func TestBootstrapConcludesPatchProfileFact(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps()
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁在 app.py 中把 eval 换成 int，修复了未授权代码执行入口"
	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if len(client.declared) != 1 {
		t.Fatalf("expected only the reserved bootstrap intent to be declared, got %d", len(client.declared))
	}
	reserved := client.declared[0]
	if reserved.Content != "bootstrap" || reserved.Creator != "dispatcher.bootstrap" {
		t.Fatalf("reserved intent convention mismatch: %+v", reserved)
	}
	if !reflect.DeepEqual(reserved.From, []string{"origin"}) {
		t.Fatalf("reserved intent from = %v, want [origin]", reserved.From)
	}

	if len(client.claimed) != 1 {
		t.Fatalf("expected the reserved intent to be claimed once, got %d", len(client.claimed))
	}
	if len(client.concluded) != 1 {
		t.Fatalf("expected exactly one conclusion, got %d", len(client.concluded))
	}
	call := client.concluded[0]
	if call.FactType != "patch_profile" {
		t.Fatalf("concluded fact type = %q, want patch_profile", call.FactType)
	}
	if call.Description != fact {
		t.Fatalf("concluded description = %q, want driver fact %q", call.Description, fact)
	}
	if call.IntentID != "i1" {
		t.Fatalf("concluded intent = %q, want reserved i1", call.IntentID)
	}

	if len(driver.tasks) != 1 {
		t.Fatalf("expected one driver task, got %d", len(driver.tasks))
	}
	task := driver.tasks[0]
	if task.Type != string(worker.TaskBootstrap) || task.ProjectID != "p1" || task.IntentID != "i1" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if strings.Contains(task.Prompt, "{graph_yaml}") || strings.Contains(task.Prompt, "{fact_ids}") {
		t.Fatalf("driver prompt still contains unresolved placeholders: %q", task.Prompt)
	}
	if !strings.Contains(task.Prompt, "SCENE-TEMPLATE") {
		t.Fatalf("driver prompt is missing the scene template: %q", task.Prompt)
	}
}

func TestBootstrapDeclaresInversionIntents(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps()
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Fact:     &fact,
		Intents: []worker.IntentSpec{
			{From: []string{"origin"}, Description: "验证未授权请求能否到达 eval 入口"},
			{From: []string{"goal"}, Description: "验证修复分支之外的回退路径"},
		},
	}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if len(client.declared) != 3 {
		t.Fatalf("expected reserved + 2 inversion intents, got %d", len(client.declared))
	}
	want := []string{"f001"}
	for i, intent := range client.declared[1:] {
		if !reflect.DeepEqual(intent.From, want) {
			t.Fatalf("intent %d from = %v, want patch_profile fact %v", i, intent.From, want)
		}
	}
	if client.declared[1].Content != "验证未授权请求能否到达 eval 入口" {
		t.Fatalf("intent 0 description = %q", client.declared[1].Content)
	}
	if client.declared[2].Content != "验证修复分支之外的回退路径" {
		t.Fatalf("intent 1 description = %q", client.declared[2].Content)
	}
}

func TestBootstrapSkipsWhenNoPatchSource(t *testing.T) {
	bootstrapWorkdir(t)
	deps := stubBootstrapDeps()

	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true}}
	project := bootstrapProject(filepath.Join(t.TempDir(), "missing.diff"))

	err := runBootstrap(context.Background(), client, driver, project, deps)
	if err == nil {
		t.Fatal("expected an error for a project without a patch source")
	}
	if !strings.Contains(err.Error(), "origin") || !strings.Contains(err.Error(), "neither") {
		t.Fatalf("error does not explain the missing patch source: %v", err)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("no graph writes expected, got declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
}

func TestBootstrapReleasesIntentOnDriverFailure(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps()
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", err: errors.New("worker exploded")}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err == nil {
		t.Fatal("expected the driver error to surface")
	}
	if len(client.released) != 1 || client.released[0] != "i1" {
		t.Fatalf("expected reserved intent i1 to be released, got %v", client.released)
	}
	if len(client.concluded) != 0 {
		t.Fatalf("a failed execution must not conclude a fact, got %d", len(client.concluded))
	}
}

func TestBootstrapPromptRendersPlaceholdersAndContext(t *testing.T) {
	profile, err := patchintel.ParseDiff(bootstrapSampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	profile.CVE = "CVE-2025-3248"
	rules := []patchintel.Rule{{ID: 7, Pattern: "unauth code execution", Confidence: 0.7}}
	project := graph.Project{ID: "p1", Title: "demo", Origin: "/tmp/patch.diff", Goal: "确认可利用"}

	prompt := bootstrapPrompt(project, profile, rules, "GRAPH:\n{graph_yaml}\nFACTS:{fact_ids}\nSCENE")

	for _, placeholder := range []string{"{graph_yaml}", "{fact_ids}"} {
		if strings.Contains(prompt, placeholder) {
			t.Fatalf("prompt still contains %s: %q", placeholder, prompt)
		}
	}
	for _, want := range []string{"origin", "goal", "CVE-2025-3248", "unauth code execution", "SCENE", "+x = int(input())"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, prompt)
		}
	}
}

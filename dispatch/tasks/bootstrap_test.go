package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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

	declareAttempts []graph.Node
	declared        []graph.Node
	claimed         []string
	released        []string
	concluded       []concludeCall
	heartbeats      int
	heartbeatIntent []string

	// declareFail lets a test make specific declarations fail after they have
	// been recorded as attempted.
	declareFail func(graph.Node) error
	// releaseErr makes the error path of ReleaseIntent observable.
	releaseErr error
	// concludeEmptyID simulates a conclude that resolves the intent but returns
	// no fact id.
	concludeEmptyID bool
	// heartbeatCh is closed on the first heartbeat so a fake Execute can prove
	// the runner kept the lease alive while the worker was busy.
	heartbeatCh   chan struct{}
	heartbeatOnce sync.Once

	factSeq int
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
	c.declareAttempts = append(c.declareAttempts, n)
	if c.declareFail != nil {
		if err := c.declareFail(n); err != nil {
			return graph.Node{}, err
		}
	}
	c.declared = append(c.declared, n)
	return graph.Node{
		ID:      fmt.Sprintf("i%d", len(c.declareAttempts)),
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

func (c *fakeGraphClient) HeartbeatIntent(_ context.Context, _, intentID, _ string, _ time.Duration) (graph.Node, error) {
	c.heartbeats++
	c.heartbeatIntent = append(c.heartbeatIntent, intentID)
	if c.heartbeatCh != nil {
		c.heartbeatOnce.Do(func() { close(c.heartbeatCh) })
	}
	return graph.Node{ID: intentID, Kind: "intent", State: "claimed"}, nil
}

func (c *fakeGraphClient) ReleaseIntent(_ context.Context, _, intentID, _ string) (graph.Node, error) {
	c.released = append(c.released, intentID)
	if c.releaseErr != nil {
		return graph.Node{}, c.releaseErr
	}
	return graph.Node{ID: intentID, Kind: "intent", State: "open"}, nil
}

func (c *fakeGraphClient) ConcludeIntent(_ context.Context, _, intentID, workerName, description, factType string) (graph.Node, graph.Node, error) {
	c.concluded = append(c.concluded, concludeCall{
		IntentID:    intentID,
		Worker:      workerName,
		Description: description,
		FactType:    factType,
	})
	c.factSeq++
	factID := fmt.Sprintf("f%03d", c.factSeq)
	if c.concludeEmptyID {
		factID = ""
	}
	fact := graph.Node{ID: factID, Kind: "fact", Type: factType, Content: description, State: "resolved"}
	intent := graph.Node{ID: intentID, Kind: "intent", State: "resolved"}
	return fact, intent, nil
}

type fakeDriver struct {
	name    string
	result  worker.Result
	err     error
	execute func(context.Context, worker.Task) (worker.Result, error)
	tasks   []worker.Task
}

func (d *fakeDriver) Name() string                  { return d.name }
func (d *fakeDriver) Supports(worker.TaskType) bool { return true }
func (d *fakeDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *fakeDriver) Execute(ctx context.Context, t worker.Task) (worker.Result, error) {
	d.tasks = append(d.tasks, t)
	if d.execute != nil {
		return d.execute(ctx, t)
	}
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

// bootstrapHintExport is the real export shape: hints are mappings, not scalar
// strings. Any project with an operator hint used to break bootstrap here.
const bootstrapHintExport = `project:
  title: demo
  origin: /tmp/patch.diff
  goal: verify the fix
hints:
- id: h001
  content: 先看输入解析路径
  creator: human
  created_at: "2026-01-01 00:00:00"
facts:
- id: origin
- id: goal
intents: []
`

func bootstrapReservedExport(state, to string) string {
	toLine := "  to: null\n"
	if to != "" {
		toLine = "  to: " + to + "\n"
	}
	return "facts:\n- id: origin\n- id: goal\nintents:\n" +
		"- id: b900\n" +
		"  type: inversion\n" +
		"  content: bootstrap\n" +
		"  from:\n  - origin\n" +
		"  state: " + state + "\n" +
		toLine +
		"  creator: dispatcher.bootstrap\n"
}

const bootstrapIdeaReservedExport = `facts:
- id: origin
- id: goal
intents:
- id: i900
  type: inversion
  content: exploit_idea
  from:
  - origin
  state: open
  creator: dispatcher.bootstrap
`

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

// stubInversionOutput builds a realistic inversion result without importing
// taskManager (the task package must not depend on it).
func stubInversionOutput(t *testing.T) *patchintel.InversionOutput {
	t.Helper()
	idea, candidate, err := patchintel.BuildIdeaFromResponse(`{
		"harm": "攻击者可执行任意代码",
		"condition": "目标暴露 eval 入口",
		"vuln_type": "RCE",
		"exploit_point": {"title": "eval 未授权代码执行", "type": "RCE", "confidence": "high"}
	}`, nil)
	if err != nil {
		t.Fatalf("build stub inversion idea: %v", err)
	}
	return &patchintel.InversionOutput{Idea: idea, CandidateRule: candidate}
}

// stubBootstrapDeps removes the rule base, scene template and LLM inversion
// lookups so the tests stay offline and never initialise the process-wide
// config database. The prompt loader still runs so the placeholder contract is
// exercised.
func stubBootstrapDeps(t *testing.T) bootstrapDeps {
	t.Helper()
	return bootstrapDeps{
		searchRules: func(string) []patchintel.Rule { return nil },
		loadScene:   func(string) (string, string) { return "SCENE-TEMPLATE", "test" },
		loadPrompt:  worker.LoadPrompt,
		invert: func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error) {
			return stubInversionOutput(t), nil
		},
		heartbeatInterval: time.Hour,
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

func bootstrapFactPointer(s string) *string { return &s }

// --- tests -----------------------------------------------------------------

func TestBootstrapWritesPatchProfileAndExploitIdeaFacts(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	var invertCalls int
	var invertProfile *patchintel.PatchProfile
	deps.invert = func(_ context.Context, profile *patchintel.PatchProfile, _ []patchintel.Rule, _ string) (*patchintel.InversionOutput, error) {
		invertCalls++
		invertProfile = profile
		return stubInversionOutput(t), nil
	}

	fact := "补丁在 app.py 中把 eval 换成 int，修复了未授权代码执行入口"
	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if invertCalls != 1 {
		t.Fatalf("expected exactly one inversion call, got %d", invertCalls)
	}
	if invertProfile == nil || !strings.Contains(invertProfile.RawDiff, "eval") {
		t.Fatalf("inversion did not receive the ingested patch profile: %+v", invertProfile)
	}

	if len(client.declared) != 2 {
		t.Fatalf("expected reserved + exploit_idea intents, got %d: %+v", len(client.declared), client.declared)
	}
	reserved := client.declared[0]
	if reserved.Content != "bootstrap" || reserved.Creator != "dispatcher.bootstrap" {
		t.Fatalf("reserved intent convention mismatch: %+v", reserved)
	}
	if !reflect.DeepEqual(reserved.From, []string{"origin"}) {
		t.Fatalf("reserved intent from = %v, want [origin]", reserved.From)
	}
	ideaIntent := client.declared[1]
	if ideaIntent.Content != "exploit_idea" || ideaIntent.Creator != "dispatcher.bootstrap" {
		t.Fatalf("exploit_idea intent convention mismatch: %+v", ideaIntent)
	}
	if !reflect.DeepEqual(ideaIntent.From, []string{"f001"}) {
		t.Fatalf("exploit_idea intent from = %v, want the patch_profile fact [f001]", ideaIntent.From)
	}

	if !reflect.DeepEqual(client.claimed, []string{"i1", "i2"}) {
		t.Fatalf("claimed = %v, want the reserved and exploit_idea intents", client.claimed)
	}
	if len(client.released) != 0 {
		t.Fatalf("a successful bootstrap must not release anything, got %v", client.released)
	}

	if len(client.concluded) != 2 {
		t.Fatalf("expected two conclusions, got %d: %+v", len(client.concluded), client.concluded)
	}
	profileCall := client.concluded[0]
	if profileCall.FactType != "patch_profile" || profileCall.IntentID != "i1" {
		t.Fatalf("first conclusion = %+v, want patch_profile from i1", profileCall)
	}
	if profileCall.Description != fact {
		t.Fatalf("patch_profile description = %q, want driver fact %q", profileCall.Description, fact)
	}
	ideaCall := client.concluded[1]
	if ideaCall.FactType != "exploit_idea" || ideaCall.IntentID != "i2" {
		t.Fatalf("second conclusion = %+v, want exploit_idea from i2", ideaCall)
	}
	for _, want := range []string{"eval 未授权代码执行", "攻击者可执行任意代码"} {
		if !strings.Contains(ideaCall.Description, want) {
			t.Fatalf("exploit_idea description is missing %q: %q", want, ideaCall.Description)
		}
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
	if !strings.Contains(task.Prompt, "覆盖") || !strings.Contains(task.Prompt, "intents.from") {
		t.Fatalf("driver prompt must state that intents.from is overridden: %q", task.Prompt)
	}
}

func TestBootstrapDeclaresInversionIntents(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
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

	// reserved bootstrap + exploit_idea + 2 inversion intents.
	if len(client.declared) != 4 {
		t.Fatalf("expected 4 declarations, got %d: %+v", len(client.declared), client.declared)
	}
	want := []string{"f001"}
	for i, intent := range client.declared[2:] {
		if !reflect.DeepEqual(intent.From, want) {
			t.Fatalf("intent %d from = %v, want patch_profile fact %v", i, intent.From, want)
		}
	}
	if client.declared[2].Content != "验证未授权请求能否到达 eval 入口" {
		t.Fatalf("intent 0 description = %q", client.declared[2].Content)
	}
	if client.declared[3].Content != "验证修复分支之外的回退路径" {
		t.Fatalf("intent 1 description = %q", client.declared[3].Content)
	}
}

func TestBootstrapSkipsWhenNoPatchSource(t *testing.T) {
	bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)

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

func TestBootstrapSkipsWhenDisabled(t *testing.T) {
	bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)

	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true}}
	project := bootstrapProject(filepath.Join(t.TempDir(), "missing.diff"))
	project.BootstrapEnabled = false

	if err := runBootstrap(context.Background(), client, driver, project, deps); err != nil {
		t.Fatalf("a disabled bootstrap must be a no-op, got %v", err)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("a disabled bootstrap must not write the graph, got declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
	if len(driver.tasks) != 0 {
		t.Fatalf("a disabled bootstrap must not run the worker")
	}
}

func TestBootstrapReleasesIntentOnDriverFailure(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	var invertCalls int
	deps.invert = func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error) {
		invertCalls++
		return stubInversionOutput(t), nil
	}

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
	if invertCalls != 0 {
		t.Fatalf("inversion must not run when the worker refused or failed, got %d calls", invertCalls)
	}
}

func TestBootstrapReleasesIntentWhenInversionFails(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)
	deps.invert = func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error) {
		return nil, errors.New("model unavailable")
	}

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapInitialExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	if err == nil || !strings.Contains(err.Error(), "inversion") {
		t.Fatalf("expected an inversion error, got %v", err)
	}
	if !reflect.DeepEqual(client.released, []string{"i1"}) {
		t.Fatalf("release = %v, want [i1]", client.released)
	}
	if len(client.concluded) != 0 {
		t.Fatalf("a failed inversion must not conclude a fact, got %d", len(client.concluded))
	}
}

func TestBootstrapReturnsReleaseErrorAlongsideFailure(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: bootstrapInitialExport, releaseErr: errors.New("release denied")}
	driver := &fakeDriver{name: "native-test", err: errors.New("worker exploded")}

	err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"worker exploded", "release denied"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must include %q, got %v", want, err)
		}
	}
}

func TestBootstrapSuppressesWhenReservedIntentIsClaimed(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: bootstrapReservedExport("claimed", "")}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("a claimed bootstrap must be a silent no-op, got %v", err)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("a claimed bootstrap must not write, got declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
	if len(driver.tasks) != 0 {
		t.Fatal("a claimed bootstrap must not run the worker")
	}
}

func TestBootstrapSuppressesWhenReservedIntentIsConcluded(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: bootstrapReservedExport("resolved", "f777")}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("a concluded bootstrap must be a silent no-op, got %v", err)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("a concluded bootstrap must not write, got declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
	if len(driver.tasks) != 0 {
		t.Fatal("a concluded bootstrap must not run the worker")
	}
}

func TestBootstrapReusesOpenReservedIntent(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapReservedExport("open", "")}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}
	if len(client.declared) != 1 || client.declared[0].Content != "exploit_idea" {
		t.Fatalf("an existing open bootstrap must be reused, not declared again: %+v", client.declared)
	}
	if !reflect.DeepEqual(client.claimed, []string{"b900", "i1"}) {
		t.Fatalf("claimed = %v, want reused b900 then the exploit_idea intent", client.claimed)
	}
	if len(client.concluded) != 2 || client.concluded[0].IntentID != "b900" || client.concluded[0].FactType != "patch_profile" {
		t.Fatalf("unexpected conclusions: %+v", client.concluded)
	}
}

func TestBootstrapIgnoresReservedNonBootstrapIntent(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapIdeaReservedExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}
	if len(client.declared) < 1 || client.declared[0].Content != "bootstrap" {
		t.Fatalf("a leftover exploit_idea intent must not suppress bootstrap: %+v", client.declared)
	}
	if !reflect.DeepEqual(client.claimed, []string{"i1", "i2"}) {
		t.Fatalf("claimed = %v, want a fresh bootstrap then the exploit_idea intent", client.claimed)
	}
	for _, id := range client.claimed {
		if id == "i900" {
			t.Fatalf("bootstrap must not claim the leftover exploit_idea intent")
		}
	}
}

func TestBootstrapToleratesHintsInExport(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapHintExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("bootstrap must tolerate hints in the export: %v", err)
	}
	if len(client.concluded) != 2 {
		t.Fatalf("expected both facts to be written, got %+v", client.concluded)
	}
}

func TestBootstrapDeclaresEveryIntentAfterConclusion(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{
		export: bootstrapInitialExport,
		declareFail: func(n graph.Node) error {
			if strings.Contains(n.Content, "回退路径") {
				return errors.New("graph store unavailable")
			}
			return nil
		},
	}
	driver := &fakeDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Fact:     &fact,
		Intents: []worker.IntentSpec{
			{Description: "验证未授权请求能否到达 eval 入口"},
			{Description: "验证修复分支之外的回退路径"},
		},
	}}

	err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	if err == nil {
		t.Fatal("expected the declaration failure to surface")
	}
	// reserved, exploit_idea, first inversion, second inversion.
	if len(client.declareAttempts) != 4 {
		t.Fatalf("every inversion intent must be attempted, got %d attempts: %+v",
			len(client.declareAttempts), client.declareAttempts)
	}
	if len(client.declared) != 3 {
		t.Fatalf("the successful declarations must be kept, got %d", len(client.declared))
	}
	if len(client.released) != 0 {
		t.Fatalf("a concluded bootstrap must not be released, got %v", client.released)
	}
	if len(client.concluded) != 2 {
		t.Fatalf("both facts must already be written, got %+v", client.concluded)
	}
}

func TestBootstrapReturnsErrorWhenConclusionHasNoFactID(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapInitialExport, concludeEmptyID: true}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	if err == nil || !strings.Contains(err.Error(), "fact id") {
		t.Fatalf("expected a missing fact id error, got %v", err)
	}
	if len(client.declared) != 1 {
		t.Fatalf("without a patch_profile fact id the exploit_idea intent must not be declared, got %+v", client.declared)
	}
}

func TestBootstrapHeartbeatsWhileExecuting(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	deps.heartbeatInterval = 5 * time.Millisecond
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{
		export:      bootstrapInitialExport,
		heartbeatCh: make(chan struct{}),
	}
	driver := &fakeDriver{name: "native-test"}
	driver.execute = func(context.Context, worker.Task) (worker.Result, error) {
		select {
		case <-client.heartbeatCh:
		case <-time.After(2 * time.Second):
			t.Errorf("runner did not heartbeat the reserved intent while executing")
		}
		return worker.Result{Accepted: true, Fact: bootstrapFactPointer(fact)}, nil
	}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}
	if client.heartbeats == 0 {
		t.Fatal("expected at least one heartbeat")
	}
	if len(client.heartbeatIntent) == 0 || client.heartbeatIntent[0] != "i1" {
		t.Fatalf("heartbeat intent = %v, want the reserved bootstrap i1", client.heartbeatIntent)
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
	for _, want := range []string{"origin", "goal", "CVE-2025-3248", "unauth code execution", "SCENE", "+x = int(input())",
		"覆盖", "intents.from", "patch_profile"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, prompt)
		}
	}
}

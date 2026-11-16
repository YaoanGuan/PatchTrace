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
	"sync/atomic"
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
	heartbeats      atomic.Int64
	heartbeatIntent []string

	// declareFail lets a test make specific declarations fail after they have
	// been recorded as attempted.
	declareFail func(graph.Node) error
	// concludeFail lets a test fail a specific conclusion after it has been
	// recorded as attempted.
	concludeFail func(concludeCall) error
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
	c.heartbeats.Add(1)
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
	call := concludeCall{
		IntentID:    intentID,
		Worker:      workerName,
		Description: description,
		FactType:    factType,
	}
	c.concluded = append(c.concluded, call)
	if c.concludeFail != nil {
		if err := c.concludeFail(call); err != nil {
			return graph.Node{}, graph.Node{}, err
		}
	}
	c.factSeq++
	factID := fmt.Sprintf("f%03d", c.factSeq)
	if c.concludeEmptyID {
		factID = ""
	}
	fact := graph.Node{ID: factID, Kind: "fact", Type: factType, Content: description, State: "resolved"}
	intent := graph.Node{ID: intentID, Kind: "intent", State: "resolved"}
	return fact, intent, nil
}

func (c *fakeGraphClient) resetRecording() {
	c.declareAttempts = nil
	c.declared = nil
	c.claimed = nil
	c.released = nil
	c.concluded = nil
	c.heartbeats.Store(0)
	c.heartbeatIntent = nil
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

type bootstrapIntentFixture struct {
	id      string
	state   string
	content string
	creator string
	from    []string
	to      string
}

// bootstrapExport builds the real export shape. `to` is emitted as a scalar
// when set (concluded intent) and null otherwise.
func bootstrapExport(intents ...bootstrapIntentFixture) string {
	var b strings.Builder
	b.WriteString("facts:\n- id: origin\n- id: goal\n")
	if len(intents) == 0 {
		b.WriteString("intents: []\n")
		return b.String()
	}
	b.WriteString("intents:\n")
	for _, in := range intents {
		fmt.Fprintf(&b, "- id: %s\n  type: inversion\n  content: %s\n  from:\n", in.id, in.content)
		for _, from := range in.from {
			fmt.Fprintf(&b, "  - %s\n", from)
		}
		to := "null"
		if in.to != "" {
			to = in.to
		}
		fmt.Fprintf(&b, "  to: %s\n  state: %s\n  worker: \"\"\n  creator: %s\n  created_at: \"2026-01-01 00:00:00\"\n  concluded_at: null\n",
			to, in.state, in.creator)
	}
	return b.String()
}

func bootstrapReservedExport(state, to string) string {
	return bootstrapExport(bootstrapIntentFixture{
		id: "b900", state: state, content: "bootstrap",
		creator: "dispatcher.bootstrap", from: []string{"origin"}, to: to,
	})
}

var bootstrapIdeaReservedExport = bootstrapExport(bootstrapIntentFixture{
	id: "i900", state: "open", content: "exploit_idea",
	creator: "dispatcher.bootstrap", from: []string{"origin"},
})

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

	// reserved bootstrap + 2 inversion intents + exploit_idea: inversion
	// intents are declared before the idea fact.
	if len(client.declared) != 4 {
		t.Fatalf("expected 4 declarations, got %d: %+v", len(client.declared), client.declared)
	}
	want := []string{"f001"}
	for i, intent := range client.declared[1:3] {
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
	if client.declared[3].Content != "exploit_idea" {
		t.Fatalf("idea intent must come after the inversion intents, got %+v", client.declared)
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

func TestBootstrapNoOpResumeWhenEverythingPresent(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)
	var invertCalls int
	deps.invert = func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error) {
		invertCalls++
		return stubInversionOutput(t), nil
	}

	client := &fakeGraphClient{export: bootstrapExport(
		bootstrapIntentFixture{id: "b900", state: "resolved", content: "bootstrap", creator: "dispatcher.bootstrap", from: []string{"origin"}, to: "f777"},
		bootstrapIntentFixture{id: "n1", state: "open", content: "意图甲", creator: "worker-x", from: []string{"f777"}},
		bootstrapIntentFixture{id: "n2", state: "resolved", content: "exploit_idea", creator: "dispatcher.bootstrap", from: []string{"f777"}, to: "f778"},
	)}
	driver := &fakeDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Intents:  []worker.IntentSpec{{Description: "意图甲"}},
	}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("a complete resume must be a no-op, got %v", err)
	}
	if invertCalls != 0 {
		t.Fatalf("a concluded exploit_idea fact must skip inversion, got %d calls", invertCalls)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("a complete resume must not write, got declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
	if len(driver.tasks) != 1 {
		t.Fatalf("resume re-runs the driver to reconstruct intents, got %d tasks", len(driver.tasks))
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

func TestBootstrapReusesLeftoverIdeaIntent(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapIdeaReservedExport}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}
	if len(client.declared) != 1 || client.declared[0].Content != "bootstrap" {
		t.Fatalf("a leftover idea intent must cause reuse, not a second declaration: %+v", client.declared)
	}
	if !reflect.DeepEqual(client.claimed, []string{"i1", "i900"}) {
		t.Fatalf("claimed = %v, want a fresh bootstrap then the reused idea intent", client.claimed)
	}
	if len(client.concluded) != 2 || client.concluded[1].IntentID != "i900" || client.concluded[1].FactType != "exploit_idea" {
		t.Fatalf("unexpected conclusions: %+v", client.concluded)
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
	if client.heartbeats.Load() == 0 {
		t.Fatal("expected at least one heartbeat")
	}
	if len(client.heartbeatIntent) == 0 || client.heartbeatIntent[0] != "i1" {
		t.Fatalf("heartbeat intent = %v, want the reserved bootstrap i1", client.heartbeatIntent)
	}
}

func TestBootstrapResumesConcludedBootstrap(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)
	var invertCalls int
	deps.invert = func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error) {
		invertCalls++
		return stubInversionOutput(t), nil
	}

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapReservedExport("resolved", "f777")}
	driver := &fakeDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Fact:     &fact,
		Intents: []worker.IntentSpec{
			{Description: "意图甲"},
			{Description: "意图乙"},
		},
	}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if invertCalls != 1 {
		t.Fatalf("resume must run inversion once for the missing idea, got %d", invertCalls)
	}
	for _, call := range client.concluded {
		if call.FactType == "patch_profile" {
			t.Fatalf("a concluded bootstrap must not conclude another patch_profile: %+v", call)
		}
	}
	if len(client.concluded) != 1 || client.concluded[0].FactType != "exploit_idea" {
		t.Fatalf("resume conclusions = %+v, want exactly the exploit_idea fact", client.concluded)
	}
	wantContents := []string{"意图甲", "意图乙", "exploit_idea"}
	if len(client.declared) != len(wantContents) {
		t.Fatalf("declared = %+v, want %v", client.declared, wantContents)
	}
	for i, want := range wantContents {
		if client.declared[i].Content != want {
			t.Fatalf("declared[%d] content = %q, want %q", i, client.declared[i].Content, want)
		}
		if !reflect.DeepEqual(client.declared[i].From, []string{"f777"}) {
			t.Fatalf("declared[%d] from = %v, want the stored patch_profile fact f777", i, client.declared[i].From)
		}
	}
	if !reflect.DeepEqual(client.claimed, []string{"i3"}) {
		t.Fatalf("claimed = %v, want only the exploit_idea intent", client.claimed)
	}
	if len(client.released) != 0 {
		t.Fatalf("resume must not release anything, got %v", client.released)
	}

	// A second resume with every artifact present is a nil, zero-write no-op.
	client.export = bootstrapExport(
		bootstrapIntentFixture{id: "b900", state: "resolved", content: "bootstrap", creator: "dispatcher.bootstrap", from: []string{"origin"}, to: "f777"},
		bootstrapIntentFixture{id: "n1", state: "open", content: "意图甲", creator: "worker-x", from: []string{"f777"}},
		bootstrapIntentFixture{id: "n2", state: "open", content: "意图乙", creator: "worker-x", from: []string{"f777"}},
		bootstrapIntentFixture{id: "n3", state: "resolved", content: "exploit_idea", creator: "dispatcher.bootstrap", from: []string{"f777"}, to: "f002"},
	)
	client.resetRecording()
	client.factSeq = 1
	invertCalls = 0

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("second resume: %v", err)
	}
	if invertCalls != 0 {
		t.Fatalf("a concluded idea must skip inversion, got %d calls", invertCalls)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("second resume wrote the graph: declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
}

func TestBootstrapReusesOpenIdeaIntentAfterConcludeFailure(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	fact := "补丁修复了代码执行入口"
	client := &fakeGraphClient{export: bootstrapInitialExport}
	client.concludeFail = func(call concludeCall) error {
		if call.FactType == "exploit_idea" {
			return errors.New("conclude failed")
		}
		return nil
	}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true, Fact: &fact}}

	err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	if err == nil || !strings.Contains(err.Error(), "conclude failed") {
		t.Fatalf("expected the idea conclude failure, got %v", err)
	}
	if len(client.declared) != 2 || client.declared[1].Content != "exploit_idea" {
		t.Fatalf("first run declarations = %+v, want bootstrap + exploit_idea", client.declared)
	}
	if !reflect.DeepEqual(client.released, []string{"i2"}) {
		t.Fatalf("the failed idea conclude must release i2, got %v", client.released)
	}

	// The released intent is open, so the next call must reuse it.
	client.export = bootstrapExport(
		bootstrapIntentFixture{id: "b900", state: "resolved", content: "bootstrap", creator: "dispatcher.bootstrap", from: []string{"origin"}, to: "f001"},
		bootstrapIntentFixture{id: "i2", state: "open", content: "exploit_idea", creator: "dispatcher.bootstrap", from: []string{"f001"}},
	)
	client.resetRecording()
	client.concludeFail = nil
	client.factSeq = 1

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(client.declared) != 0 {
		t.Fatalf("retry must reuse the open idea intent, declared %+v", client.declared)
	}
	if !reflect.DeepEqual(client.claimed, []string{"i2"}) {
		t.Fatalf("retry claimed = %v, want [i2]", client.claimed)
	}
	if len(client.concluded) != 1 || client.concluded[0].FactType != "exploit_idea" || client.concluded[0].IntentID != "i2" {
		t.Fatalf("retry conclusions = %+v, want exploit_idea from i2", client.concluded)
	}
	if !strings.Contains(client.concluded[0].Description, "eval 未授权代码执行") {
		t.Fatalf("retry idea description is not the inversion idea: %q", client.concluded[0].Description)
	}
}

func TestBootstrapSkipsClaimedIdeaIntent(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)
	var invertCalls int
	deps.invert = func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error) {
		invertCalls++
		return stubInversionOutput(t), nil
	}

	client := &fakeGraphClient{export: bootstrapExport(
		bootstrapIntentFixture{id: "b900", state: "resolved", content: "bootstrap", creator: "dispatcher.bootstrap", from: []string{"origin"}, to: "f777"},
		bootstrapIntentFixture{id: "i900", state: "claimed", content: "exploit_idea", creator: "dispatcher.bootstrap", from: []string{"f777"}},
	)}
	driver := &fakeDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Intents:  []worker.IntentSpec{{Description: "意图甲"}},
	}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}
	if invertCalls != 0 {
		t.Fatalf("a claimed idea intent is owned elsewhere; inversion must be skipped, got %d calls", invertCalls)
	}
	if len(client.declared) != 1 || client.declared[0].Content != "意图甲" {
		t.Fatalf("other work must proceed: %+v", client.declared)
	}
	if !reflect.DeepEqual(client.declared[0].From, []string{"f777"}) {
		t.Fatalf("declared from = %v, want f777", client.declared[0].From)
	}
	if len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("a claimed idea must not be claimed/concluded/released, got claimed=%v concluded=%d released=%v",
			client.claimed, len(client.concluded), client.released)
	}
}

func TestBootstrapDeclaresOnlyMissingInversionIntents(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: bootstrapExport(
		bootstrapIntentFixture{id: "b900", state: "resolved", content: "bootstrap", creator: "dispatcher.bootstrap", from: []string{"origin"}, to: "f777"},
		bootstrapIntentFixture{id: "i100", state: "open", content: "已有意图", creator: "worker-x", from: []string{"f777"}},
	)}
	driver := &fakeDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Intents: []worker.IntentSpec{
			{Description: "已有意图"},
			{Description: "缺失意图"},
		},
	}}

	if err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps); err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}
	if len(client.declared) != 2 {
		t.Fatalf("declared = %+v, want only the missing intent and the idea", client.declared)
	}
	if client.declared[0].Content != "缺失意图" {
		t.Fatalf("declared[0] = %q, want 缺失意图", client.declared[0].Content)
	}
	if client.declared[1].Content != "exploit_idea" {
		t.Fatalf("declared[1] = %q, want exploit_idea", client.declared[1].Content)
	}
}

func TestBootstrapStopsHeartbeatOnPanic(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	deps.heartbeatInterval = 5 * time.Millisecond
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: bootstrapInitialExport, heartbeatCh: make(chan struct{})}
	driver := &fakeDriver{name: "native-test"}
	driver.execute = func(context.Context, worker.Task) (worker.Result, error) {
		select {
		case <-client.heartbeatCh:
		case <-time.After(2 * time.Second):
			t.Errorf("no heartbeat before the panic")
		}
		panic("driver panic")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected the driver panic to propagate")
			}
		}()
		_ = runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	}()

	before := client.heartbeats.Load()
	time.Sleep(50 * time.Millisecond)
	after := client.heartbeats.Load()
	if after != before {
		t.Fatalf("heartbeat goroutine leaked after panic: %d -> %d ticks", before, after)
	}
}

func TestBootstrapErrorsWhenConcludedIntentHasNoFactID(t *testing.T) {
	dir := bootstrapWorkdir(t)
	deps := stubBootstrapDeps(t)
	patchPath := bootstrapPatchFile(t, dir)

	client := &fakeGraphClient{export: `facts:
- id: origin
- id: goal
intents:
- id: b900
  type: inversion
  content: bootstrap
  from:
  - origin
  to: ""
  state: resolved
  creator: dispatcher.bootstrap
`}
	driver := &fakeDriver{name: "native-test", result: worker.Result{Accepted: true}}

	err := runBootstrap(context.Background(), client, driver, bootstrapProject(patchPath), deps)
	if err == nil || !strings.Contains(err.Error(), "patch_profile fact id") {
		t.Fatalf("expected a hard error for a concluded intent without a fact id, got %v", err)
	}
	if len(client.declared)+len(client.claimed)+len(client.concluded)+len(client.released) != 0 {
		t.Fatalf("no writes expected, got declared=%d claimed=%d concluded=%d released=%d",
			len(client.declared), len(client.claimed), len(client.concluded), len(client.released))
	}
	if len(driver.tasks) != 0 {
		t.Fatal("the driver must not run without a patch_profile fact id")
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

package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
)

// --- fakes -----------------------------------------------------------------

type exploreClaimCall struct {
	IntentID string
	Worker   string
	Lease    time.Duration
}

type exploreConcludeCall struct {
	IntentID    string
	Worker      string
	Description string
	FactType    string
}

type exploreReleaseCall struct {
	IntentID string
	Worker   string
}

type exploreVerdictCall struct {
	ProjectID   string
	TargetID    string
	Verdict     string
	Description string
	EvidenceRef string
}

// fakeExploreClient embeds graphserver.Client so an unexpected protocol call
// panics loudly. It records the intent lifecycle and graph writes.
type fakeExploreClient struct {
	graphserver.Client

	export    string
	exportErr error

	claimErr error
	claims   []exploreClaimCall

	heartbeats  atomic.Int64
	heartbeatCh chan struct{}
	heartbeat   sync.Once
	hbErr       error

	concludeErr error
	concludes   []exploreConcludeCall

	releaseErr error
	releases   []exploreReleaseCall
}

func (c *fakeExploreClient) ClaimIntent(_ context.Context, projectID, intentID, workerName string, lease time.Duration) (graph.Node, error) {
	c.claims = append(c.claims, exploreClaimCall{IntentID: intentID, Worker: workerName, Lease: lease})
	if c.claimErr != nil {
		return graph.Node{}, c.claimErr
	}
	return graph.Node{
		ID:        intentID,
		ProjectID: projectID,
		Kind:      "intent",
		State:     "claimed",
		Worker:    workerName,
	}, nil
}

func (c *fakeExploreClient) HeartbeatIntent(_ context.Context, projectID, intentID, workerName string, _ time.Duration) (graph.Node, error) {
	c.heartbeats.Add(1)
	if c.heartbeatCh != nil {
		c.heartbeat.Do(func() { close(c.heartbeatCh) })
	}
	return graph.Node{ID: intentID, ProjectID: projectID, State: "claimed", Worker: workerName}, c.hbErr
}

func (c *fakeExploreClient) ReleaseIntent(_ context.Context, projectID, intentID, workerName string) (graph.Node, error) {
	c.releases = append(c.releases, exploreReleaseCall{IntentID: intentID, Worker: workerName})
	return graph.Node{ID: intentID, ProjectID: projectID, State: "open"}, c.releaseErr
}

func (c *fakeExploreClient) ConcludeIntent(_ context.Context, projectID, intentID, workerName, description, factType string) (graph.Node, graph.Node, error) {
	c.concludes = append(c.concludes, exploreConcludeCall{
		IntentID: intentID, Worker: workerName, Description: description, FactType: factType,
	})
	if c.concludeErr != nil {
		return graph.Node{}, graph.Node{}, c.concludeErr
	}
	fact := graph.Node{ID: "f100", ProjectID: projectID, Kind: "fact", Type: factType, Content: description, State: "resolved"}
	return fact, graph.Node{ID: intentID, ProjectID: projectID, Kind: "intent", State: "resolved"}, nil
}

func (c *fakeExploreClient) ExportYAML(context.Context, string) (string, error) {
	if c.exportErr != nil {
		return "", c.exportErr
	}
	return c.export, nil
}

// fakeVerdictExploreClient adds the optional verdict seam. The mandated
// graphserver.Client interface has no AddVerdict, so RunExplore asserts it.
type fakeVerdictExploreClient struct {
	*fakeExploreClient
	verdictErr error
	verdicts   []exploreVerdictCall
}

func (c *fakeVerdictExploreClient) AddVerdict(_ context.Context, projectID, targetID, verdict, description, evidenceRef string) (graph.Node, error) {
	c.verdicts = append(c.verdicts, exploreVerdictCall{
		ProjectID: projectID, TargetID: targetID, Verdict: verdict,
		Description: description, EvidenceRef: evidenceRef,
	})
	if c.verdictErr != nil {
		return graph.Node{}, c.verdictErr
	}
	return graph.Node{ID: "f200", ProjectID: projectID, Kind: "fact", Type: "verdict", State: "resolved"}, nil
}

func newExploreClient() *fakeVerdictExploreClient {
	return &fakeVerdictExploreClient{fakeExploreClient: &fakeExploreClient{export: exploreExport}}
}

type fakeExploreDriver struct {
	name             string
	supportsConclude bool
	execute          func(context.Context, worker.Task) (worker.Result, error)
	conclude         func(context.Context, worker.Task) (worker.Result, error)
	execTasks        []worker.Task
	concludeTasks    []worker.Task
}

func (d *fakeExploreDriver) Name() string                  { return d.name }
func (d *fakeExploreDriver) Supports(worker.TaskType) bool { return true }
func (d *fakeExploreDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *fakeExploreDriver) Execute(ctx context.Context, t worker.Task) (worker.Result, error) {
	d.execTasks = append(d.execTasks, t)
	if d.execute != nil {
		return d.execute(ctx, t)
	}
	return worker.Result{}, nil
}
func (d *fakeExploreDriver) Conclude(ctx context.Context, t worker.Task) (worker.Result, error) {
	d.concludeTasks = append(d.concludeTasks, t)
	if d.conclude != nil {
		return d.conclude(ctx, t)
	}
	return worker.Result{}, errors.New("unexpected driver Conclude")
}
func (d *fakeExploreDriver) SupportsConclude() bool { return d.supportsConclude }

type fakeSandboxRuntime struct {
	calls atomic.Int64
}

func (r *fakeSandboxRuntime) Exec(context.Context, string, runtime.ExecRequest) (runtime.ExecResult, error) {
	r.calls.Add(1)
	return runtime.ExecResult{}, nil
}

// --- fixtures --------------------------------------------------------------

const exploreExport = `project:
  title: demo
  origin: /tmp/patch.diff
  goal: 确认补丁修复的漏洞可利用
hints:
- id: h001
  content: 先看输入解析路径
  creator: human
  created_at: "2026-01-01 00:00:00"
facts:
- id: origin
  type: origin
  content: a patch was applied
- id: goal
  type: goal
  content: the vulnerability is understood
- id: f001
  type: exploit_idea
  content: an unvalidated eval sink
intents:
- id: i001
  type: gap
  content: 验证输入能否到达 eval 入口
  from:
  - f001
  to: null
  state: claimed
  worker: native-test
  creator: worker-x
  created_at: "2026-01-01 00:00:00"
  concluded_at: null
`

// exploreWorkdir creates a temporary cwd containing the shipped prompt layout
// and changes into it for the duration of the test.
func exploreWorkdir(t *testing.T) {
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

func exploreProject() graph.Project {
	return graph.Project{
		ID:     "p1",
		Title:  "demo",
		Origin: "/tmp/patch.diff",
		Goal:   "确认补丁修复的漏洞可利用",
		Status: "active",
	}
}

func acceptedResult(description, raw string) worker.Result {
	return worker.Result{Accepted: true, Description: description, Raw: raw}
}

// --- tests -----------------------------------------------------------------

func TestExploreConcludesObservationFact(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	description := "已在沙箱确认 eval 入口可达"
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult(description,
				`{"accepted":true,"data":{"description":"已在沙箱确认 eval 入口可达"}}`), nil
		},
	}
	rt := &fakeSandboxRuntime{}

	if err := RunExplore(context.Background(), client, driver, rt, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}

	if len(client.claims) != 1 {
		t.Fatalf("ClaimIntent calls = %d, want 1", len(client.claims))
	}
	claim := client.claims[0]
	if claim.IntentID != "i001" || claim.Worker != "native-test" || claim.Lease != 5*time.Minute {
		t.Fatalf("claim = %+v, want i001/native-test/5m", claim)
	}

	if len(driver.execTasks) != 1 {
		t.Fatalf("driver Execute calls = %d, want 1", len(driver.execTasks))
	}
	task := driver.execTasks[0]
	if task.Type != string(worker.TaskExplore) || task.ProjectID != "p1" || task.IntentID != "i001" {
		t.Fatalf("task = %+v, want an explore task for p1/i001", task)
	}
	for _, want := range []string{"验证输入能否到达 eval 入口", "i001", "facts:"} {
		if !strings.Contains(task.Prompt, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, task.Prompt)
		}
	}
	for _, placeholder := range []string{"{graph_yaml}", "{intent_id}", "{intent_description}"} {
		if strings.Contains(task.Prompt, placeholder) {
			t.Fatalf("prompt still contains %s:\n%s", placeholder, task.Prompt)
		}
	}

	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	conclude := client.concludes[0]
	if conclude.Description != description {
		t.Fatalf("concluded description = %q, want %q", conclude.Description, description)
	}
	if conclude.FactType != "observation" {
		t.Fatalf("fact type = %q, want observation", conclude.FactType)
	}
	if conclude.Worker != "native-test" || conclude.IntentID != "i001" {
		t.Fatalf("conclude = %+v, want native-test/i001", conclude)
	}

	if len(client.releases) != 0 {
		t.Fatalf("a successful conclusion must not release the intent: %+v", client.releases)
	}
	if len(driver.concludeTasks) != 0 {
		t.Fatalf("a successful execute must not call driver Conclude: %+v", driver.concludeTasks)
	}
	if len(client.verdicts) != 0 {
		t.Fatalf("a verdict-free result must not write a verdict: %+v", client.verdicts)
	}
}

func TestExploreTimeoutFallsBackToConclude(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	fallback := "超时前已确认：沙箱命令无响应"
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(ctx context.Context, _ worker.Task) (worker.Result, error) {
			return worker.Result{}, ctx.Err()
		},
		conclude: func(ctx context.Context, task worker.Task) (worker.Result, error) {
			// context.WithoutCancel must detach the conclude call from the
			// cancelled execute context; a fresh timeout still applies.
			if err := ctx.Err(); err != nil {
				return worker.Result{}, err
			}
			if task.ConcludeTimeout <= 0 {
				return worker.Result{}, errors.New("conclude task has no timeout")
			}
			return acceptedResult(fallback,
				`{"accepted":true,"data":{"description":"超时前已确认：沙箱命令无响应"}}`), nil
		},
	}
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	if err := RunExplore(parent, client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}

	if len(driver.concludeTasks) != 1 {
		t.Fatalf("driver Conclude calls = %d, want 1", len(driver.concludeTasks))
	}
	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	if got := client.concludes[0].Description; got != fallback {
		t.Fatalf("concluded description = %q, want %q", got, fallback)
	}
	if got := client.concludes[0].FactType; got != "observation" {
		t.Fatalf("fact type = %q, want observation", got)
	}
	if len(client.releases) != 0 {
		t.Fatalf("a concluded fallback must not release the intent: %+v", client.releases)
	}
}

func TestExploreFailureStillWritesObjectiveFact(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return worker.Result{}, errors.New("worker exploded")
		},
		conclude: func(context.Context, worker.Task) (worker.Result, error) {
			return worker.Result{}, errors.New("conclude also failed")
		},
	}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("a concluded failure fact must return nil, got %v", err)
	}

	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	description := client.concludes[0].Description
	if !strings.HasPrefix(description, "explore 未完成：") {
		t.Fatalf("failure description = %q, want the objective explore 未完成 prefix", description)
	}
	for _, want := range []string{"worker exploded", "conclude also failed"} {
		if !strings.Contains(description, want) {
			t.Fatalf("failure description = %q, want it to mention %q", description, want)
		}
	}
	if got := client.concludes[0].FactType; got != "observation" {
		t.Fatalf("fact type = %q, want observation", got)
	}
	if len(client.releases) != 0 {
		t.Fatalf("a concluded failure must not release the intent: %+v", client.releases)
	}
}

func TestExploreFailureWithoutConcludeSupportWritesFact(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: false,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return worker.Result{}, errors.New("parse failure")
		},
	}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	if len(driver.concludeTasks) != 0 {
		t.Fatalf("driver without conclude support must not call Conclude: %+v", driver.concludeTasks)
	}
	if len(client.concludes) != 1 || !strings.HasPrefix(client.concludes[0].Description, "explore 未完成：") {
		t.Fatalf("want one objective failure fact, got %+v", client.concludes)
	}
}

func TestExploreRefusedResultWritesFailureFact(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return worker.Result{Accepted: false, Raw: `{"accepted":false,"reason":"policy_refusal"}`}, nil
		},
	}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	if len(driver.concludeTasks) != 0 {
		t.Fatalf("a refusal must not run the conclude prompt: %+v", driver.concludeTasks)
	}
	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	description := client.concludes[0].Description
	if !strings.HasPrefix(description, "explore 未完成：") || !strings.Contains(description, "refused") {
		t.Fatalf("refusal description = %q, want an objective refusal fact", description)
	}
	if len(client.releases) != 0 {
		t.Fatalf("a concluded refusal must not release the intent: %+v", client.releases)
	}
}

func TestExploreSkipsHeldIntent(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	client.claimErr = errors.Join(graph.ErrConflict, errors.New("held by another worker"))
	driver := &fakeExploreDriver{name: "native-test"}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("a held intent must skip with nil, got %v", err)
	}
	if len(driver.execTasks) != 0 || len(client.concludes) != 0 || len(client.releases) != 0 {
		t.Fatalf("a skip must not execute, conclude or release")
	}
}

func TestExploreErrorsOnMissingIntentClaim(t *testing.T) {
	exploreWorkdir(t)
	cases := []struct {
		name string
		err  error
	}{
		{name: "not found", err: graph.ErrNotFound},
		{name: "invalid", err: graph.ErrInvalid},
		{name: "forbidden", err: graph.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newExploreClient()
			client.claimErr = tc.err
			driver := &fakeExploreDriver{name: "native-test"}

			err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001")
			if !errors.Is(err, tc.err) {
				t.Fatalf("RunExplore err = %v, want %v", err, tc.err)
			}
			if len(driver.execTasks) != 0 || len(client.releases) != 0 {
				t.Fatalf("a claim failure must not execute or release")
			}
		})
	}
}

func TestExploreReleasesOnExportFailure(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	client.exportErr = errors.New("graph unavailable")
	driver := &fakeExploreDriver{name: "native-test"}

	err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001")
	if err == nil || !strings.Contains(err.Error(), "graph unavailable") {
		t.Fatalf("RunExplore err = %v, want the export failure", err)
	}
	if len(driver.execTasks) != 0 {
		t.Fatalf("an export failure must not run the worker")
	}
	if len(client.releases) != 1 || client.releases[0].IntentID != "i001" {
		t.Fatalf("releases = %+v, want one release of i001", client.releases)
	}
}

func TestExploreReleasesWhenIntentMissingFromExport(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{name: "native-test"}

	err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i999")
	if err == nil || !strings.Contains(err.Error(), "i999") {
		t.Fatalf("RunExplore err = %v, want a missing-intent error", err)
	}
	if len(driver.execTasks) != 0 {
		t.Fatalf("a missing intent must not run the worker")
	}
	if len(client.releases) != 1 || client.releases[0].IntentID != "i999" {
		t.Fatalf("releases = %+v, want one release of i999", client.releases)
	}
}

func TestExploreRejectsUnresolvedPlaceholder(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	// The intent description itself contains a placeholder. Rendering must
	// detect the survivor instead of sending a broken prompt to the model.
	client.export = strings.Replace(exploreExport, "验证输入能否到达 eval 入口", "验证 {graph_yaml} 入口", 1)
	driver := &fakeExploreDriver{name: "native-test"}

	err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001")
	if err == nil || !strings.Contains(err.Error(), "{graph_yaml}") {
		t.Fatalf("RunExplore err = %v, want an unresolved placeholder error", err)
	}
	if len(driver.execTasks) != 0 {
		t.Fatalf("an unresolved prompt must not run the worker")
	}
	if len(client.releases) != 1 {
		t.Fatalf("releases = %d, want 1", len(client.releases))
	}
}

func TestExploreHeartbeatsWhileExecuting(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	client.heartbeatCh = make(chan struct{})
	driver := &fakeExploreDriver{name: "native-test"}
	driver.execute = func(context.Context, worker.Task) (worker.Result, error) {
		select {
		case <-client.heartbeatCh:
		case <-time.After(2 * time.Second):
			t.Errorf("runner did not heartbeat the claimed intent while executing")
		}
		return acceptedResult("ok", `{"accepted":true,"data":{"description":"ok"}}`), nil
	}
	deps := defaultExploreDeps
	deps.heartbeatInterval = 5 * time.Millisecond

	if err := runExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001", deps); err != nil {
		t.Fatalf("runExplore: %v", err)
	}
	if client.heartbeats.Load() == 0 {
		t.Fatal("expected at least one intent heartbeat")
	}
	if len(client.concludes) != 1 || len(client.releases) != 0 {
		t.Fatalf("want one conclusion and no release, got concludes=%d releases=%d",
			len(client.concludes), len(client.releases))
	}
}

func TestExploreVerdictUsesExplicitTarget(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	description := "沙箱命令复现了漏洞"
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult(description,
				`{"accepted":true,"data":{"description":"沙箱命令复现了漏洞","verdict":"support","evidence_ref":"file:///tmp/run.log","target":"f001"}}`), nil
		},
	}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	if len(client.verdicts) != 1 {
		t.Fatalf("AddVerdict calls = %d, want 1", len(client.verdicts))
	}
	v := client.verdicts[0]
	if v.ProjectID != "p1" || v.TargetID != "f001" || v.Verdict != "support" {
		t.Fatalf("verdict call = %+v, want p1/f001/support", v)
	}
	if v.Description != description {
		t.Fatalf("verdict description = %q, want the concluded description", v.Description)
	}
	if v.EvidenceRef != "file:///tmp/run.log" {
		t.Fatalf("evidence ref = %q, want file:///tmp/run.log", v.EvidenceRef)
	}
	if len(client.concludes) != 1 || len(client.releases) != 0 {
		t.Fatalf("want one conclusion and no release, got concludes=%d releases=%d",
			len(client.concludes), len(client.releases))
	}
}

func TestExploreVerdictDefaultsToExploredIntent(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("证据反驳了该方向",
				`{"accepted":true,"data":{"description":"证据反驳了该方向","verdict":"refute"}}`), nil
		},
	}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	if len(client.verdicts) != 1 {
		t.Fatalf("AddVerdict calls = %d, want 1", len(client.verdicts))
	}
	if got := client.verdicts[0].TargetID; got != "i001" {
		t.Fatalf("verdict target = %q, want the explored intent i001", got)
	}
}

func TestExploreVerdictUnknownTargetFallsBackToIntent(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("证据支持该方向",
				`{"accepted":true,"data":{"description":"证据支持该方向","verdict":"support","target":"f999"}}`), nil
		},
	}

	if err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	if len(client.verdicts) != 1 {
		t.Fatalf("AddVerdict calls = %d, want 1", len(client.verdicts))
	}
	if got := client.verdicts[0].TargetID; got != "i001" {
		t.Fatalf("verdict target = %q, want the explored intent fallback", got)
	}
}

func TestExploreInvalidVerdictReturnsErrorAfterFact(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("结论",
				`{"accepted":true,"data":{"description":"结论","verdict":"maybe"}}`), nil
		},
	}

	err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001")
	if !errors.Is(err, graph.ErrInvalid) {
		t.Fatalf("RunExplore err = %v, want graph.ErrInvalid", err)
	}
	if len(client.concludes) != 1 {
		t.Fatalf("the observation fact must be written before the verdict error, concludes=%d", len(client.concludes))
	}
	if len(client.verdicts) != 0 {
		t.Fatalf("an invalid verdict must not be written: %+v", client.verdicts)
	}
	if len(client.releases) != 0 {
		t.Fatalf("a post-conclusion verdict failure must not release: %+v", client.releases)
	}
}

func TestExploreVerdictWithoutWriterReturnsErrorAfterFact(t *testing.T) {
	exploreWorkdir(t)
	// A protocol-only client has no AddVerdict method.
	client := &fakeExploreClient{export: exploreExport}
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("结论",
				`{"accepted":true,"data":{"description":"结论","verdict":"support"}}`), nil
		},
	}

	err := RunExplore(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001")
	if err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Fatalf("RunExplore err = %v, want a missing verdict writer error", err)
	}
	if len(client.concludes) != 1 {
		t.Fatalf("the observation fact must be written before the verdict error, concludes=%d", len(client.concludes))
	}
	if len(client.releases) != 0 {
		t.Fatalf("a post-conclusion verdict failure must not release: %+v", client.releases)
	}
}

func TestExplorePromptRendersEveryPlaceholder(t *testing.T) {
	prompt, err := explorePrompt("yaml={graph_yaml} id={intent_id} desc={intent_description}",
		"facts: []", "i001", "验证入口")
	if err != nil {
		t.Fatalf("explorePrompt: %v", err)
	}
	want := "yaml=facts: [] id=i001 desc=验证入口"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}

	if _, err := explorePrompt("yaml={graph_yaml} desc={intent_description}", "facts: []", "i001", "{graph_yaml}"); err == nil ||
		!strings.Contains(err.Error(), "{graph_yaml}") {
		t.Fatalf("explorePrompt err = %v, want an unresolved placeholder", err)
	}
}

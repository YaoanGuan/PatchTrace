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
	ProjectID        string
	TargetID         string
	Verdict          string
	Description      string
	EvidenceSourceID string
	EvidenceRef      string
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

func (c *fakeExploreClient) ReleaseIntent(ctx context.Context, projectID, intentID, workerName string) (graph.Node, error) {
	c.releases = append(c.releases, exploreReleaseCall{IntentID: intentID, Worker: workerName})
	// Post-execute graph writes must run on a detached context: a cancelled
	// caller must not lose the release.
	if err := ctx.Err(); err != nil {
		return graph.Node{}, err
	}
	return graph.Node{ID: intentID, ProjectID: projectID, State: "open"}, c.releaseErr
}

func (c *fakeExploreClient) ConcludeIntent(ctx context.Context, projectID, intentID, workerName, description, factType string) (graph.Node, graph.Node, error) {
	c.concludes = append(c.concludes, exploreConcludeCall{
		IntentID: intentID, Worker: workerName, Description: description, FactType: factType,
	})
	// A cancelled caller context must never be used for the objective fact:
	// R9 requires the conclusion even when Execute was timed out.
	if err := ctx.Err(); err != nil {
		return graph.Node{}, graph.Node{}, err
	}
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

func (c *fakeVerdictExploreClient) AddVerdict(ctx context.Context, projectID, targetID, verdict, description, evidenceSourceID, evidenceRef string) (graph.Node, error) {
	c.verdicts = append(c.verdicts, exploreVerdictCall{
		ProjectID: projectID, TargetID: targetID, Verdict: verdict,
		Description: description, EvidenceSourceID: evidenceSourceID, EvidenceRef: evidenceRef,
	})
	if err := ctx.Err(); err != nil {
		return graph.Node{}, err
	}
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

type exploreExecCall struct {
	ProjectID string
	Request   runtime.ExecRequest
}

type fakeSandboxRuntime struct {
	mu     sync.Mutex
	calls  []exploreExecCall
	result runtime.ExecResult
	err    error
}

func (r *fakeSandboxRuntime) Exec(_ context.Context, projectID string, req runtime.ExecRequest) (runtime.ExecResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, exploreExecCall{ProjectID: projectID, Request: req})
	r.mu.Unlock()
	if r.err != nil {
		return r.result, r.err
	}
	return r.result, nil
}

func (r *fakeSandboxRuntime) recordedCalls() []exploreExecCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]exploreExecCall(nil), r.calls...)
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
	if got := rt.recordedCalls(); len(got) != 0 {
		t.Fatalf("a command-free result must not exec in the sandbox: %+v", got)
	}
}

// TestExploreRunsSandboxVerification is R9's core acceptance test: an accepted
// explore result that asks for a command runs it through the SandboxRuntime and
// records the observed output in the objective conclusion.
func TestExploreRunsSandboxVerification(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("已验证输入可到达 eval 入口",
				`{"accepted":true,"data":{"description":"已验证输入可到达 eval 入口","command":"curl -s http://target/x","workdir":"/workspace/src","verdict":"support"}}`), nil
		},
	}
	rt := &fakeSandboxRuntime{result: runtime.ExecResult{
		Stdout:   "HTTP/1.1 200 OK\neval sink reached\n",
		Stderr:   "connection warning\n",
		ExitCode: 0,
	}}

	if err := RunExplore(context.Background(), client, driver, rt, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}

	calls := rt.recordedCalls()
	if len(calls) != 1 {
		t.Fatalf("sandbox Exec calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.ProjectID != "p1" {
		t.Fatalf("exec project = %q, want p1", call.ProjectID)
	}
	if call.Request.Command != "curl -s http://target/x" || call.Request.Workdir != "/workspace/src" {
		t.Fatalf("exec request = %+v, want the model-provided command and workdir", call.Request)
	}
	if call.Request.Timeout != exploreExecTimeout {
		t.Fatalf("exec timeout = %v, want %v", call.Request.Timeout, exploreExecTimeout)
	}

	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	description := client.concludes[0].Description
	if !strings.HasPrefix(description, "已验证输入可到达 eval 入口") {
		t.Fatalf("conclusion = %q, want it to keep the worker description", description)
	}
	for _, want := range []string{
		"curl -s http://target/x", "退出码 0", "HTTP/1.1 200 OK", "eval sink reached", "connection warning",
	} {
		if !strings.Contains(description, want) {
			t.Fatalf("conclusion is missing %q:\n%s", want, description)
		}
	}
	if got := client.concludes[0].FactType; got != "observation" {
		t.Fatalf("fact type = %q, want observation", got)
	}
	if len(client.releases) != 0 {
		t.Fatalf("a verified conclusion must not release: %+v", client.releases)
	}
	if len(driver.concludeTasks) != 0 {
		t.Fatalf("a verified conclusion must not ask the driver to conclude: %+v", driver.concludeTasks)
	}
	if len(client.verdicts) != 1 {
		t.Fatalf("AddVerdict calls = %d, want 1", len(client.verdicts))
	}
	verdict := client.verdicts[0]
	if verdict.Verdict != "support" || verdict.TargetID != "i001" {
		t.Fatalf("verdict = %+v, want support/i001", verdict)
	}
	// The verdict must cite the observation fact ConcludeIntent wrote (f100 in
	// the fake) so it has a provenance chain back to origin (R10/I1).
	if verdict.EvidenceSourceID != "f100" {
		t.Fatalf("verdict evidence source = %q, want the observation fact f100", verdict.EvidenceSourceID)
	}
	for _, want := range []string{"sandbox: command=", "exit=0", "/workspace/src"} {
		if !strings.Contains(verdict.EvidenceRef, want) {
			t.Fatalf("evidence ref = %q, want it to cite the sandbox execution (%q)", verdict.EvidenceRef, want)
		}
	}
}

func TestExploreSandboxOutputIsTruncated(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name: "native-test",
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("长输出结论",
				`{"accepted":true,"data":{"description":"长输出结论","command":"cat big.log"}}`), nil
		},
	}
	rt := &fakeSandboxRuntime{result: runtime.ExecResult{
		Stdout:   strings.Repeat("A", exploreMaxOutputRunes+50),
		Stderr:   strings.Repeat("B", exploreMaxOutputRunes+50),
		ExitCode: 0,
	}}

	if err := RunExplore(context.Background(), client, driver, rt, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	description := client.concludes[0].Description
	if !strings.Contains(description, "…(输出已截断)") {
		t.Fatalf("conclusion = %q, want truncated output markers", description)
	}
	if strings.Count(description, "A") > exploreMaxOutputRunes || strings.Count(description, "B") > exploreMaxOutputRunes {
		t.Fatalf("conclusion kept too much output: A=%d B=%d", strings.Count(description, "A"), strings.Count(description, "B"))
	}
}

// TestExploreSandboxVerificationFailureStillWritesFact covers R9's failure
// clause: a failed sandbox command still lands an objective observation, and
// the error is surfaced to the dispatcher.
func TestExploreSandboxVerificationFailureStillWritesFact(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("初步判断可触发",
				`{"accepted":true,"data":{"description":"初步判断可触发","command":"run-poc.sh","verdict":"support"}}`), nil
		},
	}
	rt := &fakeSandboxRuntime{err: errors.New("container not running")}

	err := RunExplore(context.Background(), client, driver, rt, exploreProject(), "i001")
	if err == nil || !strings.Contains(err.Error(), "container not running") {
		t.Fatalf("RunExplore err = %v, want the sandbox exec failure", err)
	}

	calls := rt.recordedCalls()
	if len(calls) != 1 || calls[0].Request.Command != "run-poc.sh" {
		t.Fatalf("sandbox Exec calls = %+v, want one run-poc.sh call", calls)
	}
	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	description := client.concludes[0].Description
	if !strings.Contains(description, "run-poc.sh") || !strings.Contains(description, "container not running") {
		t.Fatalf("failure conclusion = %q, want the command and the failure reason", description)
	}
	if got := client.concludes[0].FactType; got != "observation" {
		t.Fatalf("fact type = %q, want observation", got)
	}
	if len(client.releases) != 0 {
		t.Fatalf("a concluded sandbox failure must not release: %+v", client.releases)
	}
	if len(client.verdicts) != 0 {
		t.Fatalf("a failed verification must not record a verdict: %+v", client.verdicts)
	}
}

func TestExploreSkipsSandboxWithoutCommandOrRuntime(t *testing.T) {
	t.Run("no command", func(t *testing.T) {
		exploreWorkdir(t)
		client := newExploreClient()
		driver := &fakeExploreDriver{
			name: "native-test",
			execute: func(context.Context, worker.Task) (worker.Result, error) {
				return acceptedResult("无需命令的结论",
					`{"accepted":true,"data":{"description":"无需命令的结论"}}`), nil
			},
		}
		rt := &fakeSandboxRuntime{}

		if err := RunExplore(context.Background(), client, driver, rt, exploreProject(), "i001"); err != nil {
			t.Fatalf("RunExplore: %v", err)
		}
		if got := rt.recordedCalls(); len(got) != 0 {
			t.Fatalf("a result without data.command must not exec: %+v", got)
		}
		if got := client.concludes[0].Description; got != "无需命令的结论" {
			t.Fatalf("conclusion = %q, want the unchanged worker description", got)
		}
	})

	t.Run("nil runtime", func(t *testing.T) {
		exploreWorkdir(t)
		client := newExploreClient()
		driver := &fakeExploreDriver{
			name: "native-test",
			execute: func(context.Context, worker.Task) (worker.Result, error) {
				return acceptedResult("无运行时结论",
					`{"accepted":true,"data":{"description":"无运行时结论","command":"true"}}`), nil
			},
		}

		if err := RunExplore(context.Background(), client, driver, nil, exploreProject(), "i001"); err != nil {
			t.Fatalf("RunExplore: %v", err)
		}
		if len(client.concludes) != 1 || client.concludes[0].Description != "无运行时结论" {
			t.Fatalf("conclusion = %+v, want the unchanged worker description", client.concludes)
		}
	})

	t.Run("verdict without sandbox execution", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			command string
			rt      runtime.SandboxRuntime
		}{
			{name: "no command", rt: &fakeSandboxRuntime{}},
			{name: "nil runtime", command: `,"command":"true"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				exploreWorkdir(t)
				client := newExploreClient()
				driver := &fakeExploreDriver{
					name: "native-test",
					execute: func(context.Context, worker.Task) (worker.Result, error) {
						return acceptedResult("沙箱未执行的结论",
							`{"accepted":true,"data":{"description":"沙箱未执行的结论","verdict":"support"`+tc.command+`}}`), nil
					},
				}
				onVerdictCalled := false
				err := RunExploreWithOptions(context.Background(), client, driver, tc.rt, exploreProject(), "i001",
					ExploreOptions{OnVerdict: func(context.Context, string, string) error {
						onVerdictCalled = true
						return nil
					}})
				if err != nil {
					t.Fatalf("RunExplore: %v", err)
				}
				if len(client.verdicts) != 0 {
					t.Fatalf("a verdict without sandbox execution must be dropped: %+v", client.verdicts)
				}
				if onVerdictCalled {
					t.Fatal("the OnVerdict rule-feedback hook must not run for a dropped verdict")
				}
				if len(client.concludes) != 1 || client.concludes[0].Description != "沙箱未执行的结论" {
					t.Fatalf("conclusion = %+v, want the observation fact without a verification block", client.concludes)
				}
			})
		}
	})
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

	rt := &fakeSandboxRuntime{}
	if err := RunExplore(parent, client, driver, rt, exploreProject(), "i001"); err != nil {
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
	if got := rt.recordedCalls(); len(got) != 0 {
		t.Fatalf("a timeout fallback must not run sandbox commands: %+v", got)
	}
}

// TestExploreTimeoutReleaseUsesDetachedContext proves a cancelled caller
// context cannot lose the release after a failed post-execute graph write: the
// fake client rejects any already-cancelled context, so a release only succeeds
// when RunExplore detaches it.
func TestExploreTimeoutReleaseUsesDetachedContext(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	client.concludeErr = errors.New("graph write failed")
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(ctx context.Context, _ worker.Task) (worker.Result, error) {
			return worker.Result{}, ctx.Err()
		},
		conclude: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("超时收尾结论", `{"accepted":true,"data":{"description":"超时收尾结论"}}`), nil
		},
	}
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunExplore(parent, client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001")
	if err == nil {
		t.Fatal("a failed ConcludeIntent must surface an error")
	}
	if !strings.Contains(err.Error(), "conclude intent") {
		t.Fatalf("RunExplore err = %v, want the conclude failure", err)
	}
	if strings.Contains(err.Error(), "release intent") {
		t.Fatalf("release must succeed on a detached context, err = %v", err)
	}
	if len(client.releases) != 1 || client.releases[0].IntentID != "i001" {
		t.Fatalf("releases = %+v, want one release of i001 despite the cancelled caller", client.releases)
	}
}

// TestExploreVerdictWriteUsesDetachedContext proves the optional verdict fact is
// also written on a detached context after Execute timed out.
func TestExploreVerdictWriteUsesDetachedContext(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(ctx context.Context, _ worker.Task) (worker.Result, error) {
			return acceptedResult("结论成立",
				`{"accepted":true,"data":{"description":"结论成立","command":"true","verdict":"support"}}`), nil
		},
	}
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	// Execute ignores the cancelled context in this fake and still reports an
	// accepted result; the graph writes afterwards must not inherit it.
	if err := RunExplore(parent, client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001"); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	if len(client.concludes) != 1 {
		t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
	}
	if len(client.verdicts) != 1 {
		t.Fatalf("AddVerdict calls = %d, want 1", len(client.verdicts))
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

func TestExploreRunnerUsesConfiguredConcludeTimeout(t *testing.T) {
	run := func(t *testing.T, opts ExploreOptions, want time.Duration) {
		t.Helper()
		exploreWorkdir(t)
		client := newExploreClient()
		var observed time.Duration
		driver := &fakeExploreDriver{
			name:             "native-test",
			supportsConclude: true,
			execute: func(context.Context, worker.Task) (worker.Result, error) {
				return worker.Result{}, errors.New("execute timed out")
			},
			conclude: func(_ context.Context, task worker.Task) (worker.Result, error) {
				observed = task.ConcludeTimeout
				return acceptedResult("收尾结论",
					`{"accepted":true,"data":{"description":"收尾结论"}}`), nil
			},
		}

		if err := RunExploreWithOptions(context.Background(), client, driver, &fakeSandboxRuntime{}, exploreProject(), "i001", opts); err != nil {
			t.Fatalf("RunExploreWithOptions: %v", err)
		}
		if observed != want {
			t.Fatalf("conclude timeout = %v, want %v", observed, want)
		}
		if len(client.concludes) != 1 {
			t.Fatalf("ConcludeIntent calls = %d, want 1", len(client.concludes))
		}
	}

	t.Run("configured", func(t *testing.T) {
		run(t, ExploreOptions{ConcludeTimeout: 7 * time.Minute}, 7*time.Minute)
	})
	t.Run("default", func(t *testing.T) {
		run(t, ExploreOptions{}, exploreConcludeTimeout)
	})
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
				`{"accepted":true,"data":{"description":"沙箱命令复现了漏洞","command":"cat run.log","verdict":"support","evidence_ref":"file:///tmp/run.log","target":"f001"}}`), nil
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
	if !strings.HasPrefix(v.Description, description) {
		t.Fatalf("verdict description = %q, want the concluded description", v.Description)
	}
	if !strings.HasPrefix(v.EvidenceRef, "file:///tmp/run.log") || !strings.Contains(v.EvidenceRef, "sandbox: command=") {
		t.Fatalf("evidence ref = %q, want the model reference plus the sandbox execution", v.EvidenceRef)
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
				`{"accepted":true,"data":{"description":"证据反驳了该方向","command":"curl -s http://target/nope","verdict":"refute"}}`), nil
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
				`{"accepted":true,"data":{"description":"证据支持该方向","command":"true","verdict":"support","target":"f999"}}`), nil
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
				`{"accepted":true,"data":{"description":"结论","command":"true","verdict":"maybe"}}`), nil
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
				`{"accepted":true,"data":{"description":"结论","command":"true","verdict":"support"}}`), nil
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

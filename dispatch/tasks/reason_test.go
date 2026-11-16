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
)

// --- fakes -----------------------------------------------------------------

type reasonClaim struct {
	Worker  string
	Trigger string
	Lease   time.Duration
}

type reasonComplete struct {
	From        []string
	Description string
	Worker      string
}

// fakeReasonClient embeds graphserver.Client so an unexpected protocol call
// panics loudly. It records the reason-lease lifecycle and every graph write.
type fakeReasonClient struct {
	graphserver.Client

	export    string
	exportErr error

	claimErr   error
	claims     []reasonClaim
	claimCalls int

	heartbeats  atomic.Int64
	heartbeatCh chan struct{}
	heartbeat   sync.Once
	hbErr       error

	releases   atomic.Int64
	releaseErr error

	completes   []reasonComplete
	completeErr error

	declares   []graph.Node
	declareErr error
}

func (c *fakeReasonClient) ClaimReason(_ context.Context, projectID, workerName, trigger string, lease time.Duration) (graph.Project, error) {
	c.claimCalls++
	c.claims = append(c.claims, reasonClaim{Worker: workerName, Trigger: trigger, Lease: lease})
	if c.claimErr != nil {
		return graph.Project{}, c.claimErr
	}
	return graph.Project{ID: projectID, Status: "active"}, nil
}

func (c *fakeReasonClient) HeartbeatReason(_ context.Context, projectID, _ string, _ time.Duration) (graph.Project, error) {
	c.heartbeats.Add(1)
	if c.heartbeatCh != nil {
		c.heartbeat.Do(func() { close(c.heartbeatCh) })
	}
	return graph.Project{ID: projectID, Status: "active"}, c.hbErr
}

func (c *fakeReasonClient) ReleaseReason(_ context.Context, projectID, _ string) (graph.Project, error) {
	c.releases.Add(1)
	return graph.Project{ID: projectID}, c.releaseErr
}

func (c *fakeReasonClient) ExportYAML(context.Context, string) (string, error) {
	if c.exportErr != nil {
		return "", c.exportErr
	}
	return c.export, nil
}

func (c *fakeReasonClient) Complete(_ context.Context, projectID string, from []string, description, workerName string) (graph.Project, error) {
	c.completes = append(c.completes, reasonComplete{From: from, Description: description, Worker: workerName})
	return graph.Project{ID: projectID, Status: "completed"}, c.completeErr
}

func (c *fakeReasonClient) DeclareIntent(_ context.Context, projectID string, n graph.Node) (graph.Node, error) {
	c.declares = append(c.declares, n)
	return graph.Node{
		ID:        "i100",
		ProjectID: projectID,
		Kind:      "intent",
		Content:   n.Content,
		From:      n.From,
		Creator:   n.Creator,
		State:     "open",
	}, c.declareErr
}

// fakeGapClient adds the optional GapScore capability on top of the plain fake
// client, so RunReason's default dependency ranks gaps without a graph.Store.
type fakeGapClient struct {
	*fakeReasonClient
	gaps    []graph.Gap
	gapsErr error
}

func (c *fakeGapClient) GapScore(context.Context, string) ([]graph.Gap, error) {
	return c.gaps, c.gapsErr
}

// newReasonClient returns a gap-capable fake. Tests that need the missing
// scorer path construct fakeReasonClient directly.
func newReasonClient() *fakeGapClient {
	return &fakeGapClient{fakeReasonClient: &fakeReasonClient{export: reasonExport}}
}

type fakeReasonDriver struct {
	name    string
	result  worker.Result
	err     error
	execute func(context.Context, worker.Task) (worker.Result, error)
	tasks   []worker.Task
}

func (d *fakeReasonDriver) Name() string                  { return d.name }
func (d *fakeReasonDriver) Supports(worker.TaskType) bool { return true }
func (d *fakeReasonDriver) HealthCheck(context.Context) error {
	return nil
}
func (d *fakeReasonDriver) Execute(ctx context.Context, t worker.Task) (worker.Result, error) {
	d.tasks = append(d.tasks, t)
	if d.execute != nil {
		return d.execute(ctx, t)
	}
	return d.result, d.err
}
func (d *fakeReasonDriver) Conclude(context.Context, worker.Task) (worker.Result, error) {
	return worker.Result{}, errors.New("reason must not call driver Conclude")
}
func (d *fakeReasonDriver) SupportsConclude() bool { return false }

// --- fixtures --------------------------------------------------------------

const reasonExport = `project:
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
  state: open
  worker: ""
  creator: worker-x
  created_at: "2026-01-01 00:00:00"
  concluded_at: null
`

// reasonWorkdir creates a temporary cwd containing the shipped prompt layout
// and changes into it for the duration of the test.
func reasonWorkdir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	promptDir := filepath.Join(dir, "prompts", "graph", "default")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	contract := "# 任务\n\n```yaml\n{graph_yaml}\n```\n\n事实编号：\n\n```json\n{fact_ids}\n```\n\n开放意图：\n\n```json\n{open_intents}\n```\n"
	if err := os.WriteFile(filepath.Join(promptDir, "reason.md"), []byte(contract), 0o644); err != nil {
		t.Fatalf("write reason prompt: %v", err)
	}
	t.Chdir(dir)
}

func reasonProject() graph.Project {
	return graph.Project{
		ID:               "p1",
		Title:            "demo",
		Origin:           "/tmp/patch.diff",
		Goal:             "确认补丁修复的漏洞可利用",
		Status:           "active",
		BootstrapEnabled: true,
	}
}

func reasonPointer(s string) *string { return &s }

// --- tests -----------------------------------------------------------------

func TestReasonCompletesWhenGoalCovered(t *testing.T) {
	reasonWorkdir(t)
	description := "f001 已足以证明目标达成"
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted:     true,
		Complete:     reasonPointer(description),
		CompleteFrom: []string{"f001"},
	}}

	if err := RunReason(context.Background(), client, driver, reasonProject()); err != nil {
		t.Fatalf("RunReason: %v", err)
	}
	if len(client.completes) != 1 {
		t.Fatalf("Complete calls = %d, want 1", len(client.completes))
	}
	call := client.completes[0]
	if !reflect.DeepEqual(call.From, []string{"f001"}) {
		t.Fatalf("Complete from = %v, want [f001]", call.From)
	}
	if call.Description != description {
		t.Fatalf("Complete description = %q, want %q", call.Description, description)
	}
	if call.Worker != "native-test" {
		t.Fatalf("Complete worker = %q, want the driver name", call.Worker)
	}
	if len(client.declares) != 0 {
		t.Fatalf("a completion must not declare intents: %+v", client.declares)
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
	if client.claimCalls != 1 || client.claims[0].Trigger != "reason" || client.claims[0].Lease != 5*time.Minute || client.claims[0].Worker != "native-test" {
		t.Fatalf("claim = %+v, want trigger=reason lease=5m worker=native-test", client.claims)
	}
	if len(driver.tasks) != 1 {
		t.Fatalf("driver tasks = %d, want 1", len(driver.tasks))
	}
	if driver.tasks[0].Type != string(worker.TaskReason) || driver.tasks[0].ProjectID != "p1" {
		t.Fatalf("task = %+v, want a reason task for p1", driver.tasks[0])
	}
	if !strings.Contains(driver.tasks[0].Prompt, "验证输入能否到达 eval 入口") {
		t.Fatalf("prompt is missing the open intent: %q", driver.tasks[0].Prompt)
	}
}

func TestReasonDeclaresIntentWithValidFrom(t *testing.T) {
	reasonWorkdir(t)
	description := "验证 f001 的可利用路径"
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Intent:   reasonPointer(description),
		Intents: []worker.IntentSpec{
			{From: []string{"f001"}, Description: description},
		},
	}}

	if err := RunReason(context.Background(), client, driver, reasonProject()); err != nil {
		t.Fatalf("RunReason: %v", err)
	}
	if len(client.completes) != 0 {
		t.Fatalf("an intent must not complete the project: %+v", client.completes)
	}
	if len(client.declares) != 1 {
		t.Fatalf("DeclareIntent calls = %d, want 1", len(client.declares))
	}
	node := client.declares[0]
	if node.Content != description {
		t.Fatalf("declared content = %q, want %q", node.Content, description)
	}
	if !reflect.DeepEqual(node.From, []string{"f001"}) {
		t.Fatalf("declared from = %v, want [f001]", node.From)
	}
	if node.Creator != "native-test" {
		t.Fatalf("declared creator = %q, want the driver name", node.Creator)
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonRejectsIntentWithUnknownFrom(t *testing.T) {
	reasonWorkdir(t)
	description := "引用不存在的事实"
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Intent:   reasonPointer(description),
		Intents: []worker.IntentSpec{
			{From: []string{"f999"}, Description: description},
		},
	}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if !errors.Is(err, graph.ErrInvalid) {
		t.Fatalf("RunReason err = %v, want graph.ErrInvalid", err)
	}
	if len(client.declares) != 0 || len(client.completes) != 0 {
		t.Fatalf("an unknown from fact must write nothing: declares=%d completes=%d",
			len(client.declares), len(client.completes))
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1 even on failure", client.releases.Load())
	}
}

func TestReasonRejectsCompleteWithUnknownFrom(t *testing.T) {
	reasonWorkdir(t)
	description := "无法证明目标达成"
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted:     true,
		Complete:     reasonPointer(description),
		CompleteFrom: []string{"f999"},
	}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if !errors.Is(err, graph.ErrInvalid) {
		t.Fatalf("RunReason err = %v, want graph.ErrInvalid", err)
	}
	if len(client.completes) != 0 || len(client.declares) != 0 {
		t.Fatalf("an unknown complete fact must write nothing: completes=%d declares=%d",
			len(client.completes), len(client.declares))
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1 even on failure", client.releases.Load())
	}
}

func TestReasonPromptIncludesGapsAndHints(t *testing.T) {
	reasonWorkdir(t)
	gaps := []string{"i001：未满足阻塞 2，距原点深度 1；验证输入能否到达 eval 入口"}
	hints := []string{"先看输入解析路径"}

	prompt := reasonPrompt(graph.Project{ID: "p1"}, "facts:\n- id: f001\n",
		[]string{"origin", "goal", "f001"},
		[]string{"i001：验证输入能否到达 eval 入口"},
		gaps, hints)

	for _, want := range []string{
		"facts:\n- id: f001",
		"origin", "goal", "f001",
		"i001", "验证输入能否到达 eval 入口",
		"未满足阻塞 2", "距原点深度 1",
		"先看输入解析路径",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, prompt)
		}
	}
	for _, placeholder := range []string{"{graph_yaml}", "{fact_ids}", "{open_intents}"} {
		if strings.Contains(prompt, placeholder) {
			t.Fatalf("prompt still contains %s:\n%s", placeholder, prompt)
		}
	}
}

func TestReasonPromptKeepsTopFiveGaps(t *testing.T) {
	reasonWorkdir(t)
	gaps := []string{"gapA", "gapB", "gapC", "gapD", "gapE", "gapF"}
	prompt := reasonPrompt(graph.Project{ID: "p1"}, "facts: []", nil, nil, gaps, nil)
	for _, want := range []string{"gapA", "gapB", "gapC", "gapD", "gapE"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing top-5 gap %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "gapF") {
		t.Fatalf("prompt must truncate gaps to top-5:\n%s", prompt)
	}
}

func TestReasonNoopWritesNothing(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

	if err := RunReason(context.Background(), client, driver, reasonProject()); err != nil {
		t.Fatalf("a no-op reason must return nil, got %v", err)
	}
	if len(client.declares) != 0 || len(client.completes) != 0 {
		t.Fatalf("an empty result must write nothing: declares=%d completes=%d",
			len(client.declares), len(client.completes))
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
	if len(driver.tasks) != 1 {
		t.Fatalf("driver tasks = %d, want 1", len(driver.tasks))
	}
}

func TestReasonSkipsWhenLeaseHeldOrProjectInactive(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "held", err: fmt.Errorf("%w: reason is held by another worker", graph.ErrConflict)},
		{name: "missing", err: fmt.Errorf("%w: project p1", graph.ErrNotFound)},
		{name: "inactive", err: fmt.Errorf("%w: project p1 is stopped", graph.ErrForbidden)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reasonWorkdir(t)
			client := &fakeReasonClient{export: reasonExport, claimErr: tc.err}
			driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

			if err := RunReason(context.Background(), client, driver, reasonProject()); err != nil {
				t.Fatalf("a skip must return nil, got %v", err)
			}
			if len(driver.tasks) != 0 {
				t.Fatalf("a skip must not run the worker, got %d tasks", len(driver.tasks))
			}
			if client.releases.Load() != 0 {
				t.Fatalf("a failed claim must not release, got %d releases", client.releases.Load())
			}
			if len(client.completes)+len(client.declares) != 0 {
				t.Fatalf("a skip must write nothing")
			}
		})
	}
}

func TestReasonSurfacesClaimError(t *testing.T) {
	reasonWorkdir(t)
	client := &fakeReasonClient{export: reasonExport, claimErr: errors.New("grant store unavailable")}
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "grant store unavailable") {
		t.Fatalf("RunReason err = %v, want the claim failure", err)
	}
	if len(driver.tasks) != 0 || client.releases.Load() != 0 {
		t.Fatalf("a failed claim must not execute or release")
	}
}

func TestReasonHeartbeatsWhileExecuting(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	client.heartbeatCh = make(chan struct{})
	driver := &fakeReasonDriver{name: "native-test"}
	driver.execute = func(context.Context, worker.Task) (worker.Result, error) {
		select {
		case <-client.heartbeatCh:
		case <-time.After(2 * time.Second):
			t.Errorf("runner did not heartbeat the reason lease while executing")
		}
		return worker.Result{Accepted: true}, nil
	}
	deps := defaultReasonDeps
	deps.heartbeatInterval = 5 * time.Millisecond

	if err := runReason(context.Background(), client, driver, reasonProject(), deps); err != nil {
		t.Fatalf("runReason: %v", err)
	}
	if client.heartbeats.Load() == 0 {
		t.Fatal("expected at least one reason heartbeat")
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonReleasesOnDriverFailure(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", err: errors.New("worker exploded")}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "worker exploded") {
		t.Fatalf("RunReason err = %v, want the driver failure", err)
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1 after a driver failure", client.releases.Load())
	}
	if len(client.completes)+len(client.declares) != 0 {
		t.Fatalf("a failed execution must write nothing")
	}
}

func TestReasonJoinsReleaseError(t *testing.T) {
	reasonWorkdir(t)
	client := &fakeReasonClient{export: reasonExport, releaseErr: errors.New("release denied")}
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "release denied") {
		t.Fatalf("RunReason err = %v, want the release failure joined", err)
	}
}

func TestReasonPromptFallbackKeepsGraphContext(t *testing.T) {
	original := reasonPromptLoader
	t.Cleanup(func() { reasonPromptLoader = original })
	reasonPromptLoader = func(string, worker.TaskType, bool) (string, error) {
		return "", errors.New("prompt file missing")
	}

	prompt := reasonPrompt(graph.Project{ID: "p1"}, "facts:\n- id: f001\n",
		[]string{"origin", "f001"},
		[]string{"i001：验证输入能否到达 eval 入口"},
		nil, []string{"先看输入解析路径"})

	for _, want := range []string{
		"facts:\n- id: f001",
		"origin",
		"i001：验证输入能否到达 eval 入口",
		"先看输入解析路径",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("fallback prompt is missing %q:\n%s", want, prompt)
		}
	}
	for _, placeholder := range []string{"{graph_yaml}", "{fact_ids}", "{open_intents}"} {
		if strings.Contains(prompt, placeholder) {
			t.Fatalf("fallback prompt still contains %s:\n%s", placeholder, prompt)
		}
	}
}

func TestRenderReasonPromptRejectsLeftoverPlaceholder(t *testing.T) {
	_, err := renderReasonPrompt(
		"graph={graph_yaml} facts={fact_ids} intents={open_intents}",
		"facts: []",
		[]string{"{graph_yaml}"},
		nil, nil, nil,
	)
	if err == nil || !strings.Contains(err.Error(), "{graph_yaml}") {
		t.Fatalf("renderReasonPrompt err = %v, want an unresolved {graph_yaml}", err)
	}
}

func TestReasonRunnerToleratesClientWithoutGapScore(t *testing.T) {
	reasonWorkdir(t)
	// fakeReasonClient deliberately has no GapScore: this is the shape of a
	// protocol-only client, whose HTTP API exposes no edge endpoint.
	client := &fakeReasonClient{export: reasonExport}
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted:     true,
		Complete:     reasonPointer("目标已覆盖"),
		CompleteFrom: []string{"f001"},
	}}

	if err := RunReason(context.Background(), client, driver, reasonProject()); err != nil {
		t.Fatalf("RunReason without a gap scorer: %v", err)
	}
	if len(driver.tasks) != 1 {
		t.Fatalf("driver tasks = %d, want 1", len(driver.tasks))
	}
	if !strings.Contains(driver.tasks[0].Prompt, "## 缺口排序（top-5）\n\n```json\n[]\n```") {
		t.Fatalf("prompt must render an empty gap ranking:\n%s", driver.tasks[0].Prompt)
	}
	if len(client.completes) != 1 {
		t.Fatalf("the result branch must still complete, completes=%d", len(client.completes))
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonRejectsRefusedResult(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: false}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("RunReason err = %v, want a refused-task error", err)
	}
	if len(client.completes) != 0 || len(client.declares) != 0 {
		t.Fatalf("a refused result must write nothing: completes=%d declares=%d",
			len(client.completes), len(client.declares))
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonSurfacesExportError(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	client.exportErr = errors.New("graph unavailable")
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "graph unavailable") {
		t.Fatalf("RunReason err = %v, want the export failure", err)
	}
	if len(driver.tasks) != 0 {
		t.Fatalf("an export failure must not run the worker, got %d tasks", len(driver.tasks))
	}
	if len(client.completes)+len(client.declares) != 0 {
		t.Fatalf("an export failure must write nothing")
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonSurfacesGapScoreError(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	client.gapsErr = errors.New("gap scanner exploded")
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "gap scanner exploded") {
		t.Fatalf("RunReason err = %v, want the gap-scoring failure", err)
	}
	if len(driver.tasks) != 0 {
		t.Fatalf("a gap-scoring failure must not run the worker, got %d tasks", len(driver.tasks))
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonSurfacesCompleteError(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	client.completeErr = errors.New("complete denied")
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted:     true,
		Complete:     reasonPointer("目标已覆盖"),
		CompleteFrom: []string{"f001"},
	}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "complete denied") {
		t.Fatalf("RunReason err = %v, want the complete failure", err)
	}
	if len(client.completes) != 1 {
		t.Fatalf("Complete calls = %d, want the attempted write", len(client.completes))
	}
	if len(client.declares) != 0 {
		t.Fatalf("a completion must not declare intents: %+v", client.declares)
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonSurfacesDeclareError(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	client.declareErr = errors.New("declare denied")
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{
		Accepted: true,
		Intent:   reasonPointer("验证 f001 的可利用路径"),
		Intents: []worker.IntentSpec{
			{From: []string{"f001"}, Description: "验证 f001 的可利用路径"},
		},
	}}

	err := RunReason(context.Background(), client, driver, reasonProject())
	if err == nil || !strings.Contains(err.Error(), "declare denied") {
		t.Fatalf("RunReason err = %v, want the declare failure", err)
	}
	if len(client.declares) != 1 {
		t.Fatalf("DeclareIntent calls = %d, want the attempted write", len(client.declares))
	}
	if len(client.completes) != 0 {
		t.Fatalf("an intent must not complete the project: %+v", client.completes)
	}
	if client.releases.Load() != 1 {
		t.Fatalf("ReleaseReason calls = %d, want 1", client.releases.Load())
	}
}

func TestReasonRunnerPromptUsesGapRanking(t *testing.T) {
	reasonWorkdir(t)
	client := newReasonClient()
	client.gaps = []graph.Gap{
		{IntentID: "i001", Description: "最近的缺口", OpenBlockers: 0, DepthFromOrigin: 1},
		{IntentID: "i002", Description: "第二个缺口", OpenBlockers: 1, DepthFromOrigin: 2},
	}
	driver := &fakeReasonDriver{name: "native-test", result: worker.Result{Accepted: true}}

	if err := RunReason(context.Background(), client, driver, reasonProject()); err != nil {
		t.Fatalf("RunReason: %v", err)
	}
	if len(driver.tasks) != 1 {
		t.Fatalf("driver tasks = %d, want 1", len(driver.tasks))
	}
	prompt := driver.tasks[0].Prompt
	for _, want := range []string{"最近的缺口", "第二个缺口", "未满足阻塞 1", "距原点深度 2", "先看输入解析路径"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, prompt)
		}
	}
}

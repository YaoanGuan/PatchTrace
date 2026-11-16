package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// --- ParseResult contract ---

func TestParseResultAcceptsExploreFact(t *testing.T) {
	r, err := ParseResult(`{"accepted":true,"data":{"description":"/api/v1/validate/code 存在未授权代码执行"}}`, TaskExplore)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if !r.Accepted {
		t.Fatal("expected result to be accepted")
	}
	if r.Description == "" {
		t.Fatal("expected non-empty description")
	}
	if r.Raw == "" {
		t.Fatal("expected raw output to be preserved")
	}
}

func TestParseResultRejectsRefusal(t *testing.T) {
	r, err := ParseResult(`{"accepted":false,"reason":"policy_refusal"}`, TaskExplore)
	if err != nil {
		t.Fatalf("refusal must not be a parse error, got: %v", err)
	}
	if r.Accepted {
		t.Fatal("refusal must not be accepted")
	}
	if r.Description != "" || r.Fact != nil || r.Complete != nil || r.Intent != nil || len(r.Intents) != 0 {
		t.Fatal("refusal must not carry writable result fields")
	}
}

func TestParseResultRejectsMissingDescription(t *testing.T) {
	cases := []string{
		`{"accepted":true,"data":{}}`,
		`{"accepted":true,"data":{"description":"   "}}`,
		`{"accepted":true}`,
	}
	for _, raw := range cases {
		if _, err := ParseResult(raw, TaskExplore); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestParseResultAcceptsBootstrapIntents(t *testing.T) {
	raw := "```json\n" + `{"accepted":true,"data":{"fact":{"description":"补丁在 /validate 前新增鉴权检查"},"intents":[{"from":["f001"],"description":"验证未授权路径"},{"from":["f001","f002"],"description":"验证命令注入"}]}}` + "\n```"
	r, err := ParseResult(raw, TaskBootstrap)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if !r.Accepted || r.Fact == nil || *r.Fact == "" {
		t.Fatalf("expected accepted result with fact, got %+v", r)
	}
	if len(r.Intents) != 2 {
		t.Fatalf("expected 2 intents, got %d", len(r.Intents))
	}
	if len(r.Intents[0].From) != 1 || r.Intents[0].From[0] != "f001" || r.Intents[0].Description != "验证未授权路径" {
		t.Fatalf("unexpected first intent: %+v", r.Intents[0])
	}
	if len(r.Intents[1].From) != 2 || r.Intents[1].From[1] != "f002" {
		t.Fatalf("unexpected second intent: %+v", r.Intents[1])
	}
}

func TestParseResultStripsJSONFence(t *testing.T) {
	raw := "```json\n{\"accepted\":true,\"data\":{\"description\":\"fenced\"}}\n```"
	r, err := ParseResult(raw, TaskExplore)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if r.Description != "fenced" {
		t.Fatalf("expected fenced payload to parse, got %q", r.Description)
	}
}

func TestParseResultRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseResult("not json at all", TaskExplore); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseResultRejectsMissingAccepted(t *testing.T) {
	if _, err := ParseResult(`{"data":{"description":"x"}}`, TaskExplore); err == nil {
		t.Fatal("expected error when accepted is missing")
	}
}

func TestParseResultReasonComplete(t *testing.T) {
	r, err := ParseResult(`{"accepted":true,"data":{"complete":{"from":["f001"],"description":"证据链已覆盖目标"}}}`, TaskReason)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if r.Complete == nil || *r.Complete != "证据链已覆盖目标" {
		t.Fatalf("expected complete description, got %+v", r)
	}
	if len(r.Intents) != 0 {
		t.Fatalf("complete must not produce intents: %+v", r.Intents)
	}
}

func TestParseResultReasonCompleteFrom(t *testing.T) {
	r, err := ParseResult(`{"accepted":true,"data":{"complete":{"from":["f001","f002"],"description":"证据链已覆盖目标"}}}`, TaskReason)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if !reflect.DeepEqual(r.CompleteFrom, []string{"f001", "f002"}) {
		t.Fatalf("CompleteFrom = %v, want [f001 f002]", r.CompleteFrom)
	}
	if r.Complete == nil || *r.Complete != "证据链已覆盖目标" {
		t.Fatalf("complete description = %v", r.Complete)
	}

	if _, err := ParseResult(`{"accepted":true,"data":{"complete":{"description":"缺少来源"}}}`, TaskReason); err == nil {
		t.Fatal("expected error when complete.from is missing")
	}
	if _, err := ParseResult(`{"accepted":true,"data":{"complete":{"from":[],"description":"空来源"}}}`, TaskReason); err == nil {
		t.Fatal("expected error when complete.from is empty")
	}
}

func TestParseResultReasonNoop(t *testing.T) {
	r, err := ParseResult(`{"accepted":true,"data":{}}`, TaskReason)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if !r.Accepted || r.Complete != nil || r.Intent != nil || len(r.Intents) != 0 {
		t.Fatalf("expected empty accepted result, got %+v", r)
	}
}

func TestParseResultReasonNormalizesSingularIntent(t *testing.T) {
	r, err := ParseResult(`{"accepted":true,"data":{"intent":{"from":["f003"],"description":"验证鉴权绕过"}}}`, TaskReason)
	if err != nil {
		t.Fatalf("ParseResult returned error: %v", err)
	}
	if r.Intent == nil || *r.Intent != "验证鉴权绕过" {
		t.Fatalf("expected singular intent description, got %+v", r.Intent)
	}
	if len(r.Intents) != 1 || len(r.Intents[0].From) != 1 || r.Intents[0].From[0] != "f003" {
		t.Fatalf("expected normalized intent spec, got %+v", r.Intents)
	}
}

func TestParseResultReasonRejectsIntentWithEmptyFrom(t *testing.T) {
	if _, err := ParseResult(`{"accepted":true,"data":{"intents":[{"from":[],"description":"x"}]}}`, TaskReason); err == nil {
		t.Fatal("expected error for intent with empty from")
	}
}

func TestParseResultReasonRejectsConflictingShapes(t *testing.T) {
	cases := []string{
		`{"accepted":true,"data":{"complete":{"from":["f001"],"description":"x"},"intents":[{"from":["f001"],"description":"y"}]}}`,
		`{"accepted":true,"data":{"intent":{"from":["f001"],"description":"x"},"intents":[{"from":["f001"],"description":"y"}]}}`,
		`{"accepted":true,"data":{"complete":{"from":[],"description":"x"}}}`,
	}
	for _, raw := range cases {
		if _, err := ParseResult(raw, TaskReason); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestParseResultBootstrapRequiresFact(t *testing.T) {
	if _, err := ParseResult(`{"accepted":true,"data":{"intents":[{"from":["f001"],"description":"x"}]}}`, TaskBootstrap); err == nil {
		t.Fatal("expected error when bootstrap fact is missing")
	}
}

// --- prompt loading / substitution ---

func writePromptFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLoadPromptAllTypes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"bootstrap.md":          "bootstrap",
		"bootstrap_conclude.md": "bootstrap-conclude",
		"reason.md":             "reason",
		"explore.md":            "explore",
		"explore_conclude.md":   "explore-conclude",
	}
	for name, content := range files {
		writePromptFile(t, dir, name, content)
	}

	cases := []struct {
		tt       TaskType
		conclude bool
		want     string
	}{
		{TaskBootstrap, false, "bootstrap"},
		{TaskBootstrap, true, "bootstrap-conclude"},
		{TaskReason, false, "reason"},
		{TaskExplore, false, "explore"},
		{TaskExplore, true, "explore-conclude"},
	}
	for _, tc := range cases {
		got, err := LoadPrompt(dir, tc.tt, tc.conclude)
		if err != nil {
			t.Fatalf("LoadPrompt(%s, %v): %v", tc.tt, tc.conclude, err)
		}
		if got != tc.want {
			t.Fatalf("LoadPrompt(%s, %v) = %q, want %q", tc.tt, tc.conclude, got, tc.want)
		}
	}
}

func TestLoadPromptMissingFile(t *testing.T) {
	if _, err := LoadPrompt(t.TempDir(), TaskExplore, false); err == nil {
		t.Fatal("expected error for missing prompt file")
	}
}

func TestLoadPromptReasonHasNoConclude(t *testing.T) {
	dir := t.TempDir()
	writePromptFile(t, dir, "reason.md", "reason")
	if _, err := LoadPrompt(dir, TaskReason, true); err == nil {
		t.Fatal("expected error: reason has no conclude prompt")
	}
}

func TestShippedPromptsLoad(t *testing.T) {
	dir := filepath.Join("..", "..", "prompts", "graph", "default")
	cases := []struct {
		tt           TaskType
		conclude     bool
		placeholders []string
	}{
		{TaskBootstrap, false, []string{"{graph_yaml}", "{fact_ids}"}},
		{TaskBootstrap, true, []string{"{graph_yaml}", "{intent_id}"}},
		{TaskReason, false, []string{"{graph_yaml}", "{fact_ids}", "{open_intents}"}},
		{TaskExplore, false, []string{"{graph_yaml}", "{intent_id}", "{intent_description}"}},
		{TaskExplore, true, []string{"{graph_yaml}", "{intent_id}", "{intent_description}"}},
	}
	for _, tc := range cases {
		got, err := LoadPrompt(dir, tc.tt, tc.conclude)
		if err != nil {
			t.Fatalf("LoadPrompt(%s, %v): %v", tc.tt, tc.conclude, err)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("prompt %s (conclude=%v) is empty", tc.tt, tc.conclude)
		}
		for _, placeholder := range tc.placeholders {
			if !strings.Contains(got, placeholder) {
				t.Errorf("prompt %s (conclude=%v) is missing %s", tc.tt, tc.conclude, placeholder)
			}
		}
	}
}

func TestRenderPromptSubstitutesPlaceholders(t *testing.T) {
	tmpl := "{graph_yaml}|{fact_ids}|{open_intents}|{intent_id}|{intent_description}"
	got := RenderPrompt(tmpl, map[string]string{
		"graph_yaml":         "G",
		"fact_ids":           "F",
		"open_intents":       "O",
		"intent_id":          "I",
		"intent_description": "D",
	})
	if got != "G|F|O|I|D" {
		t.Fatalf("RenderPrompt = %q", got)
	}
}

// --- native driver ---

func TestNativeExecuteLoadsPromptAndInjectsIntentID(t *testing.T) {
	dir := t.TempDir()
	writePromptFile(t, dir, "explore.md", "当前意图: {intent_id}")

	var captured string
	d := NewNative(NativeConfig{
		PromptDir: dir,
		Context: func(t Task) map[string]string {
			return map[string]string{"intent_id": t.IntentID}
		},
		Complete: func(ctx context.Context, prompt string) (string, error) {
			captured = prompt
			return `{"accepted":true,"data":{"description":"已确认"}}`, nil
		},
	})

	r, err := d.Execute(context.Background(), Task{ProjectID: "p1", IntentID: "i007", Type: string(TaskExplore)})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(captured, "i007") {
		t.Fatalf("expected intent id substituted into prompt, got %q", captured)
	}
	if strings.Contains(captured, "{intent_id}") {
		t.Fatalf("expected placeholder to be replaced, got %q", captured)
	}
	if !r.Accepted || r.Description != "已确认" {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestNativeExecuteUsesProvidedPromptVerbatim(t *testing.T) {
	var captured string
	d := NewNative(NativeConfig{
		Complete: func(ctx context.Context, prompt string) (string, error) {
			captured = prompt
			return `{"accepted":true,"data":{"description":"ok"}}`, nil
		},
	})
	if _, err := d.Execute(context.Background(), Task{Type: string(TaskExplore), Prompt: "自定义提示"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if captured != "自定义提示" {
		t.Fatalf("captured prompt = %q", captured)
	}
}

func TestNativeExecutePropagatesCompletionError(t *testing.T) {
	want := errors.New("provider down")
	d := NewNative(NativeConfig{
		Complete: func(ctx context.Context, prompt string) (string, error) {
			return "", want
		},
	})
	_, err := d.Execute(context.Background(), Task{Type: string(TaskExplore), Prompt: "x"})
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped provider error, got %v", err)
	}
}

func TestNativeExecuteRejectsInvalidTaskType(t *testing.T) {
	d := NewNative(NativeConfig{Complete: func(ctx context.Context, prompt string) (string, error) {
		t.Fatal("completion must not run for invalid task type")
		return "", nil
	}})
	if _, err := d.Execute(context.Background(), Task{Type: "unknown", Prompt: "x"}); err == nil {
		t.Fatal("expected error for unknown task type")
	}
}

func TestNativeSupportsAndName(t *testing.T) {
	d := NewNative(NativeConfig{Name: "native-a"})
	if d.Name() != "native-a" {
		t.Fatalf("Name = %q", d.Name())
	}
	for _, tt := range []TaskType{TaskBootstrap, TaskReason, TaskExplore} {
		if !d.Supports(tt) {
			t.Fatalf("expected driver to support %s", tt)
		}
	}
	if d.Supports(TaskType("unknown")) {
		t.Fatal("expected unknown task type to be unsupported")
	}
	if !d.SupportsConclude() {
		t.Fatal("native driver must support conclude")
	}
}

func TestNewNativeAppliesDefaults(t *testing.T) {
	d := NewNative(NativeConfig{})
	if d.Name() != "native" {
		t.Fatalf("default Name = %q", d.Name())
	}
}

func TestNativeConcludeUsesConcludePrompt(t *testing.T) {
	dir := t.TempDir()
	writePromptFile(t, dir, "explore_conclude.md", "收尾意图: {intent_id}")

	var captured string
	d := NewNative(NativeConfig{
		PromptDir: dir,
		Context: func(t Task) map[string]string {
			return map[string]string{"intent_id": t.IntentID}
		},
		Complete: func(ctx context.Context, prompt string) (string, error) {
			captured = prompt
			return `{"accepted":true,"data":{"description":"收尾事实"}}`, nil
		},
	})
	r, err := d.Conclude(context.Background(), Task{Type: string(TaskExplore), IntentID: "i009"})
	if err != nil {
		t.Fatalf("Conclude: %v", err)
	}
	if !strings.Contains(captured, "i009") {
		t.Fatalf("expected intent id in conclude prompt, got %q", captured)
	}
	if r.Description != "收尾事实" {
		t.Fatalf("unexpected conclude result: %+v", r)
	}
}

var allPlaceholderNames = []string{"graph_yaml", "fact_ids", "open_intents", "intent_id", "intent_description"}

func allPromptValues(t Task) map[string]string {
	return map[string]string{
		"graph_yaml":         "graph: {}",
		"fact_ids":           `["f001"]`,
		"open_intents":       "[]",
		"intent_id":          t.IntentID,
		"intent_description": "验证鉴权绕过",
	}
}

func assertNoKnownPlaceholder(t *testing.T, prompt string) {
	t.Helper()
	for _, name := range allPlaceholderNames {
		if strings.Contains(prompt, "{"+name+"}") {
			t.Fatalf("prompt still contains unresolved placeholder {%s}: %q", name, prompt)
		}
	}
}

func TestNativeExecuteRendersShippedTemplates(t *testing.T) {
	dir := filepath.Join("..", "..", "prompts", "graph", "default")
	cases := []struct {
		tt       TaskType
		intentID string
		response string
	}{
		{TaskBootstrap, "", `{"accepted":true,"data":{"fact":{"description":"补丁事实"}}}`},
		{TaskReason, "", `{"accepted":true,"data":{}}`},
		{TaskExplore, "i001", `{"accepted":true,"data":{"description":"探索结论"}}`},
	}
	for _, tc := range cases {
		t.Run(string(tc.tt), func(t *testing.T) {
			var captured string
			d := NewNative(NativeConfig{
				PromptDir: dir,
				Context:   allPromptValues,
				Complete: func(ctx context.Context, prompt string) (string, error) {
					captured = prompt
					return tc.response, nil
				},
			})
			if _, err := d.Execute(context.Background(), Task{Type: string(tc.tt), IntentID: tc.intentID}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			assertNoKnownPlaceholder(t, captured)
			if !strings.Contains(captured, "graph: {}") {
				t.Fatalf("context values were not rendered into prompt: %q", captured)
			}
		})
	}
}

func TestNativeExecuteRejectsUnresolvedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	writePromptFile(t, dir, "explore.md", "需要图谱: {graph_yaml}")
	d := NewNative(NativeConfig{
		PromptDir: dir,
		Complete: func(ctx context.Context, prompt string) (string, error) {
			t.Fatal("completion must not run with unresolved placeholders")
			return "", nil
		},
	})
	_, err := d.Execute(context.Background(), Task{Type: string(TaskExplore)})
	if err == nil || !strings.Contains(err.Error(), "graph_yaml") {
		t.Fatalf("expected unresolved placeholder error, got %v", err)
	}
}

func TestNativeConcludeIgnoresProvidedPrompt(t *testing.T) {
	dir := t.TempDir()
	writePromptFile(t, dir, "explore.md", "EXECUTE {intent_id}")
	writePromptFile(t, dir, "explore_conclude.md", "CONCLUDE {intent_id}")

	var captured string
	d := NewNative(NativeConfig{
		PromptDir: dir,
		Context: func(t Task) map[string]string {
			return map[string]string{"intent_id": t.IntentID}
		},
		Complete: func(ctx context.Context, prompt string) (string, error) {
			captured = prompt
			return `{"accepted":true,"data":{"description":"收尾"}}`, nil
		},
	})
	if _, err := d.Conclude(context.Background(), Task{
		Type:     string(TaskExplore),
		IntentID: "i042",
		Prompt:   "原始执行提示不得复用",
	}); err != nil {
		t.Fatalf("Conclude: %v", err)
	}
	if !strings.Contains(captured, "CONCLUDE") || !strings.Contains(captured, "i042") {
		t.Fatalf("expected conclude template prompt, got %q", captured)
	}
	if strings.Contains(captured, "EXECUTE") || strings.Contains(captured, "原始执行提示不得复用") {
		t.Fatalf("conclude must not reuse the execute prompt, got %q", captured)
	}
}

func TestNativeConcludeRendersShippedTemplates(t *testing.T) {
	dir := filepath.Join("..", "..", "prompts", "graph", "default")
	cases := []struct {
		tt       TaskType
		intentID string
		response string
	}{
		{TaskBootstrap, "i001", `{"accepted":true,"data":{"fact":{"description":"收尾事实"}}}`},
		{TaskExplore, "i002", `{"accepted":true,"data":{"description":"收尾结论"}}`},
	}
	for _, tc := range cases {
		t.Run(string(tc.tt), func(t *testing.T) {
			var captured string
			d := NewNative(NativeConfig{
				PromptDir: dir,
				Context:   allPromptValues,
				Complete: func(ctx context.Context, prompt string) (string, error) {
					captured = prompt
					return tc.response, nil
				},
			})
			if _, err := d.Conclude(context.Background(), Task{Type: string(tc.tt), IntentID: tc.intentID}); err != nil {
				t.Fatalf("Conclude: %v", err)
			}
			assertNoKnownPlaceholder(t, captured)
			if !strings.Contains(captured, "graph: {}") {
				t.Fatalf("context values were not rendered into conclude prompt: %q", captured)
			}
		})
	}
}

func TestNativeConcludeRejectsReason(t *testing.T) {
	d := NewNative(NativeConfig{
		Complete: func(ctx context.Context, prompt string) (string, error) {
			t.Fatal("completion must not run for reason conclude")
			return "", nil
		},
	})
	if _, err := d.Conclude(context.Background(), Task{Type: string(TaskReason), Prompt: "任意提示"}); err == nil {
		t.Fatal("expected error: reason has no conclude template")
	}
}

func TestNativeHealthCheckUsesCompletion(t *testing.T) {
	called := false
	d := NewNative(NativeConfig{Complete: func(ctx context.Context, prompt string) (string, error) {
		called = true
		return "pong", nil
	}})
	if err := d.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if !called {
		t.Fatal("expected health check to probe the completion path")
	}
}

func TestNativeHealthCheckPropagatesError(t *testing.T) {
	want := errors.New("unhealthy")
	d := NewNative(NativeConfig{Complete: func(ctx context.Context, prompt string) (string, error) {
		return "", want
	}})
	if err := d.HealthCheck(context.Background()); !errors.Is(err, want) {
		t.Fatalf("expected provider error, got %v", err)
	}
}

// --- registry / health window ---

type stubDriver struct {
	name       string
	supported  map[TaskType]bool
	healthErr  error
	healthCall int
}

func (s *stubDriver) Name() string { return s.name }

func (s *stubDriver) Supports(tt TaskType) bool { return s.supported[tt] }

func (s *stubDriver) HealthCheck(ctx context.Context) error {
	s.healthCall++
	return s.healthErr
}

func (s *stubDriver) Execute(ctx context.Context, t Task) (Result, error) { return Result{}, nil }

func (s *stubDriver) Conclude(ctx context.Context, t Task) (Result, error) { return Result{}, nil }

func (s *stubDriver) SupportsConclude() bool { return true }

func TestRegistryMarksUnhealthyForCooldown(t *testing.T) {
	now := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	d := &stubDriver{name: "w1", supported: map[TaskType]bool{TaskExplore: true}}
	reg := NewRegistry(time.Minute, d)
	reg.now = func() time.Time { return now }

	reg.MarkUnhealthy("w1")
	if !reg.IsUnhealthy("w1") {
		t.Fatal("expected worker to be unhealthy inside cooldown")
	}
	if got := reg.RetryAfter("w1"); got != time.Minute {
		t.Fatalf("RetryAfter = %v, want 1m", got)
	}
	if got := reg.Available(TaskExplore); len(got) != 0 {
		t.Fatalf("expected no available workers, got %d", len(got))
	}

	now = now.Add(2 * time.Minute)
	if reg.IsUnhealthy("w1") {
		t.Fatal("expected worker healthy after cooldown")
	}
	if got := reg.Available(TaskExplore); len(got) != 1 || got[0].Name() != "w1" {
		t.Fatalf("expected worker available after cooldown, got %+v", got)
	}
}

func TestRegistryAvailableFiltersByTaskType(t *testing.T) {
	explore := &stubDriver{name: "explore", supported: map[TaskType]bool{TaskExplore: true}}
	reason := &stubDriver{name: "reason", supported: map[TaskType]bool{TaskReason: true}}
	reg := NewRegistry(time.Minute, explore, reason)

	got := reg.Available(TaskReason)
	if len(got) != 1 || got[0].Name() != "reason" {
		t.Fatalf("Available(reason) = %+v", got)
	}
}

func TestRegistryCheckMarksUnhealthyOnFailure(t *testing.T) {
	want := errors.New("probe failed")
	d := &stubDriver{name: "w1", supported: map[TaskType]bool{TaskExplore: true}, healthErr: want}
	reg := NewRegistry(time.Minute, d)

	if err := reg.Check(context.Background(), d); !errors.Is(err, want) {
		t.Fatalf("Check error = %v", err)
	}
	if !reg.IsUnhealthy("w1") {
		t.Fatal("expected failed health check to mark worker unhealthy")
	}

	d.healthErr = nil
	if err := reg.Check(context.Background(), d); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if reg.IsUnhealthy("w1") {
		t.Fatal("expected successful health check to clear cooldown")
	}
}

func TestRegistryFindAndDrivers(t *testing.T) {
	a := &stubDriver{name: "a", supported: map[TaskType]bool{TaskExplore: true}}
	b := &stubDriver{name: "b", supported: map[TaskType]bool{TaskExplore: true}}
	reg := NewRegistry(time.Minute, a)
	reg.Register(b)

	if got, ok := reg.Find("b"); !ok || got.Name() != "b" {
		t.Fatalf("Find(b) = %v, %v", got, ok)
	}
	if _, ok := reg.Find("missing"); ok {
		t.Fatal("expected missing driver lookup to fail")
	}
	if got := reg.Drivers(); len(got) != 2 {
		t.Fatalf("Drivers() returned %d entries", len(got))
	}
}

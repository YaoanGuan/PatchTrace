package llm

import (
	"strings"
	"testing"
)

func TestContextManagerExportDebugSnapshot(t *testing.T) {
	cm := NewContextManager()
	cm.SetEventHandler(func(string, string, int) {})
	cm.SetSystemPrompt(&SystemPromptX{SystemPrompt: "system-prompt", ContextId: "ctx-1"})
	cm.SetExtraSystemPrompt("extra-prompt", "ctx-1")
	cm.SetTaskList(&TaskListX{
		ContextId: "ctx-1",
		TaskList: []map[string]string{
			{"TaskID": "T.0", "TaskContent": "inspect project"},
		},
	})
	cm.AddKeyMessage(&EnvMessageX{Key: "Env", Content: map[string]string{"url": "http://example.com"}, ContextId: "ctx-1"})
	cm.AddMessage(&MessageX{ContextId: "ctx-1", Msg: Message{Role: RoleUser, Content: "hello"}})
	cm.AddMessage(&MessageX{ContextId: "ctx-1", Msg: Message{Role: RoleAssistant, Content: "world"}})

	snapshotAny, err := ExportMemoryDebugSnapshot(cm, "ctx-1")
	if err != nil {
		t.Fatalf("ExportMemoryDebugSnapshot returned error: %v", err)
	}
	snapshot, ok := snapshotAny.(ContextManagerDebugSnapshot)
	if !ok {
		t.Fatalf("unexpected snapshot type: %T", snapshotAny)
	}
	if snapshot.MemoryType != "ContextManager" {
		t.Fatalf("unexpected memory type: %s", snapshot.MemoryType)
	}
	if snapshot.SystemPrompt != "system-prompt" {
		t.Fatalf("unexpected system prompt: %q", snapshot.SystemPrompt)
	}
	if snapshot.ExtraSystemPrompt != "extra-prompt" {
		t.Fatalf("unexpected extra prompt: %q", snapshot.ExtraSystemPrompt)
	}
	if len(snapshot.FlatMessages) != 2 {
		t.Fatalf("expected 2 flat messages, got %d", len(snapshot.FlatMessages))
	}
	if len(snapshot.RenderedContext) < 3 {
		t.Fatalf("expected rendered context to include prompt/task/messages, got %d entries", len(snapshot.RenderedContext))
	}
	if len(snapshot.KeyMessages["Env"]) != 1 {
		t.Fatalf("expected key message Env to be exported, got %+v", snapshot.KeyMessages)
	}
}

func TestCaptureAndExportMemoryDebugText(t *testing.T) {
	cm := NewContextManager()
	msgs := []Message{
		{Role: RoleSystem, Content: "system"},
		{Role: RoleUser, Content: "user asks"},
		{Role: RoleAssistant, Content: "assistant answers", ToolCalls: []ToolCall{{ID: "call-1", Name: "Tool-X", Arguments: "{\"x\":1}"}}},
		{Role: RoleTool, ToolCallID: "call-1", Content: "{\"ok\":true}"},
	}

	CaptureMemoryDebugContext(cm, "ctx-1", msgs)
	text, err := ExportMemoryDebugText(cm, "ctx-1")
	if err != nil {
		t.Fatalf("ExportMemoryDebugText returned error: %v", err)
	}
	for _, want := range []string{"# message_count: 4", "role: system", "role: user", "role: assistant", "role: tool", "Tool-X", "\"ok\":true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected exported debug text to contain %q, got: %s", want, text)
		}
	}
}

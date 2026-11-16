package DecisionBrain

import (
	"AIxVuln/llm"
	"testing"
)

func TestBrainMemoryExportDebugSnapshot(t *testing.T) {
	br := NewBrainMemory()
	br.SetSystemPrompt("brain-system")
	br.SetTaskContent("audit task")
	br.EnsurePinnedMessages([]llm.Message{{Role: llm.RoleUser, Content: "pinned user"}})
	br.AddKeyMessage("SharedInfo", map[string]string{"k": "v"}, false)
	br.SetEnvInfo(`{"url":"http://example.com"}`)
	br.AddMessage(llm.Message{Role: llm.RoleUser, Content: "user asks"})
	br.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "brain answers"})

	snapshot := br.ExportDebugSnapshot()
	if snapshot.MemoryType != "BrainMemory" {
		t.Fatalf("unexpected memory type: %s", snapshot.MemoryType)
	}
	if snapshot.SystemPrompt != "brain-system" {
		t.Fatalf("unexpected system prompt: %q", snapshot.SystemPrompt)
	}
	if snapshot.TaskContent != "audit task" {
		t.Fatalf("unexpected task content: %q", snapshot.TaskContent)
	}
	if len(snapshot.PinnedMessages) != 1 || snapshot.PinnedMessages[0].Content != "pinned user" {
		t.Fatalf("unexpected pinned messages: %+v", snapshot.PinnedMessages)
	}
	if len(snapshot.FlatMessages) != 2 {
		t.Fatalf("expected 2 flat messages, got %d", len(snapshot.FlatMessages))
	}
	envInfo, ok := snapshot.EnvInfo.(map[string]interface{})
	if !ok {
		t.Fatalf("expected env info to be decoded as map, got %T", snapshot.EnvInfo)
	}
	if envInfo["url"] != "http://example.com" {
		t.Fatalf("unexpected env info: %+v", envInfo)
	}
	if len(snapshot.RenderedContext) < 3 {
		t.Fatalf("expected rendered context to include system panel and messages, got %d", len(snapshot.RenderedContext))
	}
}

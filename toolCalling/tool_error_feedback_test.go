package toolCalling

import (
	"AIxVuln/llm"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type stubLLMClient struct {
	response llm.Response
	err      error
}

func (c stubLLMClient) Chat(ctx context.Context, model string, messages []llm.Message, tools []llm.ToolDef) (llm.Response, error) {
	return c.response, c.err
}

type requiredNameTool struct{}

func (requiredNameTool) Name() string { return "DemoTool" }

func (requiredNameTool) Description() string { return "demo" }

func (requiredNameTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"name"},
	}
}

func (requiredNameTool) Execute(args map[string]interface{}) string {
	return Success(args)
}

func TestInvalidToolArgumentsFailureIncludesToolContext(t *testing.T) {
	raw := InvalidToolArgumentsFailure("DemoTool", `{"count":1}`, errors.New("name is required"))
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal failure payload: %v", err)
	}
	if got := decoded["success"]; got != false {
		t.Fatalf("success = %#v, want false", got)
	}
	if got := decoded["failure_kind"]; got != "invalid_arguments" {
		t.Fatalf("failure_kind = %#v, want invalid_arguments", got)
	}
	if got := decoded["tool_name"]; got != "DemoTool" {
		t.Fatalf("tool_name = %#v, want DemoTool", got)
	}
	if got := decoded["tool_arguments_raw"]; got != `{"count":1}` {
		t.Fatalf("tool_arguments_raw = %#v", got)
	}
	if got := decoded["error"]; !strings.Contains(got.(string), "name is required") {
		t.Fatalf("error = %#v, want validation detail", got)
	}
}

func TestToolManagerReturnsStructuredToolErrorForInvalidArguments(t *testing.T) {
	manager := NewToolManager()
	manager.Register(requiredNameTool{})
	client := stubLLMClient{
		response: llm.Response{
			ToolCalls: []llm.ToolCall{
				{
					ID:        "call-1",
					Name:      "DemoTool",
					Arguments: `{"count":1}`,
				},
			},
		},
	}

	assistantMessage, toolMessages, err := manager.ToolCallRequestWithLabel(
		context.Background(),
		client,
		[]llm.Message{{Role: llm.RoleUser, Content: "test"}},
		"test-model",
		"TestAgent",
		"TestAgent",
	)
	if err != nil {
		t.Fatalf("ToolCallRequestWithLabel returned err: %v", err)
	}
	if len(assistantMessage.ToolCalls) != 1 {
		t.Fatalf("assistant tool calls = %d, want 1", len(assistantMessage.ToolCalls))
	}
	if len(toolMessages) != 1 {
		t.Fatalf("tool message count = %d, want 1", len(toolMessages))
	}
	if toolMessages[0].Role != llm.RoleTool {
		t.Fatalf("tool message role = %q, want %q", toolMessages[0].Role, llm.RoleTool)
	}
	if toolMessages[0].ToolCallID != "call-1" {
		t.Fatalf("tool message call id = %q, want call-1", toolMessages[0].ToolCallID)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(toolMessages[0].Content), &decoded); err != nil {
		t.Fatalf("unmarshal tool message: %v", err)
	}
	if got := decoded["failure_kind"]; got != "invalid_arguments" {
		t.Fatalf("failure_kind = %#v, want invalid_arguments", got)
	}
	if got := decoded["tool_name"]; got != "DemoTool" {
		t.Fatalf("tool_name = %#v, want DemoTool", got)
	}
	if got := decoded["tool_arguments_raw"]; got != `{"count":1}` {
		t.Fatalf("tool_arguments_raw = %#v", got)
	}
	if got := decoded["error"]; !strings.Contains(got.(string), "name is required") {
		t.Fatalf("error = %#v, want validation detail", got)
	}
}

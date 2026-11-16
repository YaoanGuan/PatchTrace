package llm

import "testing"

func TestResolveLLMBackend(t *testing.T) {
	tests := []struct {
		name    string
		apiMode string
		baseURL string
		want    string
	}{
		{name: "chat default", apiMode: "chat", baseURL: "https://api.openai.com/v1", want: BackendChat},
		{name: "responses explicit", apiMode: "responses", baseURL: "https://api.openai.com/v1", want: BackendResponses},
		{name: "anthropic explicit", apiMode: "anthropic", baseURL: "https://api.openai.com/v1", want: BackendAnthropic},
		{name: "claude explicit", apiMode: "claude", baseURL: "https://api.openai.com/v1", want: BackendAnthropic},
		{name: "anthropic detected by base url", apiMode: "chat", baseURL: "https://api.anthropic.com", want: BackendAnthropic},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveLLMBackend(tc.apiMode, tc.baseURL); got != tc.want {
				t.Fatalf("ResolveLLMBackend(%q, %q) = %q, want %q", tc.apiMode, tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestAnthropicEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		endpoint string
		want     string
	}{
		{name: "root base", baseURL: "https://api.anthropic.com", endpoint: "/messages", want: "https://api.anthropic.com/v1/messages"},
		{name: "v1 base", baseURL: "https://api.anthropic.com/v1", endpoint: "/messages", want: "https://api.anthropic.com/v1/messages"},
		{name: "exact endpoint", baseURL: "https://api.anthropic.com/v1/messages", endpoint: "/messages", want: "https://api.anthropic.com/v1/messages"},
		{name: "proxy path", baseURL: "https://proxy.example.com/anthropic", endpoint: "/models", want: "https://proxy.example.com/anthropic/v1/models"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := anthropicEndpointURL(tc.baseURL, tc.endpoint); got != tc.want {
				t.Fatalf("anthropicEndpointURL(%q, %q) = %q, want %q", tc.baseURL, tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestMessagesToAnthropicMessages(t *testing.T) {
	system, messages := messagesToAnthropicMessages([]Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "hello"},
		{
			Role:    RoleAssistant,
			Content: "calling tool",
			ToolCalls: []ToolCall{{
				ID:        "call_1",
				Name:      "lookup",
				Arguments: `{"q":"anthropic"}`,
			}},
		},
		{Role: RoleTool, ToolCallID: "call_1", Content: `{"success":true,"result":"ok","error":""}`},
		{Role: RoleUser, Content: "continue"},
	})
	if system != "sys" {
		t.Fatalf("unexpected system prompt: %q", system)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 anthropic messages, got %d", len(messages))
	}
	if messages[1].Role != "assistant" || len(messages[1].Content) != 2 {
		t.Fatalf("unexpected assistant message: %#v", messages[1])
	}
	if messages[1].Content[1].Type != "tool_use" || string(messages[1].Content[1].Input) != `{"q":"anthropic"}` {
		t.Fatalf("unexpected tool_use block: %#v", messages[1].Content[1])
	}
	if messages[2].Role != "user" || len(messages[2].Content) != 2 {
		t.Fatalf("unexpected merged user/tool_result message: %#v", messages[2])
	}
	if messages[2].Content[0].Type != "tool_result" || messages[2].Content[1].Type != "text" {
		t.Fatalf("unexpected content order: %#v", messages[2].Content)
	}
}

func TestFromAnthropicResponse(t *testing.T) {
	resp, err := fromAnthropicResponse(&anthropicResponse{
		Content: []anthropicContentBlock{
			{Type: "text", Text: "done"},
			{Type: "tool_use", ID: "tool_1", Name: "lookup", Input: []byte(`{"q":"x"}`)},
		},
		Usage: anthropicResponseUsage{InputTokens: 10, OutputTokens: 5},
	})
	if err != nil {
		t.Fatalf("fromAnthropicResponse error: %v", err)
	}
	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Arguments != `{"q":"x"}` {
		t.Fatalf("unexpected tool calls: %#v", resp.ToolCalls)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}

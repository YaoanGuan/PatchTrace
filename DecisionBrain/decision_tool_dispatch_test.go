package DecisionBrain

import (
	"AIxVuln/llm"
	"AIxVuln/toolCalling"
	"encoding/json"
	"testing"
)

func TestExecuteDecisionToolCallHandlesNonToolPrefixedExternalTools(t *testing.T) {
	db := &DecisionBrain{
		externalTools:      make([]llm.ToolDef, 0),
		externalToolCaller: make(map[string]func(map[string]interface{}) string),
	}
	called := false
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"question"},
	}
	db.RegisterExternalTool("AskPocKnowledgeTool", "desc", schema, func(args map[string]interface{}) string {
		called = true
		if args["question"] != "medguard" {
			return toolCalling.Fail("unexpected question")
		}
		return toolCalling.Success("knowledge ok")
	})

	var wsEvents []map[string]interface{}
	msg := db.executeDecisionToolCall(llm.ToolCall{
		ID:        "call-1",
		Name:      "AskPocKnowledgeTool",
		Arguments: `{"question":"medguard"}`,
	}, map[string]map[string]interface{}{"AskPocKnowledgeTool": schema}, func(_ string, data interface{}) {
		msgData, ok := data.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected websocket payload type: %T", data)
		}
		wsEvents = append(wsEvents, msgData)
	})

	if !called {
		t.Fatal("expected external tool handler to be called")
	}
	if msg.Role != llm.RoleTool {
		t.Fatalf("unexpected role: %s", msg.Role)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
		t.Fatalf("unmarshal tool payload: %v", err)
	}
	if ok, _ := payload["success"].(bool); !ok {
		t.Fatalf("expected success payload, got: %#v", payload)
	}
	if len(wsEvents) != 1 {
		t.Fatalf("expected 1 websocket event, got %d", len(wsEvents))
	}
	if got := wsEvents[0]["name"]; got != "AskPocKnowledgeTool" {
		t.Fatalf("unexpected websocket tool name: %#v", got)
	}
}

package toolCalling

import (
	"AIxVuln/taskManager"
	"encoding/json"
	"testing"
)

func TestAskToolUsageGuideToolDelegatesToEphemeralAgent(t *testing.T) {
	oldRunner := runToolUsageGuideQuery
	t.Cleanup(func() {
		runToolUsageGuideQuery = oldRunner
	})

	runToolUsageGuideQuery = func(_ *taskManager.Task, req toolUsageGuideRequest) (toolUsageGuideResult, error) {
		if req.Question != "jadx MCP 怎么传 aix-mcp 路径？" {
			t.Fatalf("unexpected question: %q", req.Question)
		}
		if req.SessionID != "" {
			t.Fatalf("unexpected session id: %q", req.SessionID)
		}
		if req.CloseSession {
			t.Fatalf("unexpected close_session=true")
		}
		return toolUsageGuideResult{
			Answer:    "使用 aix-mcp://xxx.apk 作为文件参数。",
			SessionID: "tool-usage-guide-session-test",
			Created:   true,
		}, nil
	}

	tool := NewAskToolUsageGuideTool(nil)
	resp := tool.Execute(map[string]interface{}{
		"question": "jadx MCP 怎么传 aix-mcp 路径？",
	})

	var payload struct {
		Success bool                   `json:"success"`
		Error   string                 `json:"error"`
		Result  map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success, got error: %s", payload.Error)
	}
	if payload.Result["answer"] != "使用 aix-mcp://xxx.apk 作为文件参数。" {
		t.Fatalf("unexpected answer payload: %+v", payload.Result)
	}
	if payload.Result["session_id"] != "tool-usage-guide-session-test" {
		t.Fatalf("expected session_id in payload, got %+v", payload.Result)
	}
	if payload.Result["destroyed"] != false {
		t.Fatalf("expected destroyed=false, got %+v", payload.Result)
	}
	if payload.Result["session_active"] != true {
		t.Fatalf("expected session_active=true, got %+v", payload.Result)
	}
}

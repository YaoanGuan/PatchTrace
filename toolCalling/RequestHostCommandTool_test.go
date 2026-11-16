package toolCalling

import (
	"AIxVuln/taskManager"
	"encoding/json"
	"testing"
)

func TestRequestHostCommandToolRejectsSourceCodeDirPaths(t *testing.T) {
	task := &taskManager.Task{}
	task.SetHostCommandRequestHandler(func(req taskManager.HostCommandRequest) taskManager.HostCommandResult {
		t.Fatal("handler should not be called when absolute sandbox source paths are used")
		return taskManager.HostCommandResult{}
	})
	tool := NewRequestHostCommandTool(task)

	raw := tool.Execute(map[string]interface{}{
		"command": "frida -Uf -l /workdir/sourceCodeDir/scripts/hook.js",
		"reason":  "need host runtime",
	})

	var payload struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if payload.Success {
		t.Fatal("expected tool call to fail")
	}
	if payload.Error == "" {
		t.Fatal("expected validation error")
	}
}

func TestRequestHostCommandToolDoesNotAcceptWorkdirOverride(t *testing.T) {
	task := &taskManager.Task{}
	task.SetHostCommandRequestHandler(func(req taskManager.HostCommandRequest) taskManager.HostCommandResult {
		if req.Workdir != "" {
			t.Fatalf("expected empty workdir override, got %q", req.Workdir)
		}
		return taskManager.HostCommandResult{
			RequestID: "req-1",
			Status:    "completed",
			Approved:  true,
			Command:   req.Command,
			Workdir:   "/tmp/source",
		}
	})
	tool := NewRequestHostCommandTool(task)

	raw := tool.Execute(map[string]interface{}{
		"command": "ls ./scripts",
		"reason":  "need host runtime",
		"workdir": "/projectDir",
	})

	var payload struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	if !payload.Success {
		t.Fatal("expected tool call to succeed")
	}
}

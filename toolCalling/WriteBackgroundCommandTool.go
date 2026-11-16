package toolCalling

import (
	"AIxVuln/taskManager"
)

type WriteBackgroundCommandTool struct {
	task *taskManager.Task
}

func NewWriteBackgroundCommandTool(task *taskManager.Task) *WriteBackgroundCommandTool {
	return &WriteBackgroundCommandTool{task: task}
}

func (h *WriteBackgroundCommandTool) Name() string {
	return "WriteBackgroundCommandTool"
}

func (h *WriteBackgroundCommandTool) Description() string {
	return "Write input to a shared running background command session in the current project. Use this for interactive programs such as gdb, shells, frida CLI, or any prompt waiting on stdin. You can also close stdin explicitly."
}

func (h *WriteBackgroundCommandTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Background command session id. Required.",
			},
			"input": map[string]interface{}{
				"type":        "string",
				"description": "Exact text to write to stdin. Include your own newline when needed.",
			},
			"close_stdin": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to close stdin after writing.",
				"default":     false,
			},
		},
		"required": []string{"session_id"},
	}
}

func (h *WriteBackgroundCommandTool) Execute(parameters map[string]interface{}) string {
	sessionID := stringParam(parameters["session_id"])
	if sessionID == "" {
		return Fail("Missing 'session_id' parameter")
	}
	input := stringParam(parameters["input"])
	closeStdin, _ := parameters["close_stdin"].(bool)
	if input == "" && !closeStdin {
		return Fail("Either 'input' must be non-empty or 'close_stdin' must be true")
	}
	snapshot, err := taskManager.WriteBackgroundProcess(h.task.GetProjectName(), sessionID, h.task.GetTaskId(), input, closeStdin)
	if err != nil {
		return Fail(err.Error())
	}
	syncTaskBackgroundCommandKeyInfo(h.task)
	return Success(snapshot)
}

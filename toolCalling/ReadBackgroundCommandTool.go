package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
)

type ReadBackgroundCommandTool struct {
	task *taskManager.Task
}

func NewReadBackgroundCommandTool(task *taskManager.Task) *ReadBackgroundCommandTool {
	return &ReadBackgroundCommandTool{task: task}
}

func (h *ReadBackgroundCommandTool) Name() string {
	return "ReadBackgroundCommandTool"
}

func (h *ReadBackgroundCommandTool) Description() string {
	return "Read incremental stdout/stderr from a shared background command session in the current project. You can either poll immediately or block for wait_ms until new output, a status change, or process exit occurs."
}

func (h *ReadBackgroundCommandTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Background command session id. Required.",
			},
			"stdout_offset": map[string]interface{}{
				"type":        "integer",
				"description": "Previous stdout offset. Use 0 on first read.",
				"minimum":     0,
				"default":     0,
			},
			"stderr_offset": map[string]interface{}{
				"type":        "integer",
				"description": "Previous stderr offset. Use 0 on first read.",
				"minimum":     0,
				"default":     0,
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum bytes to read from each of stdout and stderr.",
				"minimum":     1024,
				"default":     16384,
				"maximum":     262144,
			},
			"wait_ms": map[string]interface{}{
				"type":        "integer",
				"description": "How long to block waiting for new output or state change before returning. 0 means return immediately.",
				"minimum":     0,
				"default":     0,
				"maximum":     600000,
			},
		},
		"required": []string{"session_id"},
	}
}

func (h *ReadBackgroundCommandTool) Execute(parameters map[string]interface{}) string {
	sessionID := stringParam(parameters["session_id"])
	if sessionID == "" {
		return Fail("Missing 'session_id' parameter")
	}
	stdoutOffset := int64(0)
	if raw := parameters["stdout_offset"]; raw != nil {
		n, err := misc.GetIntParam(raw)
		if err != nil {
			return Fail("Error converting stdout_offset to int")
		}
		stdoutOffset = int64(n)
	}
	stderrOffset := int64(0)
	if raw := parameters["stderr_offset"]; raw != nil {
		n, err := misc.GetIntParam(raw)
		if err != nil {
			return Fail("Error converting stderr_offset to int")
		}
		stderrOffset = int64(n)
	}
	limit := 16 * 1024
	if raw := parameters["limit"]; raw != nil {
		n, err := misc.GetIntParam(raw)
		if err != nil {
			return Fail("Error converting limit to int")
		}
		limit = n
	}
	waitMs := 0
	if raw := parameters["wait_ms"]; raw != nil {
		n, err := misc.GetIntParam(raw)
		if err != nil {
			return Fail("Error converting wait_ms to int")
		}
		waitMs = n
	}
	result, err := taskManager.ReadBackgroundProcess(h.task.GetProjectName(), sessionID, h.task.GetTaskId(), stdoutOffset, stderrOffset, limit, waitMs)
	if err != nil {
		return Fail(err.Error())
	}
	syncTaskBackgroundCommandKeyInfo(h.task)
	return Success(result)
}

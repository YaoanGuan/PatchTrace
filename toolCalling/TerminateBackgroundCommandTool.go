package toolCalling

import (
	"AIxVuln/taskManager"
)

type TerminateBackgroundCommandTool struct {
	task *taskManager.Task
}

func NewTerminateBackgroundCommandTool(task *taskManager.Task) *TerminateBackgroundCommandTool {
	return &TerminateBackgroundCommandTool{task: task}
}

func (h *TerminateBackgroundCommandTool) Name() string {
	return "TerminateBackgroundCommandTool"
}

func (h *TerminateBackgroundCommandTool) Description() string {
	return "Terminate a visible background command session from the current project. Prefer this when a long-running or interactive process is no longer needed, is stuck, or must be restarted cleanly."
}

func (h *TerminateBackgroundCommandTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Background command session id. Required.",
			},
			"signal": map[string]interface{}{
				"type":        "string",
				"description": "Termination signal. Supported values include TERM, KILL, and INT. Default TERM.",
				"default":     "TERM",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Optional reason recorded for later reads.",
			},
		},
		"required": []string{"session_id"},
	}
}

func (h *TerminateBackgroundCommandTool) Execute(parameters map[string]interface{}) string {
	sessionID := stringParam(parameters["session_id"])
	if sessionID == "" {
		return Fail("Missing 'session_id' parameter")
	}
	signal := stringParam(parameters["signal"])
	if signal == "" {
		signal = "TERM"
	}
	snapshot, err := taskManager.TerminateBackgroundProcess(h.task.GetProjectName(), sessionID, h.task.GetTaskId(), "agent", stringParam(parameters["reason"]), signal)
	if err != nil {
		return Fail(err.Error())
	}
	syncTaskBackgroundCommandKeyInfo(h.task)
	return Success(snapshot)
}

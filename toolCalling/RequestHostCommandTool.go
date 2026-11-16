package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"strings"
)

type RequestHostCommandTool struct {
	task *taskManager.Task
}

func NewRequestHostCommandTool(task *taskManager.Task) *RequestHostCommandTool {
	return &RequestHostCommandTool{task: task}
}

func (h *RequestHostCommandTool) Name() string {
	return "RequestHostCommandTool"
}

func (h *RequestHostCommandTool) Description() string {
	return "Request a host-machine command execution. By default it enters user approval, but it may be auto-approved when the system-wide full automation mode is enabled. Use this only when MCP tools, sandbox tools, or existing environment tools are insufficient. After approval, the command starts as a background process, waits up to 5 seconds, and returns either the completed result or a compact session_timeout marker with session_id for follow-up reads and writes. Returned background sessions are visible to other Agents in the same current project/lab, so they can continue reading/writing the same stdio session when needed. The command runs on the host with its working directory fixed to the host-mapped project workdir. For files inside the project source tree, you MUST use relative paths such as sourceCodeDir/... from that workdir. Do NOT use /workdir/sourceCodeDir/... , /sourceCodeDir/... , or $AIX_SOURCE_DIR/... inside the command."
}

func (h *RequestHostCommandTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The exact host command to execute after approval. Required.",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Why host execution is necessary and why MCP / sandbox / existing tools are insufficient. Required.",
			},
			"timeout_sec": map[string]interface{}{
				"type":        "integer",
				"description": "Optional hard-kill timeout in seconds for the approved host background process. 0 means no timeout. This does NOT change the fixed 5-second initial wait window before a session_timeout response is returned, so usually omit it unless you intentionally want the process auto-terminated later.",
				"minimum":     0,
				"default":     0,
				"maximum":     86400,
			},
			"max_output_chars": map[string]interface{}{
				"type":        "integer",
				"description": "Optional maximum number of output characters returned to the agent.",
				"minimum":     1024,
				"default":     16384,
				"maximum":     131072,
			},
		},
		"required": []string{"command", "reason"},
	}
}

func (h *RequestHostCommandTool) Execute(parameters map[string]interface{}) string {
	handler := h.task.GetHostCommandRequestHandler()
	if handler == nil {
		return Fail("host command approval handler is unavailable")
	}
	command := strings.TrimSpace(stringParam(parameters["command"]))
	if command == "" {
		return Fail("Missing 'command' parameter")
	}
	reason := strings.TrimSpace(stringParam(parameters["reason"]))
	if reason == "" {
		return Fail("Missing 'reason' parameter")
	}
	if misc.ContainsVisibleSourceRootReference(command) || strings.Contains(command, "$AIX_SOURCE_DIR") || strings.Contains(command, "${AIX_SOURCE_DIR}") || strings.Contains(command, "$AIX_WORK_DIR") || strings.Contains(command, "${AIX_WORK_DIR}") {
		return Fail("Host commands run with the project workdir as their working directory. For files under /workdir/sourceCodeDir, use relative paths such as sourceCodeDir/... instead of /workdir/sourceCodeDir/... , /sourceCodeDir/... , $AIX_SOURCE_DIR/... , or $AIX_WORK_DIR/...")
	}
	req := taskManager.HostCommandRequest{
		Command:    command,
		Reason:     reason,
		Background: true,
		WaitMs:     commandExecutionInitialWaitMs,
	}
	if timeout := parameters["timeout_sec"]; timeout != nil {
		n, err := misc.GetIntParam(timeout)
		if err != nil {
			return Fail("Error converting timeout_sec to int")
		}
		req.TimeoutSec = n
	}
	if limit := parameters["max_output_chars"]; limit != nil {
		n, err := misc.GetIntParam(limit)
		if err != nil {
			return Fail("Error converting max_output_chars to int")
		}
		req.MaxOutputChars = n
	}
	result := handler(req)
	syncTaskBackgroundCommandKeyInfo(h.task)
	if isDeferredHostCommand(result) {
		return Success(buildDeferredHostCommandPayload(result))
	}
	payload := buildHostCommandPayload(result)
	if hostCommandSucceeded(result) {
		return Success(payload)
	}
	return Fail(payload)
}

func stringParam(raw interface{}) string {
	if raw == nil {
		return ""
	}
	s, _ := raw.(string)
	return s
}

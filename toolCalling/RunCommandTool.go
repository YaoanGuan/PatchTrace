package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"strings"
)

type RunCommandTool struct {
	task *taskManager.Task
}

func NewRunCommandTool(task *taskManager.Task) *RunCommandTool {
	return &RunCommandTool{task: task}
}

func (h *RunCommandTool) Name() string {
	return "RunCommandTool"
}
func (h *RunCommandTool) Description() string {
	return "Execute sandbox commands. Internally this starts a background process, waits up to 5 seconds, and returns either the completed result or a compact session_timeout marker with session_id for follow-up reads and writes. Returned background sessions are visible to other Agents in the same current project/lab, so they can continue reading/writing the same stdio session when needed. The shared sandbox workdir is /workdir, and the project source tree is mapped at /workdir/sourceCodeDir. Modifying files inside /workdir/sourceCodeDir will automatically synchronize changes to the same path in other workdir-mapped containers. (" + misc.AttackSandboxPrompt() + ")"
}
func (h *RunCommandTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Commands to be executed",
			},
			"timeout_sec": map[string]interface{}{
				"type":        "integer",
				"description": "Optional hard-kill timeout in seconds for the background process. 0 means no timeout. This does NOT change the fixed 5-second initial wait window before a session_timeout response is returned, so usually omit it unless you intentionally want the process auto-terminated later.",
				"minimum":     0,
				"default":     0,
				"maximum":     86400,
			},
		},
	}
}

func (h *RunCommandTool) Execute(parameters map[string]interface{}) string {
	cmdTemp := parameters["command"]
	if cmdTemp == nil {
		return Fail("Missing 'command' parameter")
	}
	cmd := cmdTemp.(string)
	timeoutSec := 0
	timeoutRaw := parameters["timeout_sec"]
	if timeoutRaw != nil {
		timeoutTmp, err := misc.GetIntParam(timeoutRaw)
		if err != nil {
			return Fail("Error converting timeout_sec to int")
		}
		timeoutSec = timeoutTmp
	}
	snapshot, err := taskManager.StartSandboxBackgroundProcess(taskManager.BackgroundProcessStartOptions{
		ProjectName: h.task.GetProjectName(),
		OwnerTaskID: h.task.GetTaskId(),
		Actor:       h.task.GetTaskActor(),
		Command:     cmd,
		Workdir:     misc.SandboxWorkDir,
		TimeoutSec:  timeoutSec,
	})
	if err != nil {
		return Fail(withHostCommandFallbackHint(err.Error()))
	}
	syncTaskBackgroundCommandKeyInfo(h.task)
	readResult, err := taskManager.ReadBackgroundProcessUntilExitOrTimeout(
		h.task.GetProjectName(),
		snapshot.SessionID,
		h.task.GetTaskId(),
		0,
		0,
		commandExecutionOutputLimit,
		commandExecutionInitialWaitMs,
	)
	if err != nil {
		return Fail(withHostCommandFallbackHint(err.Error()))
	}
	syncTaskBackgroundCommandKeyInfo(h.task)
	if isDeferredBackgroundSession(readResult) {
		return Success(buildDeferredBackgroundPayload(readResult))
	}
	payload := buildBackgroundCommandPayload(readResult)
	if backgroundCommandSucceeded(readResult) {
		return Success(payload)
	}
	return Fail(payload)
}

func withHostCommandFallbackHint(msg string) string {
	msg = strings.TrimSpace(msg)
	hint := "本次命令在 sandbox 中运行；如果错误是由环境限制、宿主机依赖、ADB/Frida/设备连接或宿主机网络差异导致的，可改用 RequestHostCommandTool 申请执行宿主机命令。"
	if msg == "" {
		return hint
	}
	return msg + "\n\n" + hint
}

package toolCalling

import (
	"AIxVuln/taskManager"
	"strings"
)

func syncTaskBackgroundCommandKeyInfo(task *taskManager.Task) {
	if task == nil {
		return
	}
	projectName := strings.TrimSpace(task.GetProjectName())
	if projectName == "" {
		return
	}
	backgroundSessions := taskManager.BuildBackgroundProcessPanel(projectName)
	task.AddKeyMessage("BackgroundCommandSessions", backgroundSessions, false)
	if handler := task.GetGlobalStatusPanelHandler(); handler != nil {
		handler(taskManager.BuildGlobalStatusPanel(projectName))
		return
	}
	task.AddKeyMessage("GlobalStatusPanel", taskManager.BuildGlobalStatusPanel(projectName), false)
}

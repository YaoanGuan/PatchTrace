package toolCalling

import (
	"AIxVuln/taskManager"
	"strings"
)

func helperLookupSessionScope(task *taskManager.Task, scopeOverride string) string {
	scopeOverride = strings.TrimSpace(scopeOverride)
	if scopeOverride != "" {
		return scopeOverride
	}
	if task == nil {
		return ""
	}

	projectName := strings.TrimSpace(task.GetProjectName())
	sourceRoot := ""
	if task.GetSm() != nil {
		sourceRoot = strings.TrimSpace(task.GetSourceCodePath())
	}
	taskID := strings.TrimSpace(task.GetTaskId())
	actor := task.GetTaskActor()

	parts := make([]string, 0, 6)
	for _, item := range []string{
		projectName,
		sourceRoot,
		taskID,
		strings.TrimSpace(actor.GroupID),
		strings.TrimSpace(actor.DigitalHumanID),
		strings.TrimSpace(actor.AgentToolName),
	} {
		if item == "" {
			continue
		}
		parts = append(parts, item)
	}
	return strings.Join(parts, "|")
}

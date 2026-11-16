package toolCalling

import (
	"AIxVuln/llm"
	"strings"
)

func initializeEphemeralLookupTask(memory llm.Memory, contextID string, taskContent string) {
	memory.SetTaskList(&llm.TaskListX{
		ContextId: contextID,
		TaskList: []map[string]string{
			{
				"TaskContent": strings.TrimSpace(taskContent),
			},
		},
	})
}

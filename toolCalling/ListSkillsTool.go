package toolCalling

import (
	"AIxVuln/skill"
	"AIxVuln/taskManager"
)

type ListSkillsTool struct {
	task *taskManager.Task
}

func NewListSkillsTool(task *taskManager.Task) *ListSkillsTool {
	return &ListSkillsTool{task: task}
}

func (h *ListSkillsTool) Name() string {
	return "ListSkillsTool"
}

func (h *ListSkillsTool) Description() string {
	return "List the currently available local skills. Use this first when the task explicitly mentions a skill or when a reusable workflow/instruction set may help."
}

func (h *ListSkillsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (h *ListSkillsTool) Execute(parameters map[string]interface{}) string {
	items, err := skill.List()
	if err != nil {
		return Fail("list skills failed: " + err.Error())
	}
	return Success(items)
}

package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/skill"
	"AIxVuln/taskManager"
)

type ListSkillFilesTool struct {
	task *taskManager.Task
}

func NewListSkillFilesTool(task *taskManager.Task) *ListSkillFilesTool {
	return &ListSkillFilesTool{task: task}
}

func (h *ListSkillFilesTool) Name() string {
	return "ListSkillFilesTool"
}

func (h *ListSkillFilesTool) Description() string {
	return "List files inside a skill directory without reading the full contents. Use relative paths from the skill root. Call this only when SKILL.md indicates you need a specific script, reference, or asset."
}

func (h *ListSkillFilesTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"skillName": map[string]interface{}{
				"type":        "string",
				"description": "Skill directory name from ListSkillsTool.(required)",
			},
			"relativePath": map[string]interface{}{
				"type":        "string",
				"description": "Optional directory path relative to the skill root. Default './'.",
			},
			"maxDepth": map[string]interface{}{
				"type":        "integer",
				"description": "Optional recursive depth. Default 2, max 5.",
				"minimum":     1,
				"default":     2,
				"maximum":     5,
			},
		},
		"required": []string{"skillName"},
	}
}

func (h *ListSkillFilesTool) Execute(parameters map[string]interface{}) string {
	raw := parameters["skillName"]
	if raw == nil {
		return Fail("Missing 'skillName' parameter")
	}
	skillName, ok := raw.(string)
	if !ok || skillName == "" {
		return Fail("Invalid 'skillName' parameter")
	}
	maxDepth := 2
	if value := parameters["maxDepth"]; value != nil {
		n, err := misc.GetIntParam(value)
		if err != nil {
			return Fail("Invalid 'maxDepth' parameter")
		}
		maxDepth = n
	}
	list, err := skill.ListFiles(skillName, stringParam(parameters["relativePath"]), maxDepth)
	if err != nil {
		return Fail(err.Error())
	}
	return Success(list)
}

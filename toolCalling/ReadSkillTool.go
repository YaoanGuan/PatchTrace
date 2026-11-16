package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"fmt"
)

type ReadSkillTool struct {
	task *taskManager.Task
}

func NewReadSkillTool(task *taskManager.Task) *ReadSkillTool {
	return &ReadSkillTool{task: task}
}

func (h *ReadSkillTool) Name() string {
	return "ReadSkillTool"
}

func (h *ReadSkillTool) Description() string {
	return "Read a skill file incrementally. Markdown files are automatically distilled into key steps and critical info before being returned to reduce context waste. Start with SKILL.md, then read only the specific referenced files you actually need. Use relative paths from the skill root."
}

func (h *ReadSkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"skillName": map[string]interface{}{
				"type":        "string",
				"description": "Skill directory name from ListSkillsTool.(required)",
			},
			"relativePath": map[string]interface{}{
				"type":        "string",
				"description": "Optional file path relative to the skill root. Default 'SKILL.md'.",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Character offset to start reading from. Default 0.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of characters to return. Default 12000, max 30000.",
			},
		},
		"required": []string{"skillName"},
	}
}

func (h *ReadSkillTool) Execute(parameters map[string]interface{}) string {
	raw := parameters["skillName"]
	if raw == nil {
		return Fail("Missing 'skillName' parameter")
	}
	skillName, ok := raw.(string)
	if !ok || skillName == "" {
		return Fail("Invalid 'skillName' parameter")
	}
	offset, err := readSkillIntArg(parameters["offset"], 0)
	if err != nil {
		return Fail("Invalid 'offset' parameter: " + err.Error())
	}
	limit, err := readSkillIntArg(parameters["limit"], 12000)
	if err != nil {
		return Fail("Invalid 'limit' parameter: " + err.Error())
	}
	chunk, err := loadSkillChunkForAgent(h.task, skillName, stringParam(parameters["relativePath"]), offset, limit)
	if err != nil {
		return Fail(err.Error())
	}
	return Success(chunk)
}

func readSkillIntArg(value interface{}, defaultValue int) (int, error) {
	if value == nil {
		return defaultValue, nil
	}
	n, err := misc.GetIntParam(value)
	if err != nil {
		return 0, fmt.Errorf("expected integer")
	}
	return n, nil
}

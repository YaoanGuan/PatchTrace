package toolCalling

import (
	resourcepack "AIxVuln/resource"
	"AIxVuln/taskManager"
)

type ReadResourceDocTool struct {
	task *taskManager.Task
}

func NewReadResourceDocTool(task *taskManager.Task) *ReadResourceDocTool {
	return &ReadResourceDocTool{task: task}
}

func (h *ReadResourceDocTool) Name() string {
	return "ReadResourceDocTool"
}

func (h *ReadResourceDocTool) Description() string {
	return "Read a file from local resource/docs incrementally by relative path. Use this only after listing or searching so you read the smallest relevant chunks."
}

func (h *ReadResourceDocTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"relativePath": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to resource/docs.(required)",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Character offset to start reading from. Default 0.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of characters to return. Default 8000, max 30000.",
			},
		},
		"required": []string{"relativePath"},
	}
}

func (h *ReadResourceDocTool) Execute(parameters map[string]interface{}) string {
	relativePath := stringParam(parameters["relativePath"])
	if relativePath == "" {
		return Fail("Missing 'relativePath' parameter")
	}
	offset, err := readSkillIntArg(parameters["offset"], 0)
	if err != nil {
		return Fail("Invalid 'offset' parameter: " + err.Error())
	}
	limit, err := readSkillIntArg(parameters["limit"], 8000)
	if err != nil {
		return Fail("Invalid 'limit' parameter: " + err.Error())
	}
	chunk, err := resourcepack.ReadResourceDoc(relativePath, offset, limit)
	if err != nil {
		return Fail("read resource doc failed: " + err.Error())
	}
	return Success(chunk)
}

package toolCalling

import (
	resourcepack "AIxVuln/resource"
	"AIxVuln/taskManager"
)

type ReadPocKnowledgeDocTool struct {
	task *taskManager.Task
}

func NewReadPocKnowledgeDocTool(task *taskManager.Task) *ReadPocKnowledgeDocTool {
	return &ReadPocKnowledgeDocTool{task: task}
}

func (h *ReadPocKnowledgeDocTool) Name() string {
	return "ReadPocKnowledgeDocTool"
}

func (h *ReadPocKnowledgeDocTool) Description() string {
	return "Read a file from the local PoC and penetration-knowledge corpus in resource/pocs incrementally by relative path. Use this only after searching or listing so you read the smallest relevant chunks."
}

func (h *ReadPocKnowledgeDocTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"relativePath": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the local PoC corpus root.(required)",
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

func (h *ReadPocKnowledgeDocTool) Execute(parameters map[string]interface{}) string {
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
	chunk, err := resourcepack.ReadPocKnowledgeDoc(relativePath, offset, limit)
	if err != nil {
		return Fail("read poc knowledge doc failed: " + err.Error())
	}
	return Success(chunk)
}

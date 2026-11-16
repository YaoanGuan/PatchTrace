package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"fmt"
)

type SearchFileContentsByRegexTool struct {
	task *taskManager.Task
}

func NewSearchFileContentsByRegexTool(task *taskManager.Task) *SearchFileContentsByRegexTool {
	return &SearchFileContentsByRegexTool{task: task}
}

func (h *SearchFileContentsByRegexTool) Name() string {
	return "SearchFileContentsByRegexTool"
}
func (h *SearchFileContentsByRegexTool) Description() string {
	return "Recursively search the contents of all files in the specified directory, find code lines that match the regular expression, and return the results in JSON format. This feature is used for code auditing, locating specific function calls, or variable definitions."
}
func (h *SearchFileContentsByRegexTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Root directory path for the search (based on `/workdir/sourceCodeDir`. For example, for `/workdir/sourceCodeDir`, simply input `./`). Legacy `/sourceCodeDir` prefixes are also accepted.(required)",
			},
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Single‑line regular expression for matching.(required).",
			},
		},
	}
}

func (h *SearchFileContentsByRegexTool) Execute(parameters map[string]interface{}) string {
	pathTemp := parameters["path"]
	if pathTemp == nil {
		return Fail("Missing 'path' parameter")
	}
	path := pathTemp.(string)
	root := h.task.GetSourceCodePath()
	patternTemp := parameters["pattern"]
	var pattern string
	if patternTemp == nil {
		return Fail("Pattern parameter should not empty")
	}
	pattern = patternTemp.(string)
	path = misc.NormalizeVisibleSourcePath(path)
	path = root + "/" + path
	r, e := misc.SearchFileContentsByRegex(path, pattern)
	if e != nil {
		return Fail(sanitizeTextPaths(root, e.Error()))
	}
	for i := range r {
		r[i].FilePath = relPathForLLM(root, r[i].FilePath)
	}
	return Success(map[string]interface{}{
		"total_matches": len(r),
		"hint":          fmt.Sprintf("Found %d matching lines. If too many, narrow your regex or search path.", len(r)),
		"matches":       r,
	})
}

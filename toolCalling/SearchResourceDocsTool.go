package toolCalling

import (
	resourcepack "AIxVuln/resource"
	"AIxVuln/taskManager"
	"strings"
)

type SearchResourceDocsTool struct {
	task *taskManager.Task
}

func NewSearchResourceDocsTool(task *taskManager.Task) *SearchResourceDocsTool {
	return &SearchResourceDocsTool{task: task}
}

func (h *SearchResourceDocsTool) Name() string {
	return "SearchResourceDocsTool"
}

func (h *SearchResourceDocsTool) Description() string {
	return "Search text content under local resource/docs by keyword terms and return ranked matching snippets. Use this to find the exact section before reading a document chunk."
}

func (h *SearchResourceDocsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Keyword query to search for inside resource/docs.(required)",
			},
			"pathPrefix": map[string]interface{}{
				"type":        "string",
				"description": "Optional relative subdirectory prefix under resource/docs.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of hits to return. Default 20, max 100.",
			},
		},
		"required": []string{"query"},
	}
}

func (h *SearchResourceDocsTool) Execute(parameters map[string]interface{}) string {
	query := strings.TrimSpace(stringParam(parameters["query"]))
	if query == "" {
		return Fail("Missing 'query' parameter")
	}
	limit := 20
	if raw := parameters["limit"]; raw != nil {
		n, err := readSkillIntArg(raw, 20)
		if err != nil {
			return Fail("Invalid 'limit' parameter: " + err.Error())
		}
		limit = n
	}
	hits, err := resourcepack.SearchResourceDocs(query, stringParam(parameters["pathPrefix"]), limit)
	if err != nil {
		return Fail("search resource docs failed: " + err.Error())
	}
	return Success(map[string]interface{}{
		"query":       query,
		"path_prefix": normalizeResourceDocFilterPath(stringParam(parameters["pathPrefix"])),
		"hit_count":   len(hits),
		"hits":        hits,
	})
}

package toolCalling

import (
	resourcepack "AIxVuln/resource"
	"AIxVuln/taskManager"
	"strings"
)

type SearchPocKnowledgeDocsTool struct {
	task *taskManager.Task
}

func NewSearchPocKnowledgeDocsTool(task *taskManager.Task) *SearchPocKnowledgeDocsTool {
	return &SearchPocKnowledgeDocsTool{task: task}
}

func (h *SearchPocKnowledgeDocsTool) Name() string {
	return "SearchPocKnowledgeDocsTool"
}

func (h *SearchPocKnowledgeDocsTool) Description() string {
	return "Search text content under the local PoC and penetration-knowledge corpus in resource/pocs by keyword terms and return ranked matching snippets."
}

func (h *SearchPocKnowledgeDocsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Keyword query to search for inside the local PoC corpus.(required)",
			},
			"pathPrefix": map[string]interface{}{
				"type":        "string",
				"description": "Optional relative subdirectory prefix under the local PoC corpus.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of hits to return. Default 20, max 100.",
			},
		},
		"required": []string{"query"},
	}
}

func (h *SearchPocKnowledgeDocsTool) Execute(parameters map[string]interface{}) string {
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
	hits, err := resourcepack.SearchPocKnowledgeDocs(query, stringParam(parameters["pathPrefix"]), limit)
	if err != nil {
		return Fail("search poc knowledge docs failed: " + err.Error())
	}
	return Success(map[string]interface{}{
		"roots":       resourcepack.PocKnowledgeRoots(),
		"query":       query,
		"path_prefix": normalizeResourceDocFilterPath(stringParam(parameters["pathPrefix"])),
		"hit_count":   len(hits),
		"hits":        hits,
	})
}

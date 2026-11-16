package toolCalling

import (
	resourcepack "AIxVuln/resource"
	"AIxVuln/taskManager"
	"sort"
	"strings"
)

type ListResourceDocsTool struct {
	task *taskManager.Task
}

func NewListResourceDocsTool(task *taskManager.Task) *ListResourceDocsTool {
	return &ListResourceDocsTool{task: task}
}

func (h *ListResourceDocsTool) Name() string {
	return "ListResourceDocsTool"
}

func (h *ListResourceDocsTool) Description() string {
	return "List candidate documentation files under local resource/docs. Use this first to narrow down likely tool/MCP/workflow docs before searching or reading."
}

func (h *ListResourceDocsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pathPrefix": map[string]interface{}{
				"type":        "string",
				"description": "Optional relative subdirectory prefix under resource/docs to narrow the listing.",
			},
			"keyword": map[string]interface{}{
				"type":        "string",
				"description": "Optional fuzzy keyword filter matched against file path and filename.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of files to return. Default 50, max 200.",
			},
		},
	}
}

func (h *ListResourceDocsTool) Execute(parameters map[string]interface{}) string {
	items, err := resourcepack.ListResourceDocs()
	if err != nil {
		return Fail("list resource docs failed: " + err.Error())
	}
	pathPrefix := normalizeResourceDocFilterPath(stringParam(parameters["pathPrefix"]))
	keyword := strings.ToLower(strings.TrimSpace(stringParam(parameters["keyword"])))
	limit := 50
	if raw := parameters["limit"]; raw != nil {
		n, err := readSkillIntArg(raw, 50)
		if err != nil {
			return Fail("Invalid 'limit' parameter: " + err.Error())
		}
		limit = n
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	filtered := make([]resourcepack.ResourceDoc, 0, len(items))
	for _, item := range items {
		rel := strings.TrimSpace(item.RelativePath)
		if pathPrefix != "" && rel != pathPrefix && !strings.HasPrefix(rel, pathPrefix+"/") {
			continue
		}
		if keyword != "" {
			candidate := strings.ToLower(rel + "\n" + item.Name)
			if !strings.Contains(candidate, keyword) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].RelativePath < filtered[j].RelativePath
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return Success(map[string]interface{}{
		"path_prefix": pathPrefix,
		"keyword":     strings.TrimSpace(stringParam(parameters["keyword"])),
		"item_count":  len(filtered),
		"items":       filtered,
	})
}

func normalizeResourceDocFilterPath(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = strings.TrimPrefix(raw, "/")
	raw = strings.Trim(raw, " ")
	raw = strings.TrimSuffix(raw, "/")
	if raw == "." {
		return ""
	}
	return raw
}

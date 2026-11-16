package toolCalling

import (
	"AIxVuln/misc"
	resourcepack "AIxVuln/resource"
	"AIxVuln/taskManager"
	"strings"
)

type ListAgentToolsTool struct {
	task *taskManager.Task
}

type agentToolsCatalogQuery struct {
	ToolID          string
	Category        string
	Keyword         string
	IncludeExamples bool
}

func NewListAgentToolsTool(task *taskManager.Task) *ListAgentToolsTool {
	return &ListAgentToolsTool{task: task}
}

func (h *ListAgentToolsTool) Name() string {
	return "ListAgentToolsTool"
}

func (h *ListAgentToolsTool) Description() string {
	return "List the built-in downloadable open-source helper tools exposed by the system. Call this only when the current task or skill actually needs such a tool."
}

func (h *ListAgentToolsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"toolId": map[string]interface{}{
				"type":        "string",
				"description": "Optional exact tool id filter, for example 'libgadget'.",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Optional category filter, for example 'android'.",
			},
			"keyword": map[string]interface{}{
				"type":        "string",
				"description": "Optional fuzzy keyword filter matched against id, name, filename, category, and description.",
			},
			"includeExamples": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to include verbose curl examples in the output. Default false.",
			},
		},
	}
}

func (h *ListAgentToolsTool) Execute(parameters map[string]interface{}) string {
	payload, err := buildAgentToolsCatalogPayload(agentToolsCatalogQuery{
		ToolID:          stringParam(parameters["toolId"]),
		Category:        stringParam(parameters["category"]),
		Keyword:         stringParam(parameters["keyword"]),
		IncludeExamples: boolParam(parameters["includeExamples"]),
	})
	if err != nil {
		return Fail("list agent tools failed: " + err.Error())
	}
	return Success(payload)
}

func buildAgentToolsCatalogPayload(query agentToolsCatalogQuery) (map[string]interface{}, error) {
	items, err := resourcepack.ListAgentTools()
	if err != nil {
		return nil, err
	}
	baseURL := misc.GetAgentToolsBaseURL()
	toolID := strings.TrimSpace(query.ToolID)
	category := strings.TrimSpace(query.Category)
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	entries := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if toolID != "" && !strings.EqualFold(item.ID, toolID) {
			continue
		}
		if category != "" && !strings.EqualFold(item.Category, category) {
			continue
		}
		if keyword != "" && !matchesAgentToolKeyword(item, keyword) {
			continue
		}
		downloadPath := "/agent_tools/" + item.ID + "/download"
		downloadURL := downloadPath
		if baseURL != "" {
			downloadURL = strings.TrimRight(baseURL, "/") + downloadPath
		}
		entry := map[string]interface{}{
			"id":            item.ID,
			"name":          item.Name,
			"filename":      item.Filename,
			"category":      item.Category,
			"description":   item.Description,
			"size":          item.Size,
			"sha256":        item.SHA256,
			"download_path": downloadPath,
			"download_url":  downloadURL,
		}
		if query.IncludeExamples {
			entry["curl_example"] = "curl -L '" + downloadURL + "' -o ./" + item.Filename
		}
		entries = append(entries, entry)
	}
	return map[string]interface{}{
		"usage":    "仅在任务或 skill 真的需要系统内置辅助工具时再调用本工具。下载时直接使用返回的 download_url，不要自行猜测地址。",
		"base_url": baseURL,
		"filters": map[string]interface{}{
			"tool_id":          toolID,
			"category":         category,
			"keyword":          strings.TrimSpace(query.Keyword),
			"include_examples": query.IncludeExamples,
		},
		"item_count": len(entries),
		"items":      entries,
	}, nil
}

func matchesAgentToolKeyword(item resourcepack.AgentTool, keyword string) bool {
	return strings.Contains(strings.ToLower(item.ID), keyword) ||
		strings.Contains(strings.ToLower(item.Name), keyword) ||
		strings.Contains(strings.ToLower(item.Filename), keyword) ||
		strings.Contains(strings.ToLower(item.Category), keyword) ||
		strings.Contains(strings.ToLower(item.Description), keyword)
}

func boolParam(raw interface{}) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		}
	}
	return false
}

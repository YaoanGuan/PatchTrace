package taskManager

import "strings"

func BuildBackgroundProcessPanel(projectName string) map[string]interface{} {
	return BuildBackgroundProcessPanelFromSnapshots(ListActiveBackgroundProcesses(projectName))
}

func BuildBackgroundProcessPanelFromSnapshots(items []BackgroundProcessSnapshot) map[string]interface{} {
	type brief struct {
		SessionID       string `json:"session_id"`
		Source          string `json:"source,omitempty"`
		Status          string `json:"status"`
		Running         bool   `json:"running"`
		PID             int    `json:"pid,omitempty"`
		CommandPreview  string `json:"command_preview,omitempty"`
		StartedAt       string `json:"started_at,omitempty"`
		AgentName       string `json:"agent_name,omitempty"`
		PersonaName     string `json:"persona_name,omitempty"`
		GroupName       string `json:"group_name,omitempty"`
		RequesterSource string `json:"requester_source,omitempty"`
	}

	briefs := make([]brief, 0, len(items))
	for _, item := range items {
		briefs = append(briefs, brief{
			SessionID:       strings.TrimSpace(item.SessionID),
			Source:          strings.TrimSpace(item.Source),
			Status:          strings.TrimSpace(item.Status),
			Running:         item.Running,
			PID:             item.PID,
			CommandPreview:  strings.TrimSpace(item.CommandPreview),
			StartedAt:       strings.TrimSpace(item.StartedAt),
			AgentName:       strings.TrimSpace(item.AgentName),
			PersonaName:     strings.TrimSpace(item.PersonaName),
			GroupName:       strings.TrimSpace(item.GroupName),
			RequesterSource: strings.TrimSpace(item.RequesterSource),
		})
	}

	return map[string]interface{}{
		"active_count": len(briefs),
		"items":        briefs,
		"usage_hint":   "这里列出当前项目/实验室仍在运行的后台命令。所有 Agent 都可以直接复用这些短 session_id，并通过 Read/Write/TerminateBackgroundCommandTool 共享控制 stdio。",
	}
}

func BuildGlobalStatusPanel(projectName string) map[string]interface{} {
	return BuildGlobalStatusPanelFromSnapshots(ListActiveBackgroundProcesses(projectName))
}

func BuildGlobalStatusPanelFromSnapshots(items []BackgroundProcessSnapshot) map[string]interface{} {
	return map[string]interface{}{
		"notice":               "这是项目级全局状态面板，后续可继续追加其它全局共享状态。",
		"background_processes": BuildBackgroundProcessPanelFromSnapshots(items),
	}
}

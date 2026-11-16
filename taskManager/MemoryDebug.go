package taskManager

type MemoryDebugTarget struct {
	TargetID       string `json:"target_id"`
	GroupID        string `json:"group_id,omitempty"`
	GroupName      string `json:"group_name,omitempty"`
	TargetType     string `json:"target_type"`
	Label          string `json:"label"`
	PersonaName    string `json:"persona_name,omitempty"`
	AgentToolName  string `json:"agent_tool_name,omitempty"`
	DigitalHumanID string `json:"digital_human_id,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	State          string `json:"state,omitempty"`
	Available      bool   `json:"available"`
	Reason         string `json:"reason,omitempty"`
}

type MemoryDebugExport struct {
	Target     MemoryDebugTarget `json:"target"`
	ExportedAt string            `json:"exported_at"`
	Content    string            `json:"content"`
}

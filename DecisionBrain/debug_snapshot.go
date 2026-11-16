package DecisionBrain

import (
	"AIxVuln/agents"
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"fmt"
	"sort"
	"strings"
	"time"
)

type BrainMemoryDebugSnapshot struct {
	MemoryType             string                   `json:"memory_type"`
	SystemPrompt           string                   `json:"system_prompt,omitempty"`
	PinnedMessages         []llm.Message            `json:"pinned_messages,omitempty"`
	TaskContent            string                   `json:"task_content,omitempty"`
	CompressedSummary      string                   `json:"compressed_summary,omitempty"`
	KeyMessages            map[string][]interface{} `json:"key_messages,omitempty"`
	MaxHistory             int                      `json:"max_history"`
	MessageSize            int                      `json:"message_size"`
	NoAgentRunning         bool                     `json:"no_agent_running"`
	LastUserPrompt         string                   `json:"last_user_prompt,omitempty"`
	AgentRuntimeInfo       interface{}              `json:"agent_runtime_info,omitempty"`
	EnvInfo                interface{}              `json:"env_info,omitempty"`
	ExploitIdeaList        interface{}              `json:"exploit_idea_list,omitempty"`
	ExploitChainList       interface{}              `json:"exploit_chain_list,omitempty"`
	ContainerInfo          interface{}              `json:"container_info,omitempty"`
	LastPanelSectionHash   map[string]string        `json:"last_panel_section_hash,omitempty"`
	LastPanelSectionOutput map[string]string        `json:"last_panel_section_content,omitempty"`
	ContextBreakdown       ContextBreakdown         `json:"context_breakdown"`
	Turns                  []llm.Turn               `json:"turns"`
	FlatMessages           []llm.Message            `json:"flat_messages"`
	RenderedContext        []llm.Message            `json:"rendered_context"`
	RenderedContextSize    int                      `json:"rendered_context_tokens"`
}

func (br *BrainMemory) ExportDebugSnapshot() BrainMemoryDebugSnapshot {
	br.mu.RLock()
	turns := append([]llm.Turn(nil), br.memory...)
	pinnedMessages := append([]llm.Message(nil), br.pinnedMessages...)
	keyMessages := cloneBrainKeyMessages(br.keyMessage)
	systemPrompt := br.systemPrompt
	taskContent := br.taskContent
	compressedSummary := br.compressedSummary
	maxHistory := br.maxHistory
	msgSize := br.msgSize
	noAgentRunning := br.noAgentRunning
	lastUserPrompt := br.lastUserPrompt
	agentRuntimeInfo := br.agentRuntimeInfoJson
	envInfo := br.envInfoJson
	exploitIdeaList := br.exploitIdeaListJson
	exploitChainList := br.exploitChainListJson
	containerInfo := br.containerInfoList
	lastPanelSectionHash := cloneStringMap(br.lastPanelSectionHash)
	lastPanelSectionContent := cloneStringMap(br.lastPanelSectionContent)
	contextBreakdown := br.contextBreakdown
	enableExploitFlow := br.enableExploitFlow
	br.mu.RUnlock()
	if !enableExploitFlow {
		exploitIdeaList = ""
		exploitChainList = ""
		delete(lastPanelSectionHash, "exploit_idea_list")
		delete(lastPanelSectionHash, "exploit_chain_list")
		delete(lastPanelSectionContent, "exploit_idea_list")
		delete(lastPanelSectionContent, "exploit_chain_list")
	}

	flatMessages := llm.FlattenTurns(turns)
	renderedContext := br.GetContext()
	var exploitIdeaSnapshot interface{}
	var exploitChainSnapshot interface{}
	if enableExploitFlow {
		exploitIdeaSnapshot = llm.DecodeJSONStringOrRaw(exploitIdeaList)
		exploitChainSnapshot = llm.DecodeJSONStringOrRaw(exploitChainList)
	}
	return BrainMemoryDebugSnapshot{
		MemoryType:             "BrainMemory",
		SystemPrompt:           systemPrompt,
		PinnedMessages:         pinnedMessages,
		TaskContent:            taskContent,
		CompressedSummary:      compressedSummary,
		KeyMessages:            keyMessages,
		MaxHistory:             maxHistory,
		MessageSize:            msgSize,
		NoAgentRunning:         noAgentRunning,
		LastUserPrompt:         lastUserPrompt,
		AgentRuntimeInfo:       llm.DecodeJSONStringOrRaw(agentRuntimeInfo),
		EnvInfo:                llm.DecodeJSONStringOrRaw(envInfo),
		ExploitIdeaList:        exploitIdeaSnapshot,
		ExploitChainList:       exploitChainSnapshot,
		ContainerInfo:          llm.DecodeJSONStringOrRaw(containerInfo),
		LastPanelSectionHash:   lastPanelSectionHash,
		LastPanelSectionOutput: lastPanelSectionContent,
		ContextBreakdown:       contextBreakdown,
		Turns:                  turns,
		FlatMessages:           flatMessages,
		RenderedContext:        renderedContext,
		RenderedContextSize:    llm.CountMessagesTokens(renderedContext),
	}
}

func (br *BrainMemory) CaptureLastLLMContext(messages []llm.Message) {
	if br == nil {
		return
	}
	br.mu.Lock()
	defer br.mu.Unlock()
	br.lastLLMContext = append(br.lastLLMContext[:0], llmCloneMessages(messages)...)
}

func (br *BrainMemory) ExportLastLLMContextText() string {
	if br == nil {
		return ""
	}
	br.mu.RLock()
	captured := llmCloneMessages(br.lastLLMContext)
	br.mu.RUnlock()
	if len(captured) == 0 {
		captured = br.GetContext()
	}
	return llm.RenderMessagesForDebugText(captured)
}

func (db *DecisionBrain) ListMemoryDebugTargets() []taskManager.MemoryDebugTarget {
	targets := make([]taskManager.MemoryDebugTarget, 0)
	targets = append(targets, taskManager.MemoryDebugTarget{
		TargetID:   "decision_brain",
		TargetType: "decision_brain",
		Label:      "决策大脑",
		Available:  db != nil && db.memory != nil,
		State:      decisionBrainDebugState(db),
	})

	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	seen := make(map[string]struct{})
	appendOne := func(agentToolName string, digitalHumanID string, personaName string, agentID string, state string, available bool) {
		key := strings.TrimSpace(digitalHumanID)
		if key == "" {
			key = strings.TrimSpace(agentToolName) + "::" + strings.TrimSpace(personaName)
		}
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		targets = append(targets, taskManager.MemoryDebugTarget{
			TargetID:       "digital_human:" + key,
			TargetType:     "digital_human",
			Label:          strings.TrimSpace(personaName),
			PersonaName:    strings.TrimSpace(personaName),
			AgentToolName:  strings.TrimSpace(agentToolName),
			DigitalHumanID: strings.TrimSpace(digitalHumanID),
			AgentID:        strings.TrimSpace(agentID),
			State:          strings.TrimSpace(state),
			Available:      available,
			Reason:         unavailableReason(available, "数字人实例不可用"),
		})
	}

	for toolName, entries := range db.digitalHumanPool {
		for _, entry := range entries {
			agentID := ""
			available := entry.Agent != nil && entry.Agent.GetMemory() != nil && entry.Agent.GetTask() != nil
			if entry.Agent != nil {
				agentID = entry.Agent.GetId()
			}
			appendOne(toolName, entry.Profile.DigitalHumanID, entry.Profile.PersonaName, agentID, "idle", available)
		}
	}
	for aid, busy := range db.digitalHumanBusy {
		available := busy.Agent != nil && busy.Agent.GetMemory() != nil && busy.Agent.GetTask() != nil
		appendOne(busy.AgentToolName, busy.Profile.DigitalHumanID, busy.Profile.PersonaName, aid, "busy", available)
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].TargetType != targets[j].TargetType {
			return targets[i].TargetType < targets[j].TargetType
		}
		if targets[i].Label != targets[j].Label {
			return targets[i].Label < targets[j].Label
		}
		return targets[i].TargetID < targets[j].TargetID
	})
	return targets
}

func (db *DecisionBrain) ExportMemoryDebug(localTargetID string) (*taskManager.MemoryDebugExport, error) {
	if db == nil {
		return nil, fmt.Errorf("decision brain is nil")
	}
	localTargetID = strings.TrimSpace(localTargetID)
	if localTargetID == "" {
		return nil, fmt.Errorf("target id is required")
	}
	if localTargetID == "decision_brain" {
		if db.memory == nil {
			return nil, fmt.Errorf("decision brain memory is unavailable")
		}
		return &taskManager.MemoryDebugExport{
			Target: taskManager.MemoryDebugTarget{
				TargetID:   localTargetID,
				TargetType: "decision_brain",
				Label:      "决策大脑",
				Available:  true,
				State:      decisionBrainDebugState(db),
			},
			ExportedAt: time.Now().Format("2006-01-02 15:04:05"),
			Content:    db.memory.ExportLastLLMContextText(),
		}, nil
	}
	if !strings.HasPrefix(localTargetID, "digital_human:") {
		return nil, fmt.Errorf("unsupported memory debug target: %s", localTargetID)
	}
	identity := strings.TrimSpace(strings.TrimPrefix(localTargetID, "digital_human:"))
	agentToolName, profile, agent := db.findDigitalHumanByDebugIdentity(identity)
	if agent == nil {
		return nil, fmt.Errorf("digital human not found: %s", identity)
	}
	text, err := llm.ExportMemoryDebugText(agent.GetMemory(), agent.GetTask().GetTaskId())
	if err != nil {
		return nil, err
	}
	target := taskManager.MemoryDebugTarget{
		TargetID:       localTargetID,
		TargetType:     "digital_human",
		Label:          strings.TrimSpace(profile.PersonaName),
		PersonaName:    strings.TrimSpace(profile.PersonaName),
		AgentToolName:  strings.TrimSpace(agentToolName),
		DigitalHumanID: strings.TrimSpace(profile.DigitalHumanID),
		AgentID:        strings.TrimSpace(agent.GetId()),
		State:          strings.TrimSpace(agent.GetState()),
		Available:      true,
	}
	return &taskManager.MemoryDebugExport{
		Target:     target,
		ExportedAt: time.Now().Format("2006-01-02 15:04:05"),
		Content:    text,
	}, nil
}

func (db *DecisionBrain) findDigitalHumanByDebugIdentity(identity string) (string, agents.AgentProfile, agents.Agent) {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	for toolName, entries := range db.digitalHumanPool {
		for _, entry := range entries {
			if digitalHumanIdentityKey(toolName, entry.Profile) == identity {
				return toolName, entry.Profile, entry.Agent
			}
		}
	}
	for _, busy := range db.digitalHumanBusy {
		if digitalHumanIdentityKey(busy.AgentToolName, busy.Profile) == identity {
			return busy.AgentToolName, busy.Profile, busy.Agent
		}
	}
	return "", agents.AgentProfile{}, nil
}

func decisionBrainDebugState(db *DecisionBrain) string {
	if db == nil {
		return ""
	}
	if db.IsBrainFinished() {
		return "finished"
	}
	if db.hasRunningAgents() {
		return "running"
	}
	return "idle"
}

func unavailableReason(available bool, reason string) string {
	if available {
		return ""
	}
	return reason
}

func llmCloneMessages(in []llm.Message) []llm.Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]llm.Message, len(in))
	copy(out, in)
	for i := range out {
		if len(out[i].ToolCalls) == 0 {
			continue
		}
		calls := make([]llm.ToolCall, len(out[i].ToolCalls))
		copy(calls, out[i].ToolCalls)
		out[i].ToolCalls = calls
	}
	return out
}

func cloneBrainKeyMessages(in map[string][]interface{}) map[string][]interface{} {
	if len(in) == 0 {
		return map[string][]interface{}{}
	}
	out := make(map[string][]interface{}, len(in))
	for key, values := range in {
		cp := make([]interface{}, len(values))
		copy(cp, values)
		out[key] = cp
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

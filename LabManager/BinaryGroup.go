package LabManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/agents"
	binaryagents "AIxVuln/agents/binary"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"context"
	"fmt"
	"strings"
	"time"
)

func binaryDecisionBrainSystemPrompt(groupName string) string {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		groupName = binaryGroupName
	}
	return prompts.BinaryDecisionBrainSystemPrompt(
		groupName,
		groupDecisionBrainKnowledgePrompt(),
		groupDecisionBrainRelayRulePrompt(),
	)
}

func binaryGroupProjectKey(labName string) string {
	labName = strings.TrimSpace(labName)
	if labName == "" {
		labName = "lab"
	}
	return labName + "_" + binaryGroupID
}

func defaultBinaryMembers() []binaryMember {
	return []binaryMember{
		{
			ToolName:       "Agent-Binary-ReverseCommonAgent",
			PersonaName:    "顾凌川",
			Gender:         "男",
			AvatarFile:     "analyze-1.png",
			Personality:    "偏执细致、擅长逆向路径拆解与调用关系复盘",
			Age:            27,
			Role:           "逆向分析研究员",
			DigitalHumanID: "binary-reverse-primary",
			AgentID:        "binary-reverse-primary",
		},
		{
			ToolName:       "Agent-Binary-AndroidCommonAgent",
			PersonaName:    "程见岳",
			Gender:         "男",
			AvatarFile:     "analyze-2.png",
			Personality:    "熟悉 APK 结构与移动端组件边界，擅长梳理 Android 攻击面",
			Age:            26,
			Role:           "Android逆向研究员",
			DigitalHumanID: "binary-android-primary",
			AgentID:        "binary-android-primary",
		},
	}
}

func buildBinaryGroupDecisionBrainConfig() groupDecisionBrainConfig {
	defs, _ := binaryagents.GetAgentDescription()
	return groupDecisionBrainConfig{
		SystemPrompt:          binaryDecisionBrainSystemPrompt(binaryGroupName),
		EnableExploitWorkflow: false,
		AgentDefinitions:      defs,
		DigitalHumanPool:      loadBinaryGroupDigitalHumanPool(defs),
	}
}

func loadBinaryGroupDigitalHumanPool(defs []agents.AgentDefinition) map[string][]agents.AgentProfile {
	allowed := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.Name) == "" {
			continue
		}
		allowed[def.Name] = struct{}{}
	}
	rowsByType := misc.GetAllDigitalHumans()
	pool := make(map[string][]agents.AgentProfile)
	for agentType, rows := range rowsByType {
		if _, ok := allowed[agentType]; !ok {
			continue
		}
		seen := make(map[string]struct{}, len(rows))
		profiles := make([]agents.AgentProfile, 0, len(rows))
		for _, r := range rows {
			identity := strings.TrimSpace(r.ID)
			if identity == "" {
				identity = strings.TrimSpace(r.PersonaName) + "\x00" + strings.TrimSpace(r.AvatarFile)
			}
			key := strings.TrimSpace(agentType) + "\x00" + identity
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			profiles = append(profiles, agents.AgentProfile{
				DigitalHumanID: r.ID,
				PersonaName:    r.PersonaName,
				Gender:         r.Gender,
				AvatarFile:     r.AvatarFile,
				Personality:    r.Personality,
				Age:            r.Age,
				ExtraSysPrompt: r.ExtraSysPrompt,
			})
		}
		pool[agentType] = profiles
	}
	return pool
}

func newBinarySecurityGroup(labName string, taskContent string, emitGroupChat func(DecisionBrain.ChatMessage), notifyChange func()) *BinarySecurityGroup {
	projectKey := binaryGroupProjectKey(labName)
	taskManager.SetRuntimeAlias(projectKey, labName)
	return &BinarySecurityGroup{
		status:        "待命",
		brainRunning:  false,
		brainFinished: false,
		projectKey:    projectKey,
		taskContent:   strings.TrimSpace(taskContent),
		brainConfig:   buildBinaryGroupDecisionBrainConfig(),
		roster:        defaultBinaryMembers(),
		emitGroupChat: emitGroupChat,
		notifyChange:  notifyChange,
	}
}

func (bg *BinarySecurityGroup) ID() string {
	return binaryGroupID
}

func (bg *BinarySecurityGroup) Name() string {
	return binaryGroupName
}

func (bg *BinarySecurityGroup) Kind() string {
	return binaryGroupKind
}

func (bg *BinarySecurityGroup) DecisionBrainName() string {
	return decisionBrainDisplayName(bg.Name())
}

func (bg *BinarySecurityGroup) Status() string {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.status
}

func (bg *BinarySecurityGroup) IsRunning() bool {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.brainRunning
}

func (bg *BinarySecurityGroup) BrainFinished() bool {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.brainFinished
}

func (bg *BinarySecurityGroup) IsDemo() bool {
	return false
}

func (bg *BinarySecurityGroup) HasMember(persona string) bool {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	for _, member := range bg.roster {
		if member.PersonaName == persona {
			return true
		}
	}
	return false
}

func (bg *BinarySecurityGroup) HandleDecisionChat(raw string, sender string) string {
	task := strings.TrimSpace(raw)
	if task == "" {
		return ""
	}
	if strings.TrimSpace(sender) == totalBrainName {
		bg.teamChatFromSenderWithPrompt(task, buildDecisionCoordinationPrompt(bg, sender, task), sender)
		return bg.coordinationAck(sender)
	}
	bg.teamChatFromSender(task, sender)
	return bg.coordinationAck(sender)
}

func (bg *BinarySecurityGroup) HandleGroupChat(raw string, sender string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "empty message"
	}
	return bg.teamChatFromSender(bg.normalizeInboundGroupChat(raw), sender)
}

func (bg *BinarySecurityGroup) NotifyDecisionBrainEvent(content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	bg.mu.Lock()
	brain := bg.prepareBrainForMessageLocked()
	bg.mu.Unlock()

	brain.NotifyDecisionBrainEvent(content)

	bg.mu.Lock()
	bg.ensureBrainRunnerLocked()
	bg.mu.Unlock()
}

func (bg *BinarySecurityGroup) NormalizeGroupChatMessage(msg DecisionBrain.ChatMessage) DecisionBrain.ChatMessage {
	msg.ChannelID = groupChannelID(bg.ID())
	msg.ChannelName = groupChannelName(bg.Name())
	msg.GroupID = bg.ID()
	msg.GroupName = bg.Name()
	if msg.Role == "user" {
		msg.SenderType = "user"
		return msg
	}
	if strings.TrimSpace(msg.PersonaName) == totalBrainName {
		msg.SenderType = "total_brain"
		return msg
	}
	if bg.isDecisionBrainPersona(strings.TrimSpace(msg.PersonaName)) {
		msg.PersonaName = bg.DecisionBrainName()
		msg.SenderType = "group_brain"
		return msg
	}
	if strings.TrimSpace(msg.PersonaName) == "" {
		msg.SenderType = "system"
		return msg
	}
	msg.SenderType = "member"
	return msg
}

func (bg *BinarySecurityGroup) StartTask() {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if bg.brain == nil || bg.status == "已停止" {
		bg.brain = bg.newDecisionBrain()
	}
	if !bg.brainRunning && bg.status != "已停止" {
		bg.status = "待命"
	}
	bg.brainFinished = false
	bg.notifyChangeLocked()
}

func (bg *BinarySecurityGroup) StopTask() {
	bg.mu.Lock()
	brain := bg.brain
	bg.brain = nil
	bg.status = "已停止"
	bg.brainRunning = false
	bg.brainFinished = false
	bg.mu.Unlock()
	if brain != nil {
		brain.Stop()
	}
	if bg.notifyChange != nil {
		bg.notifyChange()
	}
}

func (bg *BinarySecurityGroup) isDecisionBrainPersona(persona string) bool {
	persona = strings.TrimSpace(persona)
	return persona == bg.DecisionBrainName() || persona == "决策大脑"
}

func (bg *BinarySecurityGroup) GetDigitalHumanRoster() map[string]interface{} {
	brain, roster, status, _, _, _, _ := bg.snapshotRuntimeState()
	return bg.buildRosterFromState(brain, roster, status)
}

func (bg *BinarySecurityGroup) GetAgentRuntimeList() []map[string]interface{} {
	brain, roster, status, brainRunning, _, lastTask, lastSummary := bg.snapshotRuntimeState()
	return bg.buildAgentRuntimeListFromState(brain, roster, status, brainRunning, lastTask, lastSummary)
}

func (bg *BinarySecurityGroup) SharedReportCatalog() []taskManager.ReportMeta {
	bg.mu.RLock()
	brain := bg.brain
	bg.mu.RUnlock()
	if brain == nil {
		return nil
	}
	return brain.GetReportList()
}

func (bg *BinarySecurityGroup) NotifySharedReportUpdate(report taskManager.ReportMeta, sourceGroupID string, sourceGroupName string) {
	if strings.TrimSpace(sourceGroupID) == bg.ID() {
		return
	}
	bg.mu.RLock()
	brain := bg.brain
	running := bg.brainRunning
	bg.mu.RUnlock()
	if !running || brain == nil {
		return
	}
	brain.NotifyDecisionBrainEvent(buildSharedReportWakePrompt(sourceGroupName, report))
}

func (bg *BinarySecurityGroup) GetEvent(count int) []string {
	bg.mu.RLock()
	brain := bg.brain
	bg.mu.RUnlock()
	if brain == nil {
		return nil
	}
	return brain.GetEvent(count)
}

func (bg *BinarySecurityGroup) GetBrainFeed(count int) []map[string]interface{} {
	bg.mu.RLock()
	brain := bg.brain
	bg.mu.RUnlock()
	if brain == nil {
		return nil
	}
	return brain.GetBrainFeed(count)
}

func (bg *BinarySecurityGroup) GetAgentFeed(agentID string, count int) []map[string]interface{} {
	bg.mu.RLock()
	brain := bg.brain
	bg.mu.RUnlock()
	if brain == nil {
		return nil
	}
	return brain.GetAgentFeed(agentID, count)
}

func (bg *BinarySecurityGroup) GetTokenUsageSnapshot() taskManager.TokenUsageSnapshot {
	bg.mu.RLock()
	brain := bg.brain
	bg.mu.RUnlock()
	if brain == nil {
		return taskManager.TokenUsageSnapshot{}
	}
	return brain.GetTokenUsageSnapshot()
}

func (bg *BinarySecurityGroup) GetContextBreakdown() DecisionBrain.ContextBreakdown {
	bg.mu.RLock()
	brain := bg.brain
	bg.mu.RUnlock()
	if brain == nil {
		return DecisionBrain.ContextBreakdown{}
	}
	return brain.GetContextBreakdown()
}

func (bg *BinarySecurityGroup) PersistedState() taskManager.GroupState {
	bg.mu.RLock()
	brain := bg.brain
	out := taskManager.GroupState{
		GroupID:       bg.ID(),
		GroupKind:     bg.Kind(),
		Status:        bg.status,
		BrainFinished: bg.brainFinished,
		LastTask:      strings.TrimSpace(bg.lastTask),
		LastSummary:   strings.TrimSpace(bg.lastSummary),
	}
	bg.mu.RUnlock()
	if brain != nil {
		out.TokenUsage = brain.GetTokenUsageSnapshot()
		out.ReportCatalog = brain.GetReportList()
	}
	return out
}

func (bg *BinarySecurityGroup) RestorePersistedState(state taskManager.GroupState) {
	bg.mu.Lock()
	if bg.brain == nil && (state.BrainFinished || len(state.ReportCatalog) > 0 || state.TokenUsage.TotalTokens > 0 || strings.TrimSpace(state.LastTask) != "" || strings.TrimSpace(state.LastSummary) != "") {
		bg.brain = bg.newDecisionBrain()
	}
	brain := bg.brain
	bg.status = strings.TrimSpace(state.Status)
	switch bg.status {
	case "", "项目速览中", "调度中", "定向沟通中", "组内研判中", "正在运行", "决策结束", "运行结束":
		bg.status = "待命"
	}
	bg.lastTask = strings.TrimSpace(state.LastTask)
	bg.lastSummary = strings.TrimSpace(state.LastSummary)
	bg.brainRunning = false
	bg.brainFinished = state.BrainFinished
	bg.mu.Unlock()

	if brain != nil {
		brain.RestoreReportCatalog(state.ReportCatalog)
		brain.RestoreTokenUsageSnapshot(state.TokenUsage)
		brain.RestoreBrainFinished(state.BrainFinished)
	}
	if bg.notifyChange != nil {
		bg.notifyChange()
	}
}

func (bg *BinarySecurityGroup) Snapshot() map[string]interface{} {
	brain, roster, status, brainRunning, brainFinished, _, _ := bg.snapshotRuntimeState()
	rosterSnapshot := bg.buildRosterFromState(brain, roster, status)
	return map[string]interface{}{
		"groupId":       bg.ID(),
		"groupName":     bg.Name(),
		"groupKind":     bg.Kind(),
		"status":        status,
		"isRunning":     brainRunning,
		"brainFinished": brainFinished,
		"isDemo":        false,
		"description":   binaryGroupDescription(),
		"memberCount":   len(roster),
		"agentRunCount": bg.agentRuntimeCountFromState(brain, roster),
		"digitalHumans": rosterSnapshot,
		"decisionBrain": map[string]interface{}{
			"name":       bg.DecisionBrainName(),
			"status":     status,
			"group_id":   bg.ID(),
			"group_name": bg.Name(),
		},
	}
}

func (bg *BinarySecurityGroup) PrivateSnapshot() map[string]interface{} {
	brain, roster, status, brainRunning, brainFinished, lastTask, lastSummary := bg.snapshotRuntimeState()
	rosterSnapshot := bg.buildRosterFromState(brain, roster, status)
	runSnapshot := bg.buildAgentRuntimeListFromState(brain, roster, status, brainRunning, lastTask, lastSummary)
	return map[string]interface{}{
		"groupId":       bg.ID(),
		"groupName":     bg.Name(),
		"groupKind":     bg.Kind(),
		"status":        status,
		"isRunning":     brainRunning,
		"brainFinished": brainFinished,
		"isDemo":        false,
		"decisionBrain": map[string]interface{}{
			"name":       bg.DecisionBrainName(),
			"status":     status,
			"group_id":   bg.ID(),
			"group_name": bg.Name(),
		},
		"digitalHumans":    rosterSnapshot,
		"agentRuns":        runSnapshot,
		"containers":       []interface{}{},
		"envInfo":          nil,
		"tokenUsage":       bg.tokenUsageSnapshot(brain),
		"contextBreakdown": bg.contextBreakdown(brain),
	}
}

func (bg *BinarySecurityGroup) snapshotRuntimeState() (*DecisionBrain.DecisionBrain, []binaryMember, string, bool, bool, string, string) {
	bg.mu.RLock()
	defer bg.mu.RUnlock()
	return bg.brain,
		append([]binaryMember(nil), bg.roster...),
		bg.status,
		bg.brainRunning,
		bg.brainFinished,
		bg.lastTask,
		bg.lastSummary
}

func (bg *BinarySecurityGroup) buildRosterFromState(brain *DecisionBrain.DecisionBrain, roster []binaryMember, status string) map[string]interface{} {
	if brain != nil {
		if liveRoster := brain.GetDigitalHumanRoster(); len(liveRoster) > 0 {
			return liveRoster
		}
	}
	out := make(map[string]interface{})
	memberState := "idle"
	if status == "已停止" {
		memberState = "idle"
	}
	for _, member := range roster {
		item := map[string]interface{}{
			"digital_human_id": member.DigitalHumanID,
			"persona_name":     member.PersonaName,
			"gender":           member.Gender,
			"avatar_file":      member.AvatarFile,
			"personality":      member.Personality,
			"age":              member.Age,
			"state":            memberState,
			"agent_id":         member.AgentID,
			"queue_length":     0,
			"role":             member.Role,
			"last_task":        "",
			"last_summary":     "Binary research member is ready. Runtime execution depends on the current MCP configuration and dispatched task.",
		}
		list, _ := out[member.ToolName].([]map[string]interface{})
		list = append(list, item)
		out[member.ToolName] = list
	}
	return out
}

func (bg *BinarySecurityGroup) buildAgentRuntimeListFromState(brain *DecisionBrain.DecisionBrain, roster []binaryMember, status string, brainRunning bool, lastTask string, lastSummary string) []map[string]interface{} {
	brainState := "Standby"
	if status == "已停止" {
		brainState = "Stopped"
	} else if brainRunning {
		brainState = "Coordinating"
	}
	out := []map[string]interface{}{
		{
			"AgentID":     binaryBrainAgentID,
			"ToolName":    "Binary-DecisionBrain",
			"PersonaName": bg.DecisionBrainName(),
			"AvatarFile":  groupBrainAvatar,
			"RunState":    brainState,
			"RUNTask":     lastTask,
			"RUNSummary":  strings.TrimSpace(lastSummary),
		},
	}
	if brain != nil {
		out[0]["Activity"] = brain.PublicActivity()
		out[0]["activity"] = brain.PublicActivity()
	}
	if brain != nil {
		out = append(out, brain.GetAgentRuntimeList()...)
		return out
	}
	memberState := "Standby"
	if status == "已停止" {
		memberState = "Stopped"
	}
	for _, member := range roster {
		out = append(out, map[string]interface{}{
			"AgentID":     member.AgentID,
			"ToolName":    member.ToolName,
			"PersonaName": member.PersonaName,
			"AvatarFile":  member.AvatarFile,
			"RunState":    memberState,
			"RUNTask":     "",
			"RUNSummary":  "Binary research member is ready. Runtime execution depends on the current MCP configuration and dispatched task.",
		})
	}
	return out
}

func (bg *BinarySecurityGroup) agentRuntimeCountFromState(brain *DecisionBrain.DecisionBrain, roster []binaryMember) int {
	if brain != nil {
		return 1 + len(brain.GetAgentRuntimeList())
	}
	return 1 + len(roster)
}

func (bg *BinarySecurityGroup) tokenUsageSnapshot(brain *DecisionBrain.DecisionBrain) map[string]interface{} {
	if brain == nil {
		return map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
			"agents":            []interface{}{},
		}
	}
	return brain.GetTokenUsage()
}

func (bg *BinarySecurityGroup) contextBreakdown(brain *DecisionBrain.DecisionBrain) interface{} {
	if brain == nil {
		return map[string]interface{}{
			"sections":    []interface{}{},
			"total":       0,
			"max_context": 0,
		}
	}
	return brain.GetContextBreakdown()
}

func (bg *BinarySecurityGroup) emitLocked(personaName string, avatarFile string, text string) {
	if bg.emitGroupChat == nil {
		return
	}
	bg.emitGroupChat(DecisionBrain.ChatMessage{
		Role:        "system",
		Text:        text,
		Ts:          time.Now().Format("15:04:05"),
		PersonaName: personaName,
		AvatarFile:  avatarFile,
	})
}

func (bg *BinarySecurityGroup) notifyChangeLocked() {
	if bg.notifyChange != nil {
		go bg.notifyChange()
	}
}

func (bg *BinarySecurityGroup) setRuntimeStateLocked(agentID string, state string) {
	for _, run := range bg.agentRuns {
		if fmt.Sprintf("%v", run["AgentID"]) != agentID {
			continue
		}
		run["RunState"] = state
		return
	}
}

func (bg *BinarySecurityGroup) setRuntimeTaskLocked(agentID string, task string) {
	for _, run := range bg.agentRuns {
		if fmt.Sprintf("%v", run["AgentID"]) != agentID {
			continue
		}
		run["RUNTask"] = task
		run["RUNSummary"] = bg.lastSummary
		return
	}
}

func (bg *BinarySecurityGroup) normalizeInboundGroupChat(raw string) string {
	target, body, addressed := parseChatAddress(strings.TrimSpace(raw))
	if !addressed {
		return raw
	}
	if decisionBrainAliasMatches(bg, target) {
		return body
	}
	return raw
}

func (bg *BinarySecurityGroup) newDecisionBrain() *DecisionBrain.DecisionBrain {
	brain := DecisionBrain.NewDecisionBrain(bg.projectKey, bg.taskContent, bg.webOutputChan)
	brain.SetOwnerScope(bg.ID(), bg.Name())
	brain.SetHostCommandRequestHandler(bg.hostCommandHandler)
	brain.SetDecisionApprovalHandler(bg.decisionApprovalHandler)
	brain.SetSystemPrompt(bg.brainConfig.SystemPrompt)
	brain.SetExploitWorkflowEnabled(bg.brainConfig.EnableExploitWorkflow)
	brain.ReplaceAgents(bg.brainConfig.AgentDefinitions)
	brain.ReplaceDigitalHumanPool(bg.brainConfig.DigitalHumanPool)
	if strings.TrimSpace(bg.projectOverview) != "" {
		brain.SetProjectOverview(bg.projectOverview)
	}
	if bg.globalStatusPanel != nil {
		brain.SetGlobalStatusPanel(bg.globalStatusPanel)
	}
	brain.SetStatusHandler(func(status string) {
		bg.mu.Lock()
		defer bg.mu.Unlock()
		switch strings.TrimSpace(status) {
		case "决策结束":
			bg.brainFinished = true
			bg.brainRunning = false
			bg.updateLastSummaryLocked("二进制组本轮组内研判已结束，等待下一条任务。")
			if bg.status != "已停止" {
				bg.status = "待命"
			}
		case "正在运行":
			bg.brainFinished = false
			bg.updateLastSummaryLocked("二进制组决策大脑正在进行组内研判。")
			if bg.status != "已停止" {
				bg.status = "组内研判中"
			}
		}
		bg.notifyChangeLocked()
	})
	brain.SetChatObserver(func(msg DecisionBrain.ChatMessage) {
		if bg.emitGroupChat != nil {
			bg.emitGroupChat(msg)
		}
	})
	brain.SetReportObserver(bg.reportObserver)
	if bg.waitObserver != nil {
		brain.SetWaitStateHandler(bg.waitObserver)
	}
	for _, tool := range bg.decisionTools {
		brain.RegisterExternalTool(tool.Name, tool.Description, tool.Parameters, tool.Handler)
	}
	return brain
}

func (bg *BinarySecurityGroup) RegisterDecisionBrainTool(name string, description string, parameters map[string]interface{}, handler func(map[string]interface{}) string) {
	if strings.TrimSpace(name) == "" || handler == nil {
		return
	}
	bg.mu.Lock()
	replaced := false
	for i := range bg.decisionTools {
		if bg.decisionTools[i].Name != name {
			continue
		}
		bg.decisionTools[i] = binaryDecisionTool{Name: name, Description: description, Parameters: parameters, Handler: handler}
		replaced = true
		break
	}
	if !replaced {
		bg.decisionTools = append(bg.decisionTools, binaryDecisionTool{Name: name, Description: description, Parameters: parameters, Handler: handler})
	}
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil {
		brain.RegisterExternalTool(name, description, parameters, handler)
	}
}

func (bg *BinarySecurityGroup) SetHostCommandRequestHandler(handler func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult) {
	bg.mu.Lock()
	bg.hostCommandHandler = handler
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil {
		brain.SetHostCommandRequestHandler(handler)
	}
}

func (bg *BinarySecurityGroup) SetDecisionApprovalHandler(handler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult) {
	bg.mu.Lock()
	bg.decisionApprovalHandler = handler
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil {
		brain.SetDecisionApprovalHandler(handler)
	}
}

func (bg *BinarySecurityGroup) SetWebOutputChan(ch chan string) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.webOutputChan = ch
}

func (bg *BinarySecurityGroup) SetReportObserver(observer func(taskManager.ReportMeta)) {
	bg.mu.Lock()
	bg.reportObserver = observer
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil {
		brain.SetReportObserver(observer)
	}
}

func (bg *BinarySecurityGroup) SetWaitObserver(observer func(bool)) {
	bg.mu.Lock()
	bg.waitObserver = observer
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil {
		brain.SetWaitStateHandler(observer)
	}
}

func (bg *BinarySecurityGroup) SetProjectOverview(overview string) {
	overview = strings.TrimSpace(overview)
	bg.mu.Lock()
	bg.projectOverview = overview
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil && overview != "" {
		brain.SetProjectOverview(overview)
	}
}

func (bg *BinarySecurityGroup) SyncGlobalStatusPanel(panel map[string]interface{}) {
	bg.mu.Lock()
	bg.globalStatusPanel = panel
	brain := bg.brain
	bg.mu.Unlock()
	if brain != nil {
		brain.SetGlobalStatusPanel(panel)
	}
}

func (bg *BinarySecurityGroup) prepareBrainForMessageLocked() *DecisionBrain.DecisionBrain {
	if bg.brain == nil || bg.status == "已停止" {
		bg.brain = bg.newDecisionBrain()
		if bg.status == "已停止" {
			bg.status = "待命"
		}
		bg.brainFinished = false
	}
	if bg.brain.IsBrainFinished() {
		// Avoid re-entering bg.mu through the DecisionBrain status callback while
		// we already hold bg.mu here. The runner will publish the running state
		// after the message is accepted.
		bg.brain.RestoreBrainFinished(false)
		bg.brainFinished = false
	}
	return bg.brain
}

func (bg *BinarySecurityGroup) ensureBrainRunnerLocked() {
	if bg.brain == nil {
		bg.brain = bg.newDecisionBrain()
	}
	if bg.brainRunning {
		return
	}
	brain := bg.brain
	bg.brainRunning = true
	bg.brainFinished = false
	if bg.status != "已停止" {
		bg.status = "组内研判中"
	}
	bg.notifyChangeLocked()
	go func() {
		brain.Start()
		bg.mu.Lock()
		defer bg.mu.Unlock()
		if bg.brain != brain {
			return
		}
		bg.brainRunning = false
		bg.brainFinished = brain.IsBrainFinished()
		if bg.status != "已停止" {
			bg.status = "待命"
		}
		bg.notifyChangeLocked()
	}()
}

func (bg *BinarySecurityGroup) teamChatFromSender(msg string, sender string) string {
	return bg.teamChatFromSenderWithPrompt(msg, msg, sender)
}

func (bg *BinarySecurityGroup) teamChatFromSenderWithPrompt(visibleMsg string, internalMsg string, sender string) string {
	visibleMsg = strings.TrimSpace(visibleMsg)
	internalMsg = strings.TrimSpace(internalMsg)
	if internalMsg == "" {
		internalMsg = visibleMsg
	}
	if visibleMsg == "" {
		visibleMsg = internalMsg
	}
	if strings.TrimSpace(sender) == "" {
		sender = "用户"
	}

	bg.mu.Lock()
	brain := bg.prepareBrainForMessageLocked()
	bg.lastTask = visibleMsg
	if bg.lastTask == "" {
		bg.lastTask = internalMsg
	}
	if bg.lastSummary == "" {
		bg.lastSummary = "二进制组决策大脑已启动组内研判。"
	}
	bg.mu.Unlock()

	if sender != "用户" && visibleMsg != "" {
		brain.AppendChatMessage(DecisionBrain.ChatMessage{
			Role:        "system",
			Text:        visibleMsg,
			Ts:          time.Now().Format("15:04:05"),
			PersonaName: sender,
			AvatarFile:  "system.png",
		})
	}

	result := brain.TeamChat(internalMsg, sender)

	bg.mu.Lock()
	bg.ensureBrainRunnerLocked()
	bg.mu.Unlock()
	return result
}

func (bg *BinarySecurityGroup) buildAgentRuntimeListLocked() []map[string]interface{} {
	if bg.brain != nil {
		out := make([]map[string]interface{}, 0, 1+len(bg.brain.GetAgentRuntimeList()))
		brainState := "Standby"
		if bg.status == "已停止" {
			brainState = "Stopped"
		} else if bg.brainRunning {
			brainState = "Coordinating"
		}
		out = append(out, map[string]interface{}{
			"AgentID":     binaryBrainAgentID,
			"ToolName":    "Binary-DecisionBrain",
			"PersonaName": bg.DecisionBrainName(),
			"AvatarFile":  groupBrainAvatar,
			"RunState":    brainState,
			"RUNTask":     bg.lastTask,
			"RUNSummary":  strings.TrimSpace(bg.lastSummary),
		})
		out[0]["Activity"] = bg.brain.PublicActivity()
		out[0]["activity"] = bg.brain.PublicActivity()
		out = append(out, bg.brain.GetAgentRuntimeList()...)
		return out
	}

	out := make([]map[string]interface{}, 0, len(bg.roster)+1)
	brainState := "Standby"
	if bg.status == "已停止" {
		brainState = "Stopped"
	} else if bg.brainRunning {
		brainState = "Coordinating"
	}
	out = append(out, map[string]interface{}{
		"AgentID":     binaryBrainAgentID,
		"ToolName":    "Binary-DecisionBrain",
		"PersonaName": bg.DecisionBrainName(),
		"AvatarFile":  groupBrainAvatar,
		"RunState":    brainState,
		"RUNTask":     bg.lastTask,
		"RUNSummary":  strings.TrimSpace(bg.lastSummary),
	})
	memberState := "Standby"
	if bg.status == "已停止" {
		memberState = "Stopped"
	}
	for _, member := range bg.roster {
		out = append(out, map[string]interface{}{
			"AgentID":     member.AgentID,
			"ToolName":    member.ToolName,
			"PersonaName": member.PersonaName,
			"AvatarFile":  member.AvatarFile,
			"RunState":    memberState,
			"RUNTask":     "",
			"RUNSummary":  "Binary research member is ready. Runtime execution depends on the current MCP configuration and dispatched task.",
		})
	}
	return out
}

func (bg *BinarySecurityGroup) buildRosterLocked() map[string]interface{} {
	if bg.brain != nil {
		if roster := bg.brain.GetDigitalHumanRoster(); len(roster) > 0 {
			return roster
		}
	}
	out := make(map[string]interface{})
	memberState := "idle"
	if bg.status == "已停止" {
		memberState = "idle"
	}
	for _, member := range bg.roster {
		item := map[string]interface{}{
			"digital_human_id": member.DigitalHumanID,
			"persona_name":     member.PersonaName,
			"gender":           member.Gender,
			"avatar_file":      member.AvatarFile,
			"personality":      member.Personality,
			"age":              member.Age,
			"state":            memberState,
			"agent_id":         member.AgentID,
			"queue_length":     0,
			"role":             member.Role,
			"last_task":        "",
			"last_summary":     "Binary research member is ready. Runtime execution depends on the current MCP configuration and dispatched task.",
		}
		list, _ := out[member.ToolName].([]map[string]interface{})
		list = append(list, item)
		out[member.ToolName] = list
	}
	return out
}

func (bg *BinarySecurityGroup) tokenUsageSnapshotLocked() map[string]interface{} {
	if bg.brain == nil {
		return map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
			"agents":            []interface{}{},
		}
	}
	return bg.brain.GetTokenUsage()
}

func (bg *BinarySecurityGroup) contextBreakdownLocked() interface{} {
	if bg.brain == nil {
		return map[string]interface{}{
			"sections":    []interface{}{},
			"total":       0,
			"max_context": 0,
		}
	}
	return bg.brain.GetContextBreakdown()
}

func (bg *BinarySecurityGroup) sharedReportsLocked() []taskManager.ReportMeta {
	if bg.brain == nil {
		return nil
	}
	return bg.brain.GetReportList()
}

func (bg *BinarySecurityGroup) updateLastSummaryLocked(summary string) {
	summary = strings.TrimSpace(summary)
	if summary != "" {
		bg.lastSummary = summary
	}
}

func (bg *BinarySecurityGroup) ListMemoryDebugTargets() []taskManager.MemoryDebugTarget {
	bg.mu.RLock()
	brain := bg.brain
	roster := append([]binaryMember(nil), bg.roster...)
	bg.mu.RUnlock()
	targets := make([]taskManager.MemoryDebugTarget, 0)
	if brain != nil {
		targets = brain.ListMemoryDebugTargets()
	}
	targets = mergeBinaryMemoryDebugTargets(targets, roster)
	for i := range targets {
		targets[i].GroupID = bg.ID()
		targets[i].GroupName = bg.Name()
		targets[i].TargetID = bg.ID() + "::" + targets[i].TargetID
	}
	return targets
}

func (bg *BinarySecurityGroup) ExportMemoryDebug(globalTargetID string) (*taskManager.MemoryDebugExport, error) {
	bg.mu.Lock()
	brain := bg.ensureBrainForDebugLocked()
	bg.mu.Unlock()
	if brain == nil {
		return nil, fmt.Errorf("binary decision brain is unavailable")
	}
	localTargetID := strings.TrimPrefix(strings.TrimSpace(globalTargetID), bg.ID()+"::")
	exported, err := brain.ExportMemoryDebug(localTargetID)
	if err != nil {
		return nil, err
	}
	exported.Target.GroupID = bg.ID()
	exported.Target.GroupName = bg.Name()
	exported.Target.TargetID = bg.ID() + "::" + exported.Target.TargetID
	return exported, nil
}

func (bg *BinarySecurityGroup) ensureBrainForDebugLocked() *DecisionBrain.DecisionBrain {
	if bg.brain != nil {
		return bg.brain
	}
	bg.brain = bg.newDecisionBrain()
	if bg.brainFinished {
		bg.brain.RestoreBrainFinished(true)
	}
	return bg.brain
}

func mergeBinaryMemoryDebugTargets(targets []taskManager.MemoryDebugTarget, roster []binaryMember) []taskManager.MemoryDebugTarget {
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		key := strings.TrimSpace(target.DigitalHumanID)
		if key == "" {
			key = strings.TrimSpace(target.AgentToolName) + "::" + strings.TrimSpace(target.PersonaName)
		}
		if key == "" {
			key = strings.TrimSpace(target.TargetID)
		}
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	for _, member := range roster {
		key := strings.TrimSpace(member.DigitalHumanID)
		if key == "" {
			key = strings.TrimSpace(member.ToolName) + "::" + strings.TrimSpace(member.PersonaName)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		targetID := "digital_human:" + strings.TrimSpace(member.DigitalHumanID)
		if strings.TrimSpace(member.DigitalHumanID) == "" {
			targetID = "digital_human:" + strings.TrimSpace(member.ToolName) + "::" + strings.TrimSpace(member.PersonaName)
		}
		targets = append(targets, taskManager.MemoryDebugTarget{
			TargetID:       targetID,
			TargetType:     "digital_human",
			Label:          strings.TrimSpace(member.PersonaName),
			PersonaName:    strings.TrimSpace(member.PersonaName),
			AgentToolName:  strings.TrimSpace(member.ToolName),
			DigitalHumanID: strings.TrimSpace(member.DigitalHumanID),
			AgentID:        strings.TrimSpace(member.AgentID),
			State:          "idle",
			Available:      false,
			Reason:         "数字人实例不可用",
		})
		seen[key] = struct{}{}
	}
	return targets
}

func (bg *BinarySecurityGroup) coordinationAck(sender string) string {
	if strings.TrimSpace(sender) == totalBrainName {
		return fmt.Sprintf("%s已收到来自%s的协调请求，正在进行二进制组内研判并准备回报。", bg.DecisionBrainName(), sender)
	}
	return fmt.Sprintf("%s已收到来自%s的消息，当前将由二进制组决策大脑接手处理。", bg.DecisionBrainName(), sender)
}

package LabManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/ProjectManager"
	"AIxVuln/agents"
	webagents "AIxVuln/agents/web"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"fmt"
	"strings"
)

func newWebSecurityGroup(pm *ProjectManager.ProjectManager) *WebSecurityGroup {
	mode := misc.GetEffectiveWebResearchMode()
	g := &WebSecurityGroup{pm: pm, mode: mode}
	g.brainConfig = buildWebGroupDecisionBrainConfig(mode)
	g.applyDecisionBrainConfig()
	return g
}

func (g *WebSecurityGroup) applyDecisionBrainConfig() {
	if g == nil || g.pm == nil {
		return
	}
	g.pm.ConfigureDecisionBrain(ProjectManager.DecisionBrainConfig{
		SystemPrompt:          g.brainConfig.SystemPrompt,
		EnableExploitWorkflow: g.brainConfig.EnableExploitWorkflow,
		AgentDefinitions:      g.brainConfig.AgentDefinitions,
		DigitalHumanPool:      g.brainConfig.DigitalHumanPool,
	})
	g.pm.SetDecisionBrainOwnerScope(g.ID(), g.Name())
}

func buildWebGroupDecisionBrainConfig(mode string) groupDecisionBrainConfig {
	defs, _ := webagents.GetAgentDescriptionForMode(mode)
	return groupDecisionBrainConfig{
		SystemPrompt:          webDecisionBrainSystemPrompt(mode),
		EnableExploitWorkflow: true,
		AgentDefinitions:      defs,
		DigitalHumanPool:      loadWebGroupDigitalHumanPool(defs),
	}
}

func loadWebGroupDigitalHumanPool(defs []agents.AgentDefinition) map[string][]agents.AgentProfile {
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
		profiles := make([]agents.AgentProfile, 0, len(rows))
		for _, r := range rows {
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

func webDecisionBrainSystemPrompt(mode string) string {
	normalizedMode, err := misc.NormalizeWebResearchMode(mode)
	if err != nil {
		normalizedMode = misc.WebResearchModeAudit
	}
	return prompts.WebDecisionBrainSystemPrompt(
		normalizedMode,
		groupDecisionBrainKnowledgePrompt(),
		groupDecisionBrainRelayRulePrompt(),
	)
}

func (g *WebSecurityGroup) ID() string {
	return webGroupID
}

func (g *WebSecurityGroup) Name() string {
	return webGroupName
}

func (g *WebSecurityGroup) Kind() string {
	return webGroupKind
}

func (g *WebSecurityGroup) DecisionBrainName() string {
	return decisionBrainDisplayName(g.Name())
}

func normalizeWebGroupStatus(status string) string {
	status = strings.TrimSpace(status)
	switch status {
	case "", "未运行", "项目速览中", "运行结束", "决策结束":
		return "待命"
	default:
		return status
	}
}

func (g *WebSecurityGroup) Status() string {
	return normalizeWebGroupStatus(g.pm.GetStatus())
}

func (g *WebSecurityGroup) IsRunning() bool {
	return g.Status() == "正在运行"
}

func (g *WebSecurityGroup) BrainFinished() bool {
	return g.pm.GetBrainFinished()
}

func (g *WebSecurityGroup) IsDemo() bool {
	return false
}

func (g *WebSecurityGroup) HasMember(persona string) bool {
	roster := g.pm.GetDigitalHumanRoster()
	return rosterHasPersona(roster, persona)
}

func (g *WebSecurityGroup) HandleDecisionChat(raw string, sender string) string {
	task := strings.TrimSpace(raw)
	if strings.TrimSpace(sender) == totalBrainName {
		g.pm.TeamChatFromSenderWithPrompt(task, buildDecisionCoordinationPrompt(g, sender, task), sender)
		return fmt.Sprintf("%s已收到来自%s的协调请求，正在组内核实并准备回报。", g.DecisionBrainName(), sender)
	}
	g.pm.TeamChatFromSender(task, sender)
	return fmt.Sprintf("%s已收到来自%s的消息，正在转入组内调度。", g.DecisionBrainName(), sender)
}

func (g *WebSecurityGroup) HandleGroupChat(raw string, sender string) string {
	return g.pm.TeamChatFromSender(g.normalizeInboundGroupChat(raw), sender)
}

func (g *WebSecurityGroup) NotifyDecisionBrainEvent(content string) {
	if g == nil || g.pm == nil {
		return
	}
	g.pm.NotifyDecisionBrainEvent(content)
}

func (g *WebSecurityGroup) SyncGlobalStatusPanel(panel map[string]interface{}) {
	if g == nil || g.pm == nil {
		return
	}
	g.pm.SetGlobalStatusPanel(panel)
}

func (g *WebSecurityGroup) SetWaitObserver(observer func(bool)) {
	if g == nil || g.pm == nil {
		return
	}
	g.pm.SetDecisionBrainWaitHandler(observer)
}

func (g *WebSecurityGroup) SetWebOutputChan(ch chan string) {
	if g == nil || g.pm == nil {
		return
	}
	g.pm.SetMsgChan(ch)
}

func (g *WebSecurityGroup) NormalizeGroupChatMessage(msg DecisionBrain.ChatMessage) DecisionBrain.ChatMessage {
	msg.ChannelID = groupChannelID(g.ID())
	msg.ChannelName = groupChannelName(g.Name())
	msg.GroupID = g.ID()
	msg.GroupName = g.Name()
	if msg.Role == "user" {
		msg.SenderType = "user"
		return msg
	}
	if strings.TrimSpace(msg.PersonaName) == totalBrainName {
		msg.SenderType = "total_brain"
		return msg
	}
	if strings.TrimSpace(msg.PersonaName) == "" {
		msg.SenderType = "system"
		return msg
	}
	if strings.TrimSpace(msg.PersonaName) == "决策大脑" {
		msg.PersonaName = g.DecisionBrainName()
		msg.SenderType = "group_brain"
		return msg
	}
	msg.SenderType = "member"
	return msg
}

func (g *WebSecurityGroup) StartTask() {
	g.pm.StartTask()
}

func (g *WebSecurityGroup) StopTask() {
	g.pm.StopTask()
}

func (g *WebSecurityGroup) GetDigitalHumanRoster() map[string]interface{} {
	return g.pm.GetDigitalHumanRoster()
}

func (g *WebSecurityGroup) GetAgentRuntimeList() []map[string]interface{} {
	return g.pm.GetAgentRuntimeList()
}

func (g *WebSecurityGroup) SharedReportCatalog() []taskManager.ReportMeta {
	return g.pm.GetReportCatalog()
}

func (g *WebSecurityGroup) NotifySharedReportUpdate(report taskManager.ReportMeta, sourceGroupID string, sourceGroupName string) {
	if g == nil || g.pm == nil || !g.IsRunning() {
		return
	}
	if strings.TrimSpace(sourceGroupID) == g.ID() {
		return
	}
	g.pm.NotifyDecisionBrainEvent(buildSharedReportWakePrompt(sourceGroupName, report))
}

func (g *WebSecurityGroup) Snapshot() map[string]interface{} {
	agentRunCount := 0
	roster := map[string]interface{}{}
	if g != nil && g.pm != nil {
		agentRunCount = len(g.pm.GetAgentRuntimeList())
		if liveRoster := g.pm.GetDigitalHumanRoster(); len(liveRoster) > 0 {
			roster = liveRoster
		}
	}
	return map[string]interface{}{
		"groupId":       g.ID(),
		"groupName":     g.Name(),
		"groupKind":     g.Kind(),
		"status":        g.Status(),
		"isRunning":     g.IsRunning(),
		"brainFinished": g.BrainFinished(),
		"isDemo":        false,
		"description":   webGroupDescription(),
		"mode":          g.mode,
		"memberCount":   configuredDigitalHumanCount(g.brainConfig.DigitalHumanPool),
		"agentRunCount": agentRunCount,
		"digitalHumans": roster,
		"decisionBrain": map[string]interface{}{"name": g.DecisionBrainName(), "status": g.Status(), "group_id": g.ID(), "group_name": g.Name()},
	}
}

func (g *WebSecurityGroup) PrivateSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"groupId":          g.ID(),
		"groupName":        g.Name(),
		"groupKind":        g.Kind(),
		"status":           g.Status(),
		"isRunning":        g.IsRunning(),
		"brainFinished":    g.BrainFinished(),
		"mode":             g.mode,
		"isDemo":           false,
		"decisionBrain":    map[string]interface{}{"name": g.DecisionBrainName(), "status": g.Status(), "group_id": g.ID(), "group_name": g.Name()},
		"digitalHumans":    g.GetDigitalHumanRoster(),
		"agentRuns":        g.GetAgentRuntimeList(),
		"exploitIdeas":     g.pm.GetExploitIdeaList(),
		"exploitChains":    g.pm.GetExploitChainList(),
		"containers":       g.pm.GetContainerList(),
		"envInfo":          g.pm.GetEnvInfo(),
		"tokenUsage":       g.pm.GetTokenUsage(),
		"contextBreakdown": g.pm.GetContextBreakdown(),
	}
}

func (g *WebSecurityGroup) ListMemoryDebugTargets() []taskManager.MemoryDebugTarget {
	targets := g.pm.ListMemoryDebugTargets()
	for i := range targets {
		targets[i].GroupID = g.ID()
		targets[i].GroupName = g.Name()
		targets[i].TargetID = g.ID() + "::" + targets[i].TargetID
	}
	return targets
}

func (g *WebSecurityGroup) ExportMemoryDebug(globalTargetID string) (*taskManager.MemoryDebugExport, error) {
	localTargetID := strings.TrimPrefix(strings.TrimSpace(globalTargetID), g.ID()+"::")
	exported, err := g.pm.ExportMemoryDebug(localTargetID)
	if err != nil {
		return nil, err
	}
	exported.Target.GroupID = g.ID()
	exported.Target.GroupName = g.Name()
	exported.Target.TargetID = g.ID() + "::" + exported.Target.TargetID
	return exported, nil
}

func (g *WebSecurityGroup) normalizeInboundGroupChat(raw string) string {
	target, body, addressed := parseChatAddress(strings.TrimSpace(raw))
	if !addressed {
		return raw
	}
	if decisionBrainAliasMatches(g, target) {
		return body
	}
	return raw
}

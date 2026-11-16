package LabManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const totalBrainMemoryDebugLocalTargetID = "decision_brain"

func totalBrainMemoryDebugTargetID() string {
	return labControlGroupID + "::" + totalBrainMemoryDebugLocalTargetID
}

func totalBrainDecisionMemorySystemPrompt() string {
	return prompts.TotalBrainDecisionMemorySystemPrompt()
}

func (lm *LabManager) bindTotalBrainMemory(labStateDir string) error {
	memoryDir := filepath.Join(labStateDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return err
	}
	memoryFile := filepath.Join(memoryDir, "brain.json")
	mem := DecisionBrain.NewBrainMemory()
	if _, err := os.Stat(memoryFile); err == nil {
		if loadErr := mem.LoadMemoryFromFile(memoryFile); loadErr != nil {
			misc.Debug("load total brain memory failed for %s: %v", lm.labName, loadErr)
		}
	}
	lm.configureTotalBrainMemory(mem)
	_ = mem.EnsurePinnedMessages(DecisionBrain.PentestAuthorizedProjectSeedMessages())

	lm.brainMu.Lock()
	lm.totalBrainMemory = mem
	lm.totalBrainMemoryFilePath = memoryFile
	lm.brainMu.Unlock()

	lm.syncTotalBrainMemoryRuntime(mem)
	return lm.persistTotalBrainMemory(mem)
}

func (lm *LabManager) ensureTotalBrainMemory() *DecisionBrain.BrainMemory {
	lm.brainMu.RLock()
	mem := lm.totalBrainMemory
	path := strings.TrimSpace(lm.totalBrainMemoryFilePath)
	lm.brainMu.RUnlock()
	if mem != nil {
		lm.configureTotalBrainMemory(mem)
		return mem
	}
	mem = DecisionBrain.NewBrainMemory()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if loadErr := mem.LoadMemoryFromFile(path); loadErr != nil {
				misc.Debug("load total brain memory failed for %s: %v", lm.labName, loadErr)
			}
		}
	}
	lm.configureTotalBrainMemory(mem)
	_ = mem.EnsurePinnedMessages(DecisionBrain.PentestAuthorizedProjectSeedMessages())

	lm.brainMu.Lock()
	if lm.totalBrainMemory == nil {
		lm.totalBrainMemory = mem
	} else {
		mem = lm.totalBrainMemory
	}
	lm.brainMu.Unlock()

	lm.syncTotalBrainMemoryRuntime(mem)
	_ = lm.persistTotalBrainMemory(mem)
	return mem
}

func (lm *LabManager) configureTotalBrainMemory(mem *DecisionBrain.BrainMemory) {
	if mem == nil {
		return
	}
	mem.SetSystemPrompt(totalBrainDecisionMemorySystemPrompt())
	mem.SetTaskContent(strings.TrimSpace(lm.taskContent))
	mem.SetExploitWorkflowEnabled(false)
}

func (lm *LabManager) persistTotalBrainMemory(mem *DecisionBrain.BrainMemory) error {
	if mem == nil {
		return nil
	}
	lm.brainMu.RLock()
	path := strings.TrimSpace(lm.totalBrainMemoryFilePath)
	lm.brainMu.RUnlock()
	if path == "" {
		return nil
	}
	return mem.SaveMemoryToFile(path)
}

func (lm *LabManager) syncTotalBrainMemoryRuntime(mem *DecisionBrain.BrainMemory) {
	if mem == nil {
		return
	}
	lm.configureTotalBrainMemory(mem)
	mem.SetNoAgentRunning(!lm.hasRunningResearchGroups())
	mem.UpdateAgentRuntimeInfo(lm.totalBrainRuntimeAgentInfoJSON())

	if env := lm.GetEnvInfo(); len(env) > 0 {
		if js, err := json.Marshal(env); err == nil {
			mem.SetEnvInfo(string(js))
		}
	} else {
		mem.SetEnvInfo("")
	}

	if containers := lm.GetContainerList(); len(containers) > 0 {
		if js, err := json.Marshal(containers); err == nil {
			mem.SetContainerListInfo(string(js))
		}
	} else {
		mem.SetContainerListInfo("")
	}

	mem.AddKeyMessage("ProjectOverview", map[string]interface{}{
		"overview": lm.totalBrainOverviewSnapshot(),
	}, false)
	mem.AddKeyMessage("DispatchState", lm.totalBrainDispatchStateSnapshot(), false)
	mem.AddKeyMessage("GroupCatalog", lm.groupPlanningCatalog(), false)
	mem.AddKeyMessage("CollaborationState", lm.collaborationStateSnapshot(), false)
	mem.AddKeyMessage("SharedReports", lm.totalBrainSharedReportSnapshot(), false)
}

func (lm *LabManager) hasRunningResearchGroups() bool {
	for _, groupID := range lm.groupOrder {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		if group.IsRunning() {
			return true
		}
	}
	return false
}

func (lm *LabManager) totalBrainOverviewSnapshot() string {
	if overview := strings.TrimSpace(lm.getProjectOverview()); overview != "" {
		return overview
	}
	if lm.webGroup != nil && lm.webGroup.pm != nil && lm.webGroup.pm.ShouldSkipProjectOverview() {
		return buildNoSourceProjectOverview(lm.taskContent)
	}
	return ""
}

func (lm *LabManager) totalBrainDispatchStateSnapshot() map[string]interface{} {
	lm.brainMu.RLock()
	defer lm.brainMu.RUnlock()
	return map[string]interface{}{
		"status":               strings.TrimSpace(lm.totalBrainStatus),
		"last_dispatch":        strings.TrimSpace(lm.lastDispatch),
		"last_dispatch_groups": append([]string(nil), lm.lastDispatchGroups...),
		"last_dispatch_reason": strings.TrimSpace(lm.lastDispatchReason),
	}
}

func (lm *LabManager) totalBrainRuntimeAgentInfoJSON() string {
	items := make([]map[string]interface{}, 0, len(lm.groupOrder))
	for _, groupID := range lm.groupOrder {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		runState := ""
		switch {
		case group.IsRunning():
			runState = "running"
		case group.BrainFinished():
			runState = "done"
		default:
			continue
		}
		items = append(items, map[string]interface{}{
			"AgentID":           group.ID(),
			"AgentLabel":        group.Name(),
			"RunState":          runState,
			"GroupKind":         group.Kind(),
			"Status":            group.Status(),
			"DecisionBrainName": group.DecisionBrainName(),
		})
	}
	if len(items) == 0 {
		return ""
	}
	js, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(js)
}

func (lm *LabManager) totalBrainSharedReportSnapshot() []map[string]interface{} {
	reports := lm.listSharedReports()
	if len(reports) == 0 {
		return nil
	}
	if len(reports) > 20 {
		reports = reports[len(reports)-20:]
	}
	out := make([]map[string]interface{}, 0, len(reports))
	for _, report := range reports {
		out = append(out, map[string]interface{}{
			"group_id":   report.GroupID,
			"group_name": report.GroupName,
			"report_id":  report.Meta.ReportID,
			"target_id":  report.Meta.TargetID,
			"file_name":  report.Meta.FileName,
			"summary":    report.Meta.Summary,
		})
	}
	return out
}

func (lm *LabManager) appendTotalBrainMemoryMessages(messages ...llm.Message) {
	if len(messages) == 0 {
		return
	}
	mem := lm.ensureTotalBrainMemory()
	if mem == nil {
		return
	}
	lm.syncTotalBrainMemoryRuntime(mem)
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		mem.AddMessage(msg)
	}
	lm.compressTotalBrainMemoryIfNeeded(mem)
	if err := lm.persistTotalBrainMemory(mem); err != nil {
		misc.Debug("persist total brain memory failed for %s: %v", lm.labName, err)
	}
}

func (lm *LabManager) compressTotalBrainMemoryIfNeeded(mem *DecisionBrain.BrainMemory) {
	if mem == nil {
		return
	}
	model := misc.GetConfigValueDefault("decision", "MODEL", "")
	if model == "" {
		model = misc.GetConfigValueDefault("main_setting", "MODEL", "")
	}
	if err := mem.CompressIfNeeded(llm.GetResponsesClient("decision", "main_setting"), model); err != nil {
		misc.Debug("compress total brain memory failed for %s: %v", lm.labName, err)
	}
}

func (lm *LabManager) totalBrainRequestMemoryMessage(raw string, sender string, requesterGroupID string) llm.Message {
	raw = strings.TrimSpace(raw)
	sender = strings.TrimSpace(sender)
	if sender == "" {
		sender = "未知发送者"
	}
	if requesterGroupID != "" {
		return llm.Message{
			Role: llm.RoleUser,
			Content: fmt.Sprintf(
				"[跨组协作申请]\n发送者：%s\n申请组：%s\n内容：%s",
				sender,
				strings.TrimSpace(lm.groupNameByID(requesterGroupID)),
				raw,
			),
		}
	}
	return llm.Message{
		Role: llm.RoleUser,
		Content: fmt.Sprintf(
			"[总脑调度请求]\n发送者：%s\n内容：%s",
			sender,
			raw,
		),
	}
}

func (lm *LabManager) totalBrainReportRequestMemoryMessage(group researchGroup, reportType string, summary string, details string, nextStep string) llm.Message {
	groupName := ""
	groupBrainName := ""
	groupID := ""
	if group != nil {
		groupName = strings.TrimSpace(group.Name())
		groupBrainName = strings.TrimSpace(group.DecisionBrainName())
		groupID = strings.TrimSpace(group.ID())
	}
	return llm.Message{
		Role: llm.RoleUser,
		Content: fmt.Sprintf(
			"[研究组汇报处理请求]\nrequest_kind: group_report\nreporting_group_id: %s\nreporting_group_name: %s\nreporting_decision_brain: %s\nreport_type: %s\nsummary: %s\ndetails: %s\nnext_step: %s\nRequirement: reply to the reporting group's decision brain with concrete follow-up coordination so the group does not stop prematurely if work is still unresolved.\nImportant boundary: in this request you are NOT dispatching any extra group. Do NOT say another group has already been awakened, coordinated, or started. If cross-group help is needed, tell the reporting group to prepare the required artifacts under /workdir/sourceCodeDir and then call Tool-RequestLabAssistance itself.",
			groupID,
			groupName,
			groupBrainName,
			strings.TrimSpace(reportType),
			strings.TrimSpace(summary),
			strings.TrimSpace(details),
			strings.TrimSpace(nextStep),
		),
	}
}

func (lm *LabManager) totalBrainDirectiveMemoryMessage(group researchGroup, text string) llm.Message {
	groupName := ""
	groupBrainName := ""
	if group != nil {
		groupName = strings.TrimSpace(group.Name())
		groupBrainName = strings.TrimSpace(group.DecisionBrainName())
	}
	return llm.Message{
		Role: llm.RoleAssistant,
		Content: fmt.Sprintf(
			"[总脑下发协调指令]\n目标研究组：%s\n目标决策大脑：%s\n内容：%s",
			groupName,
			groupBrainName,
			strings.TrimSpace(text),
		),
	}
}

func (lm *LabManager) totalBrainGroupMessageMemoryMessage(group researchGroup, text string) llm.Message {
	groupName := ""
	groupBrainName := ""
	if group != nil {
		groupName = strings.TrimSpace(group.Name())
		groupBrainName = strings.TrimSpace(group.DecisionBrainName())
	}
	return llm.Message{
		Role: llm.RoleUser,
		Content: fmt.Sprintf(
			"[研究组决策大脑消息]\n来源研究组：%s\n来源决策大脑：%s\n内容：%s",
			groupName,
			groupBrainName,
			strings.TrimSpace(text),
		),
	}
}

func (lm *LabManager) totalBrainDispatchToolResult(plan labDispatchPlan) map[string]interface{} {
	groupTasks := make([]map[string]string, 0, len(plan.GroupIDs))
	for _, groupID := range normalizeGroupIDs(plan.GroupIDs) {
		groupTasks = append(groupTasks, map[string]string{
			"group_id":   groupID,
			"group_name": strings.TrimSpace(lm.groupNameByID(groupID)),
			"task":       strings.TrimSpace(plan.GroupTasks[groupID]),
		})
	}
	return map[string]interface{}{
		"group_ids":        normalizeGroupIDs(plan.GroupIDs),
		"group_tasks":      groupTasks,
		"dispatch_reason":  strings.TrimSpace(plan.Reason),
		"selected_groups":  plannedGroupNames(lm.groups, plan.GroupIDs),
		"request_resolved": len(plan.GroupIDs) > 0,
	}
}

func (lm *LabManager) totalBrainReportResponseToolResult(result totalBrainReportResponse) map[string]interface{} {
	return map[string]interface{}{
		"visible_message":          strings.TrimSpace(result.VisibleMessage),
		"reply_to_reporting_group": strings.TrimSpace(result.ReplyToReportingGroup),
		"coordination_reason":      strings.TrimSpace(result.CoordinationReason),
	}
}

func (lm *LabManager) totalBrainDebugState() string {
	status := strings.TrimSpace(lm.currentTotalBrainStatus())
	switch status {
	case "调度中", "定向沟通中", "项目速览中", "正在运行":
		return "running"
	case "错误":
		return "error"
	case "已停止":
		return "stopped"
	}
	if strings.TrimSpace(lm.GetStartTime()) != "" {
		return "idle"
	}
	return "idle"
}

func (lm *LabManager) totalBrainMemoryDebugTarget() taskManager.MemoryDebugTarget {
	mem := lm.ensureTotalBrainMemory()
	return taskManager.MemoryDebugTarget{
		TargetID:   totalBrainMemoryDebugTargetID(),
		GroupID:    labControlGroupID,
		GroupName:  labControlGroupName,
		TargetType: "decision_brain",
		Label:      totalBrainName,
		State:      lm.totalBrainDebugState(),
		Available:  mem != nil,
		Reason:     totalBrainUnavailableReason(mem != nil, "总脑记忆不可用"),
	}
}

func (lm *LabManager) exportTotalBrainMemoryDebug(targetID string) (*taskManager.MemoryDebugExport, error) {
	if strings.TrimSpace(targetID) != totalBrainMemoryDebugTargetID() {
		return nil, fmt.Errorf("unsupported total brain debug target: %s", targetID)
	}
	mem := lm.ensureTotalBrainMemory()
	if mem == nil {
		return nil, fmt.Errorf("total brain memory is unavailable")
	}
	lm.syncTotalBrainMemoryRuntime(mem)
	return &taskManager.MemoryDebugExport{
		Target:     lm.totalBrainMemoryDebugTarget(),
		ExportedAt: time.Now().Format("2006-01-02 15:04:05"),
		Content:    mem.ExportLastLLMContextText(),
	}, nil
}

func totalBrainUnavailableReason(available bool, reason string) string {
	if available {
		return ""
	}
	return strings.TrimSpace(reason)
}

package LabManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/ProjectManager"
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"AIxVuln/toolCalling"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	maxLabChatRecord        = 4000
	maxPersistLabEvents     = 1200
	labChatDebounce         = 2 * time.Second
	totalBrainPlanTimeout   = 120 * time.Second
	totalBrainName          = "总脑"
	totalBrainAvatar        = "system.png"
	groupBrainAvatar        = "system.png"
	webGroupID              = "web_security"
	binaryGroupID           = "binary_security"
	pentestGroupID          = "pentest_security"
	labControlGroupID       = "lab_control"
	webGroupName            = "代码审计组"
	binaryGroupName         = "二进制安全研究组"
	pentestGroupName        = "渗透测试组"
	labControlGroupName     = "实验室总脑"
	webGroupKind            = "web_security"
	binaryGroupKind         = "binary_security"
	pentestGroupKind        = "pentest_security"
	binaryBrainAgentID      = "binary-brain"
	pentestBrainAgentID     = "pentest-brain"
	defaultBrainState       = "待命"
	decisionMakersChannelID = "decision_makers"
	decisionMakersChanName  = "决策者群"
)

type LabConfig struct {
	LabID               string
	LabName             string
	SourceCodeDir       string
	SkipProjectOverview bool
	MsgChan             chan string
	TaskContent         string
}

type LabManager struct {
	mu          sync.RWMutex
	labID       string
	labName     string
	taskContent string
	msgChan     chan string
	status      string
	startTime   string
	endTime     string
	brainDone   bool

	webGroup     *WebSecurityGroup
	binaryGroup  *BinarySecurityGroup
	pentestGroup *PentestSecurityGroup
	groups       map[string]researchGroup
	groupOrder   []string

	chatFilePath      string
	chatMessages      []DecisionBrain.ChatMessage
	chatMu            sync.Mutex
	lastChatPersistAt time.Time
	chatPersistTimer  *time.Timer

	brainMu                  sync.RWMutex
	totalBrainStatus         string
	totalBrainMemory         *DecisionBrain.BrainMemory
	totalBrainMemoryFilePath string
	lastDispatch             string
	lastDispatchGroups       []string
	lastDispatchReason       string
	projectOverview          string
	hostCmdApprovals         *hostCommandApprovalManager
	labReportMu              sync.RWMutex
	labReportCatalog         []taskManager.ReportMeta
	coordinationMu           sync.RWMutex
	groupWaitStates          map[string]bool
	collaborationSessions    map[string]labCollaborationSession
}

type labDispatchPlan struct {
	GroupIDs   []string
	GroupTasks map[string]string
	Reason     string
}

type sharedReportEntry struct {
	DownloadID string
	GroupID    string
	GroupName  string
	Meta       taskManager.ReportMeta
}

type labCollaborationSession struct {
	RequesterGroupID     string
	CollaboratorGroupIDs []string
	ParticipantGroupIDs  []string
	Request              string
	Reason               string
	UpdatedAt            string
}

type totalBrainDispatchToolArgs struct {
	GroupIDs       []string `json:"group_ids"`
	DispatchReason string   `json:"dispatch_reason"`
	GroupTasks     []struct {
		GroupID string `json:"group_id"`
		Task    string `json:"task"`
	} `json:"group_tasks"`
}

type totalBrainReportResponseToolArgs struct {
	VisibleMessage        string `json:"visible_message"`
	ReplyToReportingGroup string `json:"reply_to_reporting_group"`
	CoordinationReason    string `json:"coordination_reason"`
}

type totalBrainReportResponse struct {
	VisibleMessage        string
	ReplyToReportingGroup string
	CoordinationReason    string
}

func NewLabManager(cfg LabConfig) (*LabManager, error) {
	cfg.LabName = strings.TrimSpace(cfg.LabName)
	if cfg.LabName == "" {
		return nil, fmt.Errorf("项目名称不能为空")
	}
	labID := strings.TrimSpace(cfg.LabID)
	if labID == "" {
		labID = uuid.New().String()
	}
	pm, err := ProjectManager.NewProjectManager(ProjectManager.ProjectConfig{
		ProjectID:           labID,
		ProjectName:         cfg.LabName,
		SourceCodeDir:       cfg.SourceCodeDir,
		SkipProjectOverview: cfg.SkipProjectOverview,
		MsgChan:             cfg.MsgChan,
		TaskContent:         cfg.TaskContent,
	})
	if err != nil {
		return nil, err
	}

	lm := &LabManager{
		labID:                 labID,
		labName:               cfg.LabName,
		taskContent:           cfg.TaskContent,
		msgChan:               cfg.MsgChan,
		status:                "未运行",
		webGroup:              newWebSecurityGroup(pm),
		groups:                make(map[string]researchGroup),
		groupOrder:            []string{webGroupID, binaryGroupID, pentestGroupID},
		chatMessages:          make([]DecisionBrain.ChatMessage, 0),
		totalBrainStatus:      defaultBrainState,
		labReportCatalog:      make([]taskManager.ReportMeta, 0),
		groupWaitStates:       make(map[string]bool),
		collaborationSessions: make(map[string]labCollaborationSession),
	}
	lm.binaryGroup = newBinarySecurityGroup(labID, cfg.TaskContent, func(msg DecisionBrain.ChatMessage) {
		lm.emitGroupChatMessage(binaryGroupID, msg)
	}, lm.notifyGroupsUpdated)
	lm.pentestGroup = newPentestSecurityGroup(labID, cfg.TaskContent, func(msg DecisionBrain.ChatMessage) {
		lm.emitGroupChatMessage(pentestGroupID, msg)
	}, lm.notifyGroupsUpdated)
	lm.binaryGroup.SetWebOutputChan(cfg.MsgChan)
	lm.pentestGroup.SetWebOutputChan(cfg.MsgChan)
	lm.groups[webGroupID] = lm.webGroup
	lm.groups[binaryGroupID] = lm.binaryGroup
	lm.groups[pentestGroupID] = lm.pentestGroup
	lm.webGroup.pm.SetExploitAutomationPolicyResolver(lm.webExploitAutomationPolicy)
	lm.hostCmdApprovals = newHostCommandApprovalManager(labID, lm.GetWorkDir, lm.GetSourceCodeDir, lm.GetProjectDir, func() {
		lm.sendWS("HostCommandApprovalsUpdate", lm.GetHostCommandApprovalSnapshot())
	})
	taskManager.SetBackgroundProcessObserver(labID, func(items []taskManager.BackgroundProcessSnapshot) {
		lm.sendWS("BackgroundProcessesUpdate", items)
		lm.syncGlobalStatusPanel(taskManager.BuildGlobalStatusPanelFromSnapshots(items))
	})
	lm.webGroup.pm.SetHostCommandRequestHandler(lm.handleHostCommandRequest)
	lm.webGroup.pm.SetDecisionApprovalHandler(lm.handleDecisionApprovalRequest)
	lm.binaryGroup.SetHostCommandRequestHandler(lm.handleHostCommandRequest)
	lm.binaryGroup.SetDecisionApprovalHandler(lm.handleDecisionApprovalRequest)
	lm.pentestGroup.SetHostCommandRequestHandler(lm.handleHostCommandRequest)
	lm.pentestGroup.SetDecisionApprovalHandler(lm.handleDecisionApprovalRequest)
	lm.webGroup.pm.SetChatObserver(func(msg DecisionBrain.ChatMessage) {
		lm.emitGroupChatMessage(webGroupID, msg)
	})
	lm.webGroup.SetWaitObserver(func(waiting bool) {
		lm.handleGroupWaitStateChange(webGroupID, waiting)
	})
	lm.binaryGroup.SetWaitObserver(func(waiting bool) {
		lm.handleGroupWaitStateChange(binaryGroupID, waiting)
	})
	lm.pentestGroup.SetWaitObserver(func(waiting bool) {
		lm.handleGroupWaitStateChange(pentestGroupID, waiting)
	})
	lm.webGroup.pm.SetReportObserver(func(report taskManager.ReportMeta) {
		lm.handleGroupReportUpdate(webGroupID, report)
	})
	lm.binaryGroup.SetReportObserver(func(report taskManager.ReportMeta) {
		lm.handleGroupReportUpdate(binaryGroupID, report)
	})
	lm.pentestGroup.SetReportObserver(func(report taskManager.ReportMeta) {
		lm.handleGroupReportUpdate(pentestGroupID, report)
	})
	lm.configureGroupDecisionTools()
	if err := lm.bindLabFilesAndHistory(); err != nil {
		return nil, err
	}
	lm.syncGlobalStatusPanel(taskManager.BuildGlobalStatusPanel(labID))
	return lm, nil
}

func (lm *LabManager) syncGlobalStatusPanel(panel map[string]interface{}) {
	if lm == nil {
		return
	}
	for _, group := range lm.groups {
		if group == nil {
			continue
		}
		group.SyncGlobalStatusPanel(panel)
	}
}

func (lm *LabManager) bindLabFilesAndHistory() error {
	labStateDir := filepath.Join(lm.GetProjectDir(), "lab")
	if err := os.MkdirAll(labStateDir, 0755); err != nil {
		return err
	}
	if err := lm.bindTotalBrainMemory(labStateDir); err != nil {
		return err
	}
	lm.chatFilePath = filepath.Join(labStateDir, "chat.json")
	data, err := os.ReadFile(lm.chatFilePath)
	if err == nil {
		var msgs []DecisionBrain.ChatMessage
		if json.Unmarshal(data, &msgs) == nil {
			lm.chatMessages = lm.normalizeLoadedChatMessages(msgs)
			return nil
		}
	}
	history := lm.webGroup.pm.GetChatMessages()
	if history == nil {
		history = make([]DecisionBrain.ChatMessage, 0)
	}
	lm.chatMessages = lm.normalizeLoadedChatMessages(history)
	return nil
}

func (lm *LabManager) normalizeLoadedChatMessages(msgs []DecisionBrain.ChatMessage) []DecisionBrain.ChatMessage {
	out := make([]DecisionBrain.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		if strings.TrimSpace(msg.ChannelID) == "" {
			out = append(out, lm.normalizeLegacyChatMessage(msg))
			continue
		}
		out = append(out, msg)
	}
	if len(out) > maxLabChatRecord {
		out = append([]DecisionBrain.ChatMessage(nil), out[len(out)-maxLabChatRecord:]...)
	}
	return out
}

func (lm *LabManager) normalizeLegacyChatMessage(msg DecisionBrain.ChatMessage) DecisionBrain.ChatMessage {
	if msg.Role == "user" {
		msg.ChannelID = groupChannelID(webGroupID)
		msg.ChannelName = groupChannelName(webGroupName)
		msg.GroupID = webGroupID
		msg.GroupName = webGroupName
		msg.SenderType = "user"
		return msg
	}
	if strings.TrimSpace(msg.PersonaName) == totalBrainName {
		msg.ChannelID = decisionMakersChannelID
		msg.ChannelName = decisionMakersChanName
		msg.SenderType = "total_brain"
		return msg
	}
	return lm.webGroup.NormalizeGroupChatMessage(msg)
}

func (lm *LabManager) SetProjectDir(dir string) {
	lm.webGroup.pm.SetProjectDir(dir)
	_ = lm.bindLabFilesAndHistory()
}

func (lm *LabManager) GetLabName() string {
	return lm.labName
}

func (lm *LabManager) GetProjectID() string {
	return lm.labID
}

func (lm *LabManager) GetProjectName() string {
	return lm.labName
}

func (lm *LabManager) GetTaskContent() string {
	return lm.taskContent
}

func (lm *LabManager) configureGroupDecisionTools() {
	lm.registerSharedGroupDecisionTools(webGroupID, lm.webGroup.pm.RegisterDecisionBrainTool)
	lm.registerSharedGroupDecisionTools(binaryGroupID, lm.binaryGroup.RegisterDecisionBrainTool)
	lm.registerSharedGroupDecisionTools(pentestGroupID, lm.pentestGroup.RegisterDecisionBrainTool)
}

func (lm *LabManager) registerSharedGroupDecisionTools(groupID string, registerTool func(string, string, map[string]interface{}, func(map[string]interface{}) string)) {
	if registerTool == nil {
		return
	}
	registerTool(
		"Tool-RequestLabAssistance",
		"Request cross-group support from the lab total brain. Use this when your group decision brain decides that another research group should assist. The total brain will evaluate the request against the project overview, current task, and group capabilities before deciding whether to dispatch additional groups.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"request": map[string]interface{}{
					"type":        "string",
					"description": "Why cross-group assistance is needed and what the assisting group(s) should focus on. Required.",
				},
			},
			"required": []string{"request"},
		},
		func(args map[string]interface{}) string {
			return lm.handleGroupAssistanceRequest(groupID, args)
		},
	)
	registerTool(
		"Tool-ReportToDecisionMakers",
		"Send a visible update from this group decision brain to the decision-makers channel. Use this when responding to a total-brain coordination request or when the lab-level coordination layer needs a concise progress, finding, blocker, or completion update from your group.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"report_type": map[string]interface{}{
					"type":        "string",
					"description": "Update type. Recommended values: progress, finding, blocker, completion, status.",
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Main user-facing summary of the update. Required.",
				},
				"details": map[string]interface{}{
					"type":        "string",
					"description": "Optional supporting details that should also be visible in the decision-makers channel.",
				},
				"next_step": map[string]interface{}{
					"type":        "string",
					"description": "Optional next action your group will take.",
				},
			},
			"required": []string{"summary"},
		},
		func(args map[string]interface{}) string {
			return lm.handleDecisionMakersReport(groupID, args)
		},
	)
}

func (lm *LabManager) getProjectOverview() string {
	lm.mu.RLock()
	overview := strings.TrimSpace(lm.projectOverview)
	lm.mu.RUnlock()
	if overview == "" {
		return ""
	}
	if isNoSourceProjectOverview(overview) && lm.webGroup != nil && lm.webGroup.pm != nil && !lm.webGroup.pm.ShouldSkipProjectOverview() {
		return ""
	}
	return overview
}

func (lm *LabManager) setProjectOverview(overview string) {
	lm.mu.Lock()
	lm.projectOverview = strings.TrimSpace(overview)
	lm.mu.Unlock()
	if lm.binaryGroup != nil {
		lm.binaryGroup.SetProjectOverview(overview)
	}
	if lm.pentestGroup != nil {
		lm.pentestGroup.SetProjectOverview(overview)
	}
}

func (lm *LabManager) replaceCollaborationSession(requesterGroupID string, collaboratorGroupIDs []string, request string, reason string) {
	requesterGroupID = strings.TrimSpace(requesterGroupID)
	if requesterGroupID == "" {
		return
	}
	collaborators := normalizeGroupIDs(collaboratorGroupIDs)
	if len(collaborators) == 0 {
		lm.clearCollaborationSession(requesterGroupID)
		return
	}
	participants := normalizeGroupIDs(append([]string{requesterGroupID}, collaborators...))
	lm.coordinationMu.Lock()
	lm.collaborationSessions[requesterGroupID] = labCollaborationSession{
		RequesterGroupID:     requesterGroupID,
		CollaboratorGroupIDs: collaborators,
		ParticipantGroupIDs:  participants,
		Request:              strings.TrimSpace(request),
		Reason:               strings.TrimSpace(reason),
		UpdatedAt:            time.Now().Format("2006-01-02 15:04:05"),
	}
	lm.coordinationMu.Unlock()
}

func (lm *LabManager) clearCollaborationSession(requesterGroupID string) {
	requesterGroupID = strings.TrimSpace(requesterGroupID)
	if requesterGroupID == "" {
		return
	}
	lm.coordinationMu.Lock()
	delete(lm.collaborationSessions, requesterGroupID)
	lm.coordinationMu.Unlock()
}

func (lm *LabManager) collaborationStateSnapshot() []map[string]interface{} {
	lm.coordinationMu.RLock()
	defer lm.coordinationMu.RUnlock()
	if len(lm.collaborationSessions) == 0 {
		return nil
	}
	requesters := make([]string, 0, len(lm.collaborationSessions))
	for requester := range lm.collaborationSessions {
		requesters = append(requesters, requester)
	}
	sort.Strings(requesters)
	out := make([]map[string]interface{}, 0, len(requesters))
	for _, requester := range requesters {
		session := lm.collaborationSessions[requester]
		waitingGroups := make([]string, 0, len(session.ParticipantGroupIDs))
		for _, groupID := range session.ParticipantGroupIDs {
			if lm.groupWaitStates[groupID] {
				waitingGroups = append(waitingGroups, groupID)
			}
		}
		out = append(out, map[string]interface{}{
			"requester_group_id":       session.RequesterGroupID,
			"requester_group_name":     lm.groupNameByID(session.RequesterGroupID),
			"collaborator_group_ids":   append([]string(nil), session.CollaboratorGroupIDs...),
			"collaborator_group_names": plannedGroupNames(lm.groups, session.CollaboratorGroupIDs),
			"participant_group_ids":    append([]string(nil), session.ParticipantGroupIDs...),
			"participant_group_names":  plannedGroupNames(lm.groups, session.ParticipantGroupIDs),
			"waiting_group_ids":        waitingGroups,
			"waiting_group_names":      plannedGroupNames(lm.groups, waitingGroups),
			"request":                  session.Request,
			"reason":                   session.Reason,
			"updated_at":               session.UpdatedAt,
		})
	}
	return out
}

func (lm *LabManager) syncTotalBrainRuntimeState() {
	mem := lm.ensureTotalBrainMemory()
	if mem == nil {
		return
	}
	lm.syncTotalBrainMemoryRuntime(mem)
	_ = lm.persistTotalBrainMemory(mem)
}

func (lm *LabManager) handleGroupWaitStateChange(groupID string, waiting bool) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return
	}
	lm.coordinationMu.Lock()
	prev := lm.groupWaitStates[groupID]
	if waiting {
		lm.groupWaitStates[groupID] = true
	} else {
		delete(lm.groupWaitStates, groupID)
	}
	lm.coordinationMu.Unlock()
	if prev == waiting {
		return
	}
	lm.syncTotalBrainRuntimeState()
	if waiting {
		lm.wakeWaitingRequestersForCollaborator(groupID, "该协作组已进入等待状态，请重新判断现在是否轮到你继续推进。")
	}
}

func (lm *LabManager) wakeWaitingRequestersForCollaborator(sourceGroupID string, reason string) {
	sourceGroupID = strings.TrimSpace(sourceGroupID)
	if sourceGroupID == "" {
		return
	}
	targetIDs := make([]string, 0)
	lm.coordinationMu.RLock()
	for _, session := range lm.collaborationSessions {
		if !containsGroupID(session.CollaboratorGroupIDs, sourceGroupID) {
			continue
		}
		if !lm.groupWaitStates[session.RequesterGroupID] {
			continue
		}
		targetIDs = append(targetIDs, session.RequesterGroupID)
	}
	lm.coordinationMu.RUnlock()
	lm.wakeGroupDecisionBrains(targetIDs, sourceGroupID, reason)
}

func (lm *LabManager) wakeWaitingGroupsForCollaborationEvent(sourceGroupID string, reason string) {
	sourceGroupID = strings.TrimSpace(sourceGroupID)
	if sourceGroupID == "" {
		return
	}
	targetIDs := make([]string, 0)
	seen := make(map[string]struct{})
	lm.coordinationMu.RLock()
	for _, session := range lm.collaborationSessions {
		if !containsGroupID(session.ParticipantGroupIDs, sourceGroupID) {
			continue
		}
		for _, participant := range session.ParticipantGroupIDs {
			if participant == sourceGroupID || !lm.groupWaitStates[participant] {
				continue
			}
			if _, ok := seen[participant]; ok {
				continue
			}
			seen[participant] = struct{}{}
			targetIDs = append(targetIDs, participant)
		}
	}
	lm.coordinationMu.RUnlock()
	lm.wakeGroupDecisionBrains(targetIDs, sourceGroupID, reason)
}

func (lm *LabManager) wakeWaitingGroupsForAnyReport(sourceGroupID string, sourceGroupName string, report taskManager.ReportMeta, alreadyNotified map[string]struct{}) {
	sourceGroupID = strings.TrimSpace(sourceGroupID)
	if sourceGroupID == "" {
		return
	}
	if alreadyNotified == nil {
		alreadyNotified = make(map[string]struct{})
	}
	targetIDs := make([]string, 0)
	seen := make(map[string]struct{})
	lm.coordinationMu.RLock()
	for _, session := range lm.collaborationSessions {
		if !containsGroupID(session.ParticipantGroupIDs, sourceGroupID) {
			continue
		}
		for _, participant := range session.ParticipantGroupIDs {
			participant = strings.TrimSpace(participant)
			if participant == "" || participant == sourceGroupID {
				continue
			}
			if _, ok := alreadyNotified[participant]; ok {
				continue
			}
			if _, ok := seen[participant]; ok {
				continue
			}
			seen[participant] = struct{}{}
			targetIDs = append(targetIDs, participant)
		}
	}
	for groupID := range lm.groupWaitStates {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" || groupID == sourceGroupID {
			continue
		}
		if _, ok := alreadyNotified[groupID]; ok {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		targetIDs = append(targetIDs, groupID)
	}
	lm.coordinationMu.RUnlock()
	for _, groupID := range normalizeGroupIDs(targetIDs) {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		group.NotifyDecisionBrainEvent(buildSharedReportWakePrompt(sourceGroupName, report))
	}
}

func (lm *LabManager) wakeGroupDecisionBrains(targetIDs []string, sourceGroupID string, reason string) {
	sourceGroupID = strings.TrimSpace(sourceGroupID)
	reason = strings.TrimSpace(reason)
	if len(targetIDs) == 0 {
		return
	}
	sourceGroupName := lm.groupNameByID(sourceGroupID)
	for _, groupID := range normalizeGroupIDs(targetIDs) {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		group.NotifyDecisionBrainEvent(buildCollaborationWakePrompt(sourceGroupName, reason))
	}
}

func (lm *LabManager) ensureProjectOverview() string {
	if overview := lm.getProjectOverview(); overview != "" {
		return overview
	}
	if lm.webGroup == nil || lm.webGroup.pm == nil {
		overview := buildProjectOverviewFailureFallback(lm.taskContent)
		lm.setProjectOverview(overview)
		return overview
	}
	if lm.webGroup != nil && lm.webGroup.pm != nil && lm.webGroup.pm.ShouldSkipProjectOverview() {
		overview := buildNoSourceProjectOverview(lm.taskContent)
		lm.setProjectOverview(overview)
		return overview
	}
	overview := strings.TrimSpace(lm.webGroup.pm.EnsureProjectOverview())
	if overview == "" {
		overview = buildProjectOverviewFailureFallback(lm.taskContent)
	}
	lm.setProjectOverview(overview)
	return overview
}

func buildNoSourceProjectOverview(taskContent string) string {
	base := "当前实验室未提供源码，/workdir/sourceCodeDir 仅作为空源码目录使用，/workdir 可用于后续保存下载工件、脚本、日志与中间产物。项目速览已跳过，总脑应按黑盒任务处理，优先依据用户目标、外部攻击面、运行时反馈和各研究组能力进行调度。"
	taskContent = strings.TrimSpace(taskContent)
	if taskContent == "" {
		return base
	}
	return base + "\n用户总任务：" + taskContent
}

func isNoSourceProjectOverview(overview string) bool {
	return strings.HasPrefix(strings.TrimSpace(overview), "当前实验室未提供源码")
}

func buildProjectOverviewFailureFallback(taskContent string) string {
	base := "项目速览未能成功生成，可能是 LLM 请求失败、工具执行失败或速览阶段超时。总脑将基于当前任务描述先进入保守调度，后续再结合运行时反馈、题目入口和各研究组回报逐步补全上下文，而不是因为速览失败直接中断任务。"
	taskContent = strings.TrimSpace(taskContent)
	if taskContent == "" {
		return base
	}
	return base + "\n用户总任务：" + taskContent
}

func isProjectOverviewFailureFallback(overview string) bool {
	return strings.HasPrefix(strings.TrimSpace(overview), "项目速览未能成功生成")
}

func (lm *LabManager) currentTotalBrainStatus() string {
	lm.brainMu.RLock()
	defer lm.brainMu.RUnlock()
	return strings.TrimSpace(lm.totalBrainStatus)
}

func (lm *LabManager) currentLastDispatch() string {
	lm.brainMu.RLock()
	defer lm.brainMu.RUnlock()
	return strings.TrimSpace(lm.lastDispatch)
}

func (lm *LabManager) refreshLifecycleState() {
	lm.mu.RLock()
	startTime := strings.TrimSpace(lm.startTime)
	status := strings.TrimSpace(lm.status)
	lm.mu.RUnlock()
	totalBrainStatus := lm.currentTotalBrainStatus()
	if status == "" {
		status = "未运行"
	}
	if startTime == "" {
		if status != "错误" && status != "已停止" {
			status = "未运行"
		}
		lm.mu.Lock()
		lm.status = status
		lm.brainDone = false
		if status == "未运行" {
			lm.endTime = ""
		}
		lm.mu.Unlock()
		return
	}
	if status == "错误" || status == "已停止" {
		lm.mu.Lock()
		lm.status = status
		lm.brainDone = false
		lm.mu.Unlock()
		return
	}
	if totalBrainStatus == "项目速览中" {
		lm.mu.Lock()
		lm.status = "项目速览中"
		lm.endTime = ""
		lm.brainDone = false
		lm.mu.Unlock()
		return
	}
	if totalBrainStatus != "" && totalBrainStatus != defaultBrainState {
		lm.mu.Lock()
		lm.status = totalBrainStatus
		lm.endTime = ""
		lm.brainDone = false
		lm.mu.Unlock()
		return
	}
	anyRunning := false
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		if g.IsRunning() {
			anyRunning = true
		}
	}
	if anyRunning {
		lm.mu.Lock()
		lm.status = "正在运行"
		lm.endTime = ""
		lm.brainDone = false
		lm.mu.Unlock()
		return
	}
	if status == "运行结束" {
		lm.mu.Lock()
		lm.status = "运行结束"
		if strings.TrimSpace(lm.endTime) == "" {
			lm.endTime = time.Now().Format("2006-01-02 15:04:05")
		}
		lm.brainDone = true
		lm.mu.Unlock()
		return
	}
	lm.mu.Lock()
	lm.status = "待命"
	lm.endTime = ""
	lm.brainDone = false
	lm.mu.Unlock()
}

func (lm *LabManager) GetStatus() string {
	lm.refreshLifecycleState()
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.status
}

func (lm *LabManager) GetStartTime() string {
	lm.refreshLifecycleState()
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.startTime
}

func (lm *LabManager) GetEndTime() string {
	lm.refreshLifecycleState()
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.endTime
}

func (lm *LabManager) SetStatus(status string) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "未运行"
	}
	lm.mu.Lock()
	lm.status = status
	if status != "运行结束" {
		lm.brainDone = false
	}
	lm.mu.Unlock()
}

func (lm *LabManager) SetStartTime(t string) {
	lm.mu.Lock()
	lm.startTime = strings.TrimSpace(t)
	lm.mu.Unlock()
}

func (lm *LabManager) SetEndTime(t string) {
	lm.mu.Lock()
	lm.endTime = strings.TrimSpace(t)
	lm.mu.Unlock()
}

func (lm *LabManager) IsRunning() bool {
	switch strings.TrimSpace(lm.GetStatus()) {
	case "项目速览中", "调度中", "定向沟通中", "正在运行":
		return true
	default:
		return false
	}
}

func (lm *LabManager) runningGroupCount() int {
	if lm == nil {
		return 0
	}
	count := 0
	for _, groupID := range lm.groupOrder {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		if group.IsRunning() {
			count++
		}
	}
	return count
}

func (lm *LabManager) FinishedResearchGroupCount() int {
	if lm == nil {
		return 0
	}
	count := 0
	for _, groupID := range lm.groupOrder {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		if group.BrainFinished() {
			count++
		}
	}
	return count
}

func (lm *LabManager) HasFinishedResearchGroup() bool {
	return lm.FinishedResearchGroupCount() > 0
}

func (lm *LabManager) webExploitAutomationPolicy() DecisionBrain.ExploitAutomationPolicy {
	policy := DecisionBrain.DefaultExploitAutomationPolicy()
	if misc.GetEffectiveWebResearchMode() == misc.WebResearchModeComprehensive {
		policy.AutoVerifyExploits = false
		policy.AutoPublishAnalyzeReport = false
	}
	return policy
}

func (lm *LabManager) NeedsQueuedRestartAfterChat(channelID string) bool {
	return normalizeChannelID(channelID) == groupChannelID(webGroupID)
}

func (lm *LabManager) GetSourceCodeDir() string {
	if lm == nil || lm.webGroup == nil || lm.webGroup.pm == nil {
		return ""
	}
	return lm.webGroup.pm.GetSourceCodeDir()
}

func (lm *LabManager) GetWorkDir() string {
	if lm == nil || lm.webGroup == nil || lm.webGroup.pm == nil {
		return ""
	}
	return lm.webGroup.pm.GetWorkDir()
}

func (lm *LabManager) GetProjectDir() string {
	if lm == nil || lm.webGroup == nil || lm.webGroup.pm == nil {
		return ""
	}
	return lm.webGroup.pm.GetProjectDir()
}

func (lm *LabManager) StartTask() {
	lm.SetStartTime(time.Now().Format("2006-01-02 15:04:05"))
	lm.SetEndTime("")
	if lm.webGroup != nil && lm.webGroup.pm != nil && lm.webGroup.pm.ShouldSkipProjectOverview() {
		lm.SetStatus("调度中")
		lm.setTotalBrainState("调度中", lm.taskContent, nil, "未提供源码，已跳过项目速览")
		lm.notifyGroupsUpdated()
		overview := lm.ensureProjectOverview()
		if overview != "" {
			lm.emitBrainMessage("实验室已启动。当前任务未提供源码，已跳过项目速览并进入黑盒调度阶段。总脑上下文如下：\n" + overview)
		} else {
			lm.emitBrainMessage("实验室已启动。当前任务未提供源码，已跳过项目速览并进入黑盒调度阶段。")
		}
	} else {
		lm.SetStatus("项目速览中")
		lm.setTotalBrainState("项目速览中", lm.lastDispatch, lm.lastDispatchGroups, lm.lastDispatchReason)
		lm.emitBrainMessage("实验室已启动，先执行项目速览，再由总脑决定调动哪些研究组。")
		lm.notifyGroupsUpdated()
		overview := lm.ensureProjectOverview()
		if overview != "" {
			if isProjectOverviewFailureFallback(overview) {
				lm.emitBrainMessage("项目速览未成功完成，已切换为保守概览继续调度。以下是总脑当前使用的上下文：\n" + overview)
			} else {
				lm.emitBrainMessage("项目速览完成。以下是总脑读取到的项目概览：\n" + overview)
			}
		} else {
			lm.SetStatus("错误")
			lm.SetEndTime(time.Now().Format("2006-01-02 15:04:05"))
			lm.setTotalBrainState("错误", lm.taskContent, nil, "项目速览失败，无法建立可靠的总脑调度上下文")
			lm.emitBrainMessage("项目速览失败，总脑无法建立可靠的调度上下文，本次任务已结束并标记为错误。")
			lm.notifyGroupsUpdated()
			return
		}
	}
	if err := lm.ensureSharedSandbox(); err != nil {
		lm.SetStatus("错误")
		lm.SetEndTime(time.Now().Format("2006-01-02 15:04:05"))
		lm.setTotalBrainState("错误", lm.taskContent, nil, "共享 sandbox 初始化失败")
		lm.emitBrainMessage("共享 sandbox 初始化失败，实验室无法向各研究组派发需要运行时环境的任务：" + err.Error())
		lm.notifyGroupsUpdated()
		return
	}

	plan, err := lm.planDispatchByTotalBrain(lm.taskContent, "用户", "", false)
	if err != nil {
		plan = lm.fallbackDispatchPlan(lm.taskContent)
		lm.emitBrainMessage(fmt.Sprintf("总脑调度规划失败，已切回保守兜底方案：%s", err.Error()))
	}

	selected := normalizeGroupIDs(plan.GroupIDs)
	for _, groupID := range selected {
		if groupID == webGroupID {
			continue
		}
		if g, ok := lm.groups[groupID]; ok && !g.IsRunning() {
			g.StartTask()
		}
	}
	lm.dispatchPlannedTasks(plan, lm.taskContent, totalBrainName, lm.buildDispatchBrainNote(plan), false)
	if containsGroupID(selected, webGroupID) {
		lm.webGroup.StartTask()
	}
	lm.notifyGroupsUpdated()
	lm.setTotalBrainState(defaultBrainState, lm.taskContent, plannedGroupNames(lm.groups, selected), plan.Reason)
}

func (lm *LabManager) StopTask() {
	taskManager.SetBackgroundProcessObserver(lm.labID, nil)
	taskManager.CloseBackgroundProcesses(lm.labID)
	lm.webGroup.StopTask()
	lm.binaryGroup.StopTask()
	lm.pentestGroup.StopTask()
	lm.SetStatus("已停止")
	lm.SetEndTime(time.Now().Format("2006-01-02 15:04:05"))
	lm.setTotalBrainState("已停止", lm.lastDispatch, lm.lastDispatchGroups, lm.lastDispatchReason)
	lm.notifyGroupsUpdated()
}

func (lm *LabManager) GetContainerList() []taskManager.ContainerInfo {
	if lm == nil || lm.webGroup == nil || lm.webGroup.pm == nil {
		return nil
	}
	containers := lm.webGroup.pm.GetContainerList()
	if len(containers) == 0 {
		return nil
	}
	out := make([]taskManager.ContainerInfo, 0, len(containers))
	seen := make(map[string]struct{}, len(containers))
	for _, container := range containers {
		key := strings.TrimSpace(container.ContainerId)
		if key == "" {
			key = strings.TrimSpace(container.ContainerIP) + "|" + strings.TrimSpace(container.Image)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, container)
	}
	return out
}

func (lm *LabManager) groupEvents(groupID string, count int) []string {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return lm.webGroup.pm.GetEvent(count)
	case binaryGroupID:
		return lm.binaryGroup.GetEvent(count)
	case pentestGroupID:
		return lm.pentestGroup.GetEvent(count)
	default:
		return nil
	}
}

func (lm *LabManager) groupBrainFeed(groupID string, count int) []map[string]interface{} {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return lm.webGroup.pm.GetBrainFeed(count)
	case binaryGroupID:
		return lm.binaryGroup.GetBrainFeed(count)
	case pentestGroupID:
		return lm.pentestGroup.GetBrainFeed(count)
	default:
		return nil
	}
}

func (lm *LabManager) groupAgentFeed(groupID string, agentID string, count int) []map[string]interface{} {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return lm.webGroup.pm.GetAgentFeed(agentID, count)
	case binaryGroupID:
		return lm.binaryGroup.GetAgentFeed(agentID, count)
	case pentestGroupID:
		return lm.pentestGroup.GetAgentFeed(agentID, count)
	default:
		return nil
	}
}

func (lm *LabManager) rosterNamespace(groupID string) string {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return "web"
	case binaryGroupID:
		return "binary"
	case pentestGroupID:
		return "pentest"
	default:
		return strings.TrimSpace(groupID)
	}
}

func (lm *LabManager) GetEvent(count int) []string {
	out := make([]string, 0)
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		events := lm.groupEvents(groupID, count)
		for _, event := range events {
			event = strings.TrimSpace(event)
			if event == "" {
				continue
			}
			if groupID == webGroupID {
				out = append(out, event)
				continue
			}
			out = append(out, "["+g.Name()+"] "+event)
		}
	}
	if count > 0 && len(out) > count {
		return out[len(out)-count:]
	}
	return out
}

func (lm *LabManager) GetBrainFeed(count int) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		for _, item := range lm.groupBrainFeed(groupID, count) {
			copyItem := make(map[string]interface{}, len(item)+2)
			for k, v := range item {
				copyItem[k] = v
			}
			copyItem["groupId"] = g.ID()
			copyItem["groupName"] = g.Name()
			out = append(out, copyItem)
		}
	}
	if count > 0 && len(out) > count {
		return out[len(out)-count:]
	}
	return out
}

func (lm *LabManager) GetAgentFeed(agentID string, count int) []map[string]interface{} {
	for _, groupID := range lm.groupOrder {
		if feed := lm.groupAgentFeed(groupID, agentID, count); len(feed) > 0 {
			return feed
		}
	}
	return nil
}

func (lm *LabManager) GetVulnList() []taskManager.Vuln {
	return lm.webGroup.pm.GetVulnList()
}

func (lm *LabManager) SetVulns(vuln []taskManager.Vuln) {
	lm.webGroup.pm.SetVulns(vuln)
}

func (lm *LabManager) SetEvent(event []string) {
	lm.webGroup.pm.SetEvent(event)
}

func (lm *LabManager) GetReportList() map[string]string {
	reports := lm.listSharedReports()
	if len(reports) == 0 {
		return nil
	}
	out := make(map[string]string, len(reports))
	reportIDCount := make(map[string]int, len(reports))
	for _, report := range reports {
		reportIDCount[strings.TrimSpace(report.Meta.ReportID)]++
	}
	for _, report := range reports {
		path := strings.TrimSpace(report.Meta.Path)
		if path == "" {
			continue
		}
		out[report.DownloadID] = path
		reportID := strings.TrimSpace(report.Meta.ReportID)
		if reportID != "" && reportIDCount[reportID] == 1 {
			out[reportID] = path
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (lm *LabManager) GetEnvInfo() map[string]interface{} {
	if lm == nil || lm.webGroup == nil || lm.webGroup.pm == nil {
		return nil
	}
	webEnv := lm.webGroup.pm.GetEnvInfo()
	if len(webEnv) == 0 {
		return nil
	}
	return webEnv
}

func (lm *LabManager) GetExploitIdeaList() []*taskManager.ExploitIdea {
	return lm.webGroup.pm.GetExploitIdeaList()
}

func (lm *LabManager) GetExploitChainList() []*taskManager.ExploitChain {
	return lm.webGroup.pm.GetExploitChainList()
}

func (lm *LabManager) RestoreExploitState(ideas []*taskManager.ExploitIdea, chains []*taskManager.ExploitChain) {
	lm.webGroup.pm.RestoreExploitState(ideas, chains)
}

func (lm *LabManager) GetAgentRuntimeList() []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		out = append(out, g.GetAgentRuntimeList()...)
	}
	return out
}

func (lm *LabManager) GetDigitalHumanRoster() map[string]interface{} {
	out := make(map[string]interface{})
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		prefix := lm.rosterNamespace(groupID) + "::"
		for k, v := range g.GetDigitalHumanRoster() {
			out[prefix+k] = v
		}
	}
	return out
}

func (lm *LabManager) GetBrainFinished() bool {
	lm.refreshLifecycleState()
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.brainDone
}

func (lm *LabManager) HandleUserChat(msg string, channelID string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "message is required"
	}
	channelID = normalizeChannelID(channelID)
	lm.emitChatMessage(DecisionBrain.ChatMessage{
		Role:        "user",
		Text:        msg,
		Ts:          time.Now().Format("15:04:05"),
		ChannelID:   channelID,
		ChannelName: lm.channelName(channelID),
		GroupID:     channelGroupID(channelID),
		GroupName:   lm.groupNameByChannel(channelID),
		SenderType:  "user",
	})
	lm.persistChatMessagesNow()
	go func(raw string, normalizedChannelID string) {
		defer func() {
			if r := recover(); r != nil {
				misc.Debug("HandleUserChat async dispatch panic: %v", r)
			}
		}()
		lm.TeamChatFromSender(raw, "用户", normalizedChannelID)
	}(msg, channelID)
	return "message accepted"
}

func (lm *LabManager) TeamChat(msg string) string {
	return lm.TeamChatFromSender(msg, "用户", decisionMakersChannelID)
}

func (lm *LabManager) TeamChatFromSender(raw string, sender string, channelID string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "empty message"
	}
	channelID = normalizeChannelID(channelID)

	if channelID != decisionMakersChannelID {
		if groupID := channelGroupID(channelID); groupID != "" {
			return lm.routeGroupChat(groupID, raw, sender)
		}
	}

	target, body, addressed := parseChatAddress(raw)
	if addressed {
		switch {
		case isAllAlias(target):
			lm.appendTotalBrainMemoryMessages(llm.Message{
				Role:    llm.RoleUser,
				Content: fmt.Sprintf("[总脑广播请求]\n发送者：%s\n内容：%s", strings.TrimSpace(sender), strings.TrimSpace(body)),
			})
			groupIDs := append([]string(nil), lm.groupOrder...)
			groupTasks := make(map[string]string, len(groupIDs))
			for _, groupID := range groupIDs {
				groupTasks[groupID] = "@all " + body
			}
			lm.dispatchPlannedTasks(labDispatchPlan{
				GroupIDs:   groupIDs,
				GroupTasks: groupTasks,
				Reason:     "用户在决策者群显式要求向所有研究组广播同一任务",
			}, "@all "+body, totalBrainName, fmt.Sprintf("总脑已将广播同步给 %s。", strings.Join(plannedGroupNames(lm.groups, groupIDs), "、")), true)
			return "broadcast dispatched"
		case isTotalBrainAlias(target):
			return lm.dispatchByBrain(body, sender)
		default:
			if group := lm.resolveDecisionGroupByAlias(target); group != nil {
				lm.dispatchDirectToDecisionBrain(group, body, sender)
				return "decision chat dispatched"
			}
			lm.emitBrainMessage(fmt.Sprintf("决策者群中只能 @各研究组决策大脑；%s 不是可用的决策者目标。", target))
			return "target not found"
		}
	}

	return lm.dispatchByBrain(raw, sender)
}

func (lm *LabManager) routeGroupChat(groupID string, raw string, sender string) string {
	g, ok := lm.groups[groupID]
	if !ok {
		lm.emitBrainMessage("目标研究组不存在。")
		return "group not found"
	}
	g.HandleGroupChat(raw, sender)
	lm.notifyGroupsUpdated()
	return ""
}

func (lm *LabManager) dispatchByBrain(raw string, sender string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "empty message"
	}
	plan, err := lm.planDispatchByTotalBrain(raw, sender, "", false)
	if err != nil {
		plan = lm.fallbackDispatchPlan(raw)
		lm.emitBrainMessage(fmt.Sprintf("总脑规划失败，已启用保守兜底调度：%s", err.Error()))
	}
	lm.dispatchPlannedTasks(plan, raw, totalBrainName, lm.buildDispatchBrainNote(plan), true)
	return "task dispatched"
}

func (lm *LabManager) dispatchPlannedTasks(plan labDispatchPlan, raw string, sender string, brainNote string, autoStart bool) {
	groupIDs := normalizeGroupIDs(plan.GroupIDs)
	groupNames := plannedGroupNames(lm.groups, groupIDs)
	if brainNote != "" {
		lm.emitBrainMessage(brainNote)
	}
	lm.setTotalBrainState("调度中", raw, groupNames, plan.Reason)
	for _, groupID := range groupIDs {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		payload := strings.TrimSpace(plan.GroupTasks[groupID])
		if payload == "" {
			payload = strings.TrimSpace(raw)
		}
		if payload == "" {
			continue
		}
		if strings.TrimSpace(sender) == totalBrainName {
			lm.emitTotalBrainDirective(g, payload)
		}
		ack := g.HandleDecisionChat(payload, sender)
		if strings.TrimSpace(ack) != "" {
			lm.emitDecisionBrainMessage(g, ack)
		}
	}
	if autoStart {
		lm.startGroups(groupIDs, true)
	}
	lm.notifyGroupsUpdated()
	lm.setTotalBrainState(defaultBrainState, raw, groupNames, plan.Reason)
}

func (lm *LabManager) dispatchDirectToDecisionBrain(group researchGroup, raw string, sender string) {
	if group == nil {
		return
	}
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return
	}
	lm.appendTotalBrainMemoryMessages(llm.Message{
		Role: llm.RoleUser,
		Content: fmt.Sprintf(
			"[定向协调请求]\n发送者：%s\n目标研究组：%s\n目标决策大脑：%s\n内容：%s",
			strings.TrimSpace(sender),
			strings.TrimSpace(group.Name()),
			strings.TrimSpace(group.DecisionBrainName()),
			payload,
		),
	})
	lm.setTotalBrainState("定向沟通中", payload, []string{group.Name()}, "用户显式指定了目标研究组决策大脑")
	lm.emitBrainMessage(fmt.Sprintf("总脑已将该条定向消息转给%s。", group.DecisionBrainName()))
	ack := group.HandleDecisionChat(payload, sender)
	if strings.TrimSpace(ack) != "" {
		lm.emitDecisionBrainMessage(group, ack)
	}
	lm.startGroups([]string{group.ID()}, true)
	lm.notifyGroupsUpdated()
	lm.setTotalBrainState(defaultBrainState, payload, []string{group.Name()}, "用户显式指定了目标研究组决策大脑")
}

func (lm *LabManager) planDispatchByTotalBrain(raw string, sender string, requesterGroupID string, allowEmpty bool) (labDispatchPlan, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return labDispatchPlan{}, fmt.Errorf("empty dispatch task")
	}
	overview := lm.ensureProjectOverview()
	requestKind := "user_task"
	if requesterGroupID != "" {
		requestKind = "group_assistance"
	}
	payload := map[string]interface{}{
		"request_kind":     requestKind,
		"sender":           sender,
		"user_task":        raw,
		"project_overview": overview,
		"groups":           lm.groupPlanningCatalog(),
	}
	if requesterGroupID != "" {
		payload["requester_group_id"] = requesterGroupID
		payload["requester_group_name"] = lm.groupNameByID(requesterGroupID)
	}
	js, _ := json.MarshalIndent(payload, "", "  ")
	groupEnums := append([]string(nil), lm.groupOrder...)
	tool := llm.ToolDef{
		Name:        "Tool-DispatchResearchTasks",
		Description: "Select the minimum necessary set of research groups based on the project overview, the current request, and group capabilities, then generate a concrete task for each selected group. Prefer a single group when one group can handle the task end-to-end. group_ids may be empty only when request_kind=group_assistance and no extra group should be added.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"group_ids": map[string]interface{}{
					"type":        "array",
					"description": "Research group IDs to dispatch. Prefer exactly one group unless there is a clear capability gap that requires collaboration.",
					"items": map[string]interface{}{
						"type": "string",
						"enum": groupEnums,
					},
				},
				"dispatch_reason": map[string]interface{}{
					"type":        "string",
					"description": "Routing rationale. Explicitly ground it in the project overview, task goal, and why each selected group is actually needed.",
				},
				"group_tasks": map[string]interface{}{
					"type":        "array",
					"description": "Per-group task breakdown.",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"group_id": map[string]interface{}{
								"type": "string",
								"enum": groupEnums,
							},
							"task": map[string]interface{}{
								"type":        "string",
								"description": "Task content to send to that group's decision brain.",
							},
						},
						"required": []string{"group_id", "task"},
					},
				},
			},
			"required": []string{"group_ids", "dispatch_reason", "group_tasks"},
		},
	}
	tools := []llm.ToolDef{tool}
	requestMsg := lm.totalBrainRequestMemoryMessage(raw, sender, requesterGroupID)
	mem := lm.ensureTotalBrainMemory()
	var msgs []llm.Message
	if mem != nil {
		lm.syncTotalBrainMemoryRuntime(mem)
		mem.AddMessage(requestMsg)
		lm.compressTotalBrainMemoryIfNeeded(mem)
		_ = lm.persistTotalBrainMemory(mem)
		msgs = mem.GetContext()
	} else {
		msgs = []llm.Message{
			{Role: llm.RoleSystem, Content: totalBrainDecisionMemorySystemPrompt()},
			requestMsg,
			{Role: llm.RoleUser, Content: string(js)},
		}
	}
	cli := llm.GetResponsesClient("decision", "main_setting")
	model := misc.GetConfigValueDefault("decision", "MODEL", "")
	if model == "" {
		model = misc.GetConfigValueDefault("main_setting", "MODEL", "")
	}
	contextLimitRecoveries := 0
	for attempt := 0; attempt < 2; attempt++ {
		if mem != nil {
			mem.CaptureLastLLMContext(msgs)
		}
		ctx, cancel := context.WithTimeout(context.Background(), totalBrainPlanTimeout)
		resp, err := llm.RequestLLMWithOpts(cli, ctx, model, msgs, tools, llm.RequestLLMOpts{ProjectName: lm.labID, AgentLabel: totalBrainName})
		cancel()
		if err != nil {
			if mem != nil && llm.IsContextLimitError(err) && contextLimitRecoveries < 2 {
				contextLimitRecoveries++
				beforeTokens := llm.CountMessagesTokens(msgs)
				if compressErr := mem.CompressIfNeeded(cli, model); compressErr != nil {
					misc.Debug("compress total brain memory failed for %s: %v", lm.labName, compressErr)
				}
				msgs = mem.GetContext()
				afterTokens := llm.CountMessagesTokens(msgs)
				misc.Debug("total brain context limit hit, compressed memory and retrying (attempt=%d before=%d after=%d): %v", contextLimitRecoveries, beforeTokens, afterTokens, err)
				continue
			}
			return labDispatchPlan{}, err
		}
		contextLimitRecoveries = 0
		msg := llm.ResponseToMessage(resp)
		if mem != nil {
			mem.AddMessage(msg)
		}
		var correction llm.Message
		handledUnexpectedTool := false
		for _, tc := range msg.ToolCalls {
			switch tc.Name {
			case tool.Name:
				plan, err := lm.parseDispatchPlanArgs(raw, tc.Arguments, allowEmpty)
				toolResult := llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
				}
				if err == nil {
					toolResult.Content = toolCalling.Success(lm.totalBrainDispatchToolResult(plan))
					if mem != nil {
						mem.AddMessage(toolResult)
						lm.compressTotalBrainMemoryIfNeeded(mem)
						_ = lm.persistTotalBrainMemory(mem)
					}
					return plan, nil
				}
				toolResult.Content = toolCalling.InvalidToolArgumentsFailure(tc.Name, tc.Arguments, err)
				correction = llm.Message{Role: llm.RoleUser, Content: "The tool arguments were invalid: " + err.Error() + ". Call Tool-DispatchResearchTasks again with valid group_ids and per-group tasks."}
				if mem != nil {
					mem.AddMessage(toolResult)
					mem.AddMessage(correction)
					lm.compressTotalBrainMemoryIfNeeded(mem)
					_ = lm.persistTotalBrainMemory(mem)
					msgs = mem.GetContext()
				} else {
					msgs = append(msgs, msg, toolResult, correction)
				}
				goto nextAttempt
			default:
				handledUnexpectedTool = true
				toolResult := llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Content:    toolCalling.UnknownToolFailure(tc.Name, tc.Arguments),
				}
				if mem != nil {
					mem.AddMessage(toolResult)
				} else {
					msgs = append(msgs, toolResult)
				}
			}
		}
		if plan, ok := lm.tryParseDispatchPlanFromAssistantContent(raw, msg.Content, allowEmpty); ok {
			misc.Debug("total brain dispatch fallback accepted assistant JSON content without a valid tool call")
			return plan, nil
		}
		if handledUnexpectedTool {
			correction = llm.Message{Role: llm.RoleUser, Content: "One or more tool calls were invalid. Review the tool results, then call Tool-DispatchResearchTasks with valid arguments."}
		} else {
			correction = llm.Message{Role: llm.RoleUser, Content: "You must call Tool-DispatchResearchTasks. Do not answer with plain text only."}
		}
		if mem != nil {
			mem.AddMessage(correction)
			lm.compressTotalBrainMemoryIfNeeded(mem)
			_ = lm.persistTotalBrainMemory(mem)
			msgs = mem.GetContext()
		} else {
			msgs = append(msgs, msg, correction)
		}
	nextAttempt:
	}
	return labDispatchPlan{}, fmt.Errorf("total brain did not return a valid dispatch tool call")
}

func (lm *LabManager) tryParseDispatchPlanFromAssistantContent(raw string, content string, allowEmpty bool) (labDispatchPlan, bool) {
	for _, candidate := range extractStructuredJSONCandidates(content) {
		plan, err := lm.parseDispatchPlanArgs(raw, candidate, allowEmpty)
		if err == nil {
			return plan, true
		}
	}
	return labDispatchPlan{}, false
}

func extractStructuredJSONCandidates(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	seen := make(map[string]struct{})
	candidates := make([]string, 0, 4)
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	add(content)

	parts := strings.Split(content, "```")
	for i := 1; i < len(parts); i += 2 {
		block := strings.TrimSpace(parts[i])
		if block == "" {
			continue
		}
		if newline := strings.IndexByte(block, '\n'); newline >= 0 {
			firstLine := strings.TrimSpace(block[:newline])
			rest := strings.TrimSpace(block[newline+1:])
			if rest != "" && !strings.HasPrefix(firstLine, "{") && !strings.HasPrefix(firstLine, "[") {
				block = rest
			}
		}
		add(block)
	}

	firstBrace := strings.IndexByte(content, '{')
	lastBrace := strings.LastIndexByte(content, '}')
	if firstBrace >= 0 && lastBrace > firstBrace {
		add(content[firstBrace : lastBrace+1])
	}

	return candidates
}

func (lm *LabManager) parseDispatchPlanArgs(raw string, arguments string, allowEmpty bool) (labDispatchPlan, error) {
	var args totalBrainDispatchToolArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return labDispatchPlan{}, fmt.Errorf("parse tool arguments failed: %w", err)
	}
	groupIDs := normalizeGroupIDs(args.GroupIDs)
	if len(groupIDs) == 0 && !allowEmpty {
		return labDispatchPlan{}, fmt.Errorf("no group selected")
	}
	groupTasks := make(map[string]string)
	for _, item := range args.GroupTasks {
		groupID := strings.TrimSpace(item.GroupID)
		task := strings.TrimSpace(item.Task)
		if groupID == "" || task == "" {
			continue
		}
		if _, ok := lm.groups[groupID]; !ok {
			continue
		}
		groupTasks[groupID] = task
	}
	validGroupIDs := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		if _, ok := lm.groups[groupID]; !ok {
			continue
		}
		validGroupIDs = append(validGroupIDs, groupID)
		if strings.TrimSpace(groupTasks[groupID]) == "" {
			groupTasks[groupID] = raw
		}
	}
	if len(validGroupIDs) == 0 && !allowEmpty {
		return labDispatchPlan{}, fmt.Errorf("no valid group selected")
	}
	reason := strings.TrimSpace(args.DispatchReason)
	if reason == "" {
		reason = "总脑完成了基于项目速览的调度判断"
	}
	return labDispatchPlan{
		GroupIDs:   validGroupIDs,
		GroupTasks: groupTasks,
		Reason:     reason,
	}, nil
}

func (lm *LabManager) coordinateGroupReportByTotalBrain(group researchGroup, reportType string, summary string, details string, nextStep string) (totalBrainReportResponse, error) {
	if group == nil {
		return totalBrainReportResponse{}, fmt.Errorf("group is required")
	}
	overview := lm.ensureProjectOverview()
	payload := map[string]interface{}{
		"request_kind":             "group_report",
		"reporting_group_id":       group.ID(),
		"reporting_group_name":     group.Name(),
		"reporting_decision_brain": group.DecisionBrainName(),
		"report_type":              strings.TrimSpace(reportType),
		"summary":                  strings.TrimSpace(summary),
		"details":                  strings.TrimSpace(details),
		"next_step":                strings.TrimSpace(nextStep),
		"project_overview":         overview,
		"groups":                   lm.groupPlanningCatalog(),
	}
	js, _ := json.MarshalIndent(payload, "", "  ")
	reportTool := llm.ToolDef{
		Name:        "Tool-RespondToGroupReport",
		Description: "Generate the total brain's visible decision-makers-channel reply and the actual follow-up instruction to send back to the reporting group's decision brain after a report arrives. This tool does not dispatch or wake any additional group by itself.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"visible_message": map[string]interface{}{
					"type":        "string",
					"description": "Concise Chinese message that the total brain will show in the decision-makers channel after reviewing this report. Do not claim that another group has already been awakened, dispatched, arranged, or started in this step.",
				},
				"reply_to_reporting_group": map[string]interface{}{
					"type":        "string",
					"description": "Concrete Chinese follow-up instruction for the reporting group's decision brain. It must say whether to continue, refine, wait, or request support. If another group should help, instruct the reporting group to prepare the needed artifacts under /workdir/sourceCodeDir and then call Tool-RequestLabAssistance; do not claim the other group was already started.",
				},
				"coordination_reason": map[string]interface{}{
					"type":        "string",
					"description": "Internal rationale for this follow-up decision, grounded in the report, current task, and lab state.",
				},
			},
			"required": []string{"visible_message", "reply_to_reporting_group", "coordination_reason"},
		},
	}
	tools := []llm.ToolDef{reportTool}
	requestMsg := lm.totalBrainReportRequestMemoryMessage(group, reportType, summary, details, nextStep)
	mem := lm.ensureTotalBrainMemory()
	var msgs []llm.Message
	if mem != nil {
		lm.syncTotalBrainMemoryRuntime(mem)
		mem.AddMessage(requestMsg)
		lm.compressTotalBrainMemoryIfNeeded(mem)
		_ = lm.persistTotalBrainMemory(mem)
		msgs = mem.GetContext()
	} else {
		msgs = []llm.Message{
			{Role: llm.RoleSystem, Content: totalBrainDecisionMemorySystemPrompt()},
			requestMsg,
			{Role: llm.RoleUser, Content: string(js)},
		}
	}
	cli := llm.GetResponsesClient("decision", "main_setting")
	model := misc.GetConfigValueDefault("decision", "MODEL", "")
	if model == "" {
		model = misc.GetConfigValueDefault("main_setting", "MODEL", "")
	}
	contextLimitRecoveries := 0
	for attempt := 0; attempt < 2; attempt++ {
		if mem != nil {
			mem.CaptureLastLLMContext(msgs)
		}
		ctx, cancel := context.WithTimeout(context.Background(), totalBrainPlanTimeout)
		resp, err := llm.RequestLLMWithOpts(cli, ctx, model, msgs, tools, llm.RequestLLMOpts{ProjectName: lm.labID, AgentLabel: totalBrainName})
		cancel()
		if err != nil {
			if mem != nil && llm.IsContextLimitError(err) && contextLimitRecoveries < 2 {
				contextLimitRecoveries++
				beforeTokens := llm.CountMessagesTokens(msgs)
				if compressErr := mem.CompressIfNeeded(cli, model); compressErr != nil {
					misc.Debug("compress total brain memory failed for %s: %v", lm.labName, compressErr)
				}
				msgs = mem.GetContext()
				afterTokens := llm.CountMessagesTokens(msgs)
				misc.Debug("total brain context limit hit during group report handling, compressed memory and retrying (attempt=%d before=%d after=%d): %v", contextLimitRecoveries, beforeTokens, afterTokens, err)
				continue
			}
			return totalBrainReportResponse{}, err
		}
		contextLimitRecoveries = 0
		msg := llm.ResponseToMessage(resp)
		if mem != nil {
			mem.AddMessage(msg)
		}
		var correction llm.Message
		var toolResults []llm.Message
		var response totalBrainReportResponse
		haveResponse := false
		for _, tc := range msg.ToolCalls {
			switch tc.Name {
			case reportTool.Name:
				result, err := lm.parseTotalBrainReportResponseArgs(tc.Arguments)
				toolResult := llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
				}
				if err == nil {
					toolResult.Content = toolCalling.Success(lm.totalBrainReportResponseToolResult(result))
					toolResults = append(toolResults, toolResult)
					response = result
					haveResponse = true
					continue
				}
				toolResult.Content = toolCalling.InvalidToolArgumentsFailure(tc.Name, tc.Arguments, err)
				toolResults = append(toolResults, toolResult)
				correction = llm.Message{Role: llm.RoleUser, Content: "The Tool-RespondToGroupReport arguments were invalid: " + err.Error() + ". Call Tool-RespondToGroupReport again with a visible_message, a concrete reply_to_reporting_group, and a coordination_reason."}
			default:
				toolResults = append(toolResults, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Content:    toolCalling.UnknownToolFailure(tc.Name, tc.Arguments),
				})
				if correction.Content == "" {
					correction = llm.Message{Role: llm.RoleUser, Content: "One or more tool calls were invalid. Review the tool results, then call Tool-RespondToGroupReport with valid arguments."}
				}
			}
		}
		if haveResponse {
			if mem != nil {
				for _, item := range toolResults {
					mem.AddMessage(item)
				}
				lm.compressTotalBrainMemoryIfNeeded(mem)
				_ = lm.persistTotalBrainMemory(mem)
			}
			return response, nil
		}
		if len(toolResults) > 0 && mem != nil {
			for _, item := range toolResults {
				mem.AddMessage(item)
			}
		}
		if correction.Content == "" {
			correction = llm.Message{Role: llm.RoleUser, Content: "You must call Tool-RespondToGroupReport. Do not answer with plain text only."}
		}
		if mem != nil {
			mem.AddMessage(correction)
			lm.compressTotalBrainMemoryIfNeeded(mem)
			_ = lm.persistTotalBrainMemory(mem)
			msgs = mem.GetContext()
		} else {
			combined := []llm.Message{msg}
			combined = append(combined, toolResults...)
			combined = append(combined, correction)
			msgs = append(msgs, combined...)
		}
	}
	return totalBrainReportResponse{}, fmt.Errorf("total brain did not return a valid group report response tool call")
}

func (lm *LabManager) parseTotalBrainReportResponseArgs(arguments string) (totalBrainReportResponse, error) {
	var args totalBrainReportResponseToolArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return totalBrainReportResponse{}, fmt.Errorf("parse tool arguments failed: %w", err)
	}
	result := totalBrainReportResponse{
		VisibleMessage:        strings.TrimSpace(args.VisibleMessage),
		ReplyToReportingGroup: strings.TrimSpace(args.ReplyToReportingGroup),
		CoordinationReason:    strings.TrimSpace(args.CoordinationReason),
	}
	if result.VisibleMessage == "" {
		return totalBrainReportResponse{}, fmt.Errorf("visible_message is required")
	}
	if result.ReplyToReportingGroup == "" {
		return totalBrainReportResponse{}, fmt.Errorf("reply_to_reporting_group is required")
	}
	if result.CoordinationReason == "" {
		result.CoordinationReason = "总脑基于该组汇报与当前实验室状态完成了后续协调判断"
	}
	return result, nil
}

func (lm *LabManager) fallbackDispatchPlan(raw string) labDispatchPlan {
	task := strings.TrimSpace(raw)
	groupIDs, reason := lm.selectGroups(task)
	if len(groupIDs) == 0 {
		groupIDs = []string{webGroupID}
	}
	groupTasks := make(map[string]string, len(groupIDs))
	for _, groupID := range groupIDs {
		groupTasks[groupID] = task
	}
	if strings.TrimSpace(reason) == "" {
		reason = "总脑规划链路异常，先由启发式调度选择最贴近的研究组接管"
	}
	return labDispatchPlan{
		GroupIDs:   groupIDs,
		GroupTasks: groupTasks,
		Reason:     reason,
	}
}

func (lm *LabManager) buildDispatchBrainNote(plan labDispatchPlan) string {
	groupNames := plannedGroupNames(lm.groups, plan.GroupIDs)
	if len(groupNames) == 0 {
		return fmt.Sprintf("总脑研判：%s。当前不新增研究组协作。", strings.TrimSpace(plan.Reason))
	}
	if len(groupNames) == 1 {
		return fmt.Sprintf("总脑研判：%s。接下来我会在决策者群协调 %s 的决策大脑。", strings.TrimSpace(plan.Reason), groupNames[0])
	}
	return fmt.Sprintf("总脑研判：%s。接下来我会并行协调 %s 的决策大脑。", strings.TrimSpace(plan.Reason), strings.Join(groupNames, "、"))
}

func (lm *LabManager) groupPlanningCatalog() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(lm.groupOrder))
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		out = append(out, map[string]interface{}{
			"group_id":       g.ID(),
			"group_name":     g.Name(),
			"group_kind":     g.Kind(),
			"decision_brain": g.DecisionBrainName(),
			"description":    groupPlanningDescription(groupID),
			"is_demo":        g.IsDemo(),
			"status":         g.Status(),
		})
	}
	return out
}

func (lm *LabManager) startGroups(groupIDs []string, async bool) {
	if len(normalizeGroupIDs(groupIDs)) == 0 {
		return
	}
	if err := lm.ensureSharedSandbox(); err != nil {
		lm.emitBrainMessage("共享 sandbox 初始化失败，当前无法启动研究组执行需要运行时环境的任务：" + err.Error())
		lm.notifyGroupsUpdated()
		return
	}
	for _, groupID := range normalizeGroupIDs(groupIDs) {
		g, ok := lm.groups[groupID]
		if !ok || g.IsRunning() {
			continue
		}
		if async {
			go g.StartTask()
			continue
		}
		g.StartTask()
	}
}

func (lm *LabManager) ensureSharedSandbox() error {
	if lm == nil || lm.webGroup == nil || lm.webGroup.pm == nil {
		return fmt.Errorf("shared project manager is unavailable")
	}
	if err := lm.webGroup.pm.EnsureSandbox(); err != nil {
		return err
	}
	if lm.binaryGroup != nil {
		lm.binaryGroup.mu.RLock()
		brain := lm.binaryGroup.brain
		lm.binaryGroup.mu.RUnlock()
		if brain != nil {
			brain.RefreshDynamicDigitalHumanPrompts()
		}
	}
	if lm.pentestGroup != nil {
		lm.pentestGroup.mu.RLock()
		brain := lm.pentestGroup.brain
		lm.pentestGroup.mu.RUnlock()
		if brain != nil {
			brain.RefreshDynamicDigitalHumanPrompts()
		}
	}
	return nil
}

func (lm *LabManager) handleGroupAssistanceRequest(requesterGroupID string, args map[string]interface{}) string {
	request := stringArg(args, "request")
	if request == "" {
		return toolCalling.Fail("request is required")
	}
	requester, ok := lm.groups[requesterGroupID]
	if !ok {
		return toolCalling.Fail("requester group not found")
	}
	lm.emitDecisionBrainMessage(requester, "向总脑申请跨组协作："+request)
	lm.emitBrainMessage(fmt.Sprintf("总脑收到来自%s的协作申请，正在评估是否需要调动其它研究组。", requester.Name()))
	plan, err := lm.planDispatchByTotalBrain(request, requester.DecisionBrainName(), requesterGroupID, true)
	if err != nil {
		lm.emitBrainMessage(fmt.Sprintf("总脑未能完成协作规划：%s", err.Error()))
		return toolCalling.Fail(err.Error())
	}
	if len(plan.GroupIDs) == 0 {
		lm.clearCollaborationSession(requesterGroupID)
		lm.syncTotalBrainRuntimeState()
		lm.setTotalBrainState(defaultBrainState, request, []string{requester.Name()}, plan.Reason)
		lm.emitBrainMessage(fmt.Sprintf("总脑研判：%s。当前暂不新增协作组，由%s继续推进。", plan.Reason, requester.Name()))
		lm.notifyGroupsUpdated()
		return toolCalling.Success("总脑已评估协作申请，暂不新增研究组")
	}
	lm.replaceCollaborationSession(requesterGroupID, plan.GroupIDs, request, plan.Reason)
	lm.syncTotalBrainRuntimeState()
	lm.dispatchPlannedTasks(plan, request, totalBrainName, lm.buildDispatchBrainNote(plan), true)
	return toolCalling.Success("总脑已受理跨组协作申请并下发任务")
}

func (lm *LabManager) handleDecisionMakersReport(groupID string, args map[string]interface{}) string {
	group, ok := lm.groups[groupID]
	if !ok {
		return toolCalling.Fail("group not found")
	}
	summary := stringArg(args, "summary")
	if summary == "" {
		return toolCalling.Fail("summary is required")
	}
	reportType := strings.ToLower(stringArg(args, "report_type"))
	details := stringArg(args, "details")
	nextStep := stringArg(args, "next_step")
	reportText := formatDecisionMakersReport(reportType, summary, details, nextStep)
	lm.emitDecisionBrainMessage(group, reportText)
	lm.emitGroupBrainCoordinationMessage(groupID, fmt.Sprintf("已向决策者群同步%s：%s", decisionReportLabel(reportType), summary))
	lm.setTotalBrainState("定向沟通中", summary, []string{group.Name()}, "处理研究组汇报")
	response, err := lm.coordinateGroupReportByTotalBrain(group, reportType, summary, details, nextStep)
	if err != nil {
		response = totalBrainReportResponse{
			VisibleMessage:        fmt.Sprintf("总脑已收到来自%s的汇报；自动协调异常，先按保守策略要求该组继续检查未闭环事项。", group.Name()),
			ReplyToReportingGroup: "继续依据当前任务检查是否仍有未闭环工作；如果这只是阶段性汇报，不要因为本次汇报而停止；只有在出现新的关键进展、阻塞或真正完成时再向总脑汇报。",
			CoordinationReason:    "总脑处理研究组汇报时发生异常，已切换为保守跟进策略",
		}
		lm.emitBrainMessage(fmt.Sprintf("总脑处理来自%s的汇报时出现异常，已回退到保守跟进策略：%s", group.Name(), err.Error()))
	}
	lm.emitBrainMessage(response.VisibleMessage)
	lm.emitTotalBrainDirective(group, response.ReplyToReportingGroup)
	lm.emitGroupChatMessage(groupID, DecisionBrain.ChatMessage{
		Role:        "system",
		Text:        response.ReplyToReportingGroup,
		Ts:          time.Now().Format("15:04:05"),
		PersonaName: totalBrainName,
		AvatarFile:  totalBrainAvatar,
	})
	group.NotifyDecisionBrainEvent(buildTotalBrainReportFollowUpPrompt(response.ReplyToReportingGroup))
	lm.wakeWaitingGroupsForCollaborationEvent(groupID, fmt.Sprintf("%s已向总脑提交%s汇报：%s", group.Name(), decisionReportLabel(reportType), summary))
	lm.syncTotalBrainRuntimeState()
	lm.setTotalBrainState(defaultBrainState, summary, []string{group.Name()}, response.CoordinationReason)
	lm.notifyGroupsUpdated()
	return toolCalling.Success("Update delivered to the decision-makers channel")
}

func (lm *LabManager) handleGroupReportUpdate(sourceGroupID string, report taskManager.ReportMeta) {
	sourceGroupID = strings.TrimSpace(sourceGroupID)
	sourceGroupName := sourceGroupID
	if group, ok := lm.groups[sourceGroupID]; ok && group != nil {
		sourceGroupName = group.Name()
	}
	alreadyNotified := make(map[string]struct{})
	for _, groupID := range lm.groupOrder {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" || groupID == sourceGroupID {
			continue
		}
		group, ok := lm.groups[groupID]
		if !ok || group == nil || !group.IsRunning() {
			continue
		}
		group.NotifySharedReportUpdate(report, sourceGroupID, sourceGroupName)
		alreadyNotified[groupID] = struct{}{}
	}
	lm.wakeWaitingGroupsForAnyReport(sourceGroupID, sourceGroupName, report, alreadyNotified)
	lm.syncTotalBrainRuntimeState()
	if lm.webGroup != nil && lm.webGroup.pm != nil {
		lm.notifyGroupsUpdated()
	}
}

func (lm *LabManager) resolveGroupByPersona(persona string) researchGroup {
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		if g.HasMember(persona) {
			return g
		}
	}
	return nil
}

func (lm *LabManager) resolveDecisionGroupByAlias(target string) researchGroup {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	switch {
	case isWebGroupAlias(target):
		return lm.groups[webGroupID]
	case isBinaryGroupAlias(target):
		return lm.groups[binaryGroupID]
	case isPentestGroupAlias(target):
		return lm.groups[pentestGroupID]
	}
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		if decisionBrainAliasMatches(g, target) {
			return g
		}
	}
	return nil
}

func (lm *LabManager) selectGroups(raw string) ([]string, string) {
	s := strings.ToLower(strings.TrimSpace(raw))
	binaryKeywords := []string{"binary", "bin", "elf", "pe", "mach-o", "pwn", "fuzz", "fuzzer", "heap", "栈溢出", "二进制", "逆向", "反汇编", "反编译", "内存破坏", "崩溃", "样本", "shellcode"}
	webKeywords := []string{"web", "http", "cookie", "jwt", "sql", "sqli", "xss", "csrf", "鉴权", "路由", "接口", "源码", "反序列化", "未授权", "rce", "ssti", "上传", "目录穿越"}
	pentestKeywords := []string{"pentest", "渗透", "渗透测试", "黑盒", "端口", "banner", "指纹", "目录扫描", "子域名", "爆破", "弱口令", "登录口", "服务暴露", "入口探测", "外网", "主机发现", "nmap", "masscan", "webshell", "shell", "beacon", "meterpreter", "远控", "后渗透", "横向", "持久化", "隧道"}
	hasBinary := containsAny(s, binaryKeywords)
	hasWeb := containsAny(s, webKeywords)
	hasPentest := containsAny(s, pentestKeywords)
	switch {
	case hasBinary && hasWeb && hasPentest:
		return []string{webGroupID, binaryGroupID, pentestGroupID}, "消息同时包含源码、二进制与黑盒渗透线索，适合三组并行研判"
	case hasBinary && hasPentest:
		return []string{binaryGroupID, pentestGroupID}, "消息同时涉及二进制与黑盒渗透 / 远控链路，适合两组协同"
	case hasWeb && hasPentest:
		return []string{webGroupID, pentestGroupID}, "消息同时涉及 Web 逻辑与黑盒渗透入口，适合两组协同"
	case hasBinary && hasWeb:
		return []string{webGroupID, binaryGroupID}, "消息同时包含 Web 与二进制线索，适合多组并行研判"
	case hasBinary:
		return []string{binaryGroupID}, "消息更偏向二进制样本、逆向分析或内存利用问题"
	case hasPentest:
		return []string{pentestGroupID}, "消息更偏向黑盒渗透、入口探测、远控或后渗透问题"
	default:
		return []string{webGroupID}, "消息与现有 Web 安全审计主能力最贴近"
	}
}

func (lm *LabManager) sendWS(msgType string, data interface{}) {
	if lm.msgChan == nil {
		return
	}
	msg := ProjectManager.WebMsg{
		Type:        msgType,
		Data:        data,
		ProjectName: lm.labID,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case lm.msgChan <- string(payload):
	case <-time.After(5 * time.Second):
		misc.Debug("LabManager.sendWS: dropped message for lab=%s type=%s", lm.labName, msgType)
	}
}

func (lm *LabManager) notifyGroupsUpdated() {
	lm.refreshLifecycleState()
	lm.sendWS("LabGroupsUpdate", lm.GetGroupsSnapshot())
	lm.sendWS("LabSharedResourcesUpdate", lm.GetSharedResourcesSnapshot())
}

func (lm *LabManager) setTotalBrainState(status string, dispatch string, groups []string, reason string) {
	lm.brainMu.Lock()
	defer lm.brainMu.Unlock()
	if strings.TrimSpace(status) != "" {
		lm.totalBrainStatus = status
	}
	if strings.TrimSpace(dispatch) != "" {
		lm.lastDispatch = dispatch
	}
	if len(groups) > 0 {
		lm.lastDispatchGroups = append([]string(nil), groups...)
	}
	if strings.TrimSpace(reason) != "" {
		lm.lastDispatchReason = reason
	}
}

func (lm *LabManager) channelName(channelID string) string {
	channelID = normalizeChannelID(channelID)
	if channelID == decisionMakersChannelID {
		return decisionMakersChanName
	}
	if groupID := channelGroupID(channelID); groupID != "" {
		return groupChannelName(lm.groupNameByID(groupID))
	}
	return decisionMakersChanName
}

func (lm *LabManager) groupNameByChannel(channelID string) string {
	if groupID := channelGroupID(channelID); groupID != "" {
		return lm.groupNameByID(groupID)
	}
	return ""
}

func (lm *LabManager) groupNameByID(groupID string) string {
	if g, ok := lm.groups[strings.TrimSpace(groupID)]; ok {
		return g.Name()
	}
	return ""
}

func (lm *LabManager) GetLabBrainSnapshot() map[string]interface{} {
	overview := lm.getProjectOverview()
	lm.brainMu.RLock()
	defer lm.brainMu.RUnlock()
	groups := append([]string(nil), lm.lastDispatchGroups...)
	return map[string]interface{}{
		"name":                 totalBrainName,
		"avatar_file":          totalBrainAvatar,
		"status":               lm.totalBrainStatus,
		"last_dispatch":        lm.lastDispatch,
		"last_dispatch_groups": groups,
		"last_dispatch_reason": lm.lastDispatchReason,
		"project_overview":     overview,
	}
}

func (lm *LabManager) GetGroupsSnapshot() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(lm.groupOrder))
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		out = append(out, g.Snapshot())
	}
	return out
}

func (lm *LabManager) GetGroupCount() int {
	if lm == nil {
		return 0
	}
	return len(lm.groupOrder)
}

func (lm *LabManager) GetGroupPrivateSnapshots() map[string]interface{} {
	out := make(map[string]interface{}, len(lm.groupOrder))
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		out[groupID] = g.PrivateSnapshot()
	}
	return out
}

func (lm *LabManager) GetGroupPrivateSnapshot(groupID string) (map[string]interface{}, bool) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, false
	}
	g, ok := lm.groups[groupID]
	if !ok || g == nil {
		return nil, false
	}
	return g.PrivateSnapshot(), true
}

func accumulateTokenUsage(dst *taskManager.TokenUsageSnapshot, groupName string, snapshot taskManager.TokenUsageSnapshot) {
	dst.PromptTokens += snapshot.PromptTokens
	dst.CompletionTokens += snapshot.CompletionTokens
	dst.TotalTokens += snapshot.TotalTokens
	for _, agent := range snapshot.Agents {
		label := strings.TrimSpace(agent.Label)
		if label == "" {
			label = strings.TrimSpace(groupName)
		} else if groupName != "" {
			label = groupName + " / " + label
		}
		dst.Agents = append(dst.Agents, taskManager.AgentTokenUsageSnapshot{
			Label:            label,
			PromptTokens:     agent.PromptTokens,
			CompletionTokens: agent.CompletionTokens,
			TotalTokens:      agent.TotalTokens,
		})
	}
}

func normalizeContextBreakdown(raw interface{}) DecisionBrain.ContextBreakdown {
	switch bd := raw.(type) {
	case DecisionBrain.ContextBreakdown:
		return bd
	case map[string]interface{}:
		out := DecisionBrain.ContextBreakdown{
			Sections:   make([]DecisionBrain.ContextBreakdownSection, 0),
			Total:      intFromValue(bd["total"]),
			MaxContext: intFromValue(bd["max_context"]),
		}
		sections, _ := bd["sections"].([]interface{})
		for _, item := range sections {
			sectionMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			out.Sections = append(out.Sections, DecisionBrain.ContextBreakdownSection{
				Name:   strings.TrimSpace(fmt.Sprintf("%v", sectionMap["name"])),
				Tokens: intFromValue(sectionMap["tokens"]),
			})
		}
		return out
	default:
		return DecisionBrain.ContextBreakdown{}
	}
}

func intFromValue(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}

func (lm *LabManager) groupTokenUsageSnapshot(groupID string) taskManager.TokenUsageSnapshot {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return lm.webGroup.pm.GetTokenUsageSnapshot()
	case binaryGroupID:
		return lm.binaryGroup.GetTokenUsageSnapshot()
	case pentestGroupID:
		return lm.pentestGroup.GetTokenUsageSnapshot()
	default:
		return taskManager.TokenUsageSnapshot{}
	}
}

func (lm *LabManager) groupContextBreakdown(groupID string) DecisionBrain.ContextBreakdown {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return normalizeContextBreakdown(lm.webGroup.pm.GetContextBreakdown())
	case binaryGroupID:
		return normalizeContextBreakdown(lm.binaryGroup.GetContextBreakdown())
	case pentestGroupID:
		return normalizeContextBreakdown(lm.pentestGroup.GetContextBreakdown())
	default:
		return DecisionBrain.ContextBreakdown{}
	}
}

func (lm *LabManager) groupMemoryDebugTargets(groupID string) []taskManager.MemoryDebugTarget {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return lm.webGroup.ListMemoryDebugTargets()
	case binaryGroupID:
		return lm.binaryGroup.ListMemoryDebugTargets()
	case pentestGroupID:
		return lm.pentestGroup.ListMemoryDebugTargets()
	default:
		return nil
	}
}

func (lm *LabManager) groupPersistedState(groupID string) (taskManager.GroupState, bool) {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return lm.webGroupStateSnapshot(), true
	case binaryGroupID:
		state := lm.binaryGroup.PersistedState()
		return state, state.GroupID != ""
	case pentestGroupID:
		state := lm.pentestGroup.PersistedState()
		return state, state.GroupID != ""
	default:
		return taskManager.GroupState{}, false
	}
}

func (lm *LabManager) GetTokenUsage() map[string]interface{} {
	snapshot := lm.GetTokenUsageSnapshot()
	sort.SliceStable(snapshot.Agents, func(i, j int) bool {
		if snapshot.Agents[i].TotalTokens == snapshot.Agents[j].TotalTokens {
			return snapshot.Agents[i].Label < snapshot.Agents[j].Label
		}
		return snapshot.Agents[i].TotalTokens > snapshot.Agents[j].TotalTokens
	})
	return map[string]interface{}{
		"prompt_tokens":     snapshot.PromptTokens,
		"completion_tokens": snapshot.CompletionTokens,
		"total_tokens":      snapshot.TotalTokens,
		"agents":            snapshot.Agents,
	}
}

func (lm *LabManager) GetTokenUsageSnapshot() taskManager.TokenUsageSnapshot {
	out := taskManager.TokenUsageSnapshot{
		Agents: make([]taskManager.AgentTokenUsageSnapshot, 0),
	}
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		accumulateTokenUsage(&out, g.Name(), lm.groupTokenUsageSnapshot(groupID))
	}
	return out
}

func (lm *LabManager) RestoreTokenUsageSnapshot(snapshot taskManager.TokenUsageSnapshot) {
	lm.webGroup.pm.RestoreTokenUsageSnapshot(snapshot)
}

func (lm *LabManager) GetContextBreakdown() interface{} {
	sections := make([]DecisionBrain.ContextBreakdownSection, 0)
	total := 0
	maxContext := 0
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		bd := lm.groupContextBreakdown(groupID)
		total += bd.Total
		maxContext += bd.MaxContext
		for _, section := range bd.Sections {
			name := strings.TrimSpace(section.Name)
			if g.Name() != "" {
				name = g.Name() + " / " + name
			}
			sections = append(sections, DecisionBrain.ContextBreakdownSection{
				Name:   name,
				Tokens: section.Tokens,
			})
		}
	}
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Tokens == sections[j].Tokens {
			return sections[i].Name < sections[j].Name
		}
		return sections[i].Tokens > sections[j].Tokens
	})
	return DecisionBrain.ContextBreakdown{
		Sections:   sections,
		Total:      total,
		MaxContext: maxContext,
	}
}

func (lm *LabManager) SetEnvInfo(env map[string]interface{}) {
	lm.webGroup.pm.SetEnvInfo(env)
}

func (lm *LabManager) SetContainer(containerList []taskManager.ContainerInfo) {
	lm.webGroup.pm.SetContainer(containerList)
}

func (lm *LabManager) SetReport(reportList map[string]string) {
	if len(reportList) == 0 {
		lm.webGroup.pm.RestoreReportCatalog(nil)
		lm.binaryGroup.RestorePersistedState(taskManager.GroupState{GroupID: binaryGroupID, GroupKind: binaryGroupKind, Status: lm.binaryGroup.Status()})
		lm.pentestGroup.RestorePersistedState(taskManager.GroupState{GroupID: pentestGroupID, GroupKind: pentestGroupKind, Status: lm.pentestGroup.Status()})
		lm.restoreLabReportCatalog(nil)
		return
	}
	legacy := lm.legacyGroupStates(taskManager.ProjectInfo{ReportList: reportList})
	lm.restoreLabReportCatalog(legacyLabReportCatalog(taskManager.ProjectInfo{ReportList: reportList}))
	if state, ok := legacy[webGroupID]; ok {
		lm.webGroup.pm.RestoreReportCatalog(state.ReportCatalog)
	}
	if state, ok := legacy[binaryGroupID]; ok {
		lm.binaryGroup.RestorePersistedState(state)
	}
	if state, ok := legacy[pentestGroupID]; ok {
		lm.pentestGroup.RestorePersistedState(state)
	}
}

func (lm *LabManager) RemoveDockerAll() {
	lm.webGroup.pm.RemoveDockerAll()
}

func (lm *LabManager) SetSandboxContainerID(containerID string) {
	lm.webGroup.pm.SetSandboxContainerID(containerID)
}

func (lm *LabManager) GetSandboxContainerID() string {
	return lm.webGroup.pm.GetSandboxContainerID()
}

func (lm *LabManager) GetSharedResourcesSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"workDir":         lm.GetWorkDir(),
		"sourceCodeDir":   lm.GetSourceCodeDir(),
		"projectDir":      lm.GetProjectDir(),
		"sandboxID":       lm.GetSandboxContainerID(),
		"projectOverview": lm.getProjectOverview(),
		"reportCatalog":   lm.GetSharedReportCatalogSnapshot(),
	}
}

func (lm *LabManager) handleHostCommandRequest(ctx context.Context, task *taskManager.Task, req taskManager.HostCommandRequest) taskManager.HostCommandResult {
	if lm == nil || lm.hostCmdApprovals == nil {
		return taskManager.HostCommandResult{Status: "failed", Error: "decision approval manager is unavailable"}
	}
	actor := taskManager.TaskActor{}
	if task != nil {
		actor = task.GetTaskActor()
	}
	return lm.hostCmdApprovals.Request(ctx, actor, req)
}

func (lm *LabManager) handleDecisionApprovalRequest(ctx context.Context, task *taskManager.Task, req taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult {
	if lm == nil || lm.hostCmdApprovals == nil {
		return taskManager.DecisionApprovalResult{Status: "failed", Error: "decision approval manager is unavailable"}
	}
	actor := taskManager.TaskActor{}
	if task != nil {
		actor = task.GetTaskActor()
	}
	return lm.hostCmdApprovals.RequestDecision(ctx, actor, req)
}

func (lm *LabManager) GetHostCommandApprovalSnapshot() []hostCommandApprovalRecord {
	if lm == nil || lm.hostCmdApprovals == nil {
		return []hostCommandApprovalRecord{}
	}
	return lm.hostCmdApprovals.Snapshot()
}

func (lm *LabManager) DecideHostCommandApproval(id string, approved bool, note string, reviewer string) error {
	if lm == nil || lm.hostCmdApprovals == nil {
		return fmt.Errorf("decision approval manager is unavailable")
	}
	return lm.hostCmdApprovals.Decide(id, approved, note, reviewer)
}

func (lm *LabManager) GetBackgroundProcessSnapshot() []taskManager.BackgroundProcessSnapshot {
	if lm == nil {
		return []taskManager.BackgroundProcessSnapshot{}
	}
	return taskManager.ListActiveBackgroundProcesses(lm.labID)
}

func (lm *LabManager) KillBackgroundProcess(sessionID string) (taskManager.BackgroundProcessSnapshot, error) {
	if lm == nil {
		return taskManager.BackgroundProcessSnapshot{}, fmt.Errorf("lab manager is unavailable")
	}
	return taskManager.TerminateBackgroundProcess(lm.labID, sessionID, "", "user", "killed by user from UI", "TERM")
}

func (lm *LabManager) GetMemoryDebugTargets() []taskManager.MemoryDebugTarget {
	if lm == nil {
		return []taskManager.MemoryDebugTarget{}
	}
	targets := make([]taskManager.MemoryDebugTarget, 0)
	targets = append(targets, lm.totalBrainMemoryDebugTarget())
	for _, groupID := range lm.groupOrder {
		targets = append(targets, lm.groupMemoryDebugTargets(groupID)...)
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].GroupName != targets[j].GroupName {
			return targets[i].GroupName < targets[j].GroupName
		}
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

func (lm *LabManager) ExportMemoryDebug(targetID string) (*taskManager.MemoryDebugExport, error) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, fmt.Errorf("target id is required")
	}
	switch {
	case targetID == totalBrainMemoryDebugTargetID():
		return lm.exportTotalBrainMemoryDebug(targetID)
	case strings.HasPrefix(targetID, webGroupID+"::"):
		return lm.webGroup.ExportMemoryDebug(targetID)
	case strings.HasPrefix(targetID, binaryGroupID+"::"):
		return lm.binaryGroup.ExportMemoryDebug(targetID)
	case strings.HasPrefix(targetID, pentestGroupID+"::"):
		return lm.pentestGroup.ExportMemoryDebug(targetID)
	default:
		return nil, fmt.Errorf("unsupported debug memory target: %s", targetID)
	}
}

func (lm *LabManager) persistedLabBrainSnapshot() taskManager.LabBrainSnapshot {
	overview := lm.getProjectOverview()
	lm.brainMu.RLock()
	defer lm.brainMu.RUnlock()
	return taskManager.LabBrainSnapshot{
		Status:             strings.TrimSpace(lm.totalBrainStatus),
		LastDispatch:       strings.TrimSpace(lm.lastDispatch),
		LastDispatchGroups: append([]string(nil), lm.lastDispatchGroups...),
		LastDispatchReason: strings.TrimSpace(lm.lastDispatchReason),
		ProjectOverview:    overview,
	}
}

func (lm *LabManager) webGroupStateSnapshot() taskManager.GroupState {
	return taskManager.GroupState{
		GroupID:       webGroupID,
		GroupKind:     webGroupKind,
		Status:        lm.webGroup.Status(),
		BrainFinished: lm.webGroup.BrainFinished(),
		ContainerList: lm.webGroup.pm.GetContainerList(),
		VulnList:      lm.webGroup.pm.GetVulnList(),
		ExploitIdeas:  lm.webGroup.pm.GetExploitIdeaList(),
		ExploitChains: lm.webGroup.pm.GetExploitChainList(),
		TokenUsage:    lm.webGroup.pm.GetTokenUsageSnapshot(),
		EventList:     lm.webGroup.pm.GetEvent(maxPersistLabEvents),
		EnvInfo:       lm.webGroup.pm.GetEnvInfo(),
		ReportCatalog: lm.webGroup.pm.GetReportCatalog(),
	}
}

func (lm *LabManager) groupStateSnapshots() map[string]taskManager.GroupState {
	out := make(map[string]taskManager.GroupState, len(lm.groupOrder))
	for _, groupID := range lm.groupOrder {
		if state, ok := lm.groupPersistedState(groupID); ok {
			out[groupID] = state
		}
	}
	return out
}

func (lm *LabManager) ExportProjectInfo() taskManager.ProjectInfo {
	lm.refreshLifecycleState()
	return taskManager.ProjectInfo{
		ProjectID:           lm.GetProjectID(),
		ProjectUUID:         lm.GetProjectID(),
		ProjectName:         lm.GetProjectName(),
		WorkDir:             lm.GetWorkDir(),
		SourceCodeDir:       lm.GetSourceCodeDir(),
		SkipProjectOverview: lm.webGroup != nil && lm.webGroup.pm != nil && lm.webGroup.pm.ShouldSkipProjectOverview(),
		TaskContent:         lm.GetTaskContent(),
		SandboxID:           lm.GetSandboxContainerID(),
		StartTime:           lm.GetStartTime(),
		EndTime:             lm.GetEndTime(),
		LabStatus:           lm.GetStatus(),
		LabBrain:            lm.persistedLabBrainSnapshot(),
		GroupStates:         lm.groupStateSnapshots(),
		ContainerList:       lm.GetContainerList(),
		VulnList:            lm.GetVulnList(),
		ExploitIdeas:        lm.GetExploitIdeaList(),
		ExploitChains:       lm.GetExploitChainList(),
		TokenUsage:          lm.GetTokenUsageSnapshot(),
		EventList:           lm.GetEvent(maxPersistLabEvents),
		EnvInfo:             lm.webGroup.pm.GetEnvInfo(),
		ProjectDir:          lm.GetProjectDir(),
		ReportList:          lm.GetReportList(),
		LabReportCatalog:    lm.getLabReportCatalogSnapshot(),
	}
}

func sanitizeRestoredGroupStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", "未运行", "项目速览中", "调度中", "定向沟通中", "组内研判中", "正在运行", "决策结束", "运行结束":
		return "待命"
	default:
		return strings.TrimSpace(status)
	}
}

func sanitizeRestoredTotalBrainStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", "项目速览中", "调度中", "定向沟通中", "正在运行", "运行结束", "决策结束":
		return defaultBrainState
	default:
		return strings.TrimSpace(status)
	}
}

func (lm *LabManager) legacyGroupStates(info taskManager.ProjectInfo) map[string]taskManager.GroupState {
	webState := taskManager.GroupState{
		GroupID:       webGroupID,
		GroupKind:     webGroupKind,
		Status:        "待命",
		BrainFinished: strings.TrimSpace(info.EndTime) != "",
		ContainerList: info.ContainerList,
		VulnList:      info.VulnList,
		ExploitIdeas:  info.ExploitIdeas,
		ExploitChains: info.ExploitChains,
		TokenUsage:    info.TokenUsage,
		EventList:     info.EventList,
		EnvInfo:       info.EnvInfo,
	}
	binaryState := taskManager.GroupState{
		GroupID:     binaryGroupID,
		GroupKind:   binaryGroupKind,
		Status:      "待命",
		LastTask:    "",
		LastSummary: "",
	}
	pentestState := taskManager.GroupState{
		GroupID:     pentestGroupID,
		GroupKind:   pentestGroupKind,
		Status:      "待命",
		LastTask:    "",
		LastSummary: "",
	}
	prefixedReportKeys := make(map[string]struct{})
	for key, path := range info.ReportList {
		key = strings.TrimSpace(key)
		path = strings.TrimSpace(path)
		if key == "" || path == "" {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 && parts[0] == labControlGroupID {
			continue
		}
		if len(parts) == 2 && (parts[0] == webGroupID || parts[0] == binaryGroupID || parts[0] == pentestGroupID) {
			prefixedReportKeys[parts[1]+"\x00"+path] = struct{}{}
		}
	}
	appendReport := func(state taskManager.GroupState, reportID string, path string) taskManager.GroupState {
		reportID = strings.TrimSpace(reportID)
		if reportID == "" {
			reportID = filepath.Base(path)
		}
		for _, existing := range state.ReportCatalog {
			if strings.TrimSpace(existing.ReportID) == reportID && strings.TrimSpace(existing.Path) == path {
				return state
			}
		}
		state.ReportCatalog = append(state.ReportCatalog, taskManager.ReportMeta{
			ReportID: reportID,
			Path:     path,
			FileName: filepath.Base(path),
		})
		return state
	}
	for key, path := range info.ReportList {
		key = strings.TrimSpace(key)
		path = strings.TrimSpace(path)
		if key == "" || path == "" {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 && parts[0] == labControlGroupID {
			continue
		}
		if len(parts) == 2 && (parts[0] == webGroupID || parts[0] == binaryGroupID || parts[0] == pentestGroupID) {
			switch parts[0] {
			case webGroupID:
				webState = appendReport(webState, parts[1], path)
				webState.BrainFinished = true
			case binaryGroupID:
				binaryState = appendReport(binaryState, parts[1], path)
				binaryState.BrainFinished = true
			case pentestGroupID:
				pentestState = appendReport(pentestState, parts[1], path)
				pentestState.BrainFinished = true
			}
			continue
		}
		if _, exists := prefixedReportKeys[key+"\x00"+path]; exists {
			continue
		}
		webState = appendReport(webState, key, path)
		if strings.TrimSpace(info.EndTime) != "" {
			webState.BrainFinished = true
		}
	}
	out := map[string]taskManager.GroupState{
		webGroupID: webState,
	}
	if len(binaryState.ReportCatalog) > 0 || binaryState.BrainFinished || binaryState.LastTask != "" || binaryState.LastSummary != "" || binaryState.TokenUsage.TotalTokens > 0 {
		out[binaryGroupID] = binaryState
	}
	if len(pentestState.ReportCatalog) > 0 || pentestState.BrainFinished || pentestState.LastTask != "" || pentestState.LastSummary != "" || pentestState.TokenUsage.TotalTokens > 0 {
		out[pentestGroupID] = pentestState
	}
	return out
}

func (lm *LabManager) restoreWebGroupState(state taskManager.GroupState) {
	state.Status = sanitizeRestoredGroupStatus(state.Status)
	lm.webGroup.pm.SetStatus(state.Status)
	lm.webGroup.pm.SetVulns(state.VulnList)
	lm.webGroup.pm.SetEvent(state.EventList)
	lm.webGroup.pm.SetContainer(state.ContainerList)
	lm.webGroup.pm.SetEnvInfo(state.EnvInfo)
	lm.webGroup.pm.RestoreExploitState(state.ExploitIdeas, state.ExploitChains)
	lm.webGroup.pm.RestoreTokenUsageSnapshot(state.TokenUsage)
	lm.webGroup.pm.RestoreReportCatalog(state.ReportCatalog)
	lm.webGroup.pm.RestoreBrainFinished(state.BrainFinished)
}

func (lm *LabManager) RestoreProjectInfo(info taskManager.ProjectInfo) {
	if lm.webGroup != nil && lm.webGroup.pm != nil {
		lm.webGroup.pm.SetSkipProjectOverview(info.SkipProjectOverview)
	}
	lm.SetEnvInfo(info.EnvInfo)
	projectDir := strings.TrimSpace(info.ProjectDir)
	switch {
	case projectDir != "":
	case strings.TrimSpace(info.WorkDir) != "":
		projectDir = filepath.Dir(strings.TrimSpace(info.WorkDir))
	case strings.TrimSpace(info.SourceCodeDir) != "":
		sourceDir := filepath.Clean(strings.TrimSpace(info.SourceCodeDir))
		if strings.HasSuffix(filepath.ToSlash(sourceDir), "/"+misc.ProjectWorkDirName+"/"+misc.ProjectSourceDirName) {
			projectDir = filepath.Dir(filepath.Dir(sourceDir))
		} else {
			projectDir = filepath.Dir(sourceDir)
		}
	}
	if projectDir != "" {
		lm.SetProjectDir(projectDir)
	}
	lm.SetContainer(info.ContainerList)
	lm.SetVulns(info.VulnList)
	lm.SetSandboxContainerID(info.SandboxID)
	lm.SetStartTime(info.StartTime)
	lm.SetEndTime(info.EndTime)
	lm.restoreLabReportCatalog(info.LabReportCatalog)
	if len(info.LabReportCatalog) == 0 {
		lm.restoreLabReportCatalog(legacyLabReportCatalog(info))
	}

	labBrain := info.LabBrain
	if restoredOverview := strings.TrimSpace(labBrain.ProjectOverview); restoredOverview != "" {
		if !(isNoSourceProjectOverview(restoredOverview) && lm.webGroup != nil && lm.webGroup.pm != nil && !lm.webGroup.pm.ShouldSkipProjectOverview()) {
			lm.setProjectOverview(restoredOverview)
		}
	}
	lm.brainMu.Lock()
	lm.totalBrainStatus = sanitizeRestoredTotalBrainStatus(labBrain.Status)
	lm.lastDispatch = strings.TrimSpace(labBrain.LastDispatch)
	lm.lastDispatchGroups = append([]string(nil), labBrain.LastDispatchGroups...)
	lm.lastDispatchReason = strings.TrimSpace(labBrain.LastDispatchReason)
	lm.brainMu.Unlock()

	groupStates := info.GroupStates
	if len(groupStates) == 0 {
		groupStates = lm.legacyGroupStates(info)
	}
	if webState, ok := groupStates[webGroupID]; ok {
		lm.restoreWebGroupState(webState)
	}
	if binaryState, ok := groupStates[binaryGroupID]; ok {
		binaryState.Status = sanitizeRestoredGroupStatus(binaryState.Status)
		lm.binaryGroup.RestorePersistedState(binaryState)
	}
	if pentestState, ok := groupStates[pentestGroupID]; ok {
		pentestState.Status = sanitizeRestoredGroupStatus(pentestState.Status)
		lm.pentestGroup.RestorePersistedState(pentestState)
	}

	status := strings.TrimSpace(info.LabStatus)
	if status == "" {
		switch {
		case strings.TrimSpace(info.EndTime) != "":
			status = "运行结束"
		case strings.TrimSpace(info.StartTime) != "":
			status = "待命"
		default:
			status = "未运行"
		}
	}
	lm.SetStatus(status)
	lm.refreshLifecycleState()
}

func containsAny(s string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(s, keyword) {
			return true
		}
	}
	return false
}

func normalizeGroupIDs(groupIDs []string) []string {
	seen := make(map[string]struct{}, len(groupIDs))
	out := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		out = append(out, groupID)
	}
	sort.Strings(out)
	return out
}

func plannedGroupNames(groups map[string]researchGroup, groupIDs []string) []string {
	out := make([]string, 0, len(groupIDs))
	for _, groupID := range normalizeGroupIDs(groupIDs) {
		g, ok := groups[groupID]
		if !ok {
			continue
		}
		out = append(out, g.Name())
	}
	return out
}

func containsGroupID(groupIDs []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, groupID := range groupIDs {
		if strings.TrimSpace(groupID) == target {
			return true
		}
	}
	return false
}

func (lm *LabManager) GetSharedReportCatalogSnapshot() []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, report := range lm.listSharedReports() {
		out = append(out, map[string]interface{}{
			"reportId":   report.Meta.ReportID,
			"downloadId": report.DownloadID,
			"targetId":   report.Meta.TargetID,
			"summary":    report.Meta.Summary,
			"createdAt":  report.Meta.CreatedAt,
			"fileName":   report.Meta.FileName,
			"groupId":    report.GroupID,
			"groupName":  report.GroupName,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := strings.TrimSpace(fmt.Sprintf("%v", out[i]["createdAt"]))
		right := strings.TrimSpace(fmt.Sprintf("%v", out[j]["createdAt"]))
		if left == right {
			return strings.TrimSpace(fmt.Sprintf("%v", out[i]["reportId"])) < strings.TrimSpace(fmt.Sprintf("%v", out[j]["reportId"]))
		}
		return left > right
	})
	return out
}

func (lm *LabManager) listSharedReports() []sharedReportEntry {
	out := make([]sharedReportEntry, 0)
	for _, groupID := range lm.groupOrder {
		g, ok := lm.groups[groupID]
		if !ok {
			continue
		}
		reports := g.SharedReportCatalog()
		for _, report := range reports {
			reportID := strings.TrimSpace(report.ReportID)
			if reportID == "" {
				continue
			}
			out = append(out, sharedReportEntry{
				DownloadID: sharedReportDownloadID(groupID, reportID),
				GroupID:    g.ID(),
				GroupName:  g.Name(),
				Meta:       report,
			})
		}
	}
	for _, report := range lm.getLabReportCatalogSnapshot() {
		reportID := strings.TrimSpace(report.ReportID)
		if reportID == "" {
			continue
		}
		out = append(out, sharedReportEntry{
			DownloadID: sharedReportDownloadID(labControlGroupID, reportID),
			GroupID:    labControlGroupID,
			GroupName:  labControlGroupName,
			Meta:       report,
		})
	}
	return out
}

func sharedReportDownloadID(groupID string, reportID string) string {
	groupID = strings.TrimSpace(groupID)
	reportID = strings.TrimSpace(reportID)
	if groupID == "" {
		return reportID
	}
	return groupID + ":" + reportID
}

func (lm *LabManager) ResolveSharedReportPath(id string) (string, string, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", false
	}
	reports := lm.listSharedReports()
	var fallback *sharedReportEntry
	for _, report := range reports {
		path := strings.TrimSpace(report.Meta.Path)
		if path == "" {
			continue
		}
		if report.DownloadID == id {
			return path, strings.TrimSpace(report.Meta.FileName), true
		}
		if strings.TrimSpace(report.Meta.ReportID) == id {
			if fallback != nil {
				return "", "", false
			}
			copyReport := report
			fallback = &copyReport
		}
	}
	if fallback == nil {
		return "", "", false
	}
	return strings.TrimSpace(fallback.Meta.Path), strings.TrimSpace(fallback.Meta.FileName), true
}

func (lm *LabManager) SharedReportEntries() []sharedReportEntry {
	return lm.listSharedReports()
}

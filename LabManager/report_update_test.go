package LabManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/ProjectManager"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"fmt"
	"strings"
	"testing"
)

type fakeReportAwareGroup struct {
	id            string
	name          string
	kind          string
	running       bool
	finished      bool
	reports       []taskManager.ReportMeta
	notified      []taskManager.ReportMeta
	events        []string
	decisionChats []string
	starts        int
	snapshots     int
}

func (g *fakeReportAwareGroup) ID() string                    { return g.id }
func (g *fakeReportAwareGroup) Name() string                  { return g.name }
func (g *fakeReportAwareGroup) Kind() string                  { return g.kind }
func (g *fakeReportAwareGroup) DecisionBrainName() string     { return g.name + "·决策大脑" }
func (g *fakeReportAwareGroup) Status() string                { return "待命" }
func (g *fakeReportAwareGroup) IsRunning() bool               { return g.running }
func (g *fakeReportAwareGroup) BrainFinished() bool           { return g.finished }
func (g *fakeReportAwareGroup) IsDemo() bool                  { return false }
func (g *fakeReportAwareGroup) HasMember(persona string) bool { return false }
func (g *fakeReportAwareGroup) HandleDecisionChat(raw string, sender string) string {
	g.decisionChats = append(g.decisionChats, fmt.Sprintf("%s:%s", strings.TrimSpace(sender), strings.TrimSpace(raw)))
	return ""
}
func (g *fakeReportAwareGroup) HandleGroupChat(raw string, sender string) string { return "" }
func (g *fakeReportAwareGroup) NotifyDecisionBrainEvent(content string) {
	g.events = append(g.events, content)
}
func (g *fakeReportAwareGroup) SyncGlobalStatusPanel(panel map[string]interface{}) {}
func (g *fakeReportAwareGroup) NormalizeGroupChatMessage(msg DecisionBrain.ChatMessage) DecisionBrain.ChatMessage {
	return msg
}
func (g *fakeReportAwareGroup) StartTask() {
	g.starts++
	g.running = true
}
func (g *fakeReportAwareGroup) StopTask() {}
func (g *fakeReportAwareGroup) SharedReportCatalog() []taskManager.ReportMeta {
	return append([]taskManager.ReportMeta(nil), g.reports...)
}
func (g *fakeReportAwareGroup) NotifySharedReportUpdate(report taskManager.ReportMeta, sourceGroupID string, sourceGroupName string) {
	g.notified = append(g.notified, report)
}
func (g *fakeReportAwareGroup) PrivateSnapshot() map[string]interface{}       { return nil }
func (g *fakeReportAwareGroup) GetDigitalHumanRoster() map[string]interface{} { return nil }
func (g *fakeReportAwareGroup) GetAgentRuntimeList() []map[string]interface{} { return nil }
func (g *fakeReportAwareGroup) Snapshot() map[string]interface{} {
	g.snapshots++
	return map[string]interface{}{"description": g.name}
}

func TestHandleGroupReportUpdateOnlyNotifiesOtherRunningGroups(t *testing.T) {
	source := &fakeReportAwareGroup{id: webGroupID, name: webGroupName, kind: webGroupKind, running: true}
	runningTarget := &fakeReportAwareGroup{id: binaryGroupID, name: binaryGroupName, kind: binaryGroupKind, running: true}
	idleTarget := &fakeReportAwareGroup{id: pentestGroupID, name: pentestGroupName, kind: pentestGroupKind, running: false}

	lm := &LabManager{
		groups: map[string]researchGroup{
			source.ID():        source,
			runningTarget.ID(): runningTarget,
			idleTarget.ID():    idleTarget,
		},
		groupOrder: []string{source.ID(), runningTarget.ID(), idleTarget.ID()},
	}

	report := taskManager.ReportMeta{
		ReportID: "R.7",
		FileName: "new-findings.md",
		Summary:  "新增报告摘要",
	}
	lm.handleGroupReportUpdate(source.ID(), report)

	if len(source.notified) != 0 {
		t.Fatalf("source group should not be notified again, got %d", len(source.notified))
	}
	if len(runningTarget.notified) != 1 {
		t.Fatalf("expected running target to receive 1 notification, got %d", len(runningTarget.notified))
	}
	if len(idleTarget.notified) != 0 {
		t.Fatalf("idle target should not be notified, got %d", len(idleTarget.notified))
	}
	if runningTarget.notified[0].FileName != "new-findings.md" {
		t.Fatalf("unexpected notified report: %+v", runningTarget.notified[0])
	}
}

func TestListSharedReportsIncludesLabGeneratedReports(t *testing.T) {
	web := &fakeReportAwareGroup{
		id:      webGroupID,
		name:    webGroupName,
		kind:    webGroupKind,
		reports: []taskManager.ReportMeta{{ReportID: "R.1", Path: "/tmp/web-report.md", FileName: "web-report.md"}},
	}
	lm := &LabManager{
		groups: map[string]researchGroup{
			web.ID(): web,
		},
		groupOrder: []string{web.ID()},
		labReportCatalog: []taskManager.ReportMeta{
			{ReportID: "L.1", Path: "/tmp/lab-summary.md", FileName: "lab-summary.md", Summary: "实验室总结"},
		},
	}

	reports := lm.listSharedReports()
	if len(reports) != 2 {
		t.Fatalf("expected 2 shared reports, got %d", len(reports))
	}
	if reports[1].GroupID != labControlGroupID {
		t.Fatalf("expected lab report group id %q, got %q", labControlGroupID, reports[1].GroupID)
	}
	if reports[1].GroupName != labControlGroupName {
		t.Fatalf("expected lab report group name %q, got %q", labControlGroupName, reports[1].GroupName)
	}
	if reports[1].DownloadID != sharedReportDownloadID(labControlGroupID, "L.1") {
		t.Fatalf("unexpected lab report download id: %s", reports[1].DownloadID)
	}
}

func TestBuildTotalBrainReportFollowUpPromptSuppressesImmediateAckLoop(t *testing.T) {
	got := buildTotalBrainReportFollowUpPrompt("继续核实当前阶段是否已经真正闭环。")
	if !strings.Contains(got, "not as a separate request that requires an immediate acknowledgment report") {
		t.Fatalf("expected follow-up prompt to suppress immediate ack loop, got: %s", got)
	}
	if !strings.Contains(got, "Only call Tool-ReportToDecisionMakers again when there is meaningful new progress") {
		t.Fatalf("expected follow-up prompt to describe when to report again, got: %s", got)
	}
}

func TestParseTotalBrainReportResponseArgs(t *testing.T) {
	lm := &LabManager{}
	result, err := lm.parseTotalBrainReportResponseArgs(`{"visible_message":"总脑已收到该组汇报。","reply_to_reporting_group":"继续沿当前证据链推进。","coordination_reason":"当前任务尚未闭环。"} `)
	if err != nil {
		t.Fatalf("parseTotalBrainReportResponseArgs returned error: %v", err)
	}
	if result.VisibleMessage != "总脑已收到该组汇报。" {
		t.Fatalf("unexpected visible message: %q", result.VisibleMessage)
	}
	if result.ReplyToReportingGroup != "继续沿当前证据链推进。" {
		t.Fatalf("unexpected reporting-group reply: %q", result.ReplyToReportingGroup)
	}
	if result.CoordinationReason != "当前任务尚未闭环。" {
		t.Fatalf("unexpected coordination reason: %q", result.CoordinationReason)
	}
}

func TestBuildTotalBrainReportFollowUpPromptMentionsRequestLabAssistanceBoundary(t *testing.T) {
	got := buildTotalBrainReportFollowUpPrompt("若需要别组协作，请在准备好材料后再申请。")
	if !strings.Contains(got, "Tool-RequestLabAssistance") {
		t.Fatalf("expected follow-up prompt to mention Tool-RequestLabAssistance boundary, got: %s", got)
	}
	if !strings.Contains(got, "/workdir/sourceCodeDir") {
		t.Fatalf("expected follow-up prompt to mention /workdir/sourceCodeDir artifact boundary, got: %s", got)
	}
}

func TestWebExploitAutomationPolicyUsesWebResearchMode(t *testing.T) {
	misc.ClearRuntimeWebResearchMode()
	defer misc.ClearRuntimeWebResearchMode()

	lm := &LabManager{}

	if err := misc.SetRuntimeWebResearchMode(misc.WebResearchModeComprehensive); err != nil {
		t.Fatalf("SetRuntimeWebResearchMode(comprehensive) error: %v", err)
	}
	policy := lm.webExploitAutomationPolicy()
	if policy.AutoVerifyExploits {
		t.Fatal("expected auto verification to be disabled in comprehensive mode")
	}
	if policy.AutoPublishAnalyzeReport {
		t.Fatal("expected auto analyzer report publication to be disabled in comprehensive mode")
	}

	if err := misc.SetRuntimeWebResearchMode(misc.WebResearchModeAudit); err != nil {
		t.Fatalf("SetRuntimeWebResearchMode(audit) error: %v", err)
	}
	policy = lm.webExploitAutomationPolicy()
	if !policy.AutoVerifyExploits {
		t.Fatal("expected auto verification to be enabled in audit mode")
	}
	if !policy.AutoPublishAnalyzeReport {
		t.Fatal("expected auto analyzer report publication to be enabled in audit mode")
	}
}

func TestGroupPlanningCatalogUsesStaticDescriptionsWithoutSnapshot(t *testing.T) {
	web := &fakeReportAwareGroup{id: webGroupID, name: webGroupName, kind: webGroupKind}
	binary := &fakeReportAwareGroup{id: binaryGroupID, name: binaryGroupName, kind: binaryGroupKind}
	pentest := &fakeReportAwareGroup{id: pentestGroupID, name: pentestGroupName, kind: pentestGroupKind}

	lm := &LabManager{
		groups: map[string]researchGroup{
			web.ID():     web,
			binary.ID():  binary,
			pentest.ID(): pentest,
		},
		groupOrder: []string{web.ID(), binary.ID(), pentest.ID()},
	}

	got := lm.groupPlanningCatalog()
	if len(got) != 3 {
		t.Fatalf("expected 3 planning entries, got %d", len(got))
	}
	if web.snapshots != 0 || binary.snapshots != 0 || pentest.snapshots != 0 {
		t.Fatalf("groupPlanningCatalog should not call Snapshot, got web=%d binary=%d pentest=%d", web.snapshots, binary.snapshots, pentest.snapshots)
	}
	if strings.TrimSpace(fmt.Sprintf("%v", got[0]["description"])) == "" {
		t.Fatalf("expected static group description, got %+v", got[0])
	}
}

func TestHandleGroupWaitStateChangeWakesWaitingRequesterForCollaborator(t *testing.T) {
	requester := &fakeReportAwareGroup{id: binaryGroupID, name: binaryGroupName, kind: binaryGroupKind}
	collaborator := &fakeReportAwareGroup{id: pentestGroupID, name: pentestGroupName, kind: pentestGroupKind}

	lm := &LabManager{
		groups: map[string]researchGroup{
			requester.ID():    requester,
			collaborator.ID(): collaborator,
		},
		groupOrder:            []string{requester.ID(), collaborator.ID()},
		groupWaitStates:       map[string]bool{requester.ID(): true},
		collaborationSessions: map[string]labCollaborationSession{requester.ID(): {RequesterGroupID: requester.ID(), CollaboratorGroupIDs: []string{collaborator.ID()}, ParticipantGroupIDs: []string{requester.ID(), collaborator.ID()}, Request: "需要渗透组协助", Reason: "等待黑盒进展"}},
	}

	lm.handleGroupWaitStateChange(collaborator.ID(), true)

	if got := len(requester.events); got != 1 {
		t.Fatalf("expected requester to be woken once, got %d", got)
	}
	if !strings.Contains(requester.events[0], collaborator.Name()) {
		t.Fatalf("expected wake prompt to mention collaborator name, got: %s", requester.events[0])
	}
	if got := len(collaborator.events); got != 0 {
		t.Fatalf("collaborator should not be woken by its own wait transition, got %d events", got)
	}
}

func TestHandleGroupReportUpdateWakesAllWaitingGroupsOnAnyReport(t *testing.T) {
	source := &fakeReportAwareGroup{id: pentestGroupID, name: pentestGroupName, kind: pentestGroupKind, running: true}
	requester := &fakeReportAwareGroup{id: binaryGroupID, name: binaryGroupName, kind: binaryGroupKind, running: false}
	otherWaiting := &fakeReportAwareGroup{id: webGroupID, name: webGroupName, kind: webGroupKind, running: false}

	lm := &LabManager{
		groups: map[string]researchGroup{
			source.ID():       source,
			requester.ID():    requester,
			otherWaiting.ID(): otherWaiting,
		},
		groupOrder:      []string{source.ID(), requester.ID(), otherWaiting.ID()},
		groupWaitStates: map[string]bool{requester.ID(): true, otherWaiting.ID(): true},
		collaborationSessions: map[string]labCollaborationSession{
			requester.ID(): {
				RequesterGroupID:     requester.ID(),
				CollaboratorGroupIDs: []string{source.ID()},
				ParticipantGroupIDs:  []string{requester.ID(), source.ID()},
				Request:              "等待其他研究组产出",
				Reason:               "协同推进中",
			},
		},
	}

	report := taskManager.ReportMeta{
		ReportID: "R.9",
		FileName: "shared-findings.md",
		Summary:  "新的协作报告",
	}
	lm.handleGroupReportUpdate(source.ID(), report)

	if got := len(requester.events); got != 1 {
		t.Fatalf("expected requester to be woken once by shared report, got %d", got)
	}
	if !strings.Contains(requester.events[0], report.ReportID) {
		t.Fatalf("expected requester wake prompt to mention report id, got: %s", requester.events[0])
	}
	if got := len(otherWaiting.events); got != 1 {
		t.Fatalf("expected unrelated waiting group to be woken once by shared report, got %d", got)
	}
	if !strings.Contains(otherWaiting.events[0], report.ReportID) {
		t.Fatalf("expected unrelated waiting group wake prompt to mention report id, got: %s", otherWaiting.events[0])
	}
	if got := len(source.events); got != 0 {
		t.Fatalf("source group should not be woken by its own report, got %d", got)
	}
}

func TestHandleGroupReportUpdateWakesCollaborationParticipantsEvenWhenNotWaiting(t *testing.T) {
	source := &fakeReportAwareGroup{id: webGroupID, name: webGroupName, kind: webGroupKind, running: true}
	requester := &fakeReportAwareGroup{id: pentestGroupID, name: pentestGroupName, kind: pentestGroupKind, running: false, finished: true}
	collaborator := &fakeReportAwareGroup{id: binaryGroupID, name: binaryGroupName, kind: binaryGroupKind, running: false, finished: true}

	lm := &LabManager{
		groups: map[string]researchGroup{
			source.ID():       source,
			requester.ID():    requester,
			collaborator.ID(): collaborator,
		},
		groupOrder: []string{source.ID(), requester.ID(), collaborator.ID()},
		collaborationSessions: map[string]labCollaborationSession{
			requester.ID(): {
				RequesterGroupID:     requester.ID(),
				CollaboratorGroupIDs: []string{source.ID(), collaborator.ID()},
				ParticipantGroupIDs:  []string{requester.ID(), source.ID(), collaborator.ID()},
				Request:              "等待代码审计结果后继续渗透",
				Reason:               "需要源码证据回流",
			},
		},
	}

	report := taskManager.ReportMeta{ReportID: "R.10", FileName: "audit-complete.md", Summary: "代码审计完成"}
	lm.handleGroupReportUpdate(source.ID(), report)

	if got := len(requester.events); got != 1 {
		t.Fatalf("expected requester to be woken once by collaboration report, got %d", got)
	}
	if !strings.Contains(requester.events[0], report.ReportID) {
		t.Fatalf("expected requester wake prompt to mention report id, got: %s", requester.events[0])
	}
	if got := len(collaborator.events); got != 1 {
		t.Fatalf("expected collaborator participant to be woken once by collaboration report, got %d", got)
	}
	if !strings.Contains(collaborator.events[0], report.ReportID) {
		t.Fatalf("expected collaborator wake prompt to mention report id, got: %s", collaborator.events[0])
	}
	if got := len(source.events); got != 0 {
		t.Fatalf("source group should not be woken by its own report, got %d", got)
	}
}

func TestDispatchPlannedTasksRoutesTaskToSelectedGroup(t *testing.T) {
	web := &fakeReportAwareGroup{id: webGroupID, name: webGroupName, kind: webGroupKind, running: false}
	pm, err := ProjectManager.NewProjectManager(ProjectManager.ProjectConfig{
		ProjectName:         "dispatch-routing-test",
		SourceCodeDir:       t.TempDir(),
		SkipProjectOverview: true,
		TaskContent:         "test task",
	})
	if err != nil {
		t.Fatalf("NewProjectManager returned error: %v", err)
	}
	lm := &LabManager{
		labName:    "test-lab",
		groups:     map[string]researchGroup{web.ID(): web},
		groupOrder: []string{web.ID()},
		webGroup:   &WebSecurityGroup{pm: pm},
	}

	lm.dispatchPlannedTasks(labDispatchPlan{
		GroupIDs: []string{web.ID()},
		GroupTasks: map[string]string{
			web.ID(): "对白盒源码进行审计",
		},
		Reason: "需要Web组接手",
	}, "协作请求", totalBrainName, "", false)

	if web.starts != 0 {
		t.Fatalf("expected no sandbox-backed auto start in this unit test, got %d", web.starts)
	}
	if len(web.decisionChats) != 1 {
		t.Fatalf("expected selected group to receive one decision-chat task, got %d", len(web.decisionChats))
	}
	if !strings.Contains(web.decisionChats[0], "对白盒源码进行审计") {
		t.Fatalf("unexpected decision chat payload: %v", web.decisionChats)
	}
}

func TestTryParseDispatchPlanFromAssistantContentAcceptsJSONFence(t *testing.T) {
	web := &fakeReportAwareGroup{id: webGroupID, name: webGroupName, kind: webGroupKind}
	pentest := &fakeReportAwareGroup{id: pentestGroupID, name: pentestGroupName, kind: pentestGroupKind}
	lm := &LabManager{
		groups: map[string]researchGroup{
			web.ID():     web,
			pentest.ID(): pentest,
		},
		groupOrder: []string{web.ID(), pentest.ID()},
	}

	content := "先给出调度结果。\n```json\n{\"group_ids\":[\"pentest_security\"],\"dispatch_reason\":\"远程黑盒目标应优先由渗透测试组推进\",\"group_tasks\":[{\"group_id\":\"pentest_security\",\"task\":\"围绕给定入口点开展黑盒攻击，优先验证弱口令、哈希绕过与目录发现。\"}]}\n```"
	plan, ok := lm.tryParseDispatchPlanFromAssistantContent("please assess the remote target", content, false)
	if !ok {
		t.Fatal("expected fenced JSON assistant content to be accepted as a dispatch fallback")
	}
	if len(plan.GroupIDs) != 1 || plan.GroupIDs[0] != pentestGroupID {
		t.Fatalf("unexpected plan group ids: %+v", plan.GroupIDs)
	}
	if !strings.Contains(plan.GroupTasks[pentestGroupID], "黑盒攻击") {
		t.Fatalf("unexpected pentest task: %+v", plan.GroupTasks)
	}
}

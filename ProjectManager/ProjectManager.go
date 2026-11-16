package ProjectManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/agents"
	"AIxVuln/dockerManager"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var externalTargetHintPattern = regexp.MustCompile(`(?i)(https?://\S+|\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?(?:/\S*)?|\b[a-z0-9.-]+\.[a-z]{2,}(?::\d+)?(?:/\S*)?)`)

type ProjectManager struct {
	mu                      sync.RWMutex
	wg                      *sync.WaitGroup
	tasks                   []*taskManager.Task
	projectID               string
	projectName             string
	taskContent             string
	agentGroupList          AgentGroupList
	goroutine               chan func()
	workDir                 string
	sourceCodeDir           string
	projectDir              string
	containerList           []taskManager.ContainerInfo
	vulnList                []taskManager.Vuln //整个项目的漏洞汇总
	eventList               []string
	agentRespList           map[string]*agents.StartResp
	reportList              map[string]string
	status                  string
	startTime               string
	endTime                 string
	stopChan                chan struct{}
	ctx                     context.Context
	cancelFunc              context.CancelFunc
	isStopping              bool
	isRunning               bool
	msgChan                 chan string
	envInfo                 map[string]interface{}
	sandboxID               string
	dm                      *dockerManager.DockerManager
	decisionBrain           *DecisionBrain.DecisionBrain
	hostCommandHandler      func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult
	decisionApprovalHandler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult
	brainRestart            chan struct{}
	projectOverview         string
	skipProjectOverview     bool
}

type ProjectConfig struct {
	ProjectID           string
	ProjectName         string
	SourceCodeDir       string
	SkipProjectOverview bool
	MsgChan             chan string
	TaskContent         string
}

type DecisionBrainConfig struct {
	SystemPrompt          string
	EnableExploitWorkflow bool
	AgentDefinitions      []agents.AgentDefinition
	DigitalHumanPool      map[string][]agents.AgentProfile
}

type AgentGroup struct {
	Name      string // Agent组名称
	AgentList []*AgentOne
	Runed     bool // 是否已经运行完成
}

type AgentOne struct {
	Agent         agents.Agent
	InheritEnv    string // 从某个Agent继承关键信息，填写AgentId
	InheritVuln   string // 从某个Agent继承Vuln信息，填写AgentId
	InheritMemory string // 从某个Agent继承记忆体信息，填写AgentId
}

type AgentGroupList struct {
	AgentList []*AgentGroup
}

func (a *AgentGroupList) putAgentGroup(ag *AgentGroup) {
	a.AgentList = append(a.AgentList, ag)
}
func (a *AgentGroupList) nextAgentGroup() *AgentGroup {
	for _, one := range a.AgentList {
		if one.Runed == false {
			return one
		}
	}
	return nil
}

func NewProjectManager(project ProjectConfig) (*ProjectManager, error) {
	project.ProjectName = strings.TrimSpace(project.ProjectName)
	if project.ProjectName == "" {
		return nil, fmt.Errorf("项目名称不能为空")
	}
	projectID := strings.TrimSpace(project.ProjectID)
	if projectID == "" {
		projectID = uuid.New().String()
	}
	absDataDir, _ := filepath.Abs(misc.GetDataDir())
	projectDir := filepath.Join(absDataDir, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, err
	}
	workDir := misc.ProjectWorkDir(projectDir)
	sourceCodeDir := misc.ProjectSourceDir(projectDir)
	if err := os.MkdirAll(sourceCodeDir, 0755); err != nil {
		return nil, err
	}
	absSrcDir, err := filepath.Abs(project.SourceCodeDir)
	if err != nil {
		return nil, err
	}
	absDstDir, err := filepath.Abs(sourceCodeDir)
	if err != nil {
		return nil, err
	}
	// When restoring projects from persistence, source and destination may already be identical.
	// Skip copy in that case to avoid expensive no-op/self-overwrite behavior.
	if absSrcDir != absDstDir {
		err = misc.CopyDir(project.SourceCodeDir, sourceCodeDir)
		if err != nil {
			return nil, err
		}
	}
	pm := ProjectManager{
		wg:                  &sync.WaitGroup{},
		projectID:           projectID,
		projectName:         project.ProjectName,
		taskContent:         project.TaskContent,
		goroutine:           make(chan func(), 100),
		workDir:             workDir,
		sourceCodeDir:       sourceCodeDir,
		projectDir:          projectDir,
		agentRespList:       make(map[string]*agents.StartResp),
		reportList:          make(map[string]string),
		status:              "未运行",
		stopChan:            make(chan struct{}, 1), // 带缓冲的通道
		isStopping:          false,
		isRunning:           false,
		brainRestart:        make(chan struct{}, 1),
		msgChan:             project.MsgChan,
		dm:                  dockerManager.NewDockerManager(),
		skipProjectOverview: normalizeProjectOverviewSkip(project.SkipProjectOverview, sourceCodeDir, project.TaskContent),
	}
	pm.ctx, pm.cancelFunc = context.WithCancel(context.Background())
	pm.goroutineOps()
	taskManager.SetDockerManager(pm.projectID, pm.dm)
	sm := dockerManager.NewServiceManager(workDir, sourceCodeDir, pm.dm)
	taskManager.SetServiceManager(pm.projectID, sm)
	taskManager.SetGoroutineChan(pm.projectID, pm.goroutine)
	pm.decisionBrain = DecisionBrain.NewDecisionBrain(projectID, project.TaskContent, project.MsgChan)
	pm.decisionBrain.SetStatusHandler(func(status string) {
		pm.status = status
	})
	return &pm, nil
}

func (pm *ProjectManager) GetDockerManager() *dockerManager.DockerManager {
	return pm.dm
}

func (pm *ProjectManager) SetMsgChan(msgChan chan string) {
	pm.msgChan = msgChan
}

func (pm *ProjectManager) GetProjectName() string {
	return pm.projectName
}

func (pm *ProjectManager) GetProjectID() string {
	return pm.projectID
}

func (pm *ProjectManager) GetTaskContent() string {
	return pm.taskContent
}

func (pm *ProjectManager) GetProjectOverview() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.projectOverview
}

func (pm *ProjectManager) SetProjectOverview(overview string) {
	overview = strings.TrimSpace(overview)
	pm.mu.Lock()
	pm.projectOverview = overview
	pm.mu.Unlock()
	if pm.decisionBrain != nil && overview != "" {
		pm.decisionBrain.SetProjectOverview(overview)
	}
}

func (pm *ProjectManager) EnsureProjectOverview() string {
	if overview := strings.TrimSpace(pm.GetProjectOverview()); overview != "" {
		if pm.decisionBrain != nil {
			pm.decisionBrain.SetProjectOverview(overview)
		}
		return overview
	}
	if err := pm.ensureProjectOverviewSandbox(); err != nil {
		misc.Debug("EnsureProjectOverview: sandbox 初始化失败: %v", err)
		return ""
	}
	overview := strings.TrimSpace(pm.runProjectOverview())
	if overview == "" {
		return ""
	}
	pm.SetProjectOverview(overview)
	return overview
}

func (pm *ProjectManager) ensureProjectOverviewSandbox() error {
	return pm.ensureSandbox()
}

func (pm *ProjectManager) RegisterDecisionBrainTool(name string, description string, parameters map[string]interface{}, handler func(map[string]interface{}) string) {
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.RegisterExternalTool(name, description, parameters, handler)
}

func (pm *ProjectManager) ConfigureDecisionBrain(cfg DecisionBrainConfig) {
	if pm.decisionBrain == nil {
		return
	}
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		pm.decisionBrain.SetSystemPrompt(cfg.SystemPrompt)
	}
	pm.decisionBrain.SetExploitWorkflowEnabled(cfg.EnableExploitWorkflow)
	pm.decisionBrain.ReplaceAgents(cfg.AgentDefinitions)
	pm.decisionBrain.ReplaceDigitalHumanPool(cfg.DigitalHumanPool)
}

func (pm *ProjectManager) SetDecisionBrainOwnerScope(groupID string, groupName string) {
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.SetOwnerScope(groupID, groupName)
}

func (pm *ProjectManager) SetExploitAutomationPolicyResolver(resolver func() DecisionBrain.ExploitAutomationPolicy) {
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.SetExploitAutomationPolicyResolver(resolver)
}

func (pm *ProjectManager) SetHostCommandRequestHandler(handler func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult) {
	pm.hostCommandHandler = handler
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.SetHostCommandRequestHandler(handler)
}

func (pm *ProjectManager) SetDecisionApprovalHandler(handler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult) {
	pm.decisionApprovalHandler = handler
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.SetDecisionApprovalHandler(handler)
}

func (pm *ProjectManager) SetGlobalStatusPanel(panel map[string]interface{}) {
	if pm == nil || pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.SetGlobalStatusPanel(panel)
}

func (pm *ProjectManager) GetStatus() string {
	return pm.status
}
func (pm *ProjectManager) GetStartTime() string {
	return pm.startTime
}
func (pm *ProjectManager) GetEndTime() string {
	return pm.endTime
}
func (pm *ProjectManager) SetStatus(status string) {
	pm.status = status
}
func (pm *ProjectManager) GetSourceCodeDir() string {
	return pm.sourceCodeDir
}

func (pm *ProjectManager) GetWorkDir() string {
	return pm.workDir
}

func sourceWorkspaceHasEntries(sourceDir string) bool {
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return false
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" || name == ".git" {
			continue
		}
		return true
	}
	return false
}

func shouldSkipProjectOverview(explicitSkip bool, sourceDir string) bool {
	if !explicitSkip {
		return false
	}
	return !sourceWorkspaceHasEntries(sourceDir)
}

func taskContentHasExternalTargetHint(taskContent string) bool {
	taskContent = strings.TrimSpace(taskContent)
	if taskContent == "" {
		return false
	}
	return externalTargetHintPattern.FindString(taskContent) != ""
}

func normalizeProjectOverviewSkip(explicitSkip bool, sourceDir string, taskContent string) bool {
	if !explicitSkip {
		return false
	}
	if !sourceWorkspaceHasEntries(sourceDir) && taskContentHasExternalTargetHint(taskContent) {
		return false
	}
	return shouldSkipProjectOverview(explicitSkip, sourceDir)
}

func (pm *ProjectManager) ShouldSkipProjectOverview() bool {
	if pm == nil {
		return true
	}
	return normalizeProjectOverviewSkip(pm.skipProjectOverview, pm.sourceCodeDir, pm.taskContent)
}

func (pm *ProjectManager) SetSkipProjectOverview(skip bool) {
	pm.skipProjectOverview = normalizeProjectOverviewSkip(skip, pm.sourceCodeDir, pm.taskContent)
}

func (pm *ProjectManager) GetProjectDir() string {
	return pm.projectDir
}

func (pm *ProjectManager) goroutineOps() {
	go func() {
		for f := range pm.goroutine {
			go func() {
				pm.wg.Add(1)
				f()
				pm.wg.Done()
			}()
		}
	}()
}

func (pm *ProjectManager) StartTask() {
	if pm.taskContent == "" {
		pm.status = "任务内容为空"
		return
	}
	if pm.isRunning {
		return
	}
	pm.status = "正在运行"
	pm.isRunning = true
	pm.isStopping = false
	pm.startTime = time.Now().Format("2006-01-02 15:04:05")
	defer func() {
		if strings.HasPrefix(pm.status, "运行失败") {
			pm.isRunning = false
			pm.endTime = time.Now().Format("2006-01-02 15:04:05")
			return
		}
		// If the brain has entered "决策结束", treat this project as paused instead of fully ended.
		// Keep the status so UI can still show the "结束项目" option.
		if pm.decisionBrain != nil && pm.decisionBrain.IsBrainFinished() {
			pm.status = "决策结束"
		} else {
			pm.status = "运行结束"
		}
		pm.isRunning = false
		pm.endTime = time.Now().Format("2006-01-02 15:04:05")
	}()

	if err := pm.ensureSandbox(); err != nil {
		pm.status = "运行失败: sandbox 初始化失败"
		misc.Debug("StartTask: sandbox 初始化失败: %v", err)
		return
	}

	// Reuse the lab-level project overview silently.
	if overviewSummary := strings.TrimSpace(pm.GetProjectOverview()); overviewSummary != "" {
		pm.decisionBrain.SetProjectOverview(overviewSummary)
	}
	pm.status = "正在运行"

	for {
		pm.wg.Add(1)
		go func() {
			defer pm.wg.Done()
			pm.decisionBrain.Start()
		}()
		pm.wg.Wait()

		// If brain entered "决策结束", do not block this runner.
		// Return now so the global queue can start the next project.
		if !pm.decisionBrain.IsBrainFinished() {
			break
		}
		return
	}
}

func (pm *ProjectManager) ensureSandbox() error {
	if sb, err := taskManager.GetSandbox(pm.projectID); err == nil && sb != nil && sb.ContainerId != "" {
		pm.sandboxID = sb.ContainerId
		if pm.decisionBrain != nil {
			pm.decisionBrain.RefreshDynamicDigitalHumanPrompts()
		}
		return nil
	}

	if strings.TrimSpace(pm.sandboxID) != "" {
		sb, err := pm.dm.RestoreSandboxByContainerID(pm.sandboxID, pm.workDir, pm.sourceCodeDir)
		if err == nil && sb != nil {
			taskManager.SetSandbox(pm.projectID, sb)
			pm.sandboxID = sb.ContainerId
			if pm.decisionBrain != nil {
				pm.decisionBrain.RefreshDynamicDigitalHumanPrompts()
			}
			return nil
		}
		misc.Debug("ensureSandbox: restore sandbox failed for project=%s sandboxID=%s err=%v; creating new sandbox", pm.projectID, pm.sandboxID, err)
	}

	sb, err := dockerManager.NewSandbox(pm.dm, pm.workDir, pm.sourceCodeDir)
	if err != nil {
		return err
	}
	taskManager.SetSandbox(pm.projectID, sb)
	pm.sandboxID = sb.ContainerId
	if pm.decisionBrain != nil {
		pm.decisionBrain.RefreshDynamicDigitalHumanPrompts()
	}
	return nil
}

func (pm *ProjectManager) EnsureSandbox() error {
	return pm.ensureSandbox()
}

// runProjectOverview runs the ProjectOverviewAgent synchronously and returns the summary.
func (pm *ProjectManager) runProjectOverview() string {
	task := taskManager.NewTask(pm.projectID)
	argsPayload := map[string]string{
		"task_content": strings.TrimSpace(pm.taskContent),
	}
	argsBytes, _ := json.Marshal(argsPayload)
	argsJSON := string(argsBytes)
	task.SetTaskActor(taskManager.TaskActor{
		AgentToolName:   "ProjectOverviewAgent",
		AgentName:       "ProjectOverviewAgent",
		RequesterSource: "system_agent",
	})
	if pm.hostCommandHandler != nil {
		task.SetHostCommandRequestHandler(func(req taskManager.HostCommandRequest) taskManager.HostCommandResult {
			return pm.hostCommandHandler(pm.ctx, task, req)
		})
	}
	if pm.decisionApprovalHandler != nil {
		task.SetDecisionApprovalHandler(func(req taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult {
			return pm.decisionApprovalHandler(pm.ctx, task, req)
		})
	}
	task.SetEventHandler(func(agentName string, event string, eventType int) {
		// Minimal event handler — overview events are not critical.
		misc.Debug("[ProjectOverview] %s: %s", agentName, event)
	})
	agent, err := agents.NewProjectOverviewAgent(task, argsJSON)
	if err != nil {
		misc.Debug("runProjectOverview: 创建 agent 失败: %s", err.Error())
		return ""
	}
	ctx := pm.ctx
	// Start the persistent StartTask loop in a goroutine, then send the task via AssignTask.
	doneCh := make(chan *agents.StartResp, 1)
	go agent.StartTask(ctx)
	agent.AssignTask(agents.TaskAssignment{
		ArgsJson: argsJSON,
		DoneCb: func(r *agents.StartResp) {
			doneCh <- r
		},
	})
	select {
	case resp := <-doneCh:
		if resp.Err != nil {
			misc.Debug("runProjectOverview: agent 执行失败: %s", resp.Err.Error())
			return ""
		}
		return resp.Summary
	case <-ctx.Done():
		misc.Debug("runProjectOverview: context 结束")
		return ""
	}
}

func (pm *ProjectManager) StopTask() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	// 防止重复调用
	if pm.isStopping {
		return
	}
	pm.status = "正在停止"
	pm.isStopping = true
	taskManager.CloseBackgroundProcesses(pm.projectID)
	if pm.decisionBrain != nil {
		pm.decisionBrain.Stop()
	}
	// 关键：调用 context 的取消函数
	if pm.cancelFunc != nil {
		pm.cancelFunc()
	}
}
func (pm *ProjectManager) GetContainerList() []taskManager.ContainerInfo {
	if pm.decisionBrain != nil {
		cl := pm.decisionBrain.GetContainerList()
		if len(cl) > 0 {
			out := make([]taskManager.ContainerInfo, 0, len(cl))
			for _, c := range cl {
				if c == nil {
					continue
				}
				out = append(out, *c)
			}
			return out
		}
	}
	return pm.containerList
}
func (pm *ProjectManager) GetEvent(count int) []string {
	if pm.decisionBrain != nil {
		e := pm.decisionBrain.GetEvent(count)
		if len(e) > 0 {
			return e
		}
	}
	l := len(pm.eventList)
	if l < count || l == 0 {
		return pm.eventList
	}
	return pm.eventList[l-count:]
}

func (pm *ProjectManager) GetBrainFeed(count int) []map[string]interface{} {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetBrainFeed(count)
}

func (pm *ProjectManager) GetAgentFeed(agentID string, count int) []map[string]interface{} {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetAgentFeed(agentID, count)
}

func (pm *ProjectManager) GetVulnList() []taskManager.Vuln {
	return pm.vulnList
}
func (pm *ProjectManager) SetVulns(vuln []taskManager.Vuln) {
	pm.vulnList = vuln
}
func (pm *ProjectManager) SetEvent(event []string) {
	const maxProjectEventCache = 2000
	if len(event) > maxProjectEventCache {
		event = append([]string(nil), event[len(event)-maxProjectEventCache:]...)
	}
	pm.eventList = event
}
func (pm *ProjectManager) SetProjectDir(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	pm.projectDir = dir
	pm.workDir = misc.ProjectWorkDir(dir)
	pm.sourceCodeDir = misc.ProjectSourceDir(dir)
	_ = os.MkdirAll(pm.sourceCodeDir, 0755)
	taskManager.SetServiceManager(pm.projectID, dockerManager.NewServiceManager(pm.workDir, pm.sourceCodeDir, pm.dm))
}
func (pm *ProjectManager) SetStartTime(t string) {
	pm.startTime = t
}
func (pm *ProjectManager) SetEndTime(t string) {
	pm.endTime = t
}
func (pm *ProjectManager) GetReportList() map[string]string {
	if pm.decisionBrain != nil {
		rl := pm.decisionBrain.GetReportList()
		if len(rl) > 0 {
			m := make(map[string]string)
			for i, item := range rl {
				path := strings.TrimSpace(item.Path)
				if path == "" {
					continue
				}
				key := strings.TrimSpace(item.ReportID)
				if key == "" {
					key = strings.TrimSpace(item.TargetID)
				}
				if key == "" {
					key = filepath.Base(path)
					if key == "" {
						key = fmt.Sprintf("report-%d", i)
					}
				}
				if _, exists := m[key]; exists {
					key = fmt.Sprintf("%s-%d", key, i)
				}
				m[key] = path
			}
			return m
		}
	}
	return pm.reportList
}

func (pm *ProjectManager) GetReportCatalog() []taskManager.ReportMeta {
	if pm.decisionBrain != nil {
		rl := pm.decisionBrain.GetReportList()
		if len(rl) > 0 {
			return rl
		}
	}
	if len(pm.reportList) == 0 {
		return nil
	}
	keys := make([]string, 0, len(pm.reportList))
	for k := range pm.reportList {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]taskManager.ReportMeta, 0, len(keys))
	for _, key := range keys {
		path := strings.TrimSpace(pm.reportList[key])
		if path == "" {
			continue
		}
		out = append(out, taskManager.ReportMeta{
			ReportID: key,
			Path:     path,
			FileName: filepath.Base(path),
		})
	}
	return out
}
func (pm *ProjectManager) GetEnvInfo() map[string]interface{} {
	if pm.decisionBrain != nil {
		env := pm.decisionBrain.GetEnvInfo()
		if len(env) > 0 {
			return env
		}
	}
	return pm.envInfo
}

func (pm *ProjectManager) GetExploitIdeaList() []*taskManager.ExploitIdea {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetExploitIdeaList()
}

func (pm *ProjectManager) GetExploitChainList() []*taskManager.ExploitChain {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetExploitChainList()
}

func (pm *ProjectManager) RestoreExploitState(ideas []*taskManager.ExploitIdea, chains []*taskManager.ExploitChain) {
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.RestoreExploitState(ideas, chains)
}

func (pm *ProjectManager) GetAgentRuntimeList() []map[string]interface{} {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetAgentRuntimeList()
}

func (pm *ProjectManager) GetDigitalHumanRoster() map[string]interface{} {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetDigitalHumanRoster()
}

func (pm *ProjectManager) GetBrainFinished() bool {
	if pm.decisionBrain == nil {
		return false
	}
	return pm.decisionBrain.IsBrainFinished()
}

func (pm *ProjectManager) ListMemoryDebugTargets() []taskManager.MemoryDebugTarget {
	if pm.decisionBrain == nil {
		return []taskManager.MemoryDebugTarget{}
	}
	return pm.decisionBrain.ListMemoryDebugTargets()
}

func (pm *ProjectManager) ExportMemoryDebug(localTargetID string) (*taskManager.MemoryDebugExport, error) {
	if pm.decisionBrain == nil {
		return nil, fmt.Errorf("decision brain is unavailable")
	}
	return pm.decisionBrain.ExportMemoryDebug(localTargetID)
}

func (pm *ProjectManager) TeamChat(msg string) string {
	if pm.decisionBrain == nil {
		return "project not started"
	}
	// If the brain has finished, mark it resumable and accept the user message.
	// Actual execution resume is triggered by project start queue dispatch.
	if pm.decisionBrain.IsBrainFinished() {
		pm.decisionBrain.RestartAfterFinished()
	}
	return pm.decisionBrain.TeamChat(msg, "用户")
}

func (pm *ProjectManager) TeamChatFromSender(msg string, sender string) string {
	return pm.TeamChatFromSenderWithPrompt(msg, msg, sender)
}

func (pm *ProjectManager) TeamChatFromSenderWithPrompt(visibleMsg string, internalMsg string, sender string) string {
	if pm.decisionBrain == nil {
		return "project not started"
	}
	if pm.decisionBrain.IsBrainFinished() {
		pm.decisionBrain.RestartAfterFinished()
	}
	if strings.TrimSpace(sender) == "" {
		sender = "用户"
	}
	visibleMsg = strings.TrimSpace(visibleMsg)
	internalMsg = strings.TrimSpace(internalMsg)
	if internalMsg == "" {
		internalMsg = visibleMsg
	}
	if visibleMsg == "" {
		visibleMsg = internalMsg
	}
	if strings.TrimSpace(sender) != "" && sender != "用户" && visibleMsg != "" {
		pm.decisionBrain.AppendChatMessage(DecisionBrain.ChatMessage{
			Role:        "system",
			Text:        visibleMsg,
			Ts:          time.Now().Format("15:04:05"),
			PersonaName: sender,
			AvatarFile:  "system.png",
		})
	}
	return pm.decisionBrain.TeamChat(internalMsg, sender)
}

func (pm *ProjectManager) AppendChatMessage(msg DecisionBrain.ChatMessage) {
	if pm.decisionBrain != nil {
		pm.decisionBrain.AppendChatMessage(msg)
	}
}

func (pm *ProjectManager) SetChatObserver(observer func(DecisionBrain.ChatMessage)) {
	if pm.decisionBrain != nil {
		pm.decisionBrain.SetChatObserver(observer)
	}
}

func (pm *ProjectManager) SetReportObserver(observer func(taskManager.ReportMeta)) {
	if pm.decisionBrain != nil {
		pm.decisionBrain.SetReportObserver(observer)
	}
}

func (pm *ProjectManager) SetDecisionBrainWaitHandler(handler func(bool)) {
	if pm.decisionBrain != nil {
		pm.decisionBrain.SetWaitStateHandler(handler)
	}
}

func (pm *ProjectManager) NotifyDecisionBrainEvent(content string) {
	if pm.decisionBrain == nil {
		return
	}
	if pm.decisionBrain.IsBrainFinished() {
		pm.decisionBrain.RestartAfterFinished()
	}
	pm.decisionBrain.NotifyDecisionBrainEvent(content)
	if !pm.isRunning {
		go pm.StartTask()
	}
}

func (pm *ProjectManager) IsDecisionBrainWaiting() bool {
	if pm.decisionBrain == nil {
		return false
	}
	return pm.decisionBrain.IsWaiting()
}

func (pm *ProjectManager) GetChatMessages() []DecisionBrain.ChatMessage {
	if pm.decisionBrain == nil {
		return nil
	}
	return pm.decisionBrain.GetChatMessages()
}

func (pm *ProjectManager) GetTokenUsage() map[string]interface{} {
	if pm.decisionBrain == nil {
		return map[string]interface{}{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
	}
	return pm.decisionBrain.GetTokenUsage()
}

func (pm *ProjectManager) GetTokenUsageSnapshot() taskManager.TokenUsageSnapshot {
	if pm.decisionBrain == nil {
		return taskManager.TokenUsageSnapshot{}
	}
	return pm.decisionBrain.GetTokenUsageSnapshot()
}

func (pm *ProjectManager) RestoreTokenUsageSnapshot(snapshot taskManager.TokenUsageSnapshot) {
	if pm.decisionBrain == nil {
		return
	}
	pm.decisionBrain.RestoreTokenUsageSnapshot(snapshot)
}

func (pm *ProjectManager) GetContextBreakdown() interface{} {
	if pm.decisionBrain == nil {
		return map[string]interface{}{"sections": []interface{}{}, "total": 0, "max_context": 0}
	}
	return pm.decisionBrain.GetContextBreakdown()
}

func (pm *ProjectManager) SetEnvInfo(env map[string]interface{}) {
	pm.envInfo = env
}

func (pm *ProjectManager) SetContainer(containerList []taskManager.ContainerInfo) {
	pm.containerList = containerList
}

func (pm *ProjectManager) SetReport(reportList map[string]string) {
	pm.reportList = reportList
}

func (pm *ProjectManager) RestoreReportCatalog(reports []taskManager.ReportMeta) {
	if pm.decisionBrain != nil {
		pm.decisionBrain.RestoreReportCatalog(reports)
	}
	pm.reportList = make(map[string]string, len(reports))
	for i, report := range reports {
		path := strings.TrimSpace(report.Path)
		if path == "" {
			continue
		}
		key := strings.TrimSpace(report.ReportID)
		if key == "" {
			key = strings.TrimSpace(report.TargetID)
		}
		if key == "" {
			key = filepath.Base(path)
		}
		if key == "" {
			key = fmt.Sprintf("report-%d", i+1)
		}
		if _, exists := pm.reportList[key]; exists {
			key = fmt.Sprintf("%s-%d", key, i+1)
		}
		pm.reportList[key] = path
	}
}

func (pm *ProjectManager) RestoreBrainFinished(finished bool) {
	if pm.decisionBrain != nil {
		pm.decisionBrain.RestoreBrainFinished(finished)
	}
}

func (pm *ProjectManager) RemoveDockerAll() {
	if pm.dm == nil {
		return
	}
	// Remove project sandbox container (aisandbox)
	if sb, err := taskManager.GetSandbox(pm.projectID); err == nil && sb != nil {
		if sb.ContainerId != "" {
			misc.Debug("RemoveDockerAll: 删除 sandbox 容器 %s", sb.ContainerId)
			_ = pm.dm.DockerRemove(sb.ContainerId)
		}
		taskManager.RemoveSandbox(pm.projectID)
		pm.sandboxID = ""
	}
	// Remove containers tracked by the DecisionBrain (runtime source of truth).
	if pm.decisionBrain != nil {
		for _, c := range pm.decisionBrain.GetContainerList() {
			if c == nil || c.ContainerId == "" {
				continue
			}
			misc.Debug("RemoveDockerAll: 删除容器 %s", c.ContainerId)
			_ = pm.dm.DockerRemove(c.ContainerId)
		}
	}
	// Also remove any containers in the local list (e.g. restored from persistence).
	for _, c := range pm.containerList {
		if c.ContainerId == "" {
			continue
		}
		_ = pm.dm.DockerRemove(c.ContainerId)
	}
}

func (pm *ProjectManager) SetSandboxContainerID(containerID string) {
	pm.sandboxID = strings.TrimSpace(containerID)
}

func (pm *ProjectManager) GetSandboxContainerID() string {
	if sb, err := taskManager.GetSandbox(pm.projectID); err == nil && sb != nil && sb.ContainerId != "" {
		pm.sandboxID = sb.ContainerId
	}
	return pm.sandboxID
}

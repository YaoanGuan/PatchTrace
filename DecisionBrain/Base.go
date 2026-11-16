package DecisionBrain

import (
	"AIxVuln/agents"
	pentestagents "AIxVuln/agents/pentest"
	"AIxVuln/dockerManager"
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func (db *DecisionBrain) startVerifiedReportForExploitChain(ec *taskManager.ExploitChain) {
	if ec == nil {
		return
	}
	args := map[string]string{"reportType": "verifier", "exploit_chain_id": ec.ExploitChainId}
	argsJson, _ := json.Marshal(args)
	_ = db.startAgent("Agent-Report-ReportCommonAgent", string(argsJson))
}

func normalizeReportMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "analyzer":
		return "analyzer"
	case "verifier":
		return "verifier"
	default:
		return "verifier"
	}
}

func reportModeFromArgsMap(argsMap map[string]interface{}) string {
	if argsMap == nil {
		return "verifier"
	}
	return normalizeReportMode(fmt.Sprint(argsMap["reportType"]))
}

func shouldDeferAgentStart(agentName string, argsMap map[string]interface{}) bool {
	switch {
	case strings.HasPrefix(agentName, "Agent-Verifier"):
		return true
	case strings.HasPrefix(agentName, "Agent-Report"):
		return reportModeFromArgsMap(argsMap) != "analyzer"
	default:
		return false
	}
}

func dispatchAnalyzeReport(starter func(string, string) string, targetKey string, targetID string) string {
	targetKey = strings.TrimSpace(targetKey)
	targetID = strings.TrimSpace(targetID)
	if starter == nil || targetKey == "" || targetID == "" {
		return ""
	}
	args := map[string]string{
		"reportType": "analyzer",
		targetKey:    targetID,
	}
	argsJSON, _ := json.Marshal(args)
	return starter("Agent-Report-ReportCommonAgent", string(argsJSON))
}

func (db *DecisionBrain) startAnalyzeReportForTarget(targetKey string, targetID string) {
	msg := dispatchAnalyzeReport(db.startAgent, targetKey, targetID)
	if strings.TrimSpace(msg) != "" {
		misc.Debug("startAnalyzeReportForTarget(%s=%s): %s", targetKey, targetID, msg)
	}
}

var auditPrompt = prompts.EvidenceAuditSystemPrompt()
var guidancePromptTemplate = prompts.GuidancePromptTemplate()

type DecisionBrain struct {
	memory                  *BrainMemory
	agentHandler            map[string]func(task *taskManager.Task, args string) (agents.Agent, error)
	agentToolDefs           map[string]llm.ToolDef
	projectName             string
	memoryFilePath          string
	store                   *ExploitStore
	exploitIdeaList         []*taskManager.ExploitIdea
	exploitChainList        []*taskManager.ExploitChain
	eventList               []string
	brainFeed               []map[string]interface{}
	agentFeed               map[string][]map[string]interface{}
	containerInfoList       []string
	containerList           []*taskManager.ContainerInfo
	reportList              []taskManager.ReportMeta
	envInfo                 map[string]interface{}
	ctxList                 []context.Context
	cancelList              []context.CancelFunc
	ctx                     context.Context
	cancelFunc              context.CancelFunc
	model                   string
	runAgentList            []*AgentRuntime
	wg                      sync.WaitGroup
	mu                      sync.Mutex
	feedMu                  sync.Mutex
	lastMsgHash             string
	notify                  chan bool
	done                    chan struct{}
	webOutputChan           chan string
	envBuildCond            *sync.Cond // 在环境没搭建完成的情况下阻塞
	poolMu                  sync.Mutex
	digitalHumanPool        map[string][]digitalHumanEntry
	digitalHumanBusy        map[string]digitalHumanBusyInfo
	digitalHumanQueue       map[string][]digitalHumanQueuedReq
	personaMemory           map[string]string
	lastSummary             map[string]string // keyed by DigitalHumanID
	lastTask                map[string]string // keyed by DigitalHumanID
	lastError               map[string]string // keyed by DigitalHumanID
	lastArgs                map[string]string // keyed by DigitalHumanID — original startAgent args
	chatFilePath            string
	chatMessages            []ChatMessage
	chatMu                  sync.Mutex
	lastChatPersistAt       time.Time
	chatPersistTimer        *time.Timer
	chatObserver            func(ChatMessage)
	reportObserver          func(taskManager.ReportMeta)
	brainFinished           bool
	statusHandler           func(status string) // callback to notify ProjectManager of status changes
	waitStateHandler        func(waiting bool)
	waiting                 bool
	externalTools           []llm.ToolDef
	externalToolCaller      map[string]func(map[string]interface{}) string
	enableExploitFlow       bool
	groupID                 string
	groupName               string
	hostCommandHandler      func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult
	decisionApprovalHandler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult
	exploitPolicy           func() ExploitAutomationPolicy
	globalStatusPanel       map[string]interface{}
	globalStatusJSON        string
}

type ChatMessage struct {
	Role        string `json:"role"` // "user" or "system"
	Text        string `json:"text"`
	Ts          string `json:"ts"`
	PersonaName string `json:"persona_name,omitempty"`
	AvatarFile  string `json:"avatar_file,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	SenderType  string `json:"sender_type,omitempty"` // user | total_brain | group_brain | member | system
}

const (
	maxDecisionEventList  = 5000
	maxDecisionChatRecord = 3000
	chatPersistDebounce   = 2 * time.Second
)

// digitalHumanEntry holds a persistent agent instance bound to a digital human profile.
type digitalHumanEntry struct {
	Profile agents.AgentProfile
	Agent   agents.Agent
}

type digitalHumanBusyInfo struct {
	AgentToolName string
	Profile       agents.AgentProfile
	Agent         agents.Agent
}

type digitalHumanQueuedReq struct {
	AgentToolName string
	Args          string
}

// trySendWS writes to webOutputChan with a timeout to prevent permanent blocking
// when the WebSocket consumer is slow or disconnected.
func (db *DecisionBrain) trySendWS(msg string) {
	if db.webOutputChan == nil {
		return
	}
	select {
	case db.webOutputChan <- msg:
	case <-time.After(5 * time.Second):
		misc.Debug("trySendWS: dropped message (channel full for 5s): %.100s", msg)
	}
}

func NewDecisionBrain(projectName string, taskContent string, webOutputChan chan string) *DecisionBrain {
	memory := NewBrainMemory()
	projectDir := filepath.Join(misc.GetDataDir(), "projects", projectName)
	memoryDir := filepath.Join(projectDir, "memory")
	_ = os.MkdirAll(memoryDir, 0755)
	memoryFile := filepath.Join(memoryDir, "brain.json")
	if _, err := os.Stat(memoryFile); err == nil {
		_ = memory.LoadMemoryFromFile(memoryFile)
	}
	memory.SetTaskContent(taskContent)
	model := misc.GetConfigValueDefault("decision", "MODEL", "")
	if model == "" {
		model = misc.GetConfigValueDefault("main_setting", "MODEL", "")
	}
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	ctx, cancel := context.WithCancel(context.Background())
	chatFile := filepath.Join(memoryDir, "chat.json")
	var chatMessages []ChatMessage
	if data, err := os.ReadFile(chatFile); err == nil {
		_ = json.Unmarshal(data, &chatMessages)
	}
	if chatMessages == nil {
		chatMessages = make([]ChatMessage, 0)
	}
	if len(chatMessages) > maxDecisionChatRecord {
		chatMessages = append([]ChatMessage(nil), chatMessages[len(chatMessages)-maxDecisionChatRecord:]...)
	}
	db := &DecisionBrain{memory: memory,
		agentHandler:       make(map[string]func(*taskManager.Task, string) (agents.Agent, error)),
		agentToolDefs:      make(map[string]llm.ToolDef),
		exploitIdeaList:    make([]*taskManager.ExploitIdea, 0),
		projectName:        projectName,
		memoryFilePath:     memoryFile,
		chatFilePath:       chatFile,
		chatMessages:       chatMessages,
		reportList:         make([]taskManager.ReportMeta, 0),
		envInfo:            make(map[string]interface{}),
		ctx:                ctx,
		cancelFunc:         cancel,
		cancelList:         make([]context.CancelFunc, 0),
		digitalHumanPool:   make(map[string][]digitalHumanEntry),
		digitalHumanBusy:   make(map[string]digitalHumanBusyInfo),
		digitalHumanQueue:  make(map[string][]digitalHumanQueuedReq),
		personaMemory:      make(map[string]string),
		lastSummary:        make(map[string]string),
		lastTask:           make(map[string]string),
		lastError:          make(map[string]string),
		lastArgs:           make(map[string]string),
		model:              model,
		runAgentList:       make([]*AgentRuntime, 0),
		wg:                 sync.WaitGroup{},
		mu:                 sync.Mutex{},
		notify:             make(chan bool, 100),
		done:               make(chan struct{}),
		webOutputChan:      webOutputChan,
		exploitChainList:   make([]*taskManager.ExploitChain, 0),
		eventList:          make([]string, 0),
		brainFeed:          make([]map[string]interface{}, 0),
		agentFeed:          make(map[string][]map[string]interface{}),
		envBuildCond:       cond,
		externalTools:      make([]llm.ToolDef, 0),
		externalToolCaller: make(map[string]func(map[string]interface{}) string),
		enableExploitFlow:  true,
	}
	store, err := NewExploitStore(projectName)
	if err != nil {
		misc.Warn("decision", "init sqlite store failed: "+err.Error(), db.SubmitEventHandler)
	} else {
		db.store = store
	}
	dm, err := taskManager.GetDockerManager(projectName)
	if err != nil {
		dm = dockerManager.NewDockerManager()
	}
	dm.SetEventHandler(db.SubmitContainerEventHandler)
	return db
}

func (db *DecisionBrain) personaMemoryKey(agentToolName string, profile agents.AgentProfile) string {
	return agentToolName + "|" + strings.TrimSpace(profile.PersonaName)
}

func (db *DecisionBrain) getPersonaMemory(agentToolName string, profile agents.AgentProfile) string {
	key := db.personaMemoryKey(agentToolName, profile)
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	return db.personaMemory[key]
}

func (db *DecisionBrain) setPersonaMemory(agentToolName string, profile agents.AgentProfile, mem string) {
	key := db.personaMemoryKey(agentToolName, profile)
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	db.personaMemory[key] = mem
}

func (db *DecisionBrain) SetOwnerScope(groupID string, groupName string) {
	db.poolMu.Lock()
	db.groupID = strings.TrimSpace(groupID)
	db.groupName = strings.TrimSpace(groupName)
	db.refreshExistingTaskBindingsLocked()
	db.poolMu.Unlock()
}

func (db *DecisionBrain) SetHostCommandRequestHandler(handler func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult) {
	db.poolMu.Lock()
	db.hostCommandHandler = handler
	db.refreshExistingTaskBindingsLocked()
	db.poolMu.Unlock()
}

func (db *DecisionBrain) SetDecisionApprovalHandler(handler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult) {
	db.poolMu.Lock()
	db.decisionApprovalHandler = handler
	db.refreshExistingTaskBindingsLocked()
	db.poolMu.Unlock()
}

func (db *DecisionBrain) refreshExistingTaskBindingsLocked() {
	for toolName, pool := range db.digitalHumanPool {
		for _, entry := range pool {
			if entry.Agent == nil {
				continue
			}
			task := entry.Agent.GetTask()
			db.applyTaskHandlers(task)
			db.applyTaskActor(task, toolName, entry.Profile)
		}
	}
	for _, busy := range db.digitalHumanBusy {
		if busy.Agent == nil {
			continue
		}
		task := busy.Agent.GetTask()
		db.applyTaskHandlers(task)
		db.applyTaskActor(task, busy.AgentToolName, busy.Profile)
	}
}

func (db *DecisionBrain) applyTaskHandlers(task *taskManager.Task) {
	if task == nil {
		return
	}
	db.mu.Lock()
	enableExploitFlow := db.enableExploitFlow
	db.mu.Unlock()
	//task.SetTaskDataHandler(db.SubmitReportWritingRequestHandler)
	task.SetReportHandler(db.AddReport)
	task.SetReportChunkReader(db.ReadReportChunk)
	task.SetEnvInfoHandler(db.SetEnvInfo)
	task.SetEventHandler(db.SubmitEventHandler)
	task.SetAgentFeedHandler(db.SubmitAgentFeedHandler)
	task.SetGlobalStatusPanelHandler(db.SetGlobalStatusPanel)
	if enableExploitFlow {
		task.SetCandidateExploitIdeaHandler(db.SubmitCandidateExploitIdeaHandler)
		task.SetExploitIdeaHandler(db.SubmitExploitIdeaHandler)
		task.SetExploitChainHandler(db.SubmitExploitChainHandler)
		task.SetExploitIdeaGetter(db.GetExploitIdeaById)
		task.SetExploitChainGetter(db.GetExploitChainById)
	} else {
		task.SetCandidateExploitIdeaHandler(nil)
		task.SetExploitIdeaHandler(nil)
		task.SetExploitChainHandler(nil)
		task.SetExploitIdeaGetter(nil)
		task.SetExploitChainGetter(nil)
	}
	task.SetGuidanceHandler(func(source, question string) string {
		return db.Guidance(source, question)
	})
	if db.hostCommandHandler != nil {
		task.SetHostCommandRequestHandler(func(req taskManager.HostCommandRequest) taskManager.HostCommandResult {
			return db.hostCommandHandler(db.ctx, task, req)
		})
	} else {
		task.SetHostCommandRequestHandler(nil)
	}
	if db.decisionApprovalHandler != nil {
		task.SetDecisionApprovalHandler(func(req taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult {
			return db.decisionApprovalHandler(db.ctx, task, req)
		})
	} else {
		task.SetDecisionApprovalHandler(nil)
	}
}

func (db *DecisionBrain) applyTaskActor(task *taskManager.Task, agentToolName string, profile agents.AgentProfile) {
	if task == nil {
		return
	}
	task.SetTaskActor(taskManager.TaskActor{
		AgentToolName:   strings.TrimSpace(agentToolName),
		AgentName:       strings.TrimSpace(agentToolName),
		DigitalHumanID:  strings.TrimSpace(profile.DigitalHumanID),
		PersonaName:     strings.TrimSpace(profile.PersonaName),
		AvatarFile:      strings.TrimSpace(profile.AvatarFile),
		GroupID:         strings.TrimSpace(db.groupID),
		GroupName:       strings.TrimSpace(db.groupName),
		RequesterSource: "digital_human",
	})
}

func (db *DecisionBrain) renderDynamicSystemPrompt(agentToolName string) (string, bool) {
	switch strings.TrimSpace(agentToolName) {
	case "Agent-Pentest-PentestCommonAgent":
		return pentestagents.BuildPentestCommonSystemPrompt(db.projectName), true
	case "Agent-Pentest-SrcCommonAgent":
		return pentestagents.BuildSrcCommonSystemPrompt(db.projectName), true
	case "Agent-Pentest-PostExploitationCommonAgent":
		return pentestagents.BuildPostExploitationCommonSystemPrompt(db.projectName), true
	default:
		return "", false
	}
}

func (db *DecisionBrain) renderSandboxAwareText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	host := misc.GetRuntimeServerHost()
	var ports []string
	if sb, err := taskManager.GetSandbox(db.projectName); err == nil && sb != nil {
		ports = sb.GetListenerPorts()
	}
	if strings.Contains(raw, "<serverip>") || strings.Contains(raw, "xxxxx") {
		return misc.RenderSandboxPromptTemplate(raw, host, ports)
	}
	return raw
}

func (db *DecisionBrain) buildDigitalHumanExtraPrompt(agentToolName string, profile agents.AgentProfile) string {
	extra := db.renderSandboxAwareText(profile.ExtraSysPrompt)
	return prompts.BuildDigitalHumanExtraPrompt(profile.PersonaName, profile.Gender, profile.Personality, profile.Age, extra)
}

func (db *DecisionBrain) applyDigitalHumanPrompts(agentToolName string, profile agents.AgentProfile, agent agents.Agent) {
	if agent == nil {
		return
	}
	task := agent.GetTask()
	if task == nil {
		return
	}
	memory := agent.GetMemory()
	if memory == nil {
		return
	}
	if systemPrompt, ok := db.renderDynamicSystemPrompt(agentToolName); ok && strings.TrimSpace(systemPrompt) != "" {
		memory.SetSystemPrompt(&llm.SystemPromptX{SystemPrompt: systemPrompt, ContextId: task.GetTaskId()})
	}
	if full := db.buildDigitalHumanExtraPrompt(agentToolName, profile); full != "" {
		memory.SetExtraSystemPrompt(full, task.GetTaskId())
	}
}

func (db *DecisionBrain) summarizePersonaMemory(agentToolName string, profile agents.AgentProfile, existing string, runTask string, runSummary string, ctxMsgs []llm.Message) string {
	// Keep only last N messages to control prompt size.
	maxMsgs := 30
	if len(ctxMsgs) > maxMsgs {
		ctxMsgs = ctxMsgs[len(ctxMsgs)-maxMsgs:]
	}
	trimmed := make([]map[string]string, 0, len(ctxMsgs))
	for _, m := range ctxMsgs {
		c := m.Content
		if len(c) > 800 {
			c = c[:800] + " ...[truncated]"
		}
		trimmed = append(trimmed, map[string]string{"role": m.Role, "content": c})
	}
	js, _ := json.Marshal(trimmed)

	sys := prompts.PersonaMemoryEditorSystemPrompt()

	user := "Persona: " + profile.PersonaName + " (" + profile.Gender + ")\n" +
		"Agent tool: " + agentToolName + "\n\n" +
		"Existing persona memory (may be empty):\n" + existing + "\n\n" +
		"This run task:\n" + runTask + "\n\n" +
		"This run summary:\n" + runSummary + "\n\n" +
		"Recent chat context (JSON array of {role,content}):\n" + string(js)

	ms := []llm.Message{
		{Role: llm.RoleSystem, Content: sys},
		{Role: llm.RoleUser, Content: user},
	}

	ctx, cancel := context.WithTimeout(db.ctx, 5*60*time.Second)
	defer cancel()
	resp, err := llm.RequestLLM(llm.GetResponsesClient("decision", "main_setting"), ctx, db.model, ms, nil, db.projectName)
	if err != nil {
		return existing
	}
	if resp.Content == "" {
		return existing
	}
	return strings.TrimSpace(resp.Content)
}

func (db *DecisionBrain) InitDigitalHumanPool(pool map[string][]agents.AgentProfile) {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	for agentToolName, profiles := range pool {
		entries := make([]digitalHumanEntry, 0, len(profiles))
		newFunc, exists := db.agentHandler[agentToolName]
		if !exists {
			misc.Debug("InitDigitalHumanPool: agentHandler not found for %s, skipping agent creation", agentToolName)
			for _, p := range profiles {
				entries = append(entries, digitalHumanEntry{Profile: p, Agent: nil})
			}
			db.digitalHumanPool[agentToolName] = entries
			continue
		}
		for _, p := range profiles {
			task := taskManager.NewTask(db.projectName)
			db.InitTask(task)
			agent, err := newFunc(task, "{}")
			if err != nil {
				misc.Debug("InitDigitalHumanPool: failed to create agent for %s/%s: %v", agentToolName, p.PersonaName, err)
				entries = append(entries, digitalHumanEntry{Profile: p, Agent: nil})
				continue
			}
			agent.SetProfile(p)
			db.applyTaskActor(task, agentToolName, p)
			db.applyDigitalHumanPrompts(agentToolName, p, agent)
			agent.SetStateHandler(db.AgentStateUpdate)
			// Start the long-running StartTask goroutine.
			ctx, cancel := context.WithCancel(db.ctx)
			db.ctxList = append(db.ctxList, ctx)
			db.cancelList = append(db.cancelList, cancel)
			go func(a agents.Agent, c context.Context) {
				a.StartTask(c)
			}(agent, ctx)
			misc.Debug("InitDigitalHumanPool: created persistent agent %s for %s (dhID=%s)", agent.GetId(), p.PersonaName, p.DigitalHumanID)
			entries = append(entries, digitalHumanEntry{Profile: p, Agent: agent})
		}
		db.digitalHumanPool[agentToolName] = entries
		if _, ok := db.digitalHumanQueue[agentToolName]; !ok {
			db.digitalHumanQueue[agentToolName] = make([]digitalHumanQueuedReq, 0)
		}
	}
	db.updateDigitalHumanRosterMemoryLocked()
}

func (db *DecisionBrain) ReplaceDigitalHumanPool(pool map[string][]agents.AgentProfile) {
	db.poolMu.Lock()
	db.digitalHumanPool = make(map[string][]digitalHumanEntry)
	db.digitalHumanBusy = make(map[string]digitalHumanBusyInfo)
	db.digitalHumanQueue = make(map[string][]digitalHumanQueuedReq)
	db.poolMu.Unlock()
	if db.lastSummary == nil {
		db.lastSummary = make(map[string]string)
	} else {
		db.lastSummary = make(map[string]string)
	}
	if db.lastTask == nil {
		db.lastTask = make(map[string]string)
	} else {
		db.lastTask = make(map[string]string)
	}
	if db.lastError == nil {
		db.lastError = make(map[string]string)
	} else {
		db.lastError = make(map[string]string)
	}
	if db.lastArgs == nil {
		db.lastArgs = make(map[string]string)
	} else {
		db.lastArgs = make(map[string]string)
	}
	if pool == nil {
		db.updateDigitalHumanRosterMemory()
		db.signal()
		return
	}
	db.InitDigitalHumanPool(pool)
	db.signal()
}

func (db *DecisionBrain) RefreshDynamicDigitalHumanPrompts() {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	for toolName, pool := range db.digitalHumanPool {
		for _, entry := range pool {
			db.applyDigitalHumanPrompts(toolName, entry.Profile, entry.Agent)
		}
	}
	for _, busy := range db.digitalHumanBusy {
		db.applyDigitalHumanPrompts(busy.AgentToolName, busy.Profile, busy.Agent)
	}
}

func (db *DecisionBrain) updateDigitalHumanRosterMemoryLocked() {
	// poolMu MUST be held.
	type rosterItem struct {
		DigitalHumanID string `json:"digital_human_id"`
		PersonaName    string `json:"persona_name"`
		Gender         string `json:"gender"`
		Personality    string `json:"personality"`
		Age            int    `json:"age"`
		AvatarFile     string `json:"avatar_file"`
		State          string `json:"state"` // idle|busy
		AgentID        string `json:"agent_id,omitempty"`
		QueueLength    int    `json:"queue_length"`
	}
	type memoryRosterItem struct {
		PersonaName string `json:"persona_name"`
		State       string `json:"state"`
		AgentID     string `json:"agent_id,omitempty"`
		QueueLength int    `json:"queue_length"`
	}

	busyByTool := make(map[string][]rosterItem)
	for aid, b := range db.digitalHumanBusy {
		dhID := strings.TrimSpace(b.Profile.DigitalHumanID)
		busyByTool[b.AgentToolName] = append(busyByTool[b.AgentToolName], rosterItem{
			DigitalHumanID: dhID,
			PersonaName:    strings.TrimSpace(b.Profile.PersonaName),
			Gender:         strings.TrimSpace(b.Profile.Gender),
			Personality:    strings.TrimSpace(b.Profile.Personality),
			Age:            b.Profile.Age,
			AvatarFile:     strings.TrimSpace(b.Profile.AvatarFile),
			State:          "busy",
			AgentID:        aid,
			QueueLength:    len(db.digitalHumanQueue[b.AgentToolName]),
		})
	}

	roster := make(map[string][]rosterItem)
	for toolName, pool := range db.digitalHumanPool {
		items := make([]rosterItem, 0, len(pool)+len(busyByTool[toolName]))
		for _, entry := range pool {
			p := entry.Profile
			items = append(items, rosterItem{
				DigitalHumanID: strings.TrimSpace(p.DigitalHumanID),
				PersonaName:    strings.TrimSpace(p.PersonaName),
				Gender:         strings.TrimSpace(p.Gender),
				Personality:    strings.TrimSpace(p.Personality),
				Age:            p.Age,
				AvatarFile:     strings.TrimSpace(p.AvatarFile),
				State:          "idle",
				QueueLength:    len(db.digitalHumanQueue[toolName]),
			})
		}
		items = append(items, busyByTool[toolName]...)
		roster[toolName] = items
	}
	// Ensure tools that are only busy (no idle pool left) still show up.
	for toolName, items := range busyByTool {
		if _, ok := roster[toolName]; ok {
			continue
		}
		roster[toolName] = items
	}

	if db.memory != nil {
		memoryRoster := make(map[string][]memoryRosterItem, len(roster))
		for toolName, items := range roster {
			compact := make([]memoryRosterItem, 0, len(items))
			for _, item := range items {
				compact = append(compact, memoryRosterItem{
					PersonaName: item.PersonaName,
					State:       item.State,
					AgentID:     item.AgentID,
					QueueLength: item.QueueLength,
				})
			}
			memoryRoster[toolName] = compact
		}
		db.memory.AddKeyMessage("DigitalHumanRoster", memoryRoster, false)
	}
	// Real-time panel update for UI
	if db.webOutputChan != nil {
		msg := WebMsg{Type: "DigitalHumanRosterUpdate", Data: roster, ProjectName: db.projectName}
		if b, err := json.Marshal(msg); err == nil {
			db.trySendWS(string(b))
		}
	}
}

func (db *DecisionBrain) GetDigitalHumanRoster() map[string]interface{} {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()

	type rosterItem struct {
		DigitalHumanID string `json:"digital_human_id"`
		PersonaName    string `json:"persona_name"`
		Gender         string `json:"gender"`
		Personality    string `json:"personality"`
		Age            int    `json:"age"`
		AvatarFile     string `json:"avatar_file"`
		State          string `json:"state"` // idle|busy
		AgentID        string `json:"agent_id,omitempty"`
		QueueLength    int    `json:"queue_length"`
		LastSummary    string `json:"last_summary,omitempty"`
		LastTask       string `json:"last_task,omitempty"`
		LastError      string `json:"last_error,omitempty"`
		ContextSize    int    `json:"context_size"`
		MaxContext     int    `json:"max_context"`
	}

	busyByTool := make(map[string][]rosterItem)
	for aid, b := range db.digitalHumanBusy {
		dhID := strings.TrimSpace(b.Profile.DigitalHumanID)
		var ctxSize, maxCtx int
		if b.Agent != nil {
			if mem := b.Agent.GetMemory(); mem != nil {
				ctxSize = mem.GetMsgSize(b.Agent.GetId())
				maxCtx = mem.GetMaxHistory()
			}
		}
		busyByTool[b.AgentToolName] = append(busyByTool[b.AgentToolName], rosterItem{
			DigitalHumanID: dhID,
			PersonaName:    strings.TrimSpace(b.Profile.PersonaName),
			Gender:         strings.TrimSpace(b.Profile.Gender),
			Personality:    strings.TrimSpace(b.Profile.Personality),
			Age:            b.Profile.Age,
			AvatarFile:     strings.TrimSpace(b.Profile.AvatarFile),
			State:          "busy",
			AgentID:        aid,
			QueueLength:    len(db.digitalHumanQueue[b.AgentToolName]),
			LastSummary:    db.lastSummary[dhID],
			LastTask:       db.lastTask[dhID],
			LastError:      db.lastError[dhID],
			ContextSize:    ctxSize,
			MaxContext:     maxCtx,
		})
	}

	roster := make(map[string][]rosterItem)
	for toolName, pool := range db.digitalHumanPool {
		items := make([]rosterItem, 0, len(pool)+len(busyByTool[toolName]))
		for _, entry := range pool {
			p := entry.Profile
			dhID := strings.TrimSpace(p.DigitalHumanID)
			var ctxSize, maxCtx int
			if entry.Agent != nil {
				if mem := entry.Agent.GetMemory(); mem != nil {
					ctxSize = mem.GetMsgSize(entry.Agent.GetId())
					maxCtx = mem.GetMaxHistory()
				}
			}
			items = append(items, rosterItem{
				DigitalHumanID: dhID,
				PersonaName:    strings.TrimSpace(p.PersonaName),
				Gender:         strings.TrimSpace(p.Gender),
				Personality:    strings.TrimSpace(p.Personality),
				Age:            p.Age,
				AvatarFile:     strings.TrimSpace(p.AvatarFile),
				State:          "idle",
				QueueLength:    len(db.digitalHumanQueue[toolName]),
				LastSummary:    db.lastSummary[dhID],
				LastTask:       db.lastTask[dhID],
				LastError:      db.lastError[dhID],
				ContextSize:    ctxSize,
				MaxContext:     maxCtx,
			})
		}
		items = append(items, busyByTool[toolName]...)
		roster[toolName] = items
	}
	for toolName, items := range busyByTool {
		if _, ok := roster[toolName]; ok {
			continue
		}
		roster[toolName] = items
	}

	out := make(map[string]interface{}, len(roster))
	for k, v := range roster {
		out[k] = v
	}
	return out
}

func digitalHumanIdentityKey(agentToolName string, profile agents.AgentProfile) string {
	if dhID := strings.TrimSpace(profile.DigitalHumanID); dhID != "" {
		return dhID
	}
	return strings.TrimSpace(agentToolName) + "::" + strings.TrimSpace(profile.PersonaName)
}

func agentCapabilitySummary(agentToolName string) (role string, capability string, collaborationHint string) {
	switch {
	case strings.Contains(agentToolName, "Analyze"):
		return "代码审计数字人", "擅长静态阅读源码、搜索调用链、定位可疑逻辑与高风险入口；不能执行命令或直接验证运行结果。", "当你已经定位到可疑代码、需要继续做运行时验证或环境操作时，应请决策大脑安排 Verifier 或 Ops。"
	case strings.Contains(agentToolName, "Verifier"):
		return "漏洞验证数字人", "擅长在已搭建环境中执行命令、复现漏洞、收集运行证据并判断可利用性。", "当你需要别人先搭环境、提供入口信息，或需要源码层面的深挖时，应请决策大脑协调 Ops、EnvScout 或 Analyze。"
	case strings.Contains(agentToolName, "Ops") && strings.Contains(agentToolName, "Scout"):
		return "环境侦察数字人", "擅长在现有环境中摸清 URL、端口、账号、路径、配置位置等运行情报，不负责深度漏洞分析。", "当你拿到环境情报后仍需要漏洞分析或验证，应请决策大脑安排 Analyze 或 Verifier。"
	case strings.Contains(agentToolName, "Ops"):
		return "环境运维数字人", "擅长搭建和维护测试环境、操作容器、安装依赖、准备数据库与运行条件。", "当环境已经就绪且需要分析源码或验证漏洞时，应请决策大脑安排 Analyze 或 Verifier。"
	case strings.Contains(agentToolName, "Report"):
		return "报告撰写数字人", "擅长基于已有证据与结论整理正式中文报告，不负责新的漏洞分析、逆向或环境操作。", "当你缺少足够事实、证据或结论时，应请决策大脑先安排对应技术成员补齐。"
	case strings.Contains(agentToolName, "PostExploitation"):
		return "后渗透数字人", "擅长处理命令执行会话、C2 上线、后渗透链路、持久化与横向移动相关分析，但不会凭空假设已经拿到 foothold。", "当还没有真实立足点、执行入口或目标面信息时，应请决策大脑先安排渗透测试数字人补齐前置条件；当任务更偏单目标逻辑漏洞或应用攻击面深挖时，应请决策大脑改派 SRC 数字人。"
	case strings.Contains(agentToolName, "SrcCommonAgent"):
		return "SRC深挖数字人", "擅长围绕单个目标做业务逻辑、授权边界、工作流绕过、隐藏功能与全面攻击面深挖。", "当任务更偏外部入口突破、初始 getshell，或已经进入 C2 / 横向 / 后渗透阶段时，应请决策大脑改派渗透测试数字人或后渗透数字人。"
	case strings.Contains(agentToolName, "Pentest"):
		return "渗透测试数字人", "擅长黑盒渗透中的入口探测、服务枚举、认证边界测试、弱口令、初始突破与外部攻击面研判。", "当任务进入单目标逻辑漏洞 / 攻击面深挖阶段时，应请决策大脑改派 SRC 数字人；当任务进入已获取立足点后的 C2、持久化或后渗透阶段时，应请决策大脑改派后渗透数字人。"
	case strings.Contains(agentToolName, "Android"):
		return "Android逆向数字人", "擅长 APK/DEX/JAR/AAB 静态分析，梳理导出组件、权限、IPC、硬编码密钥、动态加载与移动端攻击面。", "当任务更偏原生二进制逆向、环境验证或非 Android 代码审计时，应请决策大脑改派更合适的成员。"
	case strings.Contains(agentToolName, "Reverse"):
		return "二进制逆向数字人", "擅长原生二进制/样本逆向，分析函数行为、危险调用、控制流、利用面与可疑逻辑。", "当任务更偏 Android 包分析、源码审计或运行时验证时，应请决策大脑改派更合适的成员。"
	case strings.Contains(agentToolName, "ProjectOverview"):
		return "项目概览数字人", "擅长快速理解项目结构、技术栈与整体背景，帮助团队建立全局认知。", "当需要进入具体漏洞分析、验证或环境操作时，应请决策大脑安排对应的专长成员。"
	case strings.Contains(agentToolName, "General"):
		return "通用杂项数字人", "擅长处理通用脚本、仓库协作、杂项执行与跨领域的小型支撑任务。", "当任务已经明确属于环境、审计、验证、逆向或报告等专门领域时，应请决策大脑改派专业成员。"
	default:
		return "研究组数字人", "负责处理与自己工具能力相匹配的研究任务。", "当当前任务明显超出你的能力边界时，应立即向决策大脑说明并请求改派。"
	}
}

func (db *DecisionBrain) buildSameGroupMemberDirectory(selfToolName string, selfProfile agents.AgentProfile) map[string]interface{} {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()

	type memberItem struct {
		DigitalHumanID    string `json:"digital_human_id,omitempty"`
		PersonaName       string `json:"persona_name"`
		AgentToolName     string `json:"agent_tool_name"`
		Role              string `json:"role"`
		CapabilitySummary string `json:"capability_summary,omitempty"`
		CollaborationHint string `json:"collaboration_hint,omitempty"`
		State             string `json:"state"`
		QueueLength       int    `json:"queue_length"`
		IsSelf            bool   `json:"is_self"`
	}

	members := make([]memberItem, 0)
	seen := make(map[string]struct{})
	selfKey := digitalHumanIdentityKey(selfToolName, selfProfile)
	appendMember := func(agentToolName string, profile agents.AgentProfile, state string, queueLength int) {
		key := digitalHumanIdentityKey(agentToolName, profile)
		if _, exists := seen[key]; exists {
			return
		}
		role, capability, collaborationHint := agentCapabilitySummary(agentToolName)
		isSelf := key == selfKey
		if isSelf {
			capability = ""
			collaborationHint = ""
		}
		members = append(members, memberItem{
			DigitalHumanID:    strings.TrimSpace(profile.DigitalHumanID),
			PersonaName:       strings.TrimSpace(profile.PersonaName),
			AgentToolName:     strings.TrimSpace(agentToolName),
			Role:              role,
			CapabilitySummary: capability,
			CollaborationHint: collaborationHint,
			State:             state,
			QueueLength:       queueLength,
			IsSelf:            isSelf,
		})
		seen[key] = struct{}{}
	}

	for toolName, pool := range db.digitalHumanPool {
		for _, entry := range pool {
			appendMember(toolName, entry.Profile, "idle", len(db.digitalHumanQueue[toolName]))
		}
	}
	for _, busy := range db.digitalHumanBusy {
		appendMember(busy.AgentToolName, busy.Profile, "busy", len(db.digitalHumanQueue[busy.AgentToolName]))
	}

	sort.Slice(members, func(i, j int) bool {
		if members[i].Role != members[j].Role {
			return members[i].Role < members[j].Role
		}
		if members[i].PersonaName != members[j].PersonaName {
			return members[i].PersonaName < members[j].PersonaName
		}
		return members[i].AgentToolName < members[j].AgentToolName
	})

	selfRole, _, _ := agentCapabilitySummary(selfToolName)
	return map[string]interface{}{
		"usage": "这是你当前所在研究组的成员目录，重点用于了解其它同组成员能做什么。先看这里再判断任务是否适合自己。如果任务明显更适合同组其他成员，不要硬做；请用 <BrainMessage> 向决策大脑说明能力缺口，并点名请求更合适的成员协作或接手。",
		"current_member": map[string]interface{}{
			"persona_name":    strings.TrimSpace(selfProfile.PersonaName),
			"agent_tool_name": strings.TrimSpace(selfToolName),
			"role":            selfRole,
		},
		"member_count": len(members),
		"members":      members,
	}
}

func formatFollowUpChatMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "[Team Chat") || strings.HasPrefix(raw, "[BrainMessage") {
		return raw
	}
	return "[Team Chat from 用户 - to you] " + raw
}

func (db *DecisionBrain) injectSameGroupMemberDirectory(agent agents.Agent, agentToolName string, profile agents.AgentProfile) {
	if agent == nil || agent.GetMemory() == nil {
		return
	}
	directory := db.buildSameGroupMemberDirectory(agentToolName, profile)
	agent.GetMemory().AddKeyMessage(&llm.EnvMessageX{
		Key:       "SameGroupMemberDirectory",
		Content:   directory,
		AppendEnv: false,
	})
}

func (db *DecisionBrain) updateDigitalHumanRosterMemory() {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	db.updateDigitalHumanRosterMemoryLocked()
	db.signal()
}

func (db *DecisionBrain) acquireDigitalHuman(agentToolName string) (digitalHumanEntry, bool) {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	pool := db.digitalHumanPool[agentToolName]
	if len(pool) == 0 {
		return digitalHumanEntry{}, false
	}
	entry := pool[0]
	db.digitalHumanPool[agentToolName] = pool[1:]
	return entry, true
}

func (db *DecisionBrain) acquireDigitalHumanByID(agentToolName string, dhID string) (digitalHumanEntry, bool) {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	pool := db.digitalHumanPool[agentToolName]
	for i, entry := range pool {
		if strings.TrimSpace(entry.Profile.DigitalHumanID) == dhID {
			db.digitalHumanPool[agentToolName] = append(pool[:i], pool[i+1:]...)
			misc.Debug("acquireDigitalHumanByID: 成功获取 personaName=%s, dhID=%s", entry.Profile.PersonaName, entry.Profile.DigitalHumanID)
			return entry, true
		}
	}
	misc.Debug("acquireDigitalHumanByID: 未找到 dhID=%s in pool for %s", dhID, agentToolName)
	return digitalHumanEntry{}, false
}

func (db *DecisionBrain) releaseDigitalHuman(agentID string) {
	db.poolMu.Lock()
	busy, ok := db.digitalHumanBusy[agentID]
	if !ok {
		db.poolMu.Unlock()
		return
	}
	delete(db.digitalHumanBusy, agentID)
	db.digitalHumanPool[busy.AgentToolName] = append(db.digitalHumanPool[busy.AgentToolName], digitalHumanEntry{Profile: busy.Profile, Agent: busy.Agent})

	q := db.digitalHumanQueue[busy.AgentToolName]
	if len(q) == 0 {
		db.updateDigitalHumanRosterMemoryLocked()
		db.poolMu.Unlock()
		return
	}
	// Dequeue one and try to start it.
	next := q[0]
	if len(q) == 1 {
		db.digitalHumanQueue[busy.AgentToolName] = nil
	} else {
		db.digitalHumanQueue[busy.AgentToolName] = q[1:]
	}
	db.updateDigitalHumanRosterMemoryLocked()
	db.poolMu.Unlock()

	go func() {
		startMsg := db.startAgent(next.AgentToolName, next.Args)
		// If this is a Verifier task that was queued, update the ExploitIdea/Chain
		// state from "等待验证" to "正在验证" now that the agent is actually running.
		if strings.Contains(next.AgentToolName, "Verifier") && strings.Contains(startMsg, "Agent ran successfully") {
			var argsMap map[string]interface{}
			if misc.UnmarshalJSONLenient(next.Args, &argsMap) == nil {
				if eid, ok := argsMap["exploit_idea_id"]; ok && fmt.Sprint(eid) != "" {
					if ei, err := db.GetExploitIdeaById(fmt.Sprint(eid)); err == nil && ei.State == "等待验证" {
						ei.State = "正在验证"
						misc.Debug("releaseDigitalHuman: 排队任务启动成功，ExploitIdea %s 状态更新为正在验证", fmt.Sprint(eid))
						db.flushExploitIdeaList()
					}
				}
				if cid, ok := argsMap["exploit_chain_id"]; ok && fmt.Sprint(cid) != "" {
					if ec, err := db.GetExploitChainById(fmt.Sprint(cid)); err == nil && ec.State == "等待验证" {
						ec.State = "正在验证"
						misc.Debug("releaseDigitalHuman: 排队任务启动成功，ExploitChain %s 状态更新为正在验证", fmt.Sprint(cid))
						db.flushExploitChainList()
					}
				}
			}
		}
	}()
}

func (db *DecisionBrain) Stop() {
	select {
	case <-db.done:
		return
	default:
		close(db.done)
	}
	if db.cancelFunc != nil {
		db.cancelFunc()
	}
	db.setWaiting(false)
	for _, c := range db.cancelList {
		if c != nil {
			c()
		}
	}
	// Wake up any goroutines waiting for env build.
	if db.envBuildCond != nil {
		db.envBuildCond.L.Lock()
		db.envBuildCond.Broadcast()
		db.envBuildCond.L.Unlock()
	}
	if db.store != nil {
		_ = db.store.Close()
	}
	db.chatMu.Lock()
	if db.chatPersistTimer != nil {
		db.chatPersistTimer.Stop()
		db.chatPersistTimer = nil
	}
	db.saveChatMessagesLocked()
	db.lastChatPersistAt = time.Now()
	db.chatMu.Unlock()
}

func (db *DecisionBrain) signal() {
	db.setWaiting(false)
	select {
	case db.notify <- true:
	default:
	}
}

func (db *DecisionBrain) nudgeAllIdleDecisionBrain(reason string) {
	if db == nil || db.memory == nil || db.brainFinished {
		return
	}
	if !db.hasRegisteredAgents() || db.hasRunningAgents() {
		return
	}
	reportCount := len(db.GetReportList())
	reportGuidance := "ReportCatalog currently has no report. If the current task has reached a meaningful conclusion, dispatch a suitable digital human to submit at least one Chinese Markdown report via ReportVulnTool before completion."
	if reportCount > 0 {
		reportGuidance = fmt.Sprintf("ReportCatalog currently has %d report(s). If the current milestone is done, send a concise update through Tool-ReportToDecisionMakers before waiting or finishing.", reportCount)
	}
	content := strings.TrimSpace(fmt.Sprintf(`[System Nudge] All digital humans are idle, but the group has not been formally closed.
Reason: %s

Required next action:
1. If more analysis or validation is needed, dispatch the appropriate digital human now.
2. %s
3. After a meaningful milestone, call Tool-ReportToDecisionMakers so the decision-makers channel receives the update.
4. Do NOT stay idle or call Tool-Wait while everyone is idle unless you have already reported a blocker or completion.`, strings.TrimSpace(reason), reportGuidance))
	db.memory.AddMessage(llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	})
	db.signal()
}

func (db *DecisionBrain) SetStatusHandler(h func(status string)) {
	db.statusHandler = h
}

func (db *DecisionBrain) SetWaitStateHandler(h func(waiting bool)) {
	db.mu.Lock()
	db.waitStateHandler = h
	waiting := db.waiting
	db.mu.Unlock()
	if h != nil {
		h(waiting)
	}
}

func (db *DecisionBrain) SetExploitAutomationPolicyResolver(resolver func() ExploitAutomationPolicy) {
	db.mu.Lock()
	db.exploitPolicy = resolver
	db.mu.Unlock()
}

func (db *DecisionBrain) currentExploitAutomationPolicy() ExploitAutomationPolicy {
	db.mu.Lock()
	resolver := db.exploitPolicy
	db.mu.Unlock()
	if resolver == nil {
		return DefaultExploitAutomationPolicy()
	}
	return resolver()
}

// SetProjectOverview injects the project overview (language, framework, tech stack, etc.)
// into the brain's key messages so it is available from the very first decision.
func (db *DecisionBrain) SetProjectOverview(overview string) {
	db.memory.AddKeyMessage("ProjectOverview", overview, false)
}

func (db *DecisionBrain) IsBrainFinished() bool {
	return db.brainFinished
}

func (db *DecisionBrain) IsWaiting() bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.waiting
}

func (db *DecisionBrain) setWaiting(waiting bool) {
	db.mu.Lock()
	if db.waiting == waiting {
		db.mu.Unlock()
		return
	}
	db.waiting = waiting
	handler := db.waitStateHandler
	db.mu.Unlock()
	if handler != nil {
		handler(waiting)
	}
}

func (db *DecisionBrain) setBrainFinished(v bool) {
	if db.brainFinished == v {
		return
	}
	db.brainFinished = v
	if v {
		db.setWaiting(false)
	}
	if db.statusHandler != nil {
		if v {
			db.statusHandler("决策结束")
		} else {
			db.statusHandler("正在运行")
		}
	}
	if db.webOutputChan != nil {
		statusMsg := WebMsg{Type: "BrainFinished", Data: map[string]interface{}{
			"brain_finished": v,
		}, ProjectName: db.projectName}
		if b, err := json.Marshal(statusMsg); err == nil {
			db.trySendWS(string(b))
		}
	}
}

func (db *DecisionBrain) RestartAfterFinished() {
	db.setBrainFinished(false)
}

func (db *DecisionBrain) RestoreBrainFinished(finished bool) {
	db.brainFinished = finished
}

func (db *DecisionBrain) SetChatObserver(observer func(ChatMessage)) {
	db.chatObserver = observer
}

func (db *DecisionBrain) SetReportObserver(observer func(taskManager.ReportMeta)) {
	db.reportObserver = observer
}

func (db *DecisionBrain) SetSystemPrompt(prompt string) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || db.memory == nil {
		return
	}
	db.memory.SetSystemPrompt(prompt)
	if db.memoryFilePath != "" {
		_ = db.memory.SaveMemoryToFile(db.memoryFilePath)
	}
	db.signal()
}

func (db *DecisionBrain) SetExploitWorkflowEnabled(enabled bool) {
	db.mu.Lock()
	db.enableExploitFlow = enabled
	db.mu.Unlock()
	if db.memory != nil {
		db.memory.SetExploitWorkflowEnabled(enabled)
	}
	db.poolMu.Lock()
	db.refreshExistingTaskBindingsLocked()
	db.poolMu.Unlock()
	db.signal()
}

func (db *DecisionBrain) RegisterExternalTool(name string, description string, parameters map[string]interface{}, handler func(map[string]interface{}) string) {
	name = strings.TrimSpace(name)
	if name == "" || handler == nil {
		return
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	updated := false
	for i := range db.externalTools {
		if db.externalTools[i].Name != name {
			continue
		}
		db.externalTools[i] = llm.ToolDef{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		}
		updated = true
		break
	}
	if !updated {
		db.externalTools = append(db.externalTools, llm.ToolDef{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		})
	}
	db.externalToolCaller[name] = handler
}

func (db *DecisionBrain) getExternalTools() []llm.ToolDef {
	db.mu.Lock()
	defer db.mu.Unlock()
	out := make([]llm.ToolDef, len(db.externalTools))
	copy(out, db.externalTools)
	return out
}

func (db *DecisionBrain) callExternalTool(name string, args map[string]interface{}) (string, bool) {
	db.mu.Lock()
	handler := db.externalToolCaller[name]
	db.mu.Unlock()
	if handler == nil {
		return "", false
	}
	return handler(args), true
}

func (db *DecisionBrain) hasRunningAgents() bool {
	return len(db.getActiveAgentIDs()) > 0
}

func (db *DecisionBrain) getActiveAgentIDs() []string {
	var ids []string
	for _, r := range db.runAgentList {
		if r == nil || r.agent == nil {
			continue
		}
		if r.done {
			continue
		}
		// Verifier agents may be queued waiting for env build and not yet "Running".
		// As long as the runtime is not done and the agent hasn't reached a terminal state,
		// we should treat it as active to prevent the decision brain from exiting early.
		st := strings.ToLower(strings.TrimSpace(r.agent.GetState()))
		if st != "done" && st != "completed" && st != "success" {
			ids = append(ids, r.agent.GetId())
		}
	}
	return ids
}

func (db *DecisionBrain) InitTask(task *taskManager.Task) {
	db.applyTaskHandlers(task)
	m := llm.NewContextManager()
	m.SetEventHandler(db.SubmitEventHandler)
	task.SetMemory(m)
}
func (db *DecisionBrain) RegisterAgent(name string, fc func(*taskManager.Task, string) (agents.Agent, error)) {
	db.agentHandler[name] = fc
	if _, exists := db.agentToolDefs[name]; !exists {
		db.agentToolDefs[name] = llm.ToolDef{
			Name:        name,
			Description: name,
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}
	}
}

func (db *DecisionBrain) ReplaceAgents(defs []agents.AgentDefinition) {
	next := make(map[string]func(*taskManager.Task, string) (agents.Agent, error), len(defs))
	nextDefs := make(map[string]llm.ToolDef, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.Name) == "" || def.NewFunc == nil {
			continue
		}
		next[def.Name] = def.NewFunc
		nextDefs[def.Name] = llm.ToolDef{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.Args.ToMap(),
		}
	}
	db.agentHandler = next
	db.agentToolDefs = nextDefs
	db.signal()
}

func (db *DecisionBrain) hasRegisteredAgents() bool {
	return len(db.agentHandler) > 0
}

func (db *DecisionBrain) hasDigitalHumans() bool {
	db.poolMu.Lock()
	defer db.poolMu.Unlock()
	if len(db.digitalHumanPool) > 0 {
		return true
	}
	return len(db.digitalHumanBusy) > 0
}

// 这个是候选idea提交通道（未经验证的）
func (db *DecisionBrain) SubmitCandidateExploitIdeaHandler(exploitIdea *taskManager.ExploitIdea) error {
	if exploitIdea == nil {
		return fmt.Errorf("exploitIdea is nil")
	}
	if strings.TrimSpace(exploitIdea.Harm) == "" {
		return fmt.Errorf("harm is required")
	}
	if strings.TrimSpace(exploitIdea.ExtendIdea) == "" {
		return fmt.Errorf("extendIdea is required")
	}
	if strings.TrimSpace(exploitIdea.Condition) == "" {
		return fmt.Errorf("condition is required")
	}
	if exploitIdea.ExploitPoint == nil {
		return fmt.Errorf("exploitPoint is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.Title) == "" {
		return fmt.Errorf("exploitPoint.title is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.RouteOrEndpoint) == "" {
		return fmt.Errorf("exploitPoint.route_or_endpoint is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.File) == "" {
		return fmt.Errorf("exploitPoint.file is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.FunctionOrMethod) == "" {
		return fmt.Errorf("exploitPoint.function_or_method is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.Params) == "" {
		return fmt.Errorf("exploitPoint.params is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.Type) == "" {
		return fmt.Errorf("exploitPoint.type is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.PayloadIdea) == "" {
		return fmt.Errorf("exploitPoint.payload_idea is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.ExpectedImpact) == "" {
		return fmt.Errorf("exploitPoint.expected_impact is required")
	}
	if strings.TrimSpace(exploitIdea.ExploitPoint.Confidence) == "" {
		return fmt.Errorf("exploitPoint.confidence is required")
	}
	index := 0
	for {
		id := fmt.Sprintf("E.%d", index)
		exists := false
		for _, v := range db.exploitIdeaList {
			if v.ExploitIdeaId == id {
				exists = true
				break
			}
		}
		if exists {
			index++
			continue
		}
		exploitIdea.ExploitIdeaId = id
		exploitIdea.State = "未验证"
		db.exploitIdeaList = append(db.exploitIdeaList, exploitIdea)
		db.flushExploitIdeaList()
		// Broadcast updated exploitIdea list to all running analysis agents so they avoid duplicate mining.
		db.broadcastExploitIdeaSummaryToAnalyzers()
		policy := db.currentExploitAutomationPolicy()
		if policy.AutoPublishAnalyzeReport {
			db.startAnalyzeReportForTarget("exploit_idea_id", exploitIdea.ExploitIdeaId)
		}
		if policy.AutoVerifyExploits {
			if err := db.VerifyExploitIdea(exploitIdea.ExploitIdeaId); err != nil {
				return err
			}
		}
		return nil
	}
}

// buildExploitIdeaSummaryJSON returns a compact JSON summary of all discovered exploitIdeas.
func (db *DecisionBrain) buildExploitIdeaSummaryJSON() string {
	type ideaBrief struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
		File  string `json:"file"`
		Route string `json:"route"`
		State string `json:"state"`
	}
	briefs := make([]ideaBrief, 0, len(db.exploitIdeaList))
	for _, ei := range db.exploitIdeaList {
		b := ideaBrief{ID: ei.ExploitIdeaId, State: ei.State}
		if ei.ExploitPoint != nil {
			b.Title = ei.ExploitPoint.Title
			b.Type = ei.ExploitPoint.Type
			b.File = ei.ExploitPoint.File
			b.Route = ei.ExploitPoint.RouteOrEndpoint
		}
		briefs = append(briefs, b)
	}
	js, err := json.Marshal(briefs)
	if err != nil {
		return "[]"
	}
	return string(js)
}

// injectExploitIdeaSummary injects the current exploitIdea summary into a single agent's keyMessage.
func (db *DecisionBrain) injectExploitIdeaSummary(a agents.Agent) {
	summary := db.buildExploitIdeaSummaryJSON()
	a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
		Key:       "DiscoveredExploitIdeas",
		Content:   summary,
		AppendEnv: false,
	})
}

// broadcastExploitIdeaSummaryToAnalyzers builds a compact summary of all discovered
// exploitIdeas and injects it into every running analysis agent's keyMessage so they
// can see what has already been found and avoid duplicate mining.
func (db *DecisionBrain) broadcastExploitIdeaSummaryToAnalyzers() {
	summary := db.buildExploitIdeaSummaryJSON()

	// Find all running analysis agents and inject the summary.
	db.mu.Lock()
	var analyzeAgents []agents.Agent
	for _, ar := range db.runAgentList {
		if ar != nil && ar.agent != nil && !ar.done && strings.Contains(ar.agentToolName, "Analyze") {
			analyzeAgents = append(analyzeAgents, ar.agent)
		}
	}
	db.mu.Unlock()

	for _, a := range analyzeAgents {
		a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
			Key:       "DiscoveredExploitIdeas",
			Content:   summary,
			AppendEnv: false,
		})
	}
}

// 这个是验证后idea提交通道，需要审核提交的证据是否幻觉
func (db *DecisionBrain) SubmitExploitIdeaHandler(exploitIdeaId string, exploitIdeaStatus string, evidence string, poc string) error {
	var exploitIdea *taskManager.ExploitIdea
	for _, exploitIdea1 := range db.exploitIdeaList {
		if exploitIdea1.ExploitIdeaId == exploitIdeaId {
			exploitIdea = exploitIdea1
			break
		}
	}
	if exploitIdea == nil {
		return fmt.Errorf("exploitIdeaId %s not exist", exploitIdeaId)
	}
	if exploitIdea.ExploitPoint == nil {
		return fmt.Errorf("exploitIdeaId %s exploitPoint is empty", exploitIdeaId)
	}
	exploitIdea.Evidence = evidence
	exploitIdea.Poc = poc
	if exploitIdeaStatus == "Completed" {
		e := db.auditEvidence(evidence, exploitIdea.Condition, exploitIdea.Harm)
		if e != nil {
			exploitIdea.State = "审核失败，正在整改"
			exploitIdea.ReviewReason = e.Error()
			db.flushExploitIdeaList()
			return e
		}
		exploitIdea.State = "可利用"
		if db.store != nil {
			_ = db.store.UpsertExploitableIdea(exploitIdea)
		}
	} else {
		exploitIdea.State = "验证失败"
	}
	db.flushExploitIdeaList()
	return nil
}

// 这个是验证后利用链提交通道，需要审核提交的证据是否幻觉
func (db *DecisionBrain) SubmitExploitChainHandler(exploitChainId string, exploitChainStatus string, evidence string, poc string) error {
	exploitChain, err := db.GetExploitChainById(exploitChainId)
	if err != nil {
		return err
	}
	exploitChain.Evidence = evidence
	exploitChain.Poc = poc
	if exploitChainStatus == "Completed" {
		e := db.auditEvidence(evidence, "", "")
		if e != nil {
			exploitChain.State = "审核失败，正在改进后重新提交"
			exploitChain.ReviewReason = e.Error()
			db.flushExploitChainList()
			return e
		}
		exploitChain.State = "可利用"
		if db.store != nil {
			_ = db.store.UpsertExploitableChain(exploitChain)
		}
		db.startVerifiedReportForExploitChain(exploitChain)
	} else {
		exploitChain.State = "验证失败"
	}
	db.flushExploitChainList()
	return nil
}

//func (db *DecisionBrain) SubmitReportWritingRequestHandler(request taskManager.ReportWritingRequest) {
//
//}

func (db *DecisionBrain) SubmitContainerEventHandler(infoJson string) {
	db.containerInfoList = append(db.containerInfoList, infoJson)
	// Avoid double-serialization: each element is already a JSON string,
	// so wrap them as json.RawMessage before marshaling the array.
	raw := make([]json.RawMessage, 0, len(db.containerInfoList))
	for _, s := range db.containerInfoList {
		raw = append(raw, json.RawMessage(s))
	}
	js, _ := json.Marshal(raw)
	db.memory.SetContainerListInfo(string(js))
	db.AddContainerInfo(infoJson)
}
func (db *DecisionBrain) AddContainerInfo(infoStr string) {
	var containerInfo taskManager.ContainerInfo
	err := json.Unmarshal([]byte(infoStr), &containerInfo)
	if err != nil {
		misc.Warn("容器事件管理", "格式不正确："+infoStr, db.SubmitEventHandler)
	}
	if containerInfo.Type == "Remove" {
		result := make([]*taskManager.ContainerInfo, 0, len(db.containerList))
		for _, container := range db.containerList {
			if container.ContainerId != containerInfo.ContainerId {
				result = append(result, container)
			}
		}
		db.containerList = result
		s := WebMsg{Type: "ContainerRemove", Data: map[string]interface{}{"containerId": containerInfo.ContainerId}, ProjectName: db.projectName}
		js, _ := json.Marshal(s)
		db.trySendWS(string(js))
		misc.Warn("容器事件", fmt.Sprintf("删除容器：%s", infoStr), db.SubmitEventHandler)
		return
	}

	// Create/Add: update container list (dedupe)
	if containerInfo.ContainerId != "" {
		exists := false
		for i := range db.containerList {
			c := db.containerList[i]
			if c != nil && c.ContainerId == containerInfo.ContainerId {
				// Update fields in case we get a second Create for same id
				*c = containerInfo
				exists = true
				break
			}
		}
		if !exists {
			ci := containerInfo
			db.containerList = append(db.containerList, &ci)
		}
	}

	s := WebMsg{Type: "ContainerAdd", Data: containerInfo, ProjectName: db.projectName}
	js, _ := json.Marshal(s)
	db.trySendWS(string(js))
	misc.Success("容器事件", fmt.Sprintf("新容器：%s", infoStr), db.SubmitEventHandler)
}

func (db *DecisionBrain) SubmitEventHandler(mod string, msg string, level int) {
	if !shouldPublishVisibleEvent(mod, level) {
		return
	}
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s][%s]: %s", timeStr, mod, msg)
	db.feedMu.Lock()
	db.eventList = append(db.eventList, line)
	if len(db.eventList) > maxDecisionEventList {
		db.eventList = append([]string(nil), db.eventList[len(db.eventList)-maxDecisionEventList:]...)
	}
	db.feedMu.Unlock()
	s := WebMsg{Type: "string", Data: line, ProjectName: db.projectName}
	js, _ := json.Marshal(s)
	db.trySendWS(string(js))
}

func shouldPublishVisibleEvent(mod string, level int) bool {
	if level <= 0 {
		return false
	}
	return strings.TrimSpace(mod) != "记忆体"
}

func (db *DecisionBrain) flushExploitIdeaList() {
	type exploitIdeaPanelItem struct {
		ExploitIdeaId string                    `json:"exploitIdeaId"`
		Harm          string                    `json:"harm"`
		ExtendIdea    string                    `json:"extend_idea"`
		Condition     string                    `json:"condition"`
		State         string                    `json:"state"`
		ExploitPoint  *taskManager.ExploitPoint `json:"exploit_point"`
	}
	items := make([]exploitIdeaPanelItem, 0, len(db.exploitIdeaList))
	for _, e := range db.exploitIdeaList {
		if e == nil {
			continue
		}
		items = append(items, exploitIdeaPanelItem{
			ExploitIdeaId: e.ExploitIdeaId,
			Harm:          e.Harm,
			ExtendIdea:    e.ExtendIdea,
			Condition:     e.Condition,
			State:         e.State,
			ExploitPoint:  e.ExploitPoint,
		})
	}
	js, _ := json.Marshal(items)
	db.memory.SetExploitIdeaList(string(js))
	db.signal()

	// Real-time panel update for UI
	msg := WebMsg{Type: "ExploitIdeaList", Data: items, ProjectName: db.projectName}
	if b, err := json.Marshal(msg); err == nil {
		db.trySendWS(string(b))
	}
}

func (db *DecisionBrain) flushExploitChainList() {
	type exploitIdeaPanelItem struct {
		ExploitIdeaId string                    `json:"exploitIdeaId"`
		Harm          string                    `json:"harm"`
		ExtendIdea    string                    `json:"extend_idea"`
		Condition     string                    `json:"condition"`
		State         string                    `json:"state"`
		ExploitPoint  *taskManager.ExploitPoint `json:"exploit_point"`
	}
	type exploitChainPanelItem struct {
		ExploitIdea    []exploitIdeaPanelItem `json:"exploit_idea"`
		Idea           string                 `json:"idea"`
		State          string                 `json:"state"`
		ExploitChainId string                 `json:"exploit_chain_id"`
	}
	items := make([]exploitChainPanelItem, 0, len(db.exploitChainList))
	for _, ec := range db.exploitChainList {
		if ec == nil {
			continue
		}
		eideas := make([]exploitIdeaPanelItem, 0, len(ec.ExploitIdea))
		for _, e := range ec.ExploitIdea {
			if e == nil {
				continue
			}
			eideas = append(eideas, exploitIdeaPanelItem{
				ExploitIdeaId: e.ExploitIdeaId,
				Harm:          e.Harm,
				ExtendIdea:    e.ExtendIdea,
				Condition:     e.Condition,
				State:         e.State,
				ExploitPoint:  e.ExploitPoint,
			})
		}
		items = append(items, exploitChainPanelItem{
			ExploitIdea:    eideas,
			Idea:           ec.Idea,
			State:          ec.State,
			ExploitChainId: ec.ExploitChainId,
		})
	}
	js, _ := json.Marshal(items)
	db.memory.SetExploitChainList(string(js))
	db.signal()

	// Real-time panel update for UI
	msg := WebMsg{Type: "ExploitChainList", Data: items, ProjectName: db.projectName}
	if b, err := json.Marshal(msg); err == nil {
		db.trySendWS(string(b))
	}
}

func (db *DecisionBrain) startAgent(name string, args string) string {
	return db.startAgentWithFollowUp(name, args, "", "")
}

func (db *DecisionBrain) startAgentWithFollowUp(name string, args string, followUpUserMsg string, preferDHID string) string {
	_, exists := db.agentHandler[name]
	if !exists {
		misc.Debug("找不到Agent %s", name)
		return fmt.Errorf("Agent %s not found", name).Error()
	}
	args = db.sanitizeArgsForDigitalHumanTool(name, args)
	followUpUserMsg = db.sanitizeTextForDigitalHumanTool(name, followUpUserMsg)
	var entry digitalHumanEntry
	var ok bool
	if preferDHID != "" {
		misc.Debug("startAgentWithFollowUp: 尝试按ID获取数字人 preferDHID=%s, toolName=%s", preferDHID, name)
		entry, ok = db.acquireDigitalHumanByID(name, preferDHID)
	}
	if !ok {
		misc.Debug("startAgentWithFollowUp: 按ID未获取到，使用默认获取 toolName=%s", name)
		entry, ok = db.acquireDigitalHuman(name)
	}
	if !ok {
		// No idle digital human for this agent tool, enqueue.
		db.poolMu.Lock()
		db.digitalHumanQueue[name] = append(db.digitalHumanQueue[name], digitalHumanQueuedReq{AgentToolName: name, Args: args})
		queuedLen := len(db.digitalHumanQueue[name])
		db.updateDigitalHumanRosterMemoryLocked()
		db.poolMu.Unlock()
		return fmt.Sprintf("No idle digital human for %s. Task queued. Queue length: %d", name, queuedLen)
	}
	profile := entry.Profile
	agent := entry.Agent
	if agent == nil {
		misc.Debug("startAgentWithFollowUp: agent is nil for %s/%s, returning entry to pool", name, profile.PersonaName)
		db.poolMu.Lock()
		db.digitalHumanPool[name] = append(db.digitalHumanPool[name], entry)
		db.updateDigitalHumanRosterMemoryLocked()
		db.poolMu.Unlock()
		return fmt.Sprintf("Agent instance not available for %s/%s", name, profile.PersonaName)
	}
	misc.Debug("startAgentWithFollowUp: 获取到持久化Agent personaName=%s, dhID=%s, agentId=%s", profile.PersonaName, profile.DigitalHumanID, agent.GetId())
	task := agent.GetTask()
	db.applyTaskActor(task, name, profile)
	senderPersonaName := profile.PersonaName
	senderAvatarFile := profile.AvatarFile
	// Inject persona memory summary (if any) so this digital human remembers past work.
	if mem := db.getPersonaMemory(name, profile); strings.TrimSpace(mem) != "" {
		agent.GetMemory().AddKeyMessage(&llm.EnvMessageX{
			Key:       "PersonaMemory",
			Content:   db.sanitizeTextForDigitalHumanTool(name, mem),
			AppendEnv: false,
		})
	}
	db.injectReportCatalog(agent, name)
	db.injectGlobalStatusPanel(agent, name)
	// Override ExploitIdeaHandler per-agent so audit failure generates chat messages.
	task.SetExploitIdeaHandler(func(exploitIdeaId, exploitIdeaStatus, evidence, poc string) error {
		err := db.SubmitExploitIdeaHandler(exploitIdeaId, exploitIdeaStatus, evidence, poc)
		if err != nil {
			// Digital human → Decision Brain: report audit failure
			dhMsg := fmt.Sprintf("@决策大脑 我提交的 ExploitIdea %s 证据审核未通过，原因：%s。我将根据审核意见整改后重新提交。", exploitIdeaId, err.Error())
			db.BroadcastChatMessage(ChatMessage{Role: "system", Text: dhMsg, Ts: time.Now().Format("15:04:05"), PersonaName: senderPersonaName, AvatarFile: senderAvatarFile})
			// Decision Brain → Digital human: instruct to fix
			brainMsg := fmt.Sprintf("@%s 收到，ExploitIdea %s 的证据审核未通过。请根据审核意见补充真实的运行时证据后重新提交，不要编造或臆测证据内容。", senderPersonaName, exploitIdeaId)
			db.BroadcastChatMessage(ChatMessage{Role: "system", Text: brainMsg, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
		}
		return err
	})
	// Override ExploitChainHandler per-agent so audit failure generates chat messages.
	task.SetExploitChainHandler(func(exploitChainId, exploitChainStatus, evidence, poc string) error {
		err := db.SubmitExploitChainHandler(exploitChainId, exploitChainStatus, evidence, poc)
		if err != nil {
			dhMsg := fmt.Sprintf("@决策大脑 我提交的 ExploitChain %s 证据审核未通过，原因：%s。我将根据审核意见整改后重新提交。", exploitChainId, err.Error())
			db.BroadcastChatMessage(ChatMessage{Role: "system", Text: dhMsg, Ts: time.Now().Format("15:04:05"), PersonaName: senderPersonaName, AvatarFile: senderAvatarFile})
			brainMsg := fmt.Sprintf("@%s 收到，ExploitChain %s 的证据审核未通过。请根据审核意见补充真实的运行时证据后重新提交，不要编造或臆测证据内容。", senderPersonaName, exploitChainId)
			db.BroadcastChatMessage(ChatMessage{Role: "system", Text: brainMsg, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
		}
		return err
	})
	// Override GuidanceHandler per-agent so we can show Q&A in chat with identity.
	task.SetGuidanceHandler(func(source, q string) string {
		qText := "@决策大脑 " + q
		db.AppendChatMessage(ChatMessage{Role: "system", Text: qText, Ts: time.Now().Format("15:04:05"), PersonaName: senderPersonaName, AvatarFile: senderAvatarFile})
		if db.webOutputChan != nil {
			qMsg := WebMsg{Type: "UserMessage", Data: map[string]interface{}{
				"persona_name": senderPersonaName,
				"avatar_file":  senderAvatarFile,
				"agent_id":     agent.GetId(),
				"message":      qText,
			}, ProjectName: db.projectName}
			if b, err := json.Marshal(qMsg); err == nil {
				db.trySendWS(string(b))
			}
		}
		answer := db.Guidance(source, q)
		aText := "@" + senderPersonaName + " " + answer
		db.AppendChatMessage(ChatMessage{Role: "system", Text: aText, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
		if db.webOutputChan != nil {
			aMsg := WebMsg{Type: "UserMessage", Data: map[string]interface{}{
				"persona_name": "决策大脑",
				"avatar_file":  "system.png",
				"agent_id":     "",
				"message":      aText,
			}, ProjectName: db.projectName}
			if b, err := json.Marshal(aMsg); err == nil {
				db.trySendWS(string(b))
			}
		}
		return answer
	})
	var argsMap map[string]interface{}
	_ = misc.UnmarshalJSONLenient(args, &argsMap)
	reportMode := reportModeFromArgsMap(argsMap)
	// Build a human-readable task description for the chat assignment message.
	var assignText string
	{
		isReport := strings.Contains(name, "Report")
		if eid, ok := argsMap["exploit_idea_id"]; ok && fmt.Sprint(eid) != "" {
			if isReport {
				if reportMode == "analyzer" {
					assignText = "@" + senderPersonaName + " 分析报告任务: 请先为 ExploitIdea " + fmt.Sprint(eid) + " 撰写分析报告"
				} else {
					assignText = "@" + senderPersonaName + " 报告任务: 请为 ExploitIdea " + fmt.Sprint(eid) + " 撰写漏洞报告"
				}
			} else {
				assignText = "@" + senderPersonaName + " 验证任务: 请验证 ExploitIdea " + fmt.Sprint(eid)
			}
		} else if cid, ok := argsMap["exploit_chain_id"]; ok && fmt.Sprint(cid) != "" {
			if isReport {
				if reportMode == "analyzer" {
					assignText = "@" + senderPersonaName + " 分析报告任务: 请先为 ExploitChain " + fmt.Sprint(cid) + " 撰写分析报告"
				} else {
					assignText = "@" + senderPersonaName + " 报告任务: 请为 ExploitChain " + fmt.Sprint(cid) + " 撰写漏洞报告"
				}
			} else {
				assignText = "@" + senderPersonaName + " 验证任务: 请验证 ExploitChain " + fmt.Sprint(cid)
			}
		} else {
			taskContent := ""
			if tc, ok := argsMap["task_content"]; ok {
				taskContent = strings.TrimSpace(fmt.Sprint(tc))
			}
			if taskContent == "" {
				taskContent = args
			}
			assignText = "@" + senderPersonaName + " " + taskContent
		}
	}
	isDeferred := shouldDeferAgentStart(name, argsMap)
	announceAssignment := strings.TrimSpace(followUpUserMsg) == ""
	if !isDeferred && announceAssignment {
		db.AppendChatMessage(ChatMessage{Role: "system", Text: assignText, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
		if db.webOutputChan != nil {
			assignMsg := WebMsg{Type: "UserMessage", Data: map[string]interface{}{
				"persona_name": "决策大脑",
				"avatar_file":  "system.png",
				"agent_id":     "",
				"message":      assignText,
			}, ProjectName: db.projectName}
			if b, err := json.Marshal(assignMsg); err == nil {
				db.trySendWS(string(b))
			}
		}
	}
	// Set TeamMessage handler: broadcast to all other running agents' memories.
	agent.GetMemory().SetTeamMessageHandler(func(senderCtxId string, tmsg string) {
		db.broadcastTeamMessage(senderCtxId, senderPersonaName, tmsg)
	})
	// Set UserMessage handler: send to frontend only (not to other agents).
	agent.GetMemory().SetUserMessageHandler(func(senderCtxId string, umsg string) {
		db.AppendChatMessage(ChatMessage{Role: "system", Text: umsg, Ts: time.Now().Format("15:04:05"), PersonaName: senderPersonaName, AvatarFile: senderAvatarFile})
		if db.webOutputChan != nil {
			msg := WebMsg{
				Type: "UserMessage",
				Data: map[string]interface{}{
					"persona_name": senderPersonaName,
					"avatar_file":  senderAvatarFile,
					"agent_id":     agent.GetId(),
					"message":      umsg,
				},
				ProjectName: db.projectName,
			}
			if b, err := json.Marshal(msg); err == nil {
				db.trySendWS(string(b))
			}
		}
	})
	// Set BrainMessage handler: unicast message from digital human to DecisionBrain.
	agent.GetMemory().SetBrainMessageHandler(func(senderCtxId string, bmsg string) {
		chatText := "@决策大脑 " + bmsg
		db.AppendChatMessage(ChatMessage{Role: "system", Text: chatText, Ts: time.Now().Format("15:04:05"), PersonaName: senderPersonaName, AvatarFile: senderAvatarFile})
		if db.webOutputChan != nil {
			msg := WebMsg{
				Type: "UserMessage",
				Data: map[string]interface{}{
					"persona_name": senderPersonaName,
					"avatar_file":  senderAvatarFile,
					"agent_id":     agent.GetId(),
					"message":      chatText,
				},
				ProjectName: db.projectName,
			}
			if b, err := json.Marshal(msg); err == nil {
				db.trySendWS(string(b))
			}
		}
		brainContent := fmt.Sprintf("[BrainMessage from %s] %s\n\n(If you need to reply to %s, use Tool-SendMessageToDigitalHuman. Do NOT use <UserMessage> to reply — that is only for broadcasting to the user.)", senderPersonaName, bmsg, senderPersonaName)
		db.memory.AddMessage(llm.Message{
			Role:    llm.RoleUser,
			Content: brainContent,
		})
		db.signal()
	})
	ar := &AgentRuntime{agent: agent, agentToolName: name, Resp: &agents.StartResp{}, startedAt: time.Now(), done: false}
	db.runAgentList = append(db.runAgentList, ar)
	db.poolMu.Lock()
	db.digitalHumanBusy[agent.GetId()] = digitalHumanBusyInfo{AgentToolName: name, Profile: profile, Agent: agent}
	if dhID := strings.TrimSpace(profile.DigitalHumanID); dhID != "" {
		db.lastArgs[dhID] = args
	}
	db.updateDigitalHumanRosterMemoryLocked()
	db.poolMu.Unlock()
	db.injectSameGroupMemberDirectory(agent, name, profile)
	// If there is a follow-up user message (e.g. from @-mention reactivation),
	// inject it into the agent's memory so the agent sees it as a user question.
	if strings.TrimSpace(followUpUserMsg) != "" {
		followUpContent := formatFollowUpChatMessage(followUpUserMsg)
		agent.GetMemory().AddMessage(&llm.MessageX{
			ContextId: task.GetTaskId(),
			Msg: llm.Message{
				Role:    llm.RoleUser,
				Content: followUpContent + "\n\n(You MUST reply to this message using <UserMessage>your reply</UserMessage> in your next response.)",
			},
		})
	}
	// DoneCb is called by the agent's StartTask loop when executeTask finishes.
	doneCb := func(r *agents.StartResp) {
		db.AgentRunDone(agent.GetId(), r)
	}
	ready := 1
	go func() {
		if isDeferred {
			ready = 0
			db.envBuildCond.L.Lock()
			for len(db.envInfo) == 0 {
				select {
				case <-db.done:
					db.envBuildCond.L.Unlock()
					misc.Debug("[%s] 等待环境搭建期间 db.done 关闭，释放数字人", agent.GetProfile().PersonaName)
					db.AgentRunDone(agent.GetId(), &agents.StartResp{Err: fmt.Errorf("cancelled: project stopped while waiting for env build")})
					return
				case <-db.ctx.Done():
					db.envBuildCond.L.Unlock()
					misc.Debug("[%s] 等待环境搭建期间 ctx 取消，释放数字人", agent.GetProfile().PersonaName)
					db.AgentRunDone(agent.GetId(), &agents.StartResp{Err: fmt.Errorf("cancelled: context done while waiting for env build")})
					return
				default:
				}
				if agent.GetMemory().HasPendingUserMessage() {
					misc.Debug("[%s] 等待环境搭建期间收到@消息，使用临时memory副本执行一轮对话", agent.GetProfile().PersonaName)
					pendingMsgs := agent.GetMemory().PopPendingUserMessages()
					// Fire-and-forget: run a one-shot LLM reply in a separate goroutine
					// using a temporary message list. The agent's real memory is untouched.
					go func(msgs []llm.Message) {
						tmpMessages := []llm.Message{
							{Role: llm.RoleSystem, Content: prompts.VerifierWaitingSystemPrompt()},
						}
						tmpMessages = append(tmpMessages, msgs...)
						model := misc.GetConfigValueDefault("verifier", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
						cli := llm.GetResponsesClient("verifier", "main_setting")
						resp, err := llm.RequestLLM(cli, db.ctx, model, tmpMessages, nil, db.projectName)
						if err != nil {
							misc.Debug("[%s] 临时对话LLM调用失败: %s", agent.GetProfile().PersonaName, err.Error())
							return
						}
						replyText := strings.TrimSpace(resp.Content)
						if replyText == "" {
							return
						}
						// Send the reply to the frontend as a UserMessage from this digital human.
						db.AppendChatMessage(ChatMessage{Role: "system", Text: replyText, Ts: time.Now().Format("15:04:05"), PersonaName: senderPersonaName, AvatarFile: senderAvatarFile})
						if db.webOutputChan != nil {
							msg := WebMsg{
								Type: "UserMessage",
								Data: map[string]interface{}{
									"persona_name": senderPersonaName,
									"avatar_file":  senderAvatarFile,
									"agent_id":     agent.GetId(),
									"message":      replyText,
								},
								ProjectName: db.projectName,
							}
							if b, err := json.Marshal(msg); err == nil {
								db.trySendWS(string(b))
							}
						}
					}(pendingMsgs)
					// Continue waiting for env build — do NOT break out of the loop.
				}
				db.envBuildCond.Wait()
			}
			db.envBuildCond.L.Unlock()
			select {
			case <-db.done:
				misc.Debug("[%s] 环境搭建等待结束后 db.done 已关闭，释放数字人", agent.GetProfile().PersonaName)
				db.AgentRunDone(agent.GetId(), &agents.StartResp{Err: fmt.Errorf("cancelled: project stopped after env wait")})
				return
			default:
			}
			if agent != nil {
				agent.SetEnvInfo(db.envInfo)
			}
			ready = 1
		}
		// Follow-up reactivation should not replay the original assignment text in chat.
		// The visible follow-up/user message is already present; re-announcing the old task
		// makes it look like the brain dispatched the exact same task again.
		if isDeferred && announceAssignment {
			db.AppendChatMessage(ChatMessage{Role: "system", Text: assignText, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
			if db.webOutputChan != nil {
				assignMsg := WebMsg{Type: "UserMessage", Data: map[string]interface{}{
					"persona_name": "决策大脑",
					"avatar_file":  "system.png",
					"agent_id":     "",
					"message":      assignText,
				}, ProjectName: db.projectName}
				if b, err := json.Marshal(assignMsg); err == nil {
					db.trySendWS(string(b))
				}
			}
		}
		s := WebMsg{Type: "AgentStart", Data: map[string]interface{}{"AgentID": agent.GetId()}, ProjectName: db.projectName}
		js, _ := json.Marshal(s)
		db.trySendWS(string(js))
		// For analysis agents, inject the current exploitIdea summary so they see what's already been found.
		if strings.Contains(name, "Analyze") && len(db.exploitIdeaList) > 0 {
			db.injectExploitIdeaSummary(agent)
		}
		// Send task to the persistent agent via AssignTask (the agent's StartTask loop picks it up).
		agent.AssignTask(agents.TaskAssignment{ArgsJson: args, DoneCb: doneCb})
	}()
	time.Sleep(2 * time.Second)
	if ready == 0 {
		deferredRole := "verification"
		if strings.HasPrefix(name, "Agent-Report") {
			deferredRole = "report"
		}
		return "Since the test environment has not been successfully set up yet, the current " + deferredRole + " Agent has been added to the execution queue. It will automatically run once the environment setup is completed. Agent ID: " + agent.GetId()
	}
	return "Agent ran successfully, please wait for the execution results. Agent ID: " + agent.GetId()
}

// broadcastTeamMessage inserts a team message from one digital human into all other running agents' memories.
func (db *DecisionBrain) broadcastTeamMessage(senderCtxId string, senderName string, tmsg string) {
	content := fmt.Sprintf("[TeamMessage from %s] %s", senderName, tmsg)
	misc.Success("TeamMessage", content, db.SubmitEventHandler)
	// Persist and push to frontend chat panel.
	var senderAvatar string
	db.poolMu.Lock()
	for _, info := range db.digitalHumanBusy {
		if info.Profile.PersonaName == senderName {
			senderAvatar = info.Profile.AvatarFile
			break
		}
	}
	db.poolMu.Unlock()
	db.AppendChatMessage(ChatMessage{Role: "system", Text: tmsg, Ts: time.Now().Format("15:04:05"), PersonaName: senderName, AvatarFile: senderAvatar})
	if db.webOutputChan != nil {
		msg := WebMsg{
			Type: "TeamMessage",
			Data: map[string]interface{}{
				"persona_name": senderName,
				"avatar_file":  senderAvatar,
				"message":      tmsg,
			},
			ProjectName: db.projectName,
		}
		if b, err := json.Marshal(msg); err == nil {
			db.trySendWS(string(b))
		}
	}
	type runningAgentRef struct {
		agent    agents.Agent
		toolName string
	}
	db.mu.Lock()
	runningAgents := make([]runningAgentRef, 0, len(db.runAgentList))
	for _, ar := range db.runAgentList {
		if ar != nil && ar.agent != nil && !ar.done {
			runningAgents = append(runningAgents, runningAgentRef{agent: ar.agent, toolName: ar.agentToolName})
		}
	}
	db.mu.Unlock()
	for _, ref := range runningAgents {
		a := ref.agent
		taskId := a.GetTask().GetTaskId()
		if taskId == senderCtxId {
			continue
		}
		a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
			Key:       "TeamMessage",
			Content:   db.sanitizeTextForDigitalHumanTool(ref.toolName, content),
			AppendEnv: true,
			ContextId: taskId,
			NotShared: true,
		})
	}
}

// TeamChat handles a team chat message from the user.
// - @PersonaName message → insert user message into that digital human's memory
// - @all message → insert user message into ALL running digital humans' memories
// - plain message (no @) → insert user message into the DecisionBrain's memory
// Returns a summary of what happened.
func (db *DecisionBrain) TeamChat(raw string, sender string) string {
	raw = strings.TrimSpace(raw)
	misc.Debug("user@msg 收到原始消息: sender=%s, raw=%q", sender, raw)
	if raw == "" {
		misc.Debug("user@msg 消息为空，忽略")
		return "empty message"
	}

	// Parse @target
	if strings.HasPrefix(raw, "@") {
		spaceIdx := strings.IndexByte(raw, ' ')
		if spaceIdx < 0 {
			misc.Debug("user@msg @消息没有空格分隔，无法解析目标和内容")
			return "message body is empty"
		}
		target := raw[1:spaceIdx]
		body := strings.TrimSpace(raw[spaceIdx+1:])
		misc.Debug("user@msg 解析结果: target=%q, body=%q", target, body)
		if body == "" {
			misc.Debug("user@msg 消息体为空，忽略")
			return "message body is empty"
		}

		if target == "all" || target == "全体" {
			misc.Debug("user@msg 广播模式: target=%s", target)
			// Collect running agent references under lock, then inject outside lock
			// because AddMessage(user) may block while the agent is in an LLM request.
			type agentRef struct {
				mem  llm.Memory
				tid  string
				name string
				aid  string
				tool string
			}
			db.mu.Lock()
			misc.Debug("user@msg 当前 runAgentList 长度: %d", len(db.runAgentList))
			refs := make([]agentRef, 0, len(db.runAgentList))
			for _, ar := range db.runAgentList {
				if ar != nil && ar.agent != nil && !ar.done {
					refs = append(refs, agentRef{
						mem:  ar.agent.GetMemory(),
						tid:  ar.agent.GetTask().GetTaskId(),
						name: ar.agent.GetProfile().PersonaName,
						aid:  ar.agent.GetId(),
						tool: ar.agentToolName,
					})
				} else if ar != nil {
					misc.Debug("user@msg 广播跳过: agent=%v, done=%v", ar.agent != nil, ar.done)
				}
			}
			db.mu.Unlock()
			for _, ref := range refs {
				misc.Debug("user@msg 广播注入消息到运行中的数字人: name=%s, agentId=%s, taskId=%s", ref.name, ref.aid, ref.tid)
				content := fmt.Sprintf("[Team Chat from %s - to all] %s\n\n(You MUST reply to this message using <UserMessage>your reply</UserMessage> in your next response.)", sender, body)
				ref.mem.AddMessage(&llm.MessageX{
					ContextId: ref.tid,
					Msg: llm.Message{
						Role:    llm.RoleUser,
						Content: db.sanitizeTextForDigitalHumanTool(ref.tool, content),
					},
				})
			}
			misc.Debug("user@msg 广播完成，共注入 %d 个数字人，同时发送给决策大脑", len(refs))
			// Wake up Verifier/Report agents that may be waiting for env build.
			db.envBuildCond.L.Lock()
			db.envBuildCond.Broadcast()
			db.envBuildCond.L.Unlock()
			// Also send to the DecisionBrain so it can act on the message.
			db.memory.AddMessage(llm.Message{
				Role:    llm.RoleUser,
				Content: fmt.Sprintf("[Team Chat from %s - to all] %s\n\n(You MUST reply to this message using <UserMessage>your reply</UserMessage> in your next response.)", sender, body),
			})
			db.signal()
			return fmt.Sprintf("message sent to all %d running digital humans and DecisionBrain", len(refs))
		}

		// Find specific digital human by PersonaName — first check running agents.
		// Collect reference under lock, inject outside lock because AddMessage(user)
		// may block while the agent is in an LLM request.
		misc.Debug("user@msg 定向模式: 查找运行中的数字人 target=%q", target)
		var foundMem llm.Memory
		var foundTaskId, foundAgentId, foundToolName string
		db.mu.Lock()
		misc.Debug("user@msg 当前 runAgentList 长度: %d", len(db.runAgentList))
		for _, ar := range db.runAgentList {
			if ar != nil && ar.agent != nil && !ar.done {
				pName := ar.agent.GetProfile().PersonaName
				misc.Debug("user@msg 遍历运行中数字人: personaName=%q, agentId=%s, done=%v, 匹配=%v", pName, ar.agent.GetId(), ar.done, pName == target)
				if pName == target {
					foundMem = ar.agent.GetMemory()
					foundTaskId = ar.agent.GetTask().GetTaskId()
					foundAgentId = ar.agent.GetId()
					foundToolName = ar.agentToolName
					break
				}
			} else if ar != nil {
				agentName := ""
				if ar.agent != nil {
					agentName = ar.agent.GetProfile().PersonaName
				}
				misc.Debug("user@msg 跳过已完成/空agent: personaName=%q, agent!=nil=%v, done=%v", agentName, ar.agent != nil, ar.done)
			}
		}
		db.mu.Unlock()
		if foundMem != nil {
			misc.Debug("user@msg 匹配成功！注入消息到运行中的数字人: target=%s, agentId=%s, taskId=%s, memoryType=%s", target, foundAgentId, foundTaskId, foundMem.GetType())
			content := fmt.Sprintf("[Team Chat from %s - to you] %s\n\n(You MUST reply to this message using <UserMessage>your reply</UserMessage> in your next response.)", sender, body)
			foundMem.AddMessage(&llm.MessageX{
				ContextId: foundTaskId,
				Msg: llm.Message{
					Role:    llm.RoleUser,
					Content: db.sanitizeTextForDigitalHumanTool(foundToolName, content),
				},
			})
			misc.Debug("user@msg 定向消息注入完成（运行中数字人），target=%s", target)
			// Wake up Verifier/Report agents that may be waiting for env build.
			db.envBuildCond.L.Lock()
			db.envBuildCond.Broadcast()
			db.envBuildCond.L.Unlock()
			return ""
		}

		// Digital human not running — find idle one in pool and reactivate.
		misc.Debug("user@msg 运行中未找到 target=%q，开始在空闲池中查找", target)
		db.poolMu.Lock()
		var reactivateToolName string
		var reactivateDHID string
		for toolName, pool := range db.digitalHumanPool {
			for _, entry := range pool {
				p := entry.Profile
				misc.Debug("user@msg 空闲池遍历: toolName=%s, personaName=%q, dhID=%s, 匹配=%v", toolName, p.PersonaName, p.DigitalHumanID, p.PersonaName == target)
				if p.PersonaName == target {
					reactivateToolName = toolName
					reactivateDHID = strings.TrimSpace(p.DigitalHumanID)
					break
				}
			}
			if reactivateToolName != "" {
				break
			}
		}
		// Look up the original task args for this digital human.
		originalArgs := db.lastArgs[reactivateDHID]
		db.poolMu.Unlock()
		if reactivateToolName != "" {
			misc.Debug("user@msg 空闲池找到! target=%s, toolName=%s, dhID=%s, hasOriginalArgs=%v", target, reactivateToolName, reactivateDHID, originalArgs != "")
			if originalArgs == "" {
				misc.Debug("user@msg 无历史任务参数，以用户消息作为新任务启动")
				// No previous task recorded — fall back to using the user message as a new task.
				go db.startAgentWithFollowUp(reactivateToolName, fmt.Sprintf(`{"task_content":"[Team Chat from %s - to you] %s"}`, sender, body), "", reactivateDHID)
			} else {
				misc.Debug("user@msg 使用历史任务参数重新激活，并注入用户消息作为 followUp")
				// Reactivate with original task + inject user message as follow-up.
				go db.startAgentWithFollowUp(reactivateToolName, originalArgs, fmt.Sprintf("[Team Chat from %s - to you] %s", sender, body), reactivateDHID)
			}
			return ""
		}
		misc.Debug("user@msg 在运行列表和空闲池中均未找到 target=%q", target)
		// 打印当前所有空闲池内容供调试
		db.poolMu.Lock()
		for tn, pool := range db.digitalHumanPool {
			for _, entry := range pool {
				misc.Debug("user@msg 空闲池剩余: toolName=%s, personaName=%q, dhID=%s", tn, entry.Profile.PersonaName, entry.Profile.DigitalHumanID)
			}
		}
		for aid, b := range db.digitalHumanBusy {
			misc.Debug("user@msg 忙碌列表: agentId=%s, toolName=%s, personaName=%q, dhID=%s", aid, b.AgentToolName, b.Profile.PersonaName, b.Profile.DigitalHumanID)
		}
		db.poolMu.Unlock()
		return fmt.Sprintf("digital human '%s' not found", target)
	}

	// No @ prefix → insert into DecisionBrain memory.
	db.memory.AddMessage(llm.Message{
		Role:    llm.RoleUser,
		Content: fmt.Sprintf("[Team Chat from %s] %s\n\n(You MUST reply to this message using <UserMessage>your reply</UserMessage> in your next response.)", sender, raw),
	})
	db.signal()
	misc.Debug("收到用户消息：%s", fmt.Sprintf("[Team Chat from %s] %s\n\n(You MUST reply to this message using <UserMessage>your reply</UserMessage> in your next response.)", sender, raw))
	return ""
}

// AppendChatMessage adds a message to the chat log and persists to disk.
func (db *DecisionBrain) AppendChatMessage(msg ChatMessage) {
	db.chatMu.Lock()
	db.chatMessages = append(db.chatMessages, msg)
	if len(db.chatMessages) > maxDecisionChatRecord {
		db.chatMessages = append([]ChatMessage(nil), db.chatMessages[len(db.chatMessages)-maxDecisionChatRecord:]...)
	}
	now := time.Now()
	if db.lastChatPersistAt.IsZero() || now.Sub(db.lastChatPersistAt) >= chatPersistDebounce {
		db.saveChatMessagesLocked()
		db.lastChatPersistAt = now
		if db.chatPersistTimer != nil {
			db.chatPersistTimer.Stop()
			db.chatPersistTimer = nil
		}
	} else if db.chatPersistTimer == nil {
		wait := chatPersistDebounce - now.Sub(db.lastChatPersistAt)
		if wait < 100*time.Millisecond {
			wait = 100 * time.Millisecond
		}
		db.chatPersistTimer = time.AfterFunc(wait, func() {
			db.chatMu.Lock()
			defer db.chatMu.Unlock()
			db.saveChatMessagesLocked()
			db.lastChatPersistAt = time.Now()
			db.chatPersistTimer = nil
		})
	}
	db.chatMu.Unlock()
	if db.chatObserver != nil {
		db.chatObserver(msg)
	}
}

// BroadcastChatMessage appends the chat message AND pushes it to the frontend via WebSocket.
func (db *DecisionBrain) BroadcastChatMessage(msg ChatMessage) {
	db.AppendChatMessage(msg)
	if db.webOutputChan != nil {
		wsMsg := WebMsg{Type: "UserMessage", Data: map[string]interface{}{
			"persona_name": msg.PersonaName,
			"avatar_file":  msg.AvatarFile,
			"message":      msg.Text,
		}, ProjectName: db.projectName}
		if b, err := json.Marshal(wsMsg); err == nil {
			db.trySendWS(string(b))
		}
	}
}

// pushTokenUsage sends the current cumulative token usage to the frontend via WebSocket.
func (db *DecisionBrain) pushTokenUsage() {
	if db.webOutputChan == nil {
		return
	}
	pu := llm.GetProjectTokenUsage(db.projectName)
	usage := pu.Snapshot()
	msg := WebMsg{Type: "TokenUsage", Data: map[string]interface{}{
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
		"agents":            pu.AgentSnapshots(),
	}, ProjectName: db.projectName}
	if b, err := json.Marshal(msg); err == nil {
		db.trySendWS(string(b))
	}
}

// pushContextBreakdown sends the current brain context token breakdown to the frontend via WebSocket.
func (db *DecisionBrain) pushContextBreakdown() {
	if db.webOutputChan == nil {
		return
	}
	bd := db.memory.GetContextBreakdown()
	msg := WebMsg{Type: "BrainContextBreakdown", Data: bd, ProjectName: db.projectName}
	if b, err := json.Marshal(msg); err == nil {
		db.trySendWS(string(b))
	}
}

// GetContextBreakdown returns the current brain context token breakdown.
func (db *DecisionBrain) GetContextBreakdown() ContextBreakdown {
	return db.memory.GetContextBreakdown()
}

// GetTokenUsage returns the current cumulative token usage for this project.
func (db *DecisionBrain) GetTokenUsage() map[string]interface{} {
	pu := llm.GetProjectTokenUsage(db.projectName)
	usage := pu.Snapshot()
	return map[string]interface{}{
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
		"agents":            pu.AgentSnapshots(),
	}
}

func (db *DecisionBrain) GetTokenUsageSnapshot() taskManager.TokenUsageSnapshot {
	pu := llm.GetProjectTokenUsage(db.projectName)
	usage := pu.Snapshot()
	agentSnapshots := pu.AgentSnapshots()
	agents := make([]taskManager.AgentTokenUsageSnapshot, 0, len(agentSnapshots))
	for _, a := range agentSnapshots {
		agents = append(agents, taskManager.AgentTokenUsageSnapshot{
			Label:            a.Label,
			PromptTokens:     a.PromptTokens,
			CompletionTokens: a.CompletionTokens,
			TotalTokens:      a.TotalTokens,
		})
	}
	return taskManager.TokenUsageSnapshot{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		Agents:           agents,
	}
}

func (db *DecisionBrain) RestoreTokenUsageSnapshot(snapshot taskManager.TokenUsageSnapshot) {
	agents := make([]llm.AgentUsageSnapshot, 0, len(snapshot.Agents))
	for _, a := range snapshot.Agents {
		agents = append(agents, llm.AgentUsageSnapshot{
			Label:            a.Label,
			PromptTokens:     a.PromptTokens,
			CompletionTokens: a.CompletionTokens,
			TotalTokens:      a.TotalTokens,
		})
	}
	llm.RestoreProjectTokenUsage(db.projectName, snapshot.PromptTokens, snapshot.CompletionTokens, snapshot.TotalTokens, agents)
}

func (db *DecisionBrain) RestoreExploitState(ideas []*taskManager.ExploitIdea, chains []*taskManager.ExploitChain) {
	db.exploitIdeaList = make([]*taskManager.ExploitIdea, 0, len(ideas))
	for _, one := range ideas {
		if one != nil {
			db.exploitIdeaList = append(db.exploitIdeaList, one)
		}
	}
	db.exploitChainList = make([]*taskManager.ExploitChain, 0, len(chains))
	for _, one := range chains {
		if one != nil {
			db.exploitChainList = append(db.exploitChainList, one)
		}
	}
	db.flushExploitIdeaList()
	db.flushExploitChainList()
}

// GetChatMessages returns all persisted chat messages.
func (db *DecisionBrain) GetChatMessages() []ChatMessage {
	db.chatMu.Lock()
	defer db.chatMu.Unlock()
	out := make([]ChatMessage, len(db.chatMessages))
	copy(out, db.chatMessages)
	return out
}

func (db *DecisionBrain) saveChatMessagesLocked() {
	data, err := json.Marshal(db.chatMessages)
	if err != nil {
		return
	}
	_ = os.WriteFile(db.chatFilePath, data, 0644)
}

// 运行时证据审核
func (db *DecisionBrain) auditEvidence(evidence string, condition string, harm string) error {
	up := fmt.Sprintf("ExploitIdea: <condition>%s</condition>\n<harm>%s</harm>\n<evidence>%s</evidence>", condition, harm, evidence)
	if condition == "" && harm == "" {
		up = fmt.Sprintf("ExploitChain: <evidence>%s</evidence>", evidence)
	}
	cli := llm.GetResponsesClient("decision", "main_setting")
	var ms []llm.Message
	ms = append(ms, llm.Message{
		Role:    llm.RoleSystem,
		Content: auditPrompt,
	})
	ms = append(ms, llm.Message{
		Role:    llm.RoleUser,
		Content: up,
	})
	r, e := llm.RequestLLM(cli, context.Background(), db.model, ms, nil, db.projectName)
	if e != nil {
		return e
	}
	x := strings.TrimSpace(r.Content)
	if strings.ToLower(x) == "true" {
		return nil
	}
	return fmt.Errorf("%s", x)
}

// agentTypeToolInfo returns a human-readable description of the tools and
// constraints for a given agent tool name (e.g. "Agent-Analyze-AnalyzeCommonAgent").
func agentTypeToolInfo(agentToolName string, enableExploitWorkflow bool) (typeName string, toolDesc string) {
	switch {
	case strings.Contains(agentToolName, "Analyze"):
		toolDesc = "可用工具：DetectLanguageTool（语言检测）、ListSourceCodeTreeTool（目录树）、SearchFileContentsByRegexTool（正则搜索）、ReadLinesFromFileTool（读取文件）、TaskListTool（任务管理）、AskPocKnowledgeTool（PoC/打法知识检索）、AskToolUsageGuideTool（工具文档检索）、GuidanceTool（专家咨询）、ReadReportChunkTool（分段读取报告）、IssueTool（反馈问题）"
		if enableExploitWorkflow {
			toolDesc = "可用工具：DetectLanguageTool（语言检测）、ListSourceCodeTreeTool（目录树）、SearchFileContentsByRegexTool（正则搜索）、ReadLinesFromFileTool（读取文件）、TaskListTool（任务管理）、AskPocKnowledgeTool（PoC/打法知识检索）、AskToolUsageGuideTool（工具文档检索）、GuidanceTool（专家咨询）、ReadReportChunkTool（分段读取报告）、IssueCandidateExploitIdeaTool（提交漏洞线索）、IssueTool（反馈问题）"
		}
		return "代码分析数字人（Analyze）", toolDesc + `
限制：只能读取源码，不能执行任何命令，不能操作容器，不能访问运行中的目标环境。`
	case strings.Contains(agentToolName, "Verifier"):
		toolDesc = "可用工具：DetectLanguageTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、RunCommandTool（在容器内执行命令）、RunPythonCodeTool（执行Python代码）、RunPHPCodeTool（执行PHP代码）、RunSQLTool（执行SQL）、DockerLogsTool（查看容器日志）、DockerDirScanTool（扫描容器目录）、DockerFileReadTool（读取容器文件）、IssueTool"
		if enableExploitWorkflow {
			toolDesc = "可用工具：DetectLanguageTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、RunCommandTool（在容器内执行命令）、RunPythonCodeTool（执行Python代码）、RunPHPCodeTool（执行PHP代码）、RunSQLTool（执行SQL）、SubmitExploitIdeaTool（提交验证结果）、SubmitExploitChainTool（提交利用链结果）、DockerLogsTool（查看容器日志）、DockerDirScanTool（扫描容器目录）、DockerFileReadTool（读取容器文件）、GetExploitIdeaByIdTool、GetExploitChainByIdTool、IssueTool"
		}
		return "漏洞验证数字人（Verifier）", toolDesc + `
限制：可以在已搭建的目标环境中执行命令和代码，但不能搭建或修改环境本身，不能操作宿主机。`
	case strings.Contains(agentToolName, "Ops") && strings.Contains(agentToolName, "Scout"):
		return "环境侦察数字人（OpsEnvScout）", `可用工具：RunCommandTool、DetectLanguageTool、DockerRunTool、DockerLogsTool、DockerRemoveTool、DockerExecTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、IssueTool
限制：用于在已有测试环境中收集信息（用户名、密码、数据库、URL等），不负责漏洞分析。`
	case strings.Contains(agentToolName, "Ops"):
		return "运维数字人（Ops）", `可用工具：RunCommandTool（执行命令）、DetectLanguageTool、DockerRunTool（创建容器）、DockerLogsTool（容器日志）、DockerRemoveTool（删除容器）、DockerExecTool（容器内执行）、EnvSaveTool（保存环境信息）、RunSQLTool、JavaEnvTool、PHPEnvTool、NodeEnvTool、PythonEnvTool、GolangEnvTool、MySQLEnvTool、RedisEnvTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、IssueTool
限制：负责环境搭建和维护，可以操作Docker容器，但不负责漏洞分析或验证。`
	case strings.Contains(agentToolName, "Report"):
		toolDesc = "可用工具：ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、IssueTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、ReportVulnTool（生成报告）"
		if enableExploitWorkflow {
			toolDesc = "可用工具：ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、IssueTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、ReportVulnTool（生成报告）、GetExploitIdeaByIdTool、GetExploitChainByIdTool"
		}
		return "报告数字人（Report）", toolDesc + `
限制：只负责撰写漏洞报告，不能执行命令，不能操作环境，只能读取源码和已有的漏洞数据。`
	case strings.Contains(agentToolName, "Android"):
		return "Android逆向数字人（Android）", `可用工具：RunCommandTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、ReportVulnTool（提交报告）、IssueTool，以及已注入的 jadx-ai-mcp 工具
限制：主要依赖 jadx-ai-mcp 与本地辅助命令进行 APK/DEX/JAR/AAB 静态分析；不能直接操作宿主机 GUI，只能通过已提供工具完成工作。`
	case strings.Contains(agentToolName, "Reverse"):
		return "逆向分析数字人（Reverse）", `可用工具：RunCommandTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、ReportVulnTool（提交报告）、IssueTool，以及已注入的 ida-pro-mcp 工具
限制：主要依赖 ida-pro-mcp 与本地辅助命令进行二进制分析；不能直接操作宿主机 GUI，只能通过已提供工具完成工作。`
	case strings.Contains(agentToolName, "General"):
		return "通用杂项数字人（General）", `可用工具：RunCommandTool、DetectLanguageTool、ListSourceCodeTreeTool、SearchFileContentsByRegexTool、ReadLinesFromFileTool、RunPythonCodeTool、RunPHPCodeTool、RunSQLTool、DockerLogsTool、DockerDirScanTool、DockerFileReadTool、TaskListTool、AskPocKnowledgeTool、AskToolUsageGuideTool、GuidanceTool、ReadReportChunkTool（分段读取报告）、IssueTool，以及已注入的 GitHub MCP 工具
限制：用于处理杂项任务（例如仓库协作、脚本执行、定位问题等）；涉及高风险环境变更时应先明确目标并谨慎执行。`
	default:
		return "未知类型数字人", "工具信息不详。"
	}
}

// Agent任务中产生难题指导
// Guidance inherits the current brain memory as read-only context (no new
// messages are written to db.memory). The system prompt is replaced with
// guidancePromptTemplate so the LLM answers in a guidance-specific role while
// still having full awareness of the project state.
func (db *DecisionBrain) Guidance(source string, q string) string {
	cli := llm.GetResponsesClient("decision", "main_setting")
	// Snapshot the brain's current context (read-only, does not mutate db.memory).
	snapshot := db.memory.GetContext()

	// Build source info block for the prompt.
	sourceInfo := "- 提问者: " + source
	extraConstraints := ""

	// Try to find the running agent to get its tool name for richer context.
	db.mu.Lock()
	for _, a := range db.runAgentList {
		if a.agent != nil && a.agent.GetProfile().PersonaName != "" &&
			strings.Contains(source, a.agent.GetProfile().PersonaName) {
			enableExploitWorkflow := false
			if task := a.agent.GetTask(); task != nil {
				enableExploitWorkflow = task.GetCandidateExploitIdeaHandler() != nil ||
					task.GetExploitIdeaHandler() != nil ||
					task.GetExploitChainHandler() != nil ||
					task.GetExploitIdeaGetter() != nil ||
					task.GetExploitChainGetter() != nil
			}
			typeName, toolDesc := agentTypeToolInfo(a.agentToolName, enableExploitWorkflow)
			sourceInfo = fmt.Sprintf("- 提问者: %s\n- 角色类型: %s\n- %s\n%s", source, typeName, toolDesc, "提问者并没有状态面板的信息，如果他有需要可以将对应信息发给他")
			break
		}
	}
	db.mu.Unlock()

	prompt := fmt.Sprintf(guidancePromptTemplate, sourceInfo, extraConstraints)

	// Build a new message list: replace the system prompt with enriched guidance prompt,
	// keep all other messages for context, then append the guidance question.
	ms := make([]llm.Message, 0, len(snapshot)+2)
	ms = append(ms, llm.Message{
		Role:    llm.RoleSystem,
		Content: prompt,
	})
	// Copy conversation history (skip the original system prompt).
	for _, m := range snapshot {
		if m.Role == llm.RoleSystem {
			continue
		}
		ms = append(ms, m)
	}
	ms = append(ms, llm.Message{
		Role:    llm.RoleUser,
		Content: q,
	})
	r, e := llm.RequestLLM(cli, context.Background(), db.model, ms, nil, db.projectName)
	if e != nil {
		return "决策大脑暂时无法响应。"
	}
	content := r.Content
	s := WebMsg{Type: "Guidance", Data: map[string]interface{}{"source": source, "q": q, "a": content}, ProjectName: db.projectName}
	js, _ := json.Marshal(s)
	db.trySendWS(string(js))
	return content
}

func (db *DecisionBrain) AgentRunDone(agentId string, r *agents.StartResp) {
	if r == nil {
		r = &agents.StartResp{Err: fmt.Errorf("agent returned nil response")}
	}
	respSummary := agentRespSummary(r)
	respError := agentRespError(r)
	s1 := WebMsg{Type: "AgentDone", Data: map[string]interface{}{"AgentID": agentId, "Resp": respSummary, "Error": respError}, ProjectName: db.projectName}
	js1, _ := json.Marshal(s1)
	db.trySendWS(string(js1))

	// ---- Step 1: Immediately mark agent as done so the brain sees it right away. ----
	var a *AgentRuntime
	for _, v := range db.runAgentList {
		if v.agent.GetId() == agentId {
			a = v
		}
	}
	if a == nil {
		db.releaseDigitalHuman(agentId)
		return
	}
	if a.agent != nil {
		if respError != "" {
			a.agent.SetState("Error")
		} else {
			a.agent.SetState("Done")
		}
	}
	a.done = true
	a.Resp = r

	// Store last task and summary for this digital human so the roster can show it.
	truncatedSummary := truncateRunes(respSummary, 500)
	runTask := agentRunTask(a.agent)
	db.poolMu.Lock()
	busy, hasBusy := db.digitalHumanBusy[agentId]
	if hasBusy {
		if dhID := strings.TrimSpace(busy.Profile.DigitalHumanID); dhID != "" {
			db.lastSummary[dhID] = truncatedSummary
			db.lastTask[dhID] = runTask
			db.lastError[dhID] = respError
		}
	}
	db.poolMu.Unlock()

	// Release the slot — any @message will correctly go to the reactivation path.
	db.releaseDigitalHuman(agentId)

	// ---- Verifier cleanup: reset "正在验证" → "等待验证" if the agent finished
	// without submitting a result (i.e. the state was never updated by SubmitExploitIdeaHandler/SubmitExploitChainHandler). ----
	if hasBusy && strings.Contains(busy.AgentToolName, "Verifier") {
		if dhID := strings.TrimSpace(busy.Profile.DigitalHumanID); dhID != "" {
			if argsStr, ok := db.lastArgs[dhID]; ok {
				var argsMap map[string]interface{}
				if misc.UnmarshalJSONLenient(argsStr, &argsMap) == nil {
					if eid, ok := argsMap["exploit_idea_id"]; ok && fmt.Sprint(eid) != "" {
						if ei, err := db.GetExploitIdeaById(fmt.Sprint(eid)); err == nil && ei.State == "正在验证" {
							ei.State = "等待验证"
							misc.Debug("AgentRunDone: Verifier %s 结束但 ExploitIdea %s 仍为正在验证，重置为等待验证", busy.Profile.PersonaName, fmt.Sprint(eid))
							db.flushExploitIdeaList()
						}
					}
					if cid, ok := argsMap["exploit_chain_id"]; ok && fmt.Sprint(cid) != "" {
						if ec, err := db.GetExploitChainById(fmt.Sprint(cid)); err == nil && ec.State == "正在验证" {
							ec.State = "等待验证"
							misc.Debug("AgentRunDone: Verifier %s 结束但 ExploitChain %s 仍为正在验证，重置为等待验证", busy.Profile.PersonaName, fmt.Sprint(cid))
							db.flushExploitChainList()
						}
					}
				}
			}
		}
	}

	// Update runtime info and wake the brain immediately.
	compact := make([]interface{}, 0, len(db.runAgentList))
	for _, v := range db.runAgentList {
		compact = append(compact, v.GetRunInfo())
	}
	js, _ := json.Marshal(compact)
	db.memory.UpdateAgentRuntimeInfo(string(js))
	db.signal()

	// ---- Step 2: Summarize persona memory asynchronously (slow LLM call). ----
	// This no longer blocks the state update, so the brain sees "Done" right away.
	if hasBusy {
		ctxMsgs := a.agent.GetMemory().GetContext(a.agent.GetTask().GetTaskId())
		go func(busyInfo digitalHumanBusyInfo, runTask string, summary string, ctxMsgs []llm.Message) {
			existing := db.getPersonaMemory(busyInfo.AgentToolName, busyInfo.Profile)
			updated := db.summarizePersonaMemory(busyInfo.AgentToolName, busyInfo.Profile, existing, runTask, summary, ctxMsgs)
			if strings.TrimSpace(updated) != "" {
				db.setPersonaMemory(busyInfo.AgentToolName, busyInfo.Profile, updated)
			}
		}(busy, runTask, respSummary, ctxMsgs)
	}
	if !db.hasRunningAgents() {
		db.nudgeAllIdleDecisionBrain("A digital human finished and no other digital human is still active.")
	}
}

func (db *DecisionBrain) SetEnvInfo(env map[string]interface{}) {
	s := WebMsg{Type: "EnvInfo", Data: map[string]interface{}{"env": env}, ProjectName: db.projectName}
	js1, _ := json.Marshal(s)
	db.trySendWS(string(js1))

	db.envBuildCond.L.Lock()
	db.envInfo = env
	db.envBuildCond.Broadcast()
	db.envBuildCond.L.Unlock()
	js, _ := json.Marshal(env)
	db.memory.SetEnvInfo(string(js))
	db.signal()
}

// 组合利用链
func (db *DecisionBrain) SynthesizeChain(chainIdList []string, idea string) error {
	ec := &taskManager.ExploitChain{Idea: idea, State: "未验证"}
	ec.ExploitIdea = make([]*taskManager.ExploitIdea, 0, len(chainIdList))
	for _, v := range chainIdList {
		e, err := db.GetExploitIdeaById(v)
		if err != nil {
			return err
		}
		ec.ExploitIdea = append(ec.ExploitIdea, e)
	}
	index := 0
	for {
		id := fmt.Sprintf("C.%d", index)
		exists := false
		for _, v := range db.exploitChainList {
			if v != nil && v.ExploitChainId == id {
				exists = true
				break
			}
		}
		if !exists {
			ec.ExploitChainId = id
			break
		}
		index++
	}
	db.exploitChainList = append(db.exploitChainList, ec)
	policy := db.currentExploitAutomationPolicy()
	if policy.AutoPublishAnalyzeReport {
		db.startAnalyzeReportForTarget("exploit_chain_id", ec.ExploitChainId)
	}
	if policy.AutoVerifyExploits {
		err := db.VerifyExploitChain(ec.ExploitChainId)
		if err != nil {
			return err
		}
	}
	db.flushExploitChainList()
	return nil
}

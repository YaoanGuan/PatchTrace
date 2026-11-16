package LabManager

import (
	"AIxVuln/DecisionBrain"
	"AIxVuln/ProjectManager"
	"AIxVuln/agents"
	"AIxVuln/taskManager"
	"context"
	"sync"
)

type researchGroup interface {
	ID() string
	Name() string
	Kind() string
	DecisionBrainName() string
	Status() string
	IsRunning() bool
	BrainFinished() bool
	IsDemo() bool
	HasMember(persona string) bool
	HandleDecisionChat(raw string, sender string) string
	HandleGroupChat(raw string, sender string) string
	NotifyDecisionBrainEvent(content string)
	SyncGlobalStatusPanel(panel map[string]interface{})
	NormalizeGroupChatMessage(msg DecisionBrain.ChatMessage) DecisionBrain.ChatMessage
	StartTask()
	StopTask()
	SharedReportCatalog() []taskManager.ReportMeta
	NotifySharedReportUpdate(report taskManager.ReportMeta, sourceGroupID string, sourceGroupName string)
	PrivateSnapshot() map[string]interface{}
	GetDigitalHumanRoster() map[string]interface{}
	GetAgentRuntimeList() []map[string]interface{}
	Snapshot() map[string]interface{}
}

type WebSecurityGroup struct {
	pm          *ProjectManager.ProjectManager
	mode        string
	brainConfig groupDecisionBrainConfig
}

type groupDecisionBrainConfig struct {
	SystemPrompt          string
	EnableExploitWorkflow bool
	AgentDefinitions      []agents.AgentDefinition
	DigitalHumanPool      map[string][]agents.AgentProfile
}

type binaryMember struct {
	ToolName       string
	PersonaName    string
	Gender         string
	AvatarFile     string
	Personality    string
	Age            int
	Role           string
	DigitalHumanID string
	AgentID        string
}

type binaryDecisionTool struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     func(map[string]interface{}) string
}

type BinarySecurityGroup struct {
	mu                      sync.RWMutex
	status                  string
	lastTask                string
	lastSummary             string
	brainRunning            bool
	brainFinished           bool
	roster                  []binaryMember
	agentRuns               []map[string]interface{}
	projectKey              string
	taskContent             string
	projectOverview         string
	brainConfig             groupDecisionBrainConfig
	brain                   *DecisionBrain.DecisionBrain
	globalStatusPanel       map[string]interface{}
	decisionTools           []binaryDecisionTool
	hostCommandHandler      func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult
	decisionApprovalHandler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult
	webOutputChan           chan string
	reportObserver          func(taskManager.ReportMeta)
	emitGroupChat           func(DecisionBrain.ChatMessage)
	waitObserver            func(bool)
	notifyChange            func()
}

type PentestSecurityGroup struct {
	mu                      sync.RWMutex
	status                  string
	lastTask                string
	lastSummary             string
	brainRunning            bool
	brainFinished           bool
	roster                  []binaryMember
	agentRuns               []map[string]interface{}
	projectKey              string
	taskContent             string
	projectOverview         string
	brainConfig             groupDecisionBrainConfig
	brain                   *DecisionBrain.DecisionBrain
	globalStatusPanel       map[string]interface{}
	decisionTools           []binaryDecisionTool
	hostCommandHandler      func(context.Context, *taskManager.Task, taskManager.HostCommandRequest) taskManager.HostCommandResult
	decisionApprovalHandler func(context.Context, *taskManager.Task, taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult
	webOutputChan           chan string
	reportObserver          func(taskManager.ReportMeta)
	emitGroupChat           func(DecisionBrain.ChatMessage)
	waitObserver            func(bool)
	notifyChange            func()
}

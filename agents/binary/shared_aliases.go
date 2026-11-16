package binary

import shared "AIxVuln/agents"

type Agent = shared.Agent
type AgentDefinition = shared.AgentDefinition
type AgentProfile = shared.AgentProfile
type StartResp = shared.StartResp
type TaskAssignment = shared.TaskAssignment
type ToolFactory = shared.ToolFactory

var (
	ReverseToolFactories          = shared.ReverseToolFactories
	ReportToolFactories           = shared.ReportToolFactories
	BuildAgent                    = shared.BuildAgent
	BuildAgentWithMemory          = shared.BuildAgentWithMemory
	BuildAgentWithMemoryForAgent  = shared.BuildAgentWithMemoryForAgent
	CommonSystemPrompt            = shared.CommonSystemPrompt
	CommonSystemPromptForAgent    = shared.CommonSystemPromptForAgent
	KnowledgeLookupPriorityPrompt = shared.KnowledgeLookupPriorityPrompt
	debugLastMessages             = shared.DebugLastMessages
	extractAgentFinishSummary     = shared.ExtractAgentFinishSummary
)

package web

import shared "AIxVuln/agents"

type Agent = shared.Agent
type AgentDefinition = shared.AgentDefinition
type AgentProfile = shared.AgentProfile
type StartResp = shared.StartResp
type TaskAssignment = shared.TaskAssignment

var (
	AnalyzeToolFactories          = shared.AnalyzeToolFactoriesForExploitWorkflow
	BuildAgent                    = shared.BuildAgent
	BuildAgentWithMemory          = shared.BuildAgentWithMemory
	BuildAgentWithMemoryForAgent  = shared.BuildAgentWithMemoryForAgent
	CommonSystemPrompt            = shared.CommonSystemPrompt
	CommonSystemPromptForAgent    = shared.CommonSystemPromptForAgent
	KnowledgeLookupPriorityPrompt = shared.KnowledgeLookupPriorityPrompt
	OpsToolFactories              = shared.OpsToolFactories
	OverviewToolFactories         = shared.OverviewToolFactories
	ReportToolFactories           = shared.ReportToolFactoriesForExploitWorkflow
	VerifierToolFactories         = shared.VerifierToolFactoriesForExploitWorkflow
	debugLastMessages             = shared.DebugLastMessages
	extractAgentFinishSummary     = shared.ExtractAgentFinishSummary
)

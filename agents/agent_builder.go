package agents

import (
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"AIxVuln/toolCalling"
)

type ToolFactory func(*taskManager.Task) toolCalling.ToolHandler

type WorkflowCapabilities struct {
	ExploitWorkflow bool
}

type CommonToolOptions struct {
	Skills       bool
	AgentTools   bool
	ToolGuide    bool
	PocKnowledge bool
}

type AgentBuildResult struct {
	Memory llm.Memory
	Client *toolCalling.ToolManager
}

func reportReaderToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewReadReportChunkTool(task)
}

func listSkillsToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewListSkillsTool(task)
}

func listSkillFilesToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewListSkillFilesTool(task)
}

func readSkillToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewReadSkillTool(task)
}

func listAgentToolsToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewListAgentToolsTool(task)
}

func hostCommandRequestToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewRequestHostCommandTool(task)
}

func decisionApprovalToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewRequestDecisionApprovalTool(task)
}

func readBackgroundCommandToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewReadBackgroundCommandTool(task)
}

func writeBackgroundCommandToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewWriteBackgroundCommandTool(task)
}

func terminateBackgroundCommandToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewTerminateBackgroundCommandTool(task)
}

func ingestPatchToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewIngestPatchTool(task)
}

func invertPatchToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewInvertPatchTool(task)
}

func queryRuleBaseToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewQueryRuleBaseTool(task)
}

func promoteRuleToolFactory(task *taskManager.Task) toolCalling.ToolHandler {
	return toolCalling.NewPromoteRuleTool(task)
}

func BuildAgentWithMemory(task *taskManager.Task, memory llm.Memory, systemPrompt string, toolFactories []ToolFactory) AgentBuildResult {
	return BuildAgentWithMemoryAndCommonTools(task, memory, systemPrompt, toolFactories, CommonToolOptions{
		Skills:       true,
		AgentTools:   true,
		ToolGuide:    true,
		PocKnowledge: true,
	})
}

func BuildAgentWithMemoryAndCommonTools(task *taskManager.Task, memory llm.Memory, systemPrompt string, toolFactories []ToolFactory, commonTools CommonToolOptions) AgentBuildResult {
	if memory == nil {
		memory = llm.NewContextManager()
		memory.SetEventHandler(task.GetEventHandler())
		task.SetMemory(memory)
	}
	memory.SetSystemPrompt(&llm.SystemPromptX{SystemPrompt: systemPrompt, ContextId: task.GetTaskId()})

	client := toolCalling.NewToolManager()
	client.SetTask(task)
	if task != nil && task.GetSm() != nil {
		client.SetSourceRoot(task.GetSourceCodePath())
	}
	for _, factory := range toolFactories {
		if factory == nil {
			continue
		}
		client.Register(factory(task))
	}
	client.Register(decisionApprovalToolFactory(task))
	if commonTools.Skills {
		client.Register(listSkillsToolFactory(task))
		client.Register(listSkillFilesToolFactory(task))
		client.Register(readSkillToolFactory(task))
	}
	if commonTools.AgentTools {
		client.Register(listAgentToolsToolFactory(task))
	}
	if commonTools.ToolGuide {
		client.Register(toolCalling.NewAskToolUsageGuideTool(task))
	}
	if commonTools.PocKnowledge {
		client.Register(toolCalling.NewAskPocKnowledgeTool(task))
	}
	return AgentBuildResult{Memory: memory, Client: client}
}

func BuildAgentWithMemoryForAgent(task *taskManager.Task, memory llm.Memory, systemPrompt string, toolFactories []ToolFactory, agentToolName string) AgentBuildResult {
	return BuildAgentWithMemoryAndCommonTools(task, memory, systemPrompt, toolFactories, CommonToolOptions{
		Skills:       commonSystemPromptNeedsSkills(agentToolName),
		AgentTools:   commonSystemPromptNeedsAgentTools(agentToolName),
		ToolGuide:    true,
		PocKnowledge: true,
	})
}

func BuildAgent(task *taskManager.Task, systemPrompt string, toolFactories []ToolFactory) AgentBuildResult {
	return BuildAgentWithMemory(task, task.GetMemory(), systemPrompt, toolFactories)
}

func AnalyzeToolFactories() []ToolFactory {
	return AnalyzeToolFactoriesWithCapabilities(WorkflowCapabilities{})
}

func AnalyzeToolFactoriesWithCapabilities(cap WorkflowCapabilities) []ToolFactory {
	factories := []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDetectLanguageTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewTaskListTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}
	if cap.ExploitWorkflow {
		factories = append(factories[:7],
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewIssueCandidateExploitIdeaTool(task)
			},
			factories[7],
			factories[8],
		)
	}
	// PatchOracle tools are appended after the index-sensitive workflow block so
	// existing factory indices never shift.
	factories = append(factories,
		ingestPatchToolFactory,
		invertPatchToolFactory,
		queryRuleBaseToolFactory,
		promoteRuleToolFactory,
	)
	return factories
}

func ReverseToolFactories() []ToolFactory {
	return []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewTaskListTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		hostCommandRequestToolFactory,
		readBackgroundCommandToolFactory,
		writeBackgroundCommandToolFactory,
		terminateBackgroundCommandToolFactory,
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewReportVulnTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}
}

func OpsToolFactories() []ToolFactory {
	return []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDetectLanguageTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerRunTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerLogsTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerRemoveTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerExecTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewEnvSaveTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunSQLTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewJavaEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewPHPEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewNodeEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewPythonEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGolangEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewMySQLEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRedisEnvTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewTaskListTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		hostCommandRequestToolFactory,
		readBackgroundCommandToolFactory,
		writeBackgroundCommandToolFactory,
		terminateBackgroundCommandToolFactory,
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}
}

func VerifierToolFactories() []ToolFactory {
	return VerifierToolFactoriesWithCapabilities(WorkflowCapabilities{})
}

func VerifierToolFactoriesWithCapabilities(cap WorkflowCapabilities) []ToolFactory {
	factories := []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDetectLanguageTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewTaskListTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		hostCommandRequestToolFactory,
		readBackgroundCommandToolFactory,
		writeBackgroundCommandToolFactory,
		terminateBackgroundCommandToolFactory,
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPythonCodeTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPHPCodeTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunSQLTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerLogsTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerDirScanTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerFileReadTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}
	if cap.ExploitWorkflow {
		factories = append(factories[:12],
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewSubmitExploitIdeaTool(task)
			},
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewSubmitExploitChainTool(task)
			},
			factories[12],
			factories[13],
			factories[14],
			factories[15],
			factories[16],
			factories[17],
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewGetExploitIdeaByIdTool(task)
			},
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewGetExploitChainByIdTool(task)
			},
			factories[18],
			factories[19],
		)
	}
	// PatchOracle tools are appended after the index-sensitive workflow block so
	// existing factory indices never shift.
	factories = append(factories,
		ingestPatchToolFactory,
		invertPatchToolFactory,
		queryRuleBaseToolFactory,
		promoteRuleToolFactory,
	)
	return factories
}

func OverviewToolFactories() []ToolFactory {
	return []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDetectLanguageTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		hostCommandRequestToolFactory,
		readBackgroundCommandToolFactory,
		writeBackgroundCommandToolFactory,
		terminateBackgroundCommandToolFactory,
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}
}

func ReportToolFactories(reportType string) []ToolFactory {
	return ReportToolFactoriesWithCapabilities(reportType, WorkflowCapabilities{})
}

func ReportToolFactoriesWithCapabilities(reportType string, cap WorkflowCapabilities) []ToolFactory {
	base := []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewReportVulnTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}
	if cap.ExploitWorkflow {
		base = append(base[:7],
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewGetExploitIdeaByIdTool(task)
			},
			func(task *taskManager.Task) toolCalling.ToolHandler {
				return toolCalling.NewGetExploitChainByIdTool(task)
			},
			base[7],
		)
	}
	if reportType == "verifier" {
		base = append(base,
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerLogsTool(task) },
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPythonCodeTool(task) },
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPHPCodeTool(task) },
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerFileReadTool(task) },
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerDirScanTool(task) },
			func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunSQLTool(task) },
		)
	}
	return base
}

func AnalyzeToolFactoriesForExploitWorkflow() []ToolFactory {
	return AnalyzeToolFactoriesWithCapabilities(WorkflowCapabilities{ExploitWorkflow: true})
}

func VerifierToolFactoriesForExploitWorkflow() []ToolFactory {
	return VerifierToolFactoriesWithCapabilities(WorkflowCapabilities{ExploitWorkflow: true})
}

func ReportToolFactoriesForExploitWorkflow(reportType string) []ToolFactory {
	return ReportToolFactoriesWithCapabilities(reportType, WorkflowCapabilities{ExploitWorkflow: true})
}
func GeneralToolFactories() []ToolFactory {
	return []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDetectLanguageTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPythonCodeTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPHPCodeTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunSQLTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerLogsTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerDirScanTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerFileReadTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewTaskListTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		hostCommandRequestToolFactory,
		readBackgroundCommandToolFactory,
		writeBackgroundCommandToolFactory,
		terminateBackgroundCommandToolFactory,
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}

}

func SRCToolFactories() []ToolFactory {
	return []ToolFactory{
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunCommandTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDetectLanguageTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewListSourceCodeTreeTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewSearchFileContentsByRegexTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler {
			return toolCalling.NewReadLinesFromFileTool(task)
		},
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPythonCodeTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunPHPCodeTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewRunSQLTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerLogsTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerDirScanTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewDockerFileReadTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewTaskListTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewGuidanceTool(task) },
		hostCommandRequestToolFactory,
		readBackgroundCommandToolFactory,
		writeBackgroundCommandToolFactory,
		terminateBackgroundCommandToolFactory,
		reportReaderToolFactory,
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewIssueTool(task) },
		func(task *taskManager.Task) toolCalling.ToolHandler { return toolCalling.NewAgentFinishTool(task) },
	}

}

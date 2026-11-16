package agents

import (
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"testing"
)

func collectToolNames(factories []ToolFactory) []string {
	task := &taskManager.Task{}
	names := make([]string, 0, len(factories))
	for _, factory := range factories {
		if factory == nil {
			continue
		}
		names = append(names, factory(task).Name())
	}
	return names
}

func hasTool(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestAnalyzeToolFactoriesDefaultOmitsExploitWorkflowTools(t *testing.T) {
	names := collectToolNames(AnalyzeToolFactories())
	if hasTool(names, "IssueCandidateExploitIdeaTool") {
		t.Fatalf("default analyze factories should not expose exploit workflow tools: %+v", names)
	}

	exploitNames := collectToolNames(AnalyzeToolFactoriesForExploitWorkflow())
	if !hasTool(exploitNames, "IssueCandidateExploitIdeaTool") {
		t.Fatalf("exploit analyze factories should expose exploit workflow tool: %+v", exploitNames)
	}
}

func TestVerifierToolFactoriesDefaultOmitsExploitWorkflowTools(t *testing.T) {
	names := collectToolNames(VerifierToolFactories())
	for _, forbidden := range []string{"SubmitExploitIdeaTool", "SubmitExploitChainTool", "GetExploitIdeaByIdTool", "GetExploitChainByIdTool"} {
		if hasTool(names, forbidden) {
			t.Fatalf("default verifier factories should not expose %s: %+v", forbidden, names)
		}
	}

	exploitNames := collectToolNames(VerifierToolFactoriesForExploitWorkflow())
	for _, required := range []string{"SubmitExploitIdeaTool", "SubmitExploitChainTool", "GetExploitIdeaByIdTool", "GetExploitChainByIdTool"} {
		if !hasTool(exploitNames, required) {
			t.Fatalf("exploit verifier factories should expose %s: %+v", required, exploitNames)
		}
	}
}

func TestReportToolFactoriesDefaultOmitsExploitWorkflowTools(t *testing.T) {
	names := collectToolNames(ReportToolFactories("verifier"))
	for _, forbidden := range []string{"GetExploitIdeaByIdTool", "GetExploitChainByIdTool"} {
		if hasTool(names, forbidden) {
			t.Fatalf("default report factories should not expose %s: %+v", forbidden, names)
		}
	}

	exploitNames := collectToolNames(ReportToolFactoriesForExploitWorkflow("verifier"))
	for _, required := range []string{"GetExploitIdeaByIdTool", "GetExploitChainByIdTool"} {
		if !hasTool(exploitNames, required) {
			t.Fatalf("exploit report factories should expose %s: %+v", required, exploitNames)
		}
	}
}

func TestBuildAgentWithMemoryRegistersToolUsageGuideTool(t *testing.T) {
	task := &taskManager.Task{}
	memory := llm.NewContextManager()
	built := BuildAgentWithMemory(task, memory, "system", nil)
	tools := built.Client.GetTools()
	foundUsageGuide := false
	foundPocKnowledge := false
	for _, tool := range tools {
		if tool.Name == "AskToolUsageGuideTool" {
			foundUsageGuide = true
		}
		if tool.Name == "AskPocKnowledgeTool" {
			foundPocKnowledge = true
		}
	}
	if !foundUsageGuide {
		t.Fatalf("expected AskToolUsageGuideTool in common tool registry, got %+v", tools)
	}
	if !foundPocKnowledge {
		t.Fatalf("expected AskPocKnowledgeTool in common tool registry, got %+v", tools)
	}
}

func TestBuildAgentWithMemoryForAgentPrunesRoleSpecificCommonTools(t *testing.T) {
	task := &taskManager.Task{}
	memory := llm.NewContextManager()
	built := BuildAgentWithMemoryForAgent(task, memory, "system", nil, "Agent-Report-ReportCommonAgent")
	tools := built.Client.GetTools()
	for _, forbidden := range []string{"ListSkillsTool", "ListSkillFilesTool", "ReadSkillTool", "ListAgentToolsTool"} {
		for _, tool := range tools {
			if tool.Name == forbidden {
				t.Fatalf("report common registry should not expose %s: %+v", forbidden, tools)
			}
		}
	}
	for _, required := range []string{"AskToolUsageGuideTool", "AskPocKnowledgeTool"} {
		found := false
		for _, tool := range tools {
			if tool.Name == required {
				found = true
			}
		}
		if !found {
			t.Fatalf("report common registry should still expose %s: %+v", required, tools)
		}
	}
}

func TestAnalyzeAndReportFactoriesOmitHostBackgroundTools(t *testing.T) {
	for factoryName, names := range map[string][]string{
		"analyze": collectToolNames(AnalyzeToolFactories()),
		"report":  collectToolNames(ReportToolFactories("analyzer")),
	} {
		for _, forbidden := range []string{"RequestHostCommandTool", "ReadBackgroundCommandTool", "WriteBackgroundCommandTool", "TerminateBackgroundCommandTool"} {
			if hasTool(names, forbidden) {
				t.Fatalf("%s factories should not expose %s: %+v", factoryName, forbidden, names)
			}
		}
	}
}

func TestOverviewToolFactoriesIncludeRunCommandTool(t *testing.T) {
	names := collectToolNames(OverviewToolFactories())
	if !hasTool(names, "RunCommandTool") {
		t.Fatalf("overview factories should expose RunCommandTool for lightweight reconnaissance: %+v", names)
	}
}

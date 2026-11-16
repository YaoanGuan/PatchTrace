package prompts

import (
	"strings"
	"testing"
)

func TestPocKnowledgeSystemPromptRequiresDepthSwitching(t *testing.T) {
	prompt := PocKnowledgeSystemPrompt()
	required := []string{
		"adapt answer depth to the caller's question granularity",
		"only mentions a component, product family, middleware, protocol, or another broad term",
		"first return a coarse shortlist",
		"follows up with a narrower question",
		"give a much more detailed answer focused on that branch",
		"caller cannot directly read the local corpus documents",
		"Do NOT answer with only document names, file paths",
		"操作步骤",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected PocKnowledgeSystemPrompt to contain %q", item)
		}
	}
}

func TestToolUsageGuideSystemPromptRequiresDepthSwitching(t *testing.T) {
	prompt := ToolUsageGuideSystemPrompt()
	required := []string{
		"adapt answer depth to the caller's question granularity",
		"only names a tool, component, MCP, workflow, or another broad topic",
		"first return a coarse shortlist",
		"follows up with a narrower question",
		"answer in much more operational detail",
		"caller cannot directly read the docs that you read here",
		"caller cannot open the internal docs directly",
		"关键参数或路径",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected ToolUsageGuideSystemPrompt to contain %q", item)
		}
	}
}

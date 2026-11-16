package agents

import (
	"strings"
	"testing"
)

func TestCommonSystemPromptKeepsOnlyAlwaysUsefulGuidance(t *testing.T) {
	prompt := CommonSystemPrompt()
	for _, required := range []string{
		"AskPocKnowledgeTool",
		"AskToolUsageGuideTool",
		"GuidanceTool as the final escalation path",
		"call AskPocKnowledgeTool FIRST",
		"call AskToolUsageGuideTool FIRST",
		"Do NOT guess a product",
		"keep AskPocKnowledgeTool queries generic and evidence-based",
		"Specialized Handoff Rule",
		"SameGroupMemberDirectory",
		"AgentFinishTool to end this run",
		"Necessary report submission or status communication",
		"emit only the tool call itself",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt should contain %q", required)
		}
	}
	if strings.Contains(prompt, "PHPSHE") {
		t.Fatalf("prompt should not hardcode PHPSHE: %s", prompt)
	}
	for _, forbidden := range []string{
		"ListSkillsTool",
		"ListSkillFilesTool",
		"ReadSkillTool",
		"ListAgentToolsTool",
		"download_url",
		"Before installing any dependency",
		"Do NOT reinstall blindly",
		"If an executable needs interactive control",
		"After you get the session_id",
		"<aixfile>",
		"Use aix-mcp://... when the tool expects a PATH",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("default prompt should not contain role-specific guidance %q", forbidden)
		}
	}
	if !strings.Contains(prompt, "such as xxx") {
		t.Fatalf("prompt should use generic placeholder xxx for product examples: %s", prompt)
	}
}

func TestCommonSystemPromptForOperationalAgentIncludesToolGuidance(t *testing.T) {
	prompt := CommonSystemPromptForAgent("Agent-Pentest-PentestCommonAgent")
	for _, required := range []string{
		"ListSkillsTool",
		"ListSkillFilesTool",
		"ReadSkillTool",
		"ListAgentToolsTool",
		"Do NOT bulk-read every skill",
		"read SKILL.md first",
		"prefer trying that skill workflow first",
		"download_url",
		"Before installing any dependency",
		"Do NOT reinstall blindly",
		"If an executable needs interactive control",
		"After you get the session_id",
		"RequestHostCommandTool",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("operational prompt should contain %q", required)
		}
	}
	if strings.Index(prompt, "call AskPocKnowledgeTool FIRST") > strings.Index(prompt, "ListAgentToolsTool") {
		t.Fatalf("AskPocKnowledgeTool guidance should appear before generic tool-download guidance")
	}
	if strings.Index(prompt, "call AskToolUsageGuideTool FIRST") > strings.Index(prompt, "You do NOT have direct host-machine access by default") {
		t.Fatalf("AskToolUsageGuideTool guidance should appear before host-command fallback guidance")
	}
}

func TestCommonSystemPromptForMCPAgentIncludesFileReferenceGuidance(t *testing.T) {
	prompt := CommonSystemPromptForAgent("Agent-Binary-AndroidCommonAgent")
	for _, required := range []string{
		"<aixfile>",
		"Use aix-mcp://... when the tool expects a PATH",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("mcp-capable prompt should contain %q", required)
		}
	}
}

func TestCommonSystemPromptForReportAgentOmitsOperationalGuidance(t *testing.T) {
	prompt := CommonSystemPromptForAgent("Agent-Report-ReportCommonAgent")
	for _, forbidden := range []string{
		"ListSkillsTool",
		"ListAgentToolsTool",
		"RequestHostCommandTool",
		"download_url",
		"Before installing any dependency",
		"Use aix-mcp://... when the tool expects a PATH",
		"<aixfile>",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("report prompt should not contain role-specific guidance %q", forbidden)
		}
	}
}

func TestCommonSystemPromptForAgentIncludesRoleSpecificHandoffGuidance(t *testing.T) {
	pentestPrompt := CommonSystemPromptForAgent("Agent-Pentest-PentestCommonAgent")
	for _, required := range []string{
		"post-exploitation engineer",
		"SRC engineer",
		"persistence",
		"post-exploitation",
	} {
		if !strings.Contains(pentestPrompt, required) {
			t.Fatalf("pentest prompt should contain %q", required)
		}
	}

	srcPrompt := CommonSystemPromptForAgent("Agent-Pentest-SrcCommonAgent")
	for _, required := range []string{
		"business logic",
		"penetration-testing engineer",
		"post-exploitation engineer",
	} {
		if !strings.Contains(srcPrompt, required) {
			t.Fatalf("src prompt should contain %q", required)
		}
	}

	androidPrompt := CommonSystemPromptForAgent("Agent-Binary-AndroidCommonAgent")
	for _, required := range []string{
		"Android reverse engineer",
		"APK/DEX/JAR/AAB",
		"native binary reverse engineering",
	} {
		if !strings.Contains(androidPrompt, required) {
			t.Fatalf("android prompt should contain %q", required)
		}
	}

	reportPrompt := CommonSystemPromptForAgent("Agent-Report-ReportCommonAgent")
	for _, required := range []string{
		"formal report",
		"crucial evidence",
		"relevant technical teammate",
	} {
		if !strings.Contains(reportPrompt, required) {
			t.Fatalf("report prompt should contain %q", required)
		}
	}
}

func TestNormalizeSkillPromptTextAndTruncateRunes(t *testing.T) {
	if got := normalizeSkillPromptText("  hello \n  skill\tworld  "); got != "hello skill world" {
		t.Fatalf("unexpected normalized text: %q", got)
	}
	if got := truncateRunes("abcdef", 5); got != "ab..." {
		t.Fatalf("unexpected truncated text: %q", got)
	}
}

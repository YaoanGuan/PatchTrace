package agents

import (
	"strings"
	"testing"
)

func TestProjectOverviewSystemPromptIsNotWebTemplate(t *testing.T) {
	prompt := projectOverviewSystemPrompt()
	for _, forbidden := range []string{
		"**Web框架**",
		"**数据库**",
		"## 项目概览",
		"Produce a final summary in the following structured format",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt should not contain legacy web template fragment %q", forbidden)
		}
	}
}

func TestProjectOverviewSystemPromptPrefersMCPForNonWebArtifacts(t *testing.T) {
	prompt := projectOverviewSystemPrompt()
	for _, required := range []string{
		"prefer them first",
		"APK/DEX/JAR/AAB",
		"Do NOT force a fixed template",
		"Do NOT invent sections like web framework, database, routes, or entry file",
		"ListSkillsTool",
		"Do NOT bulk-read all skills",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt should contain %q", required)
		}
	}
}

func TestProjectOverviewSystemPromptSupportsEmptyWorkspaceRecon(t *testing.T) {
	prompt := projectOverviewSystemPrompt()
	for _, required := range []string{
		"empty or nearly empty /workdir/sourceCodeDir",
		"Do NOT skip the overview just because the source directory is empty",
		"use RunCommandTool for lightweight reconnaissance and fingerprinting",
		"Lightweight identification and fingerprinting for overview purposes is allowed",
		"If the task mainly gives you a URL and no source code",
		"homepage check",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt should contain %q", required)
		}
	}
}

func TestProjectOverviewTaskContentMentionsLightweightURLRecon(t *testing.T) {
	taskContent := projectOverviewTaskContent()
	for _, required := range []string{
		"If the task mainly provides a URL and no source code",
		"lightweight curl of the homepage",
		"basic fingerprinting/server-info checks is usually enough",
		"do not scan ports for URL targets",
		"bare IP/hostname without an explicit URL or port",
	} {
		if !strings.Contains(taskContent, required) {
			t.Fatalf("task content should contain %q", required)
		}
	}
}

func TestProjectOverviewPromptAvoidsPortScanForURLTargets(t *testing.T) {
	prompt := projectOverviewSystemPrompt()
	for _, required := range []string{
		"if the target clue is a URL",
		"the protocol and port are already explicit",
		"Do NOT run port scans",
		"Only when the task provides a bare IP address or hostname without a URL/port",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt should contain %q", required)
		}
	}
}

func TestBuildProjectOverviewTaskContentIncludesProjectTaskContext(t *testing.T) {
	taskContent := buildProjectOverviewTaskContent("RemoteTargetEntrypoints:\n- 10.0.160.104:80\n\n目标描述：\n内部测试系统")
	for _, required := range []string{
		"Current project task/context:",
		"RemoteTargetEntrypoints:",
		"10.0.160.104:80",
		"内部测试系统",
		"Do not ignore provided URLs, host:port entrypoints",
	} {
		if !strings.Contains(taskContent, required) {
			t.Fatalf("task content should contain %q", required)
		}
	}
}

func TestProjectOverviewTaskContentFromArgsUsesTaskContent(t *testing.T) {
	taskContent := projectOverviewTaskContentFromArgs(`{"task_content":"RemoteTargetEntrypoints:\n- 10.0.160.104:80"}`)
	if !strings.Contains(taskContent, "10.0.160.104:80") {
		t.Fatalf("expected task content from args to include entrypoint, got: %s", taskContent)
	}
}

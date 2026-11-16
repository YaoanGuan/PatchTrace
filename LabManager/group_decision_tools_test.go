package LabManager

import (
	"strings"
	"testing"
)

func TestRegisterSharedGroupDecisionToolsOmitsDirectKnowledgeTools(t *testing.T) {
	lm := &LabManager{}
	var names []string
	lm.registerSharedGroupDecisionTools(webGroupID, func(name string, _ string, _ map[string]interface{}, _ func(map[string]interface{}) string) {
		names = append(names, name)
	})

	required := map[string]bool{
		"Tool-RequestLabAssistance":   false,
		"Tool-ReportToDecisionMakers": false,
	}
	for _, name := range names {
		if name == "AskPocKnowledgeTool" || name == "AskToolUsageGuideTool" {
			t.Fatalf("decision brain should not expose direct knowledge tool %s, got %v", name, names)
		}
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("expected shared group decision tool %s, got %v", name, names)
		}
	}
}

func TestGroupDecisionBrainPromptsIncludeKnowledgePriorityWithoutPHPSHE(t *testing.T) {
	prompts := []string{
		webDecisionBrainSystemPrompt("audit"),
		binaryDecisionBrainSystemPrompt(binaryGroupName),
		pentestDecisionBrainSystemPrompt(pentestGroupName),
		groupDecisionBrainKnowledgePrompt(),
	}
	for _, prompt := range prompts {
		if !strings.Contains(prompt, "AskPocKnowledgeTool") {
			t.Fatalf("prompt should mention member-side AskPocKnowledgeTool: %s", prompt)
		}
		if !strings.Contains(prompt, "AskToolUsageGuideTool") {
			t.Fatalf("prompt should mention member-side AskToolUsageGuideTool: %s", prompt)
		}
		if strings.Contains(prompt, "available directly to you as decision-brain tools") || strings.Contains(prompt, "you may call AskPocKnowledgeTool") || strings.Contains(prompt, "you may call AskToolUsageGuideTool") {
			t.Fatalf("prompt should not give decision brain direct knowledge lookup ability: %s", prompt)
		}
		if !strings.Contains(prompt, "digital human") && !strings.Contains(prompt, "Group digital humans") {
			t.Fatalf("prompt should explain knowledge lookup is delegated to digital humans: %s", prompt)
		}
		if !strings.Contains(prompt, "Do NOT guess a CMS") && !strings.Contains(prompt, "Do NOT guess a CMS, OA system, framework, gateway, or vendor name") {
			t.Fatalf("prompt should forbid speculative product guessing: %s", prompt)
		}
		if strings.Contains(prompt, "PHPSHE") {
			t.Fatalf("prompt should not hardcode PHPSHE: %s", prompt)
		}
		if !strings.Contains(prompt, "xxx") {
			t.Fatalf("prompt should use generic placeholder xxx: %s", prompt)
		}
	}
}

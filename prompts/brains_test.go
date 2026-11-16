package prompts

import (
	"strings"
	"testing"
)

func TestPentestDecisionBrainPromptForbidsCallbackEndpointMicromanagement(t *testing.T) {
	prompt := PentestDecisionBrainSystemPrompt("渗透测试组", "", "")
	if !strings.Contains(prompt, "Do NOT prescribe reverse-shell callback IP/port details") {
		t.Fatalf("expected pentest decision brain prompt to forbid callback endpoint micromanagement")
	}
	if !strings.Contains(prompt, "Never treat a sandbox/container private IP as the callback target") {
		t.Fatalf("expected pentest decision brain prompt to forbid sandbox private IP callback targets")
	}
}

func TestPentestDecisionBrainPromptDescribesCentralTargetActionControl(t *testing.T) {
	prompt := PentestDecisionBrainSystemPrompt("渗透测试组", "", "")
	for _, required := range []string{
		"If the user has not explicitly stated that the target is a test/staging/lab environment",
		"treat it as a production/business environment",
		"service restart, shutdown, reboot",
		"target-interaction tools are centrally controlled",
		"short Chinese intent",
		"unified decision queue",
		"Do not rely on voluntary self-review prompts",
		"denial-of-service",
		"credential dumping",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("expected pentest decision brain prompt to contain %q", required)
		}
	}
}

func TestPentestDecisionBrainPromptAvoidsFixedC2ScanChain(t *testing.T) {
	prompt := PentestDecisionBrainSystemPrompt("渗透测试组", "", "")
	for _, required := range []string{
		"Do not force every finding into a C2-first chain",
		"WAF/IDS/IPS",
		"IP blocking",
		"frontend/JS analysis",
		"Use broad scanning only after targeted methods stall",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("expected pentest decision brain prompt to contain %q", required)
		}
	}
	for _, forbidden := range []string{
		"prioritize connecting to the C2 before conducting post-exploitation work",
		"following this cycle: gain execution privileges -> connect to the C2",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("pentest decision brain prompt should not force fixed chain %q", forbidden)
		}
	}
}

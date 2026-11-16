package misc

import "testing"

func TestResolveAutomationDecisionUsesConfig(t *testing.T) {
	oldConfig := GetConfigValueDefault("misc", "AUTO_MODE", "")
	ClearRuntimeAutomationMode()
	if err := SetConfigValue("misc", "AUTO_MODE", "full"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	t.Cleanup(func() {
		_ = SetConfigValue("misc", "AUTO_MODE", oldConfig)
		ClearRuntimeAutomationMode()
	})

	decision := ResolveAutomationDecision(AutomationScopeHostCommandApproval)
	if !decision.Enabled {
		t.Fatalf("expected config-driven automation decision, got %+v", decision)
	}
	if decision.Source != automationSourceConfig {
		t.Fatalf("expected config source, got %+v", decision)
	}
	if decision.Mode != AutomationModeFull {
		t.Fatalf("expected full mode, got %+v", decision)
	}
}

func TestResolveAutomationDecisionUsesRuntimeOverride(t *testing.T) {
	oldConfig := GetConfigValueDefault("misc", "AUTO_MODE", "")
	if err := SetConfigValue("misc", "AUTO_MODE", "off"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	if err := SetRuntimeAutomationMode("full"); err != nil {
		t.Fatalf("SetRuntimeAutomationMode: %v", err)
	}
	t.Cleanup(func() {
		_ = SetConfigValue("misc", "AUTO_MODE", oldConfig)
		ClearRuntimeAutomationMode()
	})

	decision := ResolveAutomationDecision(AutomationScopeHostCommandApproval)
	if !decision.Enabled {
		t.Fatalf("expected runtime-driven automation decision, got %+v", decision)
	}
	if decision.Source != automationSourceRuntime {
		t.Fatalf("expected runtime source, got %+v", decision)
	}
	if decision.Mode != AutomationModeFull {
		t.Fatalf("expected full mode, got %+v", decision)
	}
}

func TestNormalizeAutomationModeRejectsInvalidValue(t *testing.T) {
	if _, err := NormalizeAutomationMode("weird"); err == nil {
		t.Fatal("expected invalid automation mode to fail")
	}
}

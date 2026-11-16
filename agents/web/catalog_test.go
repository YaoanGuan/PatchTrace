package web

import "testing"

func TestGetAgentDescriptionForModeComprehensiveKeepsOnlyAnalyzeAndReport(t *testing.T) {
	defs, _ := GetAgentDescriptionForMode("comprehensive")
	got := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		got[def.Name] = struct{}{}
	}
	for _, required := range []string{
		"Agent-Analyze-AnalyzeCommonAgent",
		"Agent-Report-ReportCommonAgent",
	} {
		if _, ok := got[required]; !ok {
			t.Fatalf("expected %s to remain in comprehensive mode, got %+v", required, got)
		}
	}
	for _, forbidden := range []string{
		"Agent-Ops-OpsCommonAgent",
		"Agent-Ops-OpsEnvScoutAgent",
		"Agent-Verifier-VerifierCommonAgent",
	} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("expected %s to be removed in comprehensive mode, got %+v", forbidden, got)
		}
	}
}

func TestGetAgentDescriptionForModeAuditKeepsFullWebWorkflow(t *testing.T) {
	defs, _ := GetAgentDescriptionForMode("audit")
	got := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		got[def.Name] = struct{}{}
	}
	for _, required := range []string{
		"Agent-Analyze-AnalyzeCommonAgent",
		"Agent-Ops-OpsCommonAgent",
		"Agent-Ops-OpsEnvScoutAgent",
		"Agent-Report-ReportCommonAgent",
		"Agent-Verifier-VerifierCommonAgent",
	} {
		if _, ok := got[required]; !ok {
			t.Fatalf("expected %s in audit mode, got %+v", required, got)
		}
	}
}

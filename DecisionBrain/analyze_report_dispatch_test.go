package DecisionBrain

import (
	"AIxVuln/agents"
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"encoding/json"
	"testing"
)

func TestDispatchAnalyzeReportBuildsAnalyzerReportRequest(t *testing.T) {
	var gotName string
	var gotArgs string

	msg := dispatchAnalyzeReport(func(name string, args string) string {
		gotName = name
		gotArgs = args
		return "queued"
	}, "exploit_idea_id", "E.7")

	if msg != "queued" {
		t.Fatalf("unexpected starter result: %q", msg)
	}
	if gotName != "Agent-Report-ReportCommonAgent" {
		t.Fatalf("unexpected agent name: %s", gotName)
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(gotArgs), &payload); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if payload["reportType"] != "analyzer" {
		t.Fatalf("expected analyzer report type, got %+v", payload)
	}
	if payload["exploit_idea_id"] != "E.7" {
		t.Fatalf("expected exploit idea id in payload, got %+v", payload)
	}
}

func TestShouldDeferAgentStartSkipsAnalyzerReports(t *testing.T) {
	if !shouldDeferAgentStart("Agent-Verifier-VerifierCommonAgent", nil) {
		t.Fatal("verifier should wait for environment")
	}
	if shouldDeferAgentStart("Agent-Report-ReportCommonAgent", map[string]interface{}{"reportType": "analyzer"}) {
		t.Fatal("analysis-only report should not wait for environment")
	}
	if !shouldDeferAgentStart("Agent-Report-ReportCommonAgent", map[string]interface{}{"reportType": "verifier"}) {
		t.Fatal("verified report should still wait for environment")
	}
	if !shouldDeferAgentStart("Agent-Report-ReportCommonAgent", nil) {
		t.Fatal("report without explicit mode should default to verifier deferral")
	}
}

func TestSubmitCandidateExploitIdeaHandlerSkipsAutoFollowUpWhenPolicyDisablesIt(t *testing.T) {
	db := &DecisionBrain{
		memory:           NewBrainMemory(),
		exploitIdeaList:  make([]*taskManager.ExploitIdea, 0),
		exploitChainList: make([]*taskManager.ExploitChain, 0),
		notify:           make(chan bool, 4),
		agentHandler:     make(map[string]func(*taskManager.Task, string) (agents.Agent, error)),
		agentToolDefs:    make(map[string]llm.ToolDef),
		reportList:       make([]taskManager.ReportMeta, 0),
		agentFeed:        make(map[string][]map[string]interface{}),
	}
	db.SetExploitAutomationPolicyResolver(func() ExploitAutomationPolicy {
		return ExploitAutomationPolicy{
			AutoVerifyExploits:       false,
			AutoPublishAnalyzeReport: false,
		}
	})

	err := db.SubmitCandidateExploitIdeaHandler(&taskManager.ExploitIdea{
		Harm:       "impact",
		ExtendIdea: "expand",
		Condition:  "auth required",
		ExploitPoint: &taskManager.ExploitPoint{
			Title:            "sql injection",
			RouteOrEndpoint:  "/login",
			File:             "controllers/user.go",
			FunctionOrMethod: "Login",
			Params:           "username",
			Type:             "sqli",
			PayloadIdea:      "' or 1=1 --",
			ExpectedImpact:   "auth bypass",
			Confidence:       "high",
		},
	})
	if err != nil {
		t.Fatalf("SubmitCandidateExploitIdeaHandler returned error: %v", err)
	}
	if len(db.exploitIdeaList) != 1 {
		t.Fatalf("expected 1 exploit idea, got %d", len(db.exploitIdeaList))
	}
	if db.exploitIdeaList[0].State != "未验证" {
		t.Fatalf("expected exploit idea to stay unverified, got %q", db.exploitIdeaList[0].State)
	}
	if len(db.GetReportList()) != 0 {
		t.Fatalf("expected no auto-published reports, got %d", len(db.GetReportList()))
	}
}

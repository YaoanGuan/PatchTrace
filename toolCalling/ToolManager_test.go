package toolCalling

import (
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"context"
	"strings"
	"testing"
)

func TestSectionFromAgentNameBinaryAndPentest(t *testing.T) {
	cases := map[string]string{
		"ReverseCommonAgent":           "reverse",
		"AndroidCommonAgent":           "android",
		"PentestCommonAgent":           "pentest",
		"RemoteControlCommonAgent":     "remote_control",
		"PostExploitationCommonAgent":  "remote_control",
		"Agent-Pentest-SrcCommonAgent": "src",
	}
	for input, want := range cases {
		if got := sectionFromAgentName(input); got != want {
			t.Fatalf("%s: got %q want %q", input, got, want)
		}
	}
}

func TestTargetActionToolsExposeRequiredIntent(t *testing.T) {
	manager := NewToolManager()
	manager.Register(&targetActionEchoTool{})
	manager.Register(dockerLikeEchoTool{})

	defs := manager.GetTools()
	byName := map[string]map[string]interface{}{}
	for _, def := range defs {
		byName[def.Name] = def.Parameters
	}
	if !containsString(stringSliceFromAny(byName["RunCommandTool"]["required"]), "intent") {
		t.Fatalf("RunCommandTool should require intent: %#v", byName["RunCommandTool"])
	}
	props := byName["RunCommandTool"]["properties"].(map[string]interface{})
	if _, ok := props["intent"]; !ok {
		t.Fatalf("RunCommandTool properties should include intent")
	}
	if containsString(stringSliceFromAny(byName["DockerExecTool"]["required"]), "intent") {
		t.Fatalf("DockerExecTool should not require intent: %#v", byName["DockerExecTool"])
	}
}

func TestTargetActionToolMissingIntentFailsBeforeExecute(t *testing.T) {
	manager := NewToolManager()
	tool := &targetActionEchoTool{}
	manager.Register(tool)
	client := stubLLMClient{response: llm.Response{ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "RunCommandTool", Arguments: `{"command":"curl http://example.test"}`}}}}

	_, toolMessages, err := manager.ToolCallRequestWithLabel(context.Background(), client, []llm.Message{{Role: llm.RoleUser, Content: "test"}}, "test-model", "TestAgent", "TestAgent")
	if err != nil {
		t.Fatalf("ToolCallRequestWithLabel returned err: %v", err)
	}
	if tool.executed {
		t.Fatalf("tool should not execute without intent")
	}
	if len(toolMessages) != 1 || !strings.Contains(toolMessages[0].Content, "intent") {
		t.Fatalf("expected missing intent failure, got %#v", toolMessages)
	}
}

func TestTargetActionToolApprovalExecutesAndStripsIntent(t *testing.T) {
	manager := NewToolManager()
	task := &taskManager.Task{}
	var approval taskManager.DecisionApprovalRequest
	task.SetDecisionApprovalHandler(func(req taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult {
		approval = req
		return taskManager.DecisionApprovalResult{Status: "approved", Approved: true, Kind: req.Kind, Title: req.Title}
	})
	manager.SetTask(task)
	tool := &targetActionEchoTool{}
	manager.Register(tool)
	client := stubLLMClient{response: llm.Response{ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "RunCommandTool", Arguments: `{"command":"curl http://example.test","intent":"确认目标首页是否可访问"}`}}}}

	_, toolMessages, err := manager.ToolCallRequestWithLabel(context.Background(), client, []llm.Message{{Role: llm.RoleUser, Content: "test"}}, "test-model", "TestAgent", "TestAgent")
	if err != nil {
		t.Fatalf("ToolCallRequestWithLabel returned err: %v", err)
	}
	if !tool.executed {
		t.Fatalf("tool should execute after approval")
	}
	if _, ok := tool.args["intent"]; ok {
		t.Fatalf("intent should be stripped before handler execution: %#v", tool.args)
	}
	if approval.Kind != taskManager.DecisionApprovalKindTargetAction || approval.Reason != "确认目标首页是否可访问" {
		t.Fatalf("unexpected approval request: %#v", approval)
	}
	if len(toolMessages) != 1 || !strings.Contains(toolMessages[0].Content, "curl http://example.test") {
		t.Fatalf("unexpected tool response: %#v", toolMessages)
	}
}

func TestTargetActionToolRejectedBlocksExecution(t *testing.T) {
	manager := NewToolManager()
	task := &taskManager.Task{}
	task.SetDecisionApprovalHandler(func(req taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult {
		return taskManager.DecisionApprovalResult{Status: "rejected", Approved: false, Error: "用户拒绝"}
	})
	manager.SetTask(task)
	tool := &targetActionEchoTool{}
	manager.Register(tool)
	client := stubLLMClient{response: llm.Response{ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "RunCommandTool", Arguments: `{"command":"curl http://example.test","intent":"确认目标首页是否可访问"}`}}}}

	_, toolMessages, err := manager.ToolCallRequestWithLabel(context.Background(), client, []llm.Message{{Role: llm.RoleUser, Content: "test"}}, "test-model", "TestAgent", "TestAgent")
	if err != nil {
		t.Fatalf("ToolCallRequestWithLabel returned err: %v", err)
	}
	if tool.executed {
		t.Fatalf("tool should not execute after rejection")
	}
	if len(toolMessages) != 1 || !strings.Contains(toolMessages[0].Content, "用户拒绝") {
		t.Fatalf("expected rejection failure, got %#v", toolMessages)
	}
}

func TestCompactMCPToolDefPreservesRequiredIntent(t *testing.T) {
	def := compactMCPToolDef(llm.ToolDef{
		Name:        "MCP_demo_call",
		Description: strings.Repeat("x", 300),
		Parameters:  withTargetActionIntentParameter(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
	})
	if !containsString(stringSliceFromAny(def.Parameters["required"]), "intent") {
		t.Fatalf("compacted MCP schema should require intent: %#v", def.Parameters)
	}
	props := def.Parameters["properties"].(map[string]interface{})
	if _, ok := props["intent"]; !ok {
		t.Fatalf("compacted MCP schema should expose intent property: %#v", def.Parameters)
	}
}

type targetActionEchoTool struct {
	executed bool
	args     map[string]interface{}
}

func (t *targetActionEchoTool) Name() string        { return "RunCommandTool" }
func (t *targetActionEchoTool) Description() string { return "target action echo" }
func (t *targetActionEchoTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{"type": "string"},
		},
		"required": []string{"command"},
	}
}
func (t *targetActionEchoTool) Execute(args map[string]interface{}) string {
	t.executed = true
	t.args = args
	return Success(args)
}

type dockerLikeEchoTool struct{}

func (dockerLikeEchoTool) Name() string        { return "DockerExecTool" }
func (dockerLikeEchoTool) Description() string { return "docker echo" }
func (dockerLikeEchoTool) Parameters() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}}
}
func (dockerLikeEchoTool) Execute(args map[string]interface{}) string { return Success(args) }

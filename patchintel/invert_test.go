package patchintel

import (
	"strings"
	"testing"
)

func TestBuildInversionPromptInjectsRulesAndTemplate(t *testing.T) {
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	ApplyMetadata(profile, "CVE-2025-3248", "GHSA-rvqx-wpfh-mfx7", "<1.3.0", "faac4db", "langflow")
	rules := []Rule{{
		ID:                7,
		CVEID:             "CVE-2025-3248",
		Pattern:           "unauthenticated code validation executes user input",
		TriggerFeatures:   "POST /api/v1/validate/code body.code",
		VerificationNotes: "refusal text is not success",
		Confidence:        0.6,
		Version:           2,
	}}
	system, user := BuildInversionPromptWithBudget(profile, rules, "SCENE-TEMPLATE-MARKER", 20000)
	if !strings.Contains(system, "PatchOracle") {
		t.Fatalf("system prompt missing engine identity: %q", system)
	}
	if !strings.Contains(user, "SCENE-TEMPLATE-MARKER") {
		t.Fatal("scene template not injected")
	}
	if !strings.Contains(user, rules[0].Pattern) {
		t.Fatal("rule pattern not injected")
	}
	if !strings.Contains(user, "CVE-2025-3248") {
		t.Fatal("metadata not injected")
	}
	if !strings.Contains(user, "exec(code)") {
		t.Fatal("diff not injected")
	}
}

func TestBuildInversionPromptTruncatesDiffByBudget(t *testing.T) {
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	_, user := BuildInversionPromptWithBudget(profile, nil, "tpl", 40)
	if !strings.Contains(user, "[补丁 diff 已按 token 预算截断]") {
		t.Fatal("expected truncation marker")
	}
	if !strings.Contains(user, "规则库无命中") {
		t.Fatal("expected cold-start rule fallback hint")
	}
}

func TestBuildIdeaFromResponseParsesFencedJSON(t *testing.T) {
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	raw := "```json\n" + `{
  "cve_id": "",
  "vuln_type": "RCE",
  "harm": "未认证攻击者可执行任意 Python 代码",
  "condition": "langflow < 1.3.0，/api/v1/validate/code 可达",
  "extend_idea": "反弹 shell 或读取环境变量",
  "evidence_hint": "响应包含计算结果",
  "pattern": "未认证代码校验端点执行用户 Python",
  "trigger_features": "POST /api/v1/validate/code body.code",
  "verification_notes": "拒绝文本不算成功",
  "confidence": 0.8,
  "exploit_point": {
    "title": "Langflow 未认证远程代码执行",
    "route_or_endpoint": "/api/v1/validate/code",
    "file": "src/backend/base/langflow/api/v1/validate.py",
    "function_or_method": "validate_code",
    "params": "body->code",
    "type": "RCE",
    "payload_idea": "提交 exec 表达式并从响应读取计算结果",
    "expected_impact": "远程代码执行",
    "confidence": "high"
  }
}` + "\n```"
	idea, candidate, err := BuildIdeaFromResponse(raw, profile)
	if err != nil {
		t.Fatalf("BuildIdeaFromResponse: %v", err)
	}
	if idea.State != "未验证" {
		t.Fatalf("expected unverified state, got %q", idea.State)
	}
	if idea.ExploitIdeaId != "PO-"+profile.ID {
		t.Fatalf("unexpected idea id: %q", idea.ExploitIdeaId)
	}
	if idea.ExploitPoint.Title != "Langflow 未认证远程代码执行" || idea.ExploitPoint.Type != "RCE" {
		t.Fatalf("unexpected exploit point: %+v", idea.ExploitPoint)
	}
	if candidate.CVEID != profile.CVE {
		t.Fatalf("expected cve fallback from profile, got %q", candidate.CVEID)
	}
	if candidate.Confidence != 0.8 {
		t.Fatalf("unexpected candidate confidence: %v", candidate.Confidence)
	}
}

func TestBuildIdeaFromResponseRejectsIncompletePayload(t *testing.T) {
	profile, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	if _, _, err := BuildIdeaFromResponse(`{"harm":"x"}`, profile); err == nil {
		t.Fatal("expected error for incomplete payload")
	}
	if _, _, err := BuildIdeaFromResponse("not json", profile); err == nil {
		t.Fatal("expected error for non-JSON payload")
	}
}

package patchintel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
)

const (
	defaultDiffBudgetWithRules   = 22000
	defaultDiffBudgetWithoutRule = 14000
	inversionTimeout             = 180 * time.Second
)

// InversionOutput is the result of a semantic inversion.
type InversionOutput struct {
	Idea           *taskManager.ExploitIdea `json:"idea"`
	CandidateRule  Rule                     `json:"candidate_rule"`
	TemplateSource string                   `json:"template_source"`
	RulesInjected  []int64                  `json:"rules_injected"`
	Model          string                   `json:"model"`
	RawResponse    string                   `json:"-"`
	Prompt         string                   `json:"-"`
}

type inversionPayload struct {
	CVEID             string  `json:"cve_id"`
	VulnType          string  `json:"vuln_type"`
	Harm              string  `json:"harm"`
	Condition         string  `json:"condition"`
	ExtendIdea        string  `json:"extend_idea"`
	EvidenceHint      string  `json:"evidence_hint"`
	Pattern           string  `json:"pattern"`
	TriggerFeatures   string  `json:"trigger_features"`
	VerificationNotes string  `json:"verification_notes"`
	Confidence        float64 `json:"confidence"`
	ExploitPoint      struct {
		Title            string `json:"title"`
		RouteOrEndpoint  string `json:"route_or_endpoint"`
		File             string `json:"file"`
		FunctionOrMethod string `json:"function_or_method"`
		Params           string `json:"params"`
		Type             string `json:"type"`
		PayloadIdea      string `json:"payload_idea"`
		ExpectedImpact   string `json:"expected_impact"`
		Confidence       string `json:"confidence"`
	} `json:"exploit_point"`
}

const builtinSceneTemplate = `# 通用补丁语义反演场景模板

## 目标
从厂商修复补丁反向推断被修复的漏洞，并给出可执行的利用约束。

## 分析要点
1. 补丁改了什么：新增/删除了哪些校验、转义、鉴权、参数绑定或边界检查。
2. 修复语义 = 被堵住的洞：该校验缺失时，攻击者能控制什么输入、到达哪个入口。
3. 受影响版本：修复前的版本区间；补丁未覆盖的分支也要注明。
4. 端点形态：路由/参数位置（query/body/header/cookie）、认证要求。
5. 可观测证据：目标响应中能区分"利用成功"与"拒绝/报错"的特征。

## PoC 骨架要求
- 最小请求即可触发，不依赖内部状态。
- 明确 payload 构造与编码方式。
- 明确成功判定信号；拒绝文本、WAF 拦截和网络错误都不算成功。

## 误报陷阱
- 补丁可能只是重构，不一定是安全修复。
- 校验可能只加在一处，其他调用路径仍可绕过。
- 版本号匹配不代表代码路径可达。
`

// LoadSceneTemplate returns the scene template and its source marker. Missing
// templates fall back to the built-in template (CE6 cold-start fallback).
func LoadSceneTemplate(scene string) (string, string) {
	scene = sanitizeScene(scene)
	if scene == "" {
		return builtinSceneTemplate, "builtin"
	}
	dir := filepath.Join(misc.GetDataDir(), "templates", scene)
	for _, name := range []string{"template.md", "TEMPLATE.md", "template.txt", "README.md"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		return content, "file:" + path
	}
	return builtinSceneTemplate, "builtin"
}

// BuildInversionPrompt assembles the inversion prompt with the default budget.
func BuildInversionPrompt(profile *PatchProfile, rules []Rule, template string) (string, string) {
	budget := defaultDiffBudgetWithoutRule
	if len(rules) > 0 {
		budget = defaultDiffBudgetWithRules
	}
	return BuildInversionPromptWithBudget(profile, rules, template, budget)
}

// BuildInversionPromptWithBudget assembles the inversion prompt with an
// explicit diff budget in runes.
func BuildInversionPromptWithBudget(profile *PatchProfile, rules []Rule, template string, maxDiffRunes int) (string, string) {
	if strings.TrimSpace(template) == "" {
		template = builtinSceneTemplate
	}
	if maxDiffRunes <= 0 {
		maxDiffRunes = defaultDiffBudgetWithoutRule
	}
	diff := ""
	if profile != nil {
		diff = truncateRunes(profile.RawDiff, maxDiffRunes)
	}
	system := `你是 PatchOracle 的补丁语义反演引擎。给定真实厂商补丁 diff，你要反推出补丁所修复的漏洞，并输出可被沙箱验证的利用约束。
要求：
1. 只做反向推断，不臆造 diff 中不存在的入口或参数。
2. 先说明补丁改了什么、堵住了什么，再给出触发条件与证据判定要点。
3. 输出必须是单个 JSON 对象，不要输出 markdown 代码块或解释文字。
4. 若证据不足，在 verification_notes 中明确写出不确定项，并降低 confidence。`
	user := fmt.Sprintf(`## 场景模板
%s

## 补丁档案
- CVE: %s
- GHSA: %s
- 受影响版本: %s
- 修复 commit: %s
- 涉及文件: %s

## 规则库注入（top-k，按相关度与置信度排序）
%s

## 补丁 diff
%s

## 输出 JSON 字段
{
  "cve_id": "CVE 编号，未知则留空",
  "vuln_type": "漏洞类型缩写，如 RCE/SSRF/SQLI/PATH_TRAVERSAL",
  "harm": "危害：利用后能做到什么",
  "condition": "利用条件：认证、版本、配置、入口可达性",
  "extend_idea": "扩大化利用思路",
  "evidence_hint": "沙箱验证时应观察到的成功/失败证据",
  "pattern": "可沉淀为规则的漏洞模式一句话描述",
  "trigger_features": "触发特征：端点形态/参数特征/版本约束",
  "verification_notes": "验证要点与误报陷阱",
  "confidence": 0.0,
  "exploit_point": {
    "title": "漏洞标题（中文）",
    "route_or_endpoint": "路由或接口",
    "file": "文件路径",
    "function_or_method": "函数或方法",
    "params": "参数位置，例如 query->name, body->code",
    "type": "漏洞类型缩写",
    "payload_idea": "payload 构造思路",
    "expected_impact": "预期影响",
    "confidence": "high/medium/low"
  }
}`, template, metadataOrUnknown(profileCVE(profile)), metadataOrUnknown(profileGHSA(profile)),
		metadataOrUnknown(profileVersions(profile)), metadataOrUnknown(profileCommit(profile)),
		metadataOrUnknown(strings.Join(profileFiles(profile), ", ")), formatRules(rules), diff)
	return system, user
}

// Invert runs the LLM semantic inversion for one patch profile.
func Invert(ctx context.Context, task *taskManager.Task, profile *PatchProfile, rules []Rule, scene string) (*InversionOutput, error) {
	if profile == nil {
		return nil, fmt.Errorf("patch profile is nil")
	}
	template, templateSource := LoadSceneTemplate(scene)
	system, user := BuildInversionPrompt(profile, rules, template)
	model := strings.TrimSpace(misc.GetConfigValueDefault("analyze", "MODEL",
		misc.GetConfigValueDefault("main_setting", "MODEL", "")))
	if model == "" {
		return nil, fmt.Errorf("语义反演需要配置模型（[analyze] MODEL 或 [main_setting] MODEL）")
	}
	cli := llm.GetResponsesClient("analyze", "main_setting")
	if cli == nil {
		return nil, fmt.Errorf("无法初始化 LLM 客户端，请检查 [main_setting] 配置")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, inversionTimeout)
		defer cancel()
	}
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: user},
	}
	projectName := ""
	if task != nil {
		projectName = task.GetProjectName()
	}
	resp, err := llm.RequestLLMWithOpts(cli, ctx, model, messages, nil, llm.RequestLLMOpts{
		ProjectName: projectName,
		AgentLabel:  "patch-oracle-inverter",
	})
	if err != nil {
		return nil, fmt.Errorf("语义反演 LLM 调用失败: %w", err)
	}
	idea, candidate, err := BuildIdeaFromResponse(resp.Content, profile)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return &InversionOutput{
		Idea:           idea,
		CandidateRule:  candidate,
		TemplateSource: templateSource,
		RulesInjected:  ids,
		Model:          model,
		RawResponse:    resp.Content,
		Prompt:         user,
	}, nil
}

// BuildIdeaFromResponse parses the model output into an ExploitIdea plus a
// candidate rule. It is exported so tests and callers can validate parsing
// without a live LLM.
func BuildIdeaFromResponse(raw string, profile *PatchProfile) (*taskManager.ExploitIdea, Rule, error) {
	payload, err := parseInversionPayload(raw)
	if err != nil {
		return nil, Rule{}, err
	}
	if strings.TrimSpace(payload.Harm) == "" || strings.TrimSpace(payload.Condition) == "" {
		return nil, Rule{}, fmt.Errorf("语义反演输出缺少 harm 或 condition")
	}
	if strings.TrimSpace(payload.ExploitPoint.Title) == "" {
		return nil, Rule{}, fmt.Errorf("语义反演输出缺少 exploit_point.title")
	}
	profileIDValue := "unknown"
	if profile != nil && profile.ID != "" {
		profileIDValue = profile.ID
	}
	ideaID := "PO-" + profileIDValue
	vulnType := strings.ToUpper(strings.TrimSpace(payload.ExploitPoint.Type))
	if vulnType == "" {
		vulnType = strings.ToUpper(strings.TrimSpace(payload.VulnType))
	}
	if vulnType == "" {
		vulnType = "UNKNOWN"
	}
	confidence := strings.TrimSpace(payload.ExploitPoint.Confidence)
	if confidence == "" {
		confidence = "medium"
	}
	idea := &taskManager.ExploitIdea{
		ExploitIdeaId: ideaID,
		Harm:          strings.TrimSpace(payload.Harm),
		ExtendIdea:    strings.TrimSpace(payload.ExtendIdea),
		Condition:     strings.TrimSpace(payload.Condition),
		State:         "未验证",
		Evidence:      strings.TrimSpace(payload.EvidenceHint),
		ExploitPoint: &taskManager.ExploitPoint{
			Confidence:       confidence,
			ExpectedImpact:   strings.TrimSpace(payload.ExploitPoint.ExpectedImpact),
			File:             strings.TrimSpace(payload.ExploitPoint.File),
			FunctionOrMethod: strings.TrimSpace(payload.ExploitPoint.FunctionOrMethod),
			Params:           strings.TrimSpace(payload.ExploitPoint.Params),
			PayloadIdea:      strings.TrimSpace(payload.ExploitPoint.PayloadIdea),
			RouteOrEndpoint:  strings.TrimSpace(payload.ExploitPoint.RouteOrEndpoint),
			Title:            strings.TrimSpace(payload.ExploitPoint.Title),
			Type:             vulnType,
			ExploitId:        ideaID,
		},
	}
	pattern := strings.TrimSpace(payload.Pattern)
	if pattern == "" {
		pattern = strings.TrimSpace(payload.VulnType) + ": " + idea.ExploitPoint.Title
	}
	cveID := strings.TrimSpace(payload.CVEID)
	if cveID == "" && profile != nil {
		cveID = profile.CVE
	}
	candidate := Rule{
		CVEID:             cveID,
		Pattern:           pattern,
		TriggerFeatures:   strings.TrimSpace(payload.TriggerFeatures),
		VerificationNotes: strings.TrimSpace(payload.VerificationNotes),
		Confidence:        clampConfidence(payload.Confidence),
	}
	return idea, candidate, nil
}

func parseInversionPayload(raw string) (*inversionPayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("语义反演返回为空")
	}
	raw = stripCodeFence(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("语义反演输出不是 JSON 对象")
	}
	payload := &inversionPayload{}
	if err := misc.UnmarshalJSONLenient(raw[start:end+1], payload); err != nil {
		return nil, fmt.Errorf("解析语义反演 JSON 失败: %w", err)
	}
	return payload, nil
}

func stripCodeFence(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "```") {
		return raw
	}
	lines := strings.Split(raw, "\n")
	if len(lines) >= 2 {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatRules(rules []Rule) string {
	if len(rules) == 0 {
		return "（规则库无命中，使用场景模板冷启动兜底；本次验证成功的新模式请通过 PromoteRuleTool 沉淀）"
	}
	var b strings.Builder
	for i, rule := range rules {
		fmt.Fprintf(&b, "[规则 %d] id=%d version=%d confidence=%.2f\n", i+1, rule.ID, rule.Version, rule.Confidence)
		b.WriteString("模式: " + rule.Pattern + "\n")
		if rule.TriggerFeatures != "" {
			b.WriteString("触发特征: " + rule.TriggerFeatures + "\n")
		}
		if rule.VerificationNotes != "" {
			b.WriteString("验证要点: " + rule.VerificationNotes + "\n")
		}
		if rule.CVEID != "" {
			b.WriteString("来源: " + rule.CVEID + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func truncateRunes(raw string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(raw) <= maxRunes {
		return raw
	}
	runes := []rune(raw)
	return string(runes[:maxRunes]) + "\n[补丁 diff 已按 token 预算截断]"
}

func metadataOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未知"
	}
	return value
}

func profileCVE(profile *PatchProfile) string {
	if profile == nil {
		return ""
	}
	return profile.CVE
}

func profileGHSA(profile *PatchProfile) string {
	if profile == nil {
		return ""
	}
	return profile.GHSA
}

func profileVersions(profile *PatchProfile) string {
	if profile == nil {
		return ""
	}
	return profile.AffectedVersions
}

func profileCommit(profile *PatchProfile) string {
	if profile == nil {
		return ""
	}
	return profile.FixCommit
}

func profileFiles(profile *PatchProfile) []string {
	if profile == nil {
		return nil
	}
	return profile.Files
}

package marketplace

// AiToolCategory classifies how an AI tool integrates with the platform.
type AiToolCategory string

const (
	AiToolCloudComputer AiToolCategory = "CLOUD_COMPUTER"
	AiToolCLI           AiToolCategory = "AI_CLI"
	AiToolIDE           AiToolCategory = "AI_IDE"
	AiToolChatUI        AiToolCategory = "CHAT_UI"
)

// AiToolGrade is the trust grade of an integration.
type AiToolGrade string

const (
	AiToolGradeS AiToolGrade = "S"
	AiToolGradeA AiToolGrade = "A"
	AiToolGradeB AiToolGrade = "B"
	AiToolGradeC AiToolGrade = "C"
)

// AiToolStatus is the connection state of an integration.
type AiToolStatus string

const (
	AiToolConnected AiToolStatus = "CONNECTED"
	AiToolPending   AiToolStatus = "PENDING"
	AiToolSandbox   AiToolStatus = "SANDBOX"
)

// ValidationVerdict is the outcome of one agent or the consensus of both.
type ValidationVerdict string

const (
	VerdictPass    ValidationVerdict = "PASS"
	VerdictFail    ValidationVerdict = "FAIL"
	VerdictPending ValidationVerdict = "PENDING"
)

// ComplianceSeverity is the impact level of a compliance finding.
type ComplianceSeverity string

const (
	ComplianceLow      ComplianceSeverity = "LOW"
	ComplianceMedium   ComplianceSeverity = "MEDIUM"
	ComplianceHigh     ComplianceSeverity = "HIGH"
	ComplianceCritical ComplianceSeverity = "CRITICAL"
)

// ComplianceStatus is a compliance rule outcome.
type ComplianceStatus string

const (
	CompliancePass  ComplianceStatus = "PASS"
	ComplianceWarn  ComplianceStatus = "WARN"
	ComplianceBlock ComplianceStatus = "BLOCK"
)

// RiskLevel is the risk band derived from a 0-100 score.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// AiToolIntegration is one row of the access matrix.
type AiToolIntegration struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Vendor      string         `json:"vendor"`
	Category    AiToolCategory `json:"category"`
	Protocol    string         `json:"protocol"`
	Grade       AiToolGrade    `json:"grade"`
	Status      AiToolStatus   `json:"status"`
	Roles       []UserRole     `json:"roles"`
	Permissions []string       `json:"permissions"`
	Compliance  []string       `json:"compliance"`
}

// CrossValidation is a two-agent generation/review run.
type CrossValidation struct {
	TaskID           string            `json:"taskId"`
	Generator        string            `json:"generator"`
	GeneratorModel   string            `json:"generatorModel"`
	GeneratorVerdict ValidationVerdict `json:"generatorVerdict"`
	Reviewer         string            `json:"reviewer"`
	ReviewerModel    string            `json:"reviewerModel"`
	ReviewerVerdict  ValidationVerdict `json:"reviewerVerdict"`
	Consensus        ValidationVerdict `json:"consensus"`
	Confidence       float64           `json:"confidence"`
	Cost             float64           `json:"cost"`
	ElapsedMs        int               `json:"elapsedMs"`
	At               string            `json:"at"`
}

// ComplianceFinding is one PCPD-baseline rule hit.
type ComplianceFinding struct {
	ID       string             `json:"id"`
	Rule     string             `json:"rule"`
	Severity ComplianceSeverity `json:"severity"`
	Status   ComplianceStatus   `json:"status"`
	Detail   string             `json:"detail"`
}

// RiskAssessment is the 0-100 risk score with threshold and block state.
type RiskAssessment struct {
	Score     int       `json:"score"`
	Level     RiskLevel `json:"level"`
	Threshold int       `json:"threshold"`
	Blocked   bool      `json:"blocked"`
	Reasons   []string  `json:"reasons"`
}

// PricingItem is one trust-service price tag.
type PricingItem struct {
	Service string  `json:"service"`
	Unit    string  `json:"unit"`
	Price   float64 `json:"price"`
}

// ExpertOverview is GET /api/expert/overview.
type ExpertOverview struct {
	Tools      []AiToolIntegration `json:"tools"`
	Validation CrossValidation     `json:"validation"`
	Compliance []ComplianceFinding `json:"compliance"`
	Risk       RiskAssessment      `json:"risk"`
	Pricing    []PricingItem       `json:"pricing"`
}

// TaskTrust is GET /api/tasks/{id}/trust.
type TaskTrust struct {
	TaskID     string              `json:"taskId"`
	Validation CrossValidation     `json:"validation"`
	Compliance []ComplianceFinding `json:"compliance"`
	Risk       RiskAssessment      `json:"risk"`
}

// ExpertState is the persisted trust-layer snapshot.
type ExpertState struct {
	Tools      []AiToolIntegration `json:"tools"`
	Validation CrossValidation     `json:"validation"`
	Compliance []ComplianceFinding `json:"compliance"`
	Risk       RiskAssessment      `json:"risk"`
	Pricing    []PricingItem       `json:"pricing"`
}

func seedExpert() ExpertState {
	return ExpertState{
		Tools: []AiToolIntegration{
			{
				ID: "tool_grok_bot", Name: "Grok Bot", Vendor: "xAI",
				Category: AiToolCloudComputer, Protocol: "MCP",
				Grade: AiToolGradeS, Status: AiToolConnected,
				Roles:       []UserRole{RoleEmployer, RoleCommander},
				Permissions: []string{"task.publish", "task.execute", "trace.read"},
				Compliance:  []string{"PCPD"},
			},
			{
				ID: "tool_chatgpt_dot", Name: "ChatGPT Dot", Vendor: "OpenAI",
				Category: AiToolCloudComputer, Protocol: "MCP",
				Grade: AiToolGradeA, Status: AiToolPending,
				Roles:       []UserRole{RoleEmployer},
				Permissions: []string{"task.publish", "trace.read"},
				Compliance:  []string{"PCPD"},
			},
			{
				ID: "tool_claude_code", Name: "Claude Code", Vendor: "Anthropic",
				Category: AiToolCLI, Protocol: "CLI",
				Grade: AiToolGradeS, Status: AiToolConnected,
				Roles:       []UserRole{RoleEmployer, RoleCommander},
				Permissions: []string{"task.publish", "task.execute", "trace.read", "trace.write"},
				Compliance:  []string{"PCPD"},
			},
			{
				ID: "tool_codex_cli", Name: "Codex CLI", Vendor: "OpenAI",
				Category: AiToolCLI, Protocol: "CLI",
				Grade: AiToolGradeA, Status: AiToolConnected,
				Roles:       []UserRole{RoleEmployer, RoleCommander},
				Permissions: []string{"task.execute", "trace.write"},
				Compliance:  []string{"PCPD"},
			},
			{
				ID: "tool_cursor", Name: "Cursor", Vendor: "Anysphere",
				Category: AiToolIDE, Protocol: "IDE Plugin",
				Grade: AiToolGradeB, Status: AiToolSandbox,
				Roles:       []UserRole{RoleEmployer},
				Permissions: []string{"task.publish"},
				Compliance:  []string{"PCPD"},
			},
			{
				ID: "tool_trae", Name: "Trae", Vendor: "ByteDance",
				Category: AiToolIDE, Protocol: "IDE Plugin",
				Grade: AiToolGradeB, Status: AiToolConnected,
				Roles:       []UserRole{RoleEmployer, RoleCommander},
				Permissions: []string{"task.execute", "trace.write"},
				Compliance:  []string{"PCPD"},
			},
			{
				ID: "tool_chatbox", Name: "Chatbox", Vendor: "Chatbox AI",
				Category: AiToolChatUI, Protocol: "Screen",
				Grade: AiToolGradeC, Status: AiToolPending,
				Roles:       []UserRole{RoleCommander},
				Permissions: []string{"task.execute"},
				Compliance:  []string{"PCPD"},
			},
		},
		Validation: CrossValidation{
			TaskID:           "task_1001",
			Generator:        "Claude",
			GeneratorModel:   "claude-opus-5-5",
			GeneratorVerdict: VerdictPass,
			Reviewer:         "Codex",
			ReviewerModel:    "gpt-5-codex",
			ReviewerVerdict:  VerdictPass,
			Consensus:        VerdictPass,
			Confidence:       0.94,
			Cost:             12.6,
			ElapsedMs:        8421,
			At:               "2026-11-16T02:40:00Z",
		},
		Compliance: []ComplianceFinding{
			{ID: "cmp_001", Rule: "PCPD-ART-13 个人信息跨境传输", Severity: ComplianceHigh, Status: ComplianceBlock, Detail: "任务输入包含境外传输字段，需先完成跨境评估并显式阻断。"},
			{ID: "cmp_002", Rule: "PCPD-ART-17 最小必要原则", Severity: ComplianceMedium, Status: ComplianceWarn, Detail: "采集字段超出验收所需范围，建议删减非必要样本。"},
			{ID: "cmp_003", Rule: "PCPD-ART-24 数据留存期限", Severity: ComplianceLow, Status: ComplianceWarn, Detail: "默认留存 90 天，请确认与委托方约定一致。"},
			{ID: "cmp_004", Rule: "PCPD-ART-09 安全审计日志", Severity: ComplianceLow, Status: CompliancePass, Detail: "审计日志、哈希链与操作授权记录完整。"},
		},
		Risk: RiskAssessment{
			Score:     82,
			Level:     RiskHigh,
			Threshold: 70,
			Blocked:   true,
			Reasons: []string{
				"PCPD BLOCK 规则命中，已阻断结算",
				"任务涉及跨境数据传输路径，需补充合法依据",
			},
		},
		Pricing: []PricingItem{
			{Service: "交叉验证", Unit: "次", Price: 8},
			{Service: "合规扫描", Unit: "次", Price: 5},
			{Service: "风险阻断", Unit: "次", Price: 3},
		},
	}
}

// ExpertOverview returns a copy of the trust-layer overview.
func (s *Store) ExpertOverview() ExpertOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ExpertOverview{
		Tools:      cloneTools(s.expert.Tools),
		Validation: s.expert.Validation,
		Compliance: cloneCompliance(s.expert.Compliance),
		Risk:       cloneRisk(s.expert.Risk),
		Pricing:    append([]PricingItem(nil), s.expert.Pricing...),
	}
}

// TaskTrust returns the trust pipeline for a known task. Validation verdicts,
// compliance findings and risk are all derived from the task's current status
// so an unfinished task can never be reported as fully validated.
func (s *Store) TaskTrust(taskID string) (TaskTrust, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record := s.findTaskLocked(taskID)
	if record == nil {
		return TaskTrust{}, false
	}

	highRisk := isHighRiskTask(taskID)
	compliance := complianceForTask(highRisk)
	risk := riskForTask(highRisk)
	// Hard invariant: blocked is true iff a BLOCK finding exists.
	risk.Blocked = hasBlockFinding(compliance)

	return TaskTrust{
		TaskID:     taskID,
		Validation: validationForStatus(taskID, record.detail.Status),
		Compliance: compliance,
		Risk:       risk,
	}, true
}

// isHighRiskTask reports whether a seeded task triggers the PCPD cross-border
// BLOCK rule. Newly published tasks default to the non-blocking profile.
func isHighRiskTask(taskID string) bool {
	switch taskID {
	case "task_1003", "task_1004", "task_1007":
		return true
	default:
		return false
	}
}

// validationForStatus maps the task lifecycle to the cross-validation result.
func validationForStatus(taskID string, status TaskStatus) CrossValidation {
	validation := CrossValidation{
		TaskID:           taskID,
		Generator:        "Claude",
		GeneratorModel:   "claude-opus-5-5",
		GeneratorVerdict: VerdictPending,
		Reviewer:         "Codex",
		ReviewerModel:    "gpt-5-codex",
		ReviewerVerdict:  VerdictPending,
		Consensus:        VerdictPending,
		At:               "2026-11-16T02:40:00Z",
	}

	switch status {
	case StatusSubmitted, StatusAIValidating:
		validation.GeneratorVerdict = VerdictPass
		validation.Confidence = 0.60
		validation.Cost = 6.2
		validation.ElapsedMs = 3200
	case StatusValidationFailed:
		validation.GeneratorVerdict = VerdictPass
		validation.ReviewerVerdict = VerdictFail
		validation.Consensus = VerdictFail
		validation.Confidence = 0.35
		validation.Cost = 9.8
		validation.ElapsedMs = 6100
	case StatusComplianceScan, StatusSettling, StatusCompleted:
		validation.GeneratorVerdict = VerdictPass
		validation.ReviewerVerdict = VerdictPass
		validation.Consensus = VerdictPass
		validation.Confidence = 0.94
		validation.Cost = 12.6
		validation.ElapsedMs = 8421
	}

	return validation
}

// complianceForTask returns the four PCPD-baseline findings for a task. The
// cross-border rule blocks high-risk tasks and only warns on the others.
func complianceForTask(highRisk bool) []ComplianceFinding {
	crossBorderStatus := ComplianceWarn
	if highRisk {
		crossBorderStatus = ComplianceBlock
	}
	return []ComplianceFinding{
		{ID: "cmp_001", Rule: "PCPD-ART-13 个人数据跨境传输", Severity: ComplianceHigh, Status: crossBorderStatus, Detail: "任务输入包含境外传输字段，需先完成跨境评估并显式阻断。"},
		{ID: "cmp_002", Rule: "PCPD-ART-17 最小必要原则", Severity: ComplianceMedium, Status: ComplianceWarn, Detail: "采集字段超出验收所需范围，建议删减非必要样本。"},
		{ID: "cmp_003", Rule: "PCPD-ART-24 数据留存期限", Severity: ComplianceLow, Status: ComplianceWarn, Detail: "默认留存 90 天，请确认与委托方约定一致。"},
		{ID: "cmp_004", Rule: "PCPD-ART-09 安全审计日志", Severity: ComplianceLow, Status: CompliancePass, Detail: "审计日志、哈希链与操作授权记录完整。"},
	}
}

// riskForTask returns the risk assessment paired with complianceForTask.
func riskForTask(highRisk bool) RiskAssessment {
	if highRisk {
		return RiskAssessment{
			Score:     82,
			Level:     RiskHigh,
			Threshold: 70,
			Blocked:   true,
			Reasons: []string{
				"PCPD BLOCK 规则命中，已阻断结算",
				"任务涉及跨境数据传输路径，需补充合法依据",
			},
		}
	}
	return RiskAssessment{
		Score:     58,
		Level:     RiskMedium,
		Threshold: 70,
		Blocked:   false,
		Reasons: []string{
			"任务涉及跨境数据传输路径，需补充合法依据",
			"沙箱内执行授权尚未完成二次复核",
		},
	}
}

// hasBlockFinding reports whether any compliance finding is a hard block.
func hasBlockFinding(findings []ComplianceFinding) bool {
	for _, finding := range findings {
		if finding.Status == ComplianceBlock {
			return true
		}
	}
	return false
}

func cloneTools(input []AiToolIntegration) []AiToolIntegration {
	output := make([]AiToolIntegration, len(input))
	for index, tool := range input {
		output[index] = tool
		output[index].Roles = append([]UserRole(nil), tool.Roles...)
		output[index].Permissions = cloneStrings(tool.Permissions)
		output[index].Compliance = cloneStrings(tool.Compliance)
	}
	return output
}

func cloneCompliance(input []ComplianceFinding) []ComplianceFinding {
	return append([]ComplianceFinding(nil), input...)
}

func cloneRisk(input RiskAssessment) RiskAssessment {
	output := input
	output.Reasons = cloneStrings(input.Reasons)
	return output
}

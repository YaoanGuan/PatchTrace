package toolCalling

import (
	"AIxVuln/patchintel"
	"AIxVuln/taskManager"
	"strconv"
	"strings"
)

// PromoteRuleTool writes a new rule version, or records a verification outcome
// for an existing rule.
type PromoteRuleTool struct {
	task *taskManager.Task
}

func NewPromoteRuleTool(task *taskManager.Task) *PromoteRuleTool {
	return &PromoteRuleTool{task: task}
}

func (h *PromoteRuleTool) Name() string {
	return "PromoteRuleTool"
}

func (h *PromoteRuleTool) Description() string {
	return "Promote a reusable vulnerability pattern into the versioned rule base, or record a verification outcome for an existing rule. Rule changes never overwrite history: every change creates a new version and adjusts confidence."
}

func (h *PromoteRuleTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "One-sentence vulnerability pattern, for example: unauthenticated code-validation endpoint executes user Python.(required for new rules)",
			},
			"cveId": map[string]interface{}{
				"type":        "string",
				"description": "Optional source CVE id.",
			},
			"triggerFeatures": map[string]interface{}{
				"type":        "string",
				"description": "Trigger features: endpoint shape, parameter location, version constraints.",
			},
			"verificationNotes": map[string]interface{}{
				"type":        "string",
				"description": "Verification notes and false-positive traps.",
			},
			"confidence": map[string]interface{}{
				"type":        "number",
				"description": "Initial confidence in [0,1]. Default 0.5.",
			},
			"ruleId": map[string]interface{}{
				"type":        "integer",
				"description": "Existing rule id. When set together with outcome, records a verification outcome instead of creating a new pattern.",
			},
			"outcome": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"confirmed", "refuted"},
				"description": "Verification outcome for ruleId: confirmed raises confidence, refuted lowers it.",
			},
		},
	}
}

func (h *PromoteRuleTool) Execute(parameters map[string]interface{}) string {
	ruleID := 0
	if raw := parameters["ruleId"]; raw != nil {
		value, err := readSkillIntArg(raw, 0)
		if err != nil {
			return Fail("Invalid 'ruleId' parameter: " + err.Error())
		}
		ruleID = value
	}
	outcome := strings.ToLower(strings.TrimSpace(stringParam(parameters["outcome"])))
	ruleBase, err := patchintel.DefaultRuleBase()
	if err != nil {
		return Fail("open rule base failed: " + err.Error())
	}
	if ruleID > 0 && outcome != "" {
		confirmed, ok := normalizeOutcome(outcome)
		if !ok {
			return Fail("Invalid 'outcome' parameter: expected confirmed or refuted")
		}
		updated, err := ruleBase.UpdateOutcome(int64(ruleID), confirmed, stringParam(parameters["verificationNotes"]))
		if err != nil {
			return Fail("update rule outcome failed: " + err.Error())
		}
		return Success(map[string]interface{}{
			"mode":   "outcome",
			"rule":   updated,
			"result": "verification outcome recorded as a new rule version",
		})
	}
	pattern := strings.TrimSpace(stringParam(parameters["pattern"]))
	if pattern == "" {
		return Fail("Missing 'pattern' parameter (or pass ruleId + outcome to record an outcome)")
	}
	created, err := ruleBase.Promote(patchintel.Rule{
		CVEID:             stringParam(parameters["cveId"]),
		Pattern:           pattern,
		TriggerFeatures:   stringParam(parameters["triggerFeatures"]),
		VerificationNotes: stringParam(parameters["verificationNotes"]),
		Confidence:        floatParam(parameters["confidence"], 0.5),
	})
	if err != nil {
		return Fail("promote rule failed: " + err.Error())
	}
	return Success(map[string]interface{}{
		"mode":   "promote",
		"rule":   created,
		"result": "new rule version stored",
	})
}

func normalizeOutcome(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "confirmed", "confirm", "success", "true", "可利用", "验证成功":
		return true, true
	case "refuted", "refute", "failed", "false", "验证失败", "不可利用":
		return false, true
	}
	return false, false
}

func floatParam(raw interface{}, defaultValue float64) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return defaultValue
		}
		return parsed
	}
	return defaultValue
}

package toolCalling

import (
	"AIxVuln/patchintel"
	"AIxVuln/taskManager"
	"context"
	"strings"
)

// InvertPatchTool runs the patch semantic inversion chain and injects the
// resulting ExploitIdea into the existing candidate state machine.
type InvertPatchTool struct {
	task *taskManager.Task
}

func NewInvertPatchTool(task *taskManager.Task) *InvertPatchTool {
	return &InvertPatchTool{task: task}
}

func (h *InvertPatchTool) Name() string {
	return "InvertPatchTool"
}

func (h *InvertPatchTool) Description() string {
	return "Run semantic inversion on an ingested patch profile: retrieve top-k rules, call the LLM to infer the fixed vulnerability, and return an ExploitIdea triple (condition/harm/evidence). By default the result is submitted to the existing exploit-idea gate."
}

func (h *InvertPatchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"patchId": map[string]interface{}{
				"type":        "string",
				"description": "patch_id returned by IngestPatchTool.(required)",
			},
			"scene": map[string]interface{}{
				"type":        "string",
				"description": "Optional scene template override, for example langflow.",
			},
			"maxRules": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of rules injected into the prompt. Default 5, max 20.",
				"default":     5,
			},
			"submit": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to submit the inferred ExploitIdea to the existing candidate gate. Default true.",
				"default":     true,
			},
		},
		"required": []string{"patchId"},
	}
}

func (h *InvertPatchTool) Execute(parameters map[string]interface{}) string {
	patchID := stringParam(parameters["patchId"])
	if patchID == "" {
		return Fail("Missing 'patchId' parameter")
	}
	maxRules := 5
	if raw := parameters["maxRules"]; raw != nil {
		value, err := readSkillIntArg(raw, 5)
		if err != nil {
			return Fail("Invalid 'maxRules' parameter: " + err.Error())
		}
		maxRules = value
	}
	if maxRules <= 0 {
		maxRules = 5
	}
	if maxRules > 20 {
		maxRules = 20
	}
	submit := true
	if raw, ok := parameters["submit"]; ok && raw != nil {
		submit = boolParam(raw)
	}
	profile, err := patchintel.GetProfile(patchID)
	if err != nil {
		return Fail("load patch profile failed: " + err.Error())
	}
	ruleBase, err := patchintel.DefaultRuleBase()
	if err != nil {
		return Fail("open rule base failed: " + err.Error())
	}
	scene := stringParam(parameters["scene"])
	if scene == "" {
		scene = profile.Scene
	}
	query := strings.Join([]string{profile.CVE, profile.GHSA, profile.Scene, scene, strings.Join(profile.Files, " ")}, " ")
	rules, err := ruleBase.Search(query, maxRules)
	if err != nil {
		return Fail("query rule base failed: " + err.Error())
	}
	output, err := patchintel.Invert(context.Background(), h.task, profile, rules, scene)
	if err != nil {
		return Fail("semantic inversion failed: " + err.Error())
	}
	if err := ruleBase.RecordHits(output.RulesInjected); err != nil {
		// Rule hit accounting is best-effort; inversion already succeeded.
		_ = err
	}
	// When the inferred pattern already exists in the rule base, link the idea
	// to it so the sandbox verification outcome flows back into the rule base:
	// a confirmation opens a new rule version, a refutation lowers confidence
	// on the existing row in place.
	var linkedRuleID int64
	linkError := ""
	if existing, err := ruleBase.FindLatestByPattern(output.CandidateRule.Pattern); err != nil {
		linkError = err.Error()
	} else if existing != nil {
		if err := ruleBase.LinkIdea(output.Idea.ExploitIdeaId, existing.ID); err != nil {
			linkError = err.Error()
		} else {
			linkedRuleID = existing.ID
		}
	}
	submitted := false
	submitError := ""
	if submit && h.task != nil {
		if handler := h.task.GetCandidateExploitIdeaHandler(); handler != nil {
			if err := handler(output.Idea); err != nil {
				submitError = err.Error()
			} else {
				submitted = true
			}
		}
	}
	return Success(map[string]interface{}{
		"patch_id":           profile.ID,
		"idea":               output.Idea,
		"candidate_rule":     output.CandidateRule,
		"template_source":    output.TemplateSource,
		"rules_injected":     len(output.RulesInjected),
		"rules_injected_ids": output.RulesInjected,
		"model":              output.Model,
		"submitted":          submitted,
		"submit_error":       submitError,
		"linked_rule_id":     linkedRuleID,
		"link_error":         linkError,
	})
}

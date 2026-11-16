package toolCalling

import (
	"AIxVuln/patchintel"
	"AIxVuln/taskManager"
	"strings"
)

// QueryRuleBaseTool retrieves rules from the PatchOracle rule base.
type QueryRuleBaseTool struct {
	task *taskManager.Task
}

func NewQueryRuleBaseTool(task *taskManager.Task) *QueryRuleBaseTool {
	return &QueryRuleBaseTool{task: task}
}

func (h *QueryRuleBaseTool) Name() string {
	return "QueryRuleBaseTool"
}

func (h *QueryRuleBaseTool) Description() string {
	return "Query the PatchOracle rule base. Returns the latest version of matching vulnerability patterns ranked by keyword relevance and confidence. Use it to learn which patterns were already confirmed or refuted."
}

func (h *QueryRuleBaseTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Optional keyword query, for example the endpoint, framework, or vulnerability type.",
			},
			"cveId": map[string]interface{}{
				"type":        "string",
				"description": "Optional CVE id filter.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of rules to return. Default 5, max 50.",
				"default":     5,
			},
		},
	}
}

func (h *QueryRuleBaseTool) Execute(parameters map[string]interface{}) string {
	limit := 5
	if raw := parameters["limit"]; raw != nil {
		value, err := readSkillIntArg(raw, 5)
		if err != nil {
			return Fail("Invalid 'limit' parameter: " + err.Error())
		}
		limit = value
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}
	ruleBase, err := patchintel.DefaultRuleBase()
	if err != nil {
		return Fail("open rule base failed: " + err.Error())
	}
	query := strings.TrimSpace(stringParam(parameters["query"]))
	cveID := strings.TrimSpace(stringParam(parameters["cveId"]))
	var rules []patchintel.Rule
	if query == "" {
		rules, err = ruleBase.LatestPatterns(cveID, limit)
	} else {
		rules, err = ruleBase.Search(query, limit)
	}
	if err != nil {
		return Fail("query rule base failed: " + err.Error())
	}
	if cveID != "" && query != "" {
		filtered := make([]patchintel.Rule, 0, len(rules))
		for _, rule := range rules {
			if strings.EqualFold(rule.CVEID, cveID) {
				filtered = append(filtered, rule)
			}
		}
		rules = filtered
	}
	return Success(map[string]interface{}{
		"query":      query,
		"cve_id":     cveID,
		"rule_count": len(rules),
		"rules":      rules,
	})
}

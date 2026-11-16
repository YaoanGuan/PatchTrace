package toolCalling

import (
	"AIxVuln/taskManager"
	"strings"
)

type RequestDecisionApprovalTool struct {
	task *taskManager.Task
}

func NewRequestDecisionApprovalTool(task *taskManager.Task) *RequestDecisionApprovalTool {
	return &RequestDecisionApprovalTool{task: task}
}

func (h *RequestDecisionApprovalTool) Name() string {
	return "RequestDecisionApprovalTool"
}

func (h *RequestDecisionApprovalTool) Description() string {
	return "Request explicit user approval through the unified decision queue. Target-interaction tools are centrally intercepted automatically, so do not call this tool merely to approve a normal target tool execution. Use kind=target_action only for non-tool strategic decisions that still need explicit user confirmation. Use kind=abandon_idea when you found a plausible idea, tried it, failed to make it work, and are about to abandon it; the user may approve abandoning it or reject abandonment with guidance to continue. In system-wide full automation mode, target_action may be auto-approved and abandon_idea requests are auto-approved as abandoned."
}

func (h *RequestDecisionApprovalTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"kind": map[string]interface{}{
				"type":        "string",
				"enum":        []string{taskManager.DecisionApprovalKindTargetAction, taskManager.DecisionApprovalKindAbandonIdea},
				"description": "Decision type. Use target_action for non-tool strategic decisions needing explicit confirmation. Use abandon_idea before giving up a plausible but currently unsuccessful idea. Defaults to target_action.",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Short user-facing title for the decision request, for example: 批准执行破坏性验证 or 是否放弃 SSRF 思路.",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "The target system, host, URL, account, database, dataset, or scope affected by the action.",
			},
			"action": map[string]interface{}{
				"type":        "string",
				"description": "For target_action: the exact target action you want approval to perform. For abandon_idea: the idea/hypothesis/attack path you are about to abandon. Required.",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "For target_action: why this action is needed. For abandon_idea: what you tried, what evidence failed, and why you think continuing is unlikely to pay off. Required.",
			},
			"impact": map[string]interface{}{
				"type":        "string",
				"description": "For target_action: potential impact or risk. For abandon_idea: what may be missed if this idea is abandoned.",
			},
			"safer_alternative": map[string]interface{}{
				"type":        "string",
				"description": "For target_action: lower-risk alternative tried or considered. For abandon_idea: fallback path or next idea you plan to pursue after abandoning this one.",
			},
		},
		"required": []string{"action", "reason"},
	}
}

func (h *RequestDecisionApprovalTool) Execute(parameters map[string]interface{}) string {
	if h == nil || h.task == nil {
		return Fail("decision approval handler is unavailable")
	}
	handler := h.task.GetDecisionApprovalHandler()
	if handler == nil {
		return Fail("decision approval handler is unavailable")
	}
	action := strings.TrimSpace(stringParam(parameters["action"]))
	if action == "" {
		return Fail("Missing 'action' parameter")
	}
	reason := strings.TrimSpace(stringParam(parameters["reason"]))
	if reason == "" {
		return Fail("Missing 'reason' parameter")
	}
	kind := strings.TrimSpace(stringParam(parameters["kind"]))
	if kind == "" {
		kind = taskManager.DecisionApprovalKindTargetAction
	}
	if kind != taskManager.DecisionApprovalKindTargetAction && kind != taskManager.DecisionApprovalKindAbandonIdea {
		return Fail("Unsupported 'kind' parameter")
	}
	req := taskManager.DecisionApprovalRequest{
		Kind:             kind,
		Title:            strings.TrimSpace(stringParam(parameters["title"])),
		Target:           strings.TrimSpace(stringParam(parameters["target"])),
		Action:           action,
		Reason:           reason,
		Impact:           strings.TrimSpace(stringParam(parameters["impact"])),
		SaferAlternative: strings.TrimSpace(stringParam(parameters["safer_alternative"])),
	}
	if req.Title == "" {
		if kind == taskManager.DecisionApprovalKindAbandonIdea {
			req.Title = "是否放弃当前思路"
		} else {
			req.Title = "高风险目标操作审批"
		}
	}
	result := handler(req)
	if result.Approved || result.Status == "approved" {
		return Success(result)
	}
	fallback := "decision request was not approved"
	if kind == taskManager.DecisionApprovalKindAbandonIdea {
		fallback = "user asked you to continue this idea instead of abandoning it"
	}
	return FailWithMeta(firstDecisionApprovalText(result.Error, result.DecisionNote, fallback), map[string]any{
		"request_id": result.RequestID,
		"status":     result.Status,
		"kind":       result.Kind,
		"title":      result.Title,
	})
}

func firstDecisionApprovalText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

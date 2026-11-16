package toolCalling

import (
	"AIxVuln/taskManager"
	"fmt"
	"sort"
	"strings"
)

const targetActionIntentParam = "intent"

var targetActionToolNames = map[string]bool{
	"RunCommandTool":                 true,
	"RunPythonCodeTool":              true,
	"RunPHPCodeTool":                 true,
	"RunSQLTool":                     true,
	"RunSSHCommandTool":              true,
	"WriteBackgroundCommandTool":     true,
	"ReadBackgroundCommandTool":      true,
	"TerminateBackgroundCommandTool": true,
	"RequestHostCommandTool":         true,
}

func shouldRequireTargetActionIntent(toolName string) bool {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "Docker") {
		return false
	}
	if strings.HasPrefix(name, "MCP_") {
		return true
	}
	return targetActionToolNames[name]
}

func shouldRequireTargetActionApproval(toolName string) bool {
	name := strings.TrimSpace(toolName)
	if name == "RequestHostCommandTool" {
		return false
	}
	return shouldRequireTargetActionIntent(name)
}

func withTargetActionIntentParameter(parameters map[string]interface{}) map[string]interface{} {
	out := deepCloneMap(parameters)
	if len(out) == 0 {
		out = map[string]interface{}{"type": "object"}
	}
	props, _ := out["properties"].(map[string]interface{})
	if props == nil {
		props = map[string]interface{}{}
		out["properties"] = props
	}
	props[targetActionIntentParam] = map[string]interface{}{
		"type":        "string",
		"description": "必填。用一句简短中文说明本次工具调用想达到什么目的，例如：确认目标首页指纹、验证登录接口是否存在注入、向交互式会话发送退出命令。",
	}
	required := stringSliceFromAny(out["required"])
	if !containsString(required, targetActionIntentParam) {
		required = append(required, targetActionIntentParam)
	}
	out["required"] = required
	return out
}

func (fm *ToolManager) toolParametersForCall(handler ToolHandler) map[string]interface{} {
	if handler == nil {
		return map[string]interface{}{}
	}
	parameters := handler.Parameters()
	if shouldRequireTargetActionIntent(handler.Name()) {
		parameters = withTargetActionIntentParameter(parameters)
	}
	return parameters
}

func (fm *ToolManager) approveTargetActionToolCall(toolName string, args map[string]interface{}) (string, bool) {
	intent := strings.TrimSpace(stringParam(args[targetActionIntentParam]))
	if intent == "" {
		return FailWithMeta("Missing 'intent' parameter: 请用一句简短中文说明本次调用想达到什么目的。", map[string]any{
			"failure_kind": "missing_intent",
			"tool_name":    toolName,
		}), true
	}
	if !shouldRequireTargetActionApproval(toolName) {
		return "", false
	}
	if fm == nil || fm.task == nil || fm.task.GetDecisionApprovalHandler() == nil {
		return FailWithMeta("target interaction approval handler is unavailable", map[string]any{
			"failure_kind": "approval_unavailable",
			"tool_name":    toolName,
		}), true
	}
	result := fm.task.GetDecisionApprovalHandler()(taskManager.DecisionApprovalRequest{
		Kind:             taskManager.DecisionApprovalKindTargetAction,
		Title:            "目标交互工具调用审批",
		Target:           summarizeToolTarget(toolName, args),
		Action:           summarizeToolAction(toolName, args),
		Reason:           intent,
		Impact:           "该工具可能与目标系统、服务、会话或外部资源产生交互，需要用户确认后执行。",
		SaferAlternative: "优先使用只读分析、缩小范围的精确探测，或由用户补充授权与边界。",
	})
	if result.Approved || result.Status == "approved" {
		return "", false
	}
	return FailWithMeta(firstDecisionApprovalText(result.Error, result.DecisionNote, "target interaction was not approved"), map[string]any{
		"failure_kind": "decision_rejected",
		"request_id":   result.RequestID,
		"status":       result.Status,
		"kind":         result.Kind,
		"title":        result.Title,
		"tool_name":    toolName,
	}), true
}

func stripTargetActionIntent(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}
	out := make(map[string]interface{}, len(args))
	for key, value := range args {
		if key == targetActionIntentParam {
			continue
		}
		out[key] = value
	}
	return out
}

func summarizeToolTarget(toolName string, args map[string]interface{}) string {
	for _, key := range []string{"url", "target", "host", "addr", "address", "endpoint", "session_id", "uri"} {
		if value := strings.TrimSpace(stringParam(args[key])); value != "" {
			return fmt.Sprintf("%s: %s", key, truncateForDecision(value, 160))
		}
	}
	return strings.TrimSpace(toolName)
}

func summarizeToolAction(toolName string, args map[string]interface{}) string {
	keys := preferredActionKeys(toolName, args)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(stringParam(args[key]))
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, truncateForDecision(value, 260)))
	}
	if len(parts) == 0 {
		return strings.TrimSpace(toolName)
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(toolName), strings.Join(parts, "; "))
}

func preferredActionKeys(toolName string, args map[string]interface{}) []string {
	preferred := []string{"command", "cmd", "sql", "code", "input", "url", "host", "session_id", "path", "uri"}
	seen := map[string]bool{}
	keys := make([]string, 0, len(args))
	for _, key := range preferred {
		if _, ok := args[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	var rest []string
	for key := range args {
		if key == targetActionIntentParam || seen[key] {
			continue
		}
		rest = append(rest, key)
	}
	sort.Strings(rest)
	keys = append(keys, rest...)
	if len(keys) > 6 {
		keys = keys[:6]
	}
	return keys
}

func truncateForDecision(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	if max < 16 {
		return value[:max]
	}
	return value[:max] + "...[truncated]"
}

func deepCloneMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = deepCloneAny(value)
	}
	return out
}

func deepCloneAny(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return deepCloneMap(typed)
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = deepCloneAny(item)
		}
		return out
	default:
		return value
	}
}

func stringSliceFromAny(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

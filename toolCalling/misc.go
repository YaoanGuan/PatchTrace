package toolCalling

import (
	"encoding/json"
	"strings"
)

func Success(msg any) string {
	result := make(map[string]any)
	result["success"] = true
	result["result"] = msg
	result["error"] = ""
	js, _ := json.Marshal(result)
	return string(js)
}

func Fail(msg any) string {
	result := make(map[string]any)
	result["success"] = false
	result["result"] = ""
	result["error"] = msg
	js, _ := json.Marshal(result)
	return string(js)
}

func FailWithMeta(msg any, meta map[string]any) string {
	result := make(map[string]any)
	result["success"] = false
	result["result"] = ""
	result["error"] = msg
	for key, value := range meta {
		if strings.TrimSpace(key) == "" {
			continue
		}
		result[key] = value
	}
	js, _ := json.Marshal(result)
	return string(js)
}

func ToolCallFailure(kind string, toolName string, rawArgs string, msg any) string {
	meta := map[string]any{
		"failure_kind": strings.TrimSpace(kind),
	}
	if name := strings.TrimSpace(toolName); name != "" {
		meta["tool_name"] = name
	}
	if raw := strings.TrimSpace(rawArgs); raw != "" {
		meta["tool_arguments_raw"] = raw
	}
	return FailWithMeta(msg, meta)
}

func InvalidToolArgumentsFailure(toolName string, rawArgs string, err error) string {
	if err == nil {
		return ToolCallFailure("invalid_arguments", toolName, rawArgs, "invalid arguments")
	}
	return ToolCallFailure("invalid_arguments", toolName, rawArgs, "invalid arguments: "+err.Error())
}

func UnknownToolFailure(toolName string, rawArgs string) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		name = "(empty)"
	}
	return ToolCallFailure("unknown_tool", toolName, rawArgs, "unknown tool: "+name)
}

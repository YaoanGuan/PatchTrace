package toolCalling

import (
	"AIxVuln/taskManager"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const aixMCPPathPrefix = "aix-mcp://"

type mcpTaskBridgeHandler struct {
	base        ToolHandler
	rewriteArgs func(map[string]interface{}) map[string]interface{}
	rewriteResp func(string) string
}

func (h *mcpTaskBridgeHandler) Name() string {
	return h.base.Name()
}

func (h *mcpTaskBridgeHandler) Description() string {
	return h.base.Description()
}

func (h *mcpTaskBridgeHandler) Parameters() map[string]interface{} {
	return h.base.Parameters()
}

func (h *mcpTaskBridgeHandler) Execute(args map[string]interface{}) string {
	callArgs := args
	if h.rewriteArgs == nil {
		callArgs = args
	} else {
		callArgs = h.rewriteArgs(args)
	}
	result := h.base.Execute(callArgs)
	if h.rewriteResp == nil {
		return result
	}
	return h.rewriteResp(result)
}

func RegisterConfiguredMCPProfilesForAgentTypeWithTask(client *ToolManager, task *taskManager.Task, agentType string) {
	if client == nil {
		return
	}
	projectName := ""
	if task != nil {
		projectName = task.GetProjectName()
	}
	client.RegisterHandlers(wrapMCPToolHandlersForTask(task, agentType, GetMCPToolHandlersForAgentTypeWithProject(agentType, projectName))...)
}

func wrapMCPToolHandlersForTask(task *taskManager.Task, agentType string, handlers []ToolHandler) []ToolHandler {
	if len(handlers) == 0 {
		return nil
	}
	if task == nil || strings.TrimSpace(task.GetSourceCodePath()) == "" {
		return handlers
	}
	out := make([]ToolHandler, 0, len(handlers))
	for _, handler := range handlers {
		out = append(out, maybeWrapMCPToolHandlerForTask(task, agentType, handler))
	}
	return out
}

func maybeWrapMCPToolHandlerForTask(task *taskManager.Task, agentType string, handler ToolHandler) ToolHandler {
	_ = agentType
	raw, ok := handler.(*mcpToolHandler)
	if !ok || raw == nil || raw.server == nil {
		return handler
	}
	sourceRoot := strings.TrimSpace(task.GetSourceCodePath())
	if sourceRoot == "" {
		return handler
	}
	return &mcpTaskBridgeHandler{
		base: handler,
		rewriteArgs: func(args map[string]interface{}) map[string]interface{} {
			return rewriteMCPArgs(args, sourceRoot)
		},
		rewriteResp: func(result string) string {
			return rewriteMCPToolResponse(result, sourceRoot)
		},
	}
}

func rewriteMCPArgs(args map[string]interface{}, sourceRoot string) map[string]interface{} {
	if len(args) == 0 || strings.TrimSpace(sourceRoot) == "" {
		return args
	}
	out := make(map[string]interface{}, len(args))
	for k, v := range args {
		out[k] = rewriteMCPValue(k, v, sourceRoot)
	}
	return out
}

func rewriteMCPValue(parentKey string, value interface{}, sourceRoot string) interface{} {
	_ = parentKey
	switch v := value.(type) {
	case string:
		return rewriteMCPString(v, sourceRoot)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = rewriteMCPValue(parentKey, item, sourceRoot)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, item := range v {
			out[k] = rewriteMCPValue(k, item, sourceRoot)
		}
		return out
	default:
		return value
	}
}

func rewriteMCPString(raw string, sourceRoot string) string {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(sourceRoot) == "" {
		return raw
	}
	trimmed := strings.TrimSpace(raw)
	if resolved, ok := resolveAIXMCPReference(trimmed, sourceRoot); ok && trimmed == raw {
		return resolved
	}
	if !strings.Contains(raw, aixMCPPathPrefix) {
		return raw
	}
	return rewriteEmbeddedAIXMCPReferences(raw, sourceRoot)
}

func resolveAIXMCPReference(raw string, sourceRoot string) (string, bool) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, aixMCPPathPrefix) {
		return "", false
	}
	root := filepath.Clean(strings.TrimSpace(sourceRoot))
	if root == "" {
		return "", false
	}
	rel := strings.TrimPrefix(value, aixMCPPathPrefix)
	rel = strings.TrimPrefix(rel, "/")
	if decoded, err := url.PathUnescape(rel); err == nil {
		rel = decoded
	}
	if rel == "" {
		return root, true
	}
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	if !pathWithinRoot(candidate, root) {
		return "", false
	}
	return candidate, true
}

func rewriteEmbeddedAIXMCPReferences(raw string, sourceRoot string) string {
	if !strings.Contains(raw, aixMCPPathPrefix) {
		return raw
	}
	var out strings.Builder
	cursor := 0
	for cursor < len(raw) {
		relIdx := strings.Index(raw[cursor:], aixMCPPathPrefix)
		if relIdx < 0 {
			out.WriteString(raw[cursor:])
			break
		}
		idx := cursor + relIdx
		out.WriteString(raw[cursor:idx])
		quote := byte(0)
		if idx > 0 {
			prev := raw[idx-1]
			if prev == '"' || prev == '\'' {
				quote = prev
			}
		}
		end := scanExplicitPathTokenEnd(raw, idx+len(aixMCPPathPrefix), quote)
		ref := raw[idx:end]
		if resolved, ok := resolveAIXMCPReference(ref, sourceRoot); ok {
			out.WriteString(resolved)
		} else {
			out.WriteString(ref)
		}
		cursor = end
	}
	return out.String()
}

func scanExplicitPathTokenEnd(raw string, start int, quote byte) int {
	for i := start; i < len(raw); i++ {
		if quote != 0 {
			if raw[i] == quote {
				return i
			}
			continue
		}
		switch raw[i] {
		case ' ', '\t', '\n', '\r', '"', '\'', '`', ',', ';', ')', ']', '}':
			return i
		}
	}
	return len(raw)
}

func rewriteMCPToolResponse(result string, sourceRoot string) string {
	if strings.TrimSpace(result) == "" || strings.TrimSpace(sourceRoot) == "" {
		return result
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return rewriteHostPathString(result, sourceRoot)
	}
	rewritten := rewriteMCPToolResponseValue(payload, sourceRoot)
	data, err := json.Marshal(rewritten)
	if err != nil {
		return result
	}
	return string(data)
}

func rewriteMCPToolResponseValue(value interface{}, sourceRoot string) interface{} {
	switch v := value.(type) {
	case string:
		return rewriteHostPathString(v, sourceRoot)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = rewriteMCPToolResponseValue(item, sourceRoot)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, item := range v {
			out[k] = rewriteMCPToolResponseValue(item, sourceRoot)
		}
		return out
	default:
		return value
	}
}

func rewriteHostPathString(raw string, sourceRoot string) string {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(sourceRoot) == "" {
		return raw
	}
	trimmed := strings.TrimSpace(raw)
	if rewritten, ok := mapHostPathToAIXMCPReference(trimmed, sourceRoot); ok && trimmed == raw {
		return rewritten
	}
	if !containsHostPathReference(raw, sourceRoot) {
		return raw
	}
	return rewriteEmbeddedHostPathReferences(raw, sourceRoot)
}

func containsHostPathReference(raw string, sourceRoot string) bool {
	root := filepath.Clean(strings.TrimSpace(sourceRoot))
	if root == "" {
		return false
	}
	return strings.Contains(raw, root) || strings.Contains(raw, filepath.ToSlash(root))
}

func mapHostPathToAIXMCPReference(raw string, sourceRoot string) (string, bool) {
	root := filepath.Clean(strings.TrimSpace(sourceRoot))
	if root == "" {
		return "", false
	}
	if rel, ok := trimSourceRootPrefix(raw, root); ok {
		return aixMCPPathPrefix + rel, true
	}
	return "", false
}

func trimSourceRootPrefix(raw string, root string) (string, bool) {
	for _, variant := range []string{root, filepath.ToSlash(root)} {
		if variant == "" || !strings.HasPrefix(raw, variant) {
			continue
		}
		suffix := raw[len(variant):]
		if suffix == "" {
			return "", true
		}
		if suffix[0] != '/' && suffix[0] != '\\' {
			continue
		}
		return strings.TrimPrefix(filepath.ToSlash(suffix), "/"), true
	}
	return "", false
}

func rewriteEmbeddedHostPathReferences(raw string, sourceRoot string) string {
	root := filepath.Clean(strings.TrimSpace(sourceRoot))
	if root == "" {
		return raw
	}
	variants := []string{root}
	if slashRoot := filepath.ToSlash(root); slashRoot != root {
		variants = append(variants, slashRoot)
	}
	var out strings.Builder
	cursor := 0
	for cursor < len(raw) {
		idx, variant := nextHostPathReference(raw, cursor, variants)
		if idx < 0 {
			out.WriteString(raw[cursor:])
			break
		}
		out.WriteString(raw[cursor:idx])
		quote := byte(0)
		if idx > 0 {
			prev := raw[idx-1]
			if prev == '"' || prev == '\'' {
				quote = prev
			}
		}
		end := scanExplicitPathTokenEnd(raw, idx+len(variant), quote)
		ref := raw[idx:end]
		if rewritten, ok := mapHostPathToAIXMCPReference(ref, sourceRoot); ok {
			out.WriteString(rewritten)
		} else {
			out.WriteString(ref)
		}
		cursor = end
	}
	return out.String()
}

func nextHostPathReference(raw string, offset int, variants []string) (int, string) {
	bestIdx := -1
	bestVariant := ""
	for _, variant := range variants {
		idx := strings.Index(raw[offset:], variant)
		if idx < 0 {
			continue
		}
		idx += offset
		if bestIdx == -1 || idx < bestIdx {
			bestIdx = idx
			bestVariant = variant
		}
	}
	return bestIdx, bestVariant
}

func pathLooksResolvable(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return filepath.IsAbs(path)
}

func pathWithinRoot(candidate string, root string) bool {
	candidate = filepath.Clean(candidate)
	root = filepath.Clean(root)
	if candidate == root {
		return true
	}
	return strings.HasPrefix(candidate, root+string(os.PathSeparator))
}

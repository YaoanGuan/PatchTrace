package toolCalling

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"context"
	"sort"
	"strings"
	"time"
)

type ToolHandler interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(args map[string]interface{}) string
}

type ToolCallObserver func(stage string, toolCall llm.ToolCall)

type ToolManager struct {
	handlers         map[string]ToolHandler
	maxIterations    int // 每次最大调用函数数量限制
	sourceRoot       string
	toolCallObserver ToolCallObserver
	task             *taskManager.Task
}

const (
	defaultLLMReplyTokenReserve = 2048
	defaultLLMInputHardLimit    = 180000
)

func NewToolManager() *ToolManager {
	return &ToolManager{
		handlers:      make(map[string]ToolHandler),
		maxIterations: 10,
	}
}

func NewToolManagerWithMCPProfiles(profileNames ...string) *ToolManager {
	fm := NewToolManager()
	fm.RegisterMCPProfiles(profileNames...)
	return fm
}

func (fm *ToolManager) RegisterHandlers(handlers ...ToolHandler) {
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		fm.Register(handler)
	}
}

func (fm *ToolManager) RegisterMCPProfiles(profileNames ...string) {
	fm.RegisterHandlers(GetMCPToolHandlersForProfiles(profileNames...)...)
}

func (fm *ToolManager) Register(handler ToolHandler) {
	fm.handlers[handler.Name()] = handler
}

func (fm *ToolManager) SetSourceRoot(root string) {
	fm.sourceRoot = strings.TrimSpace(root)
}

func (fm *ToolManager) SetTask(task *taskManager.Task) {
	fm.task = task
}

func (fm *ToolManager) SetToolCallObserver(observer ToolCallObserver) {
	fm.toolCallObserver = observer
}

func (fm *ToolManager) notifyToolCall(stage string, toolCall llm.ToolCall) {
	if fm.toolCallObserver != nil {
		fm.toolCallObserver(stage, toolCall)
	}
}

func (fm *ToolManager) RemoveTool(name string) {
	delete(fm.handlers, name)
}

func (fm *ToolManager) GetTools() []llm.ToolDef {
	names := make([]string, 0, len(fm.handlers))
	for name := range fm.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	definitions := make([]llm.ToolDef, 0, len(names))
	for _, name := range names {
		handler := fm.handlers[name]
		parameters := fm.toolParametersForCall(handler)
		definitions = append(definitions, llm.ToolDef{
			Name:        handler.Name(),
			Description: handler.Description(),
			Parameters:  parameters,
		})
	}
	return definitions
}

// return Assistant Message、tool Result Messages、error
func (fm *ToolManager) ToolCallRequest(
	ctx context.Context,
	cli llm.Client,
	messages []llm.Message,
	model string,
	agentName string,
	projectName ...string,
) (llm.Message, []llm.Message, error) {
	return fm.ToolCallRequestWithLabel(ctx, cli, messages, model, agentName, "", projectName...)
}

// ToolCallRequestWithLabel is like ToolCallRequest but also tracks per-agent token usage via agentLabel.
func (fm *ToolManager) ToolCallRequestWithLabel(
	ctx context.Context,
	cli llm.Client,
	messages []llm.Message,
	model string,
	agentName string,
	agentLabel string,
	projectName ...string,
) (llm.Message, []llm.Message, error) {
	inputBudget := fm.estimateInputBudget(agentName)
	tools := fm.prepareToolsForRequest(fm.GetTools(), inputBudget)
	reqMessages := fm.prepareMessagesForRequest(messages, tools, inputBudget, agentName)
	opts := llm.RequestLLMOpts{AgentLabel: agentLabel}
	if len(projectName) > 0 {
		opts.ProjectName = projectName[0]
	}
	reqCtx, c := context.WithTimeout(ctx, time.Duration(600)*time.Second)
	defer c()
	resp, err := llm.RequestLLMWithOpts(cli, reqCtx, model, reqMessages, tools, opts)
	if err != nil {
		return llm.Message{}, nil, err
	}
	message := llm.ResponseToMessage(resp)
	var toolMessage []llm.Message
	for _, toolCall := range message.ToolCalls {
		fm.notifyToolCall("call", toolCall)
		handler, exists := fm.handlers[toolCall.Name]
		if !exists {
			toolMessage = append(toolMessage, llm.Message{
				Role:       llm.RoleTool,
				Content:    UnknownToolFailure(toolCall.Name, toolCall.Arguments),
				ToolCallID: toolCall.ID,
			})
			fm.notifyToolCall("result", toolCall)
			continue
		}

		callArgs, _, err := NormalizeAndValidateToolArguments(toolCall.Arguments, fm.toolParametersForCall(handler))
		if err != nil {
			toolMessage = append(toolMessage, llm.Message{
				Role:       llm.RoleTool,
				Content:    InvalidToolArgumentsFailure(toolCall.Name, toolCall.Arguments, err),
				ToolCallID: toolCall.ID,
			})
			fm.notifyToolCall("result", toolCall)
			continue
		}
		if strings.TrimSpace(fm.sourceRoot) != "" {
			rewrittenArgs, err := rewriteToolArgsWithAIXFiles(callArgs, fm.sourceRoot)
			if err != nil {
				toolMessage = append(toolMessage, llm.Message{
					Role:       llm.RoleTool,
					Content:    ToolCallFailure("argument_rewrite_failed", toolCall.Name, toolCall.Arguments, "expand <aixfile> failed: "+err.Error()),
					ToolCallID: toolCall.ID,
				})
				fm.notifyToolCall("result", toolCall)
				continue
			}
			callArgs = rewrittenArgs
		}
		if shouldRequireTargetActionIntent(handler.Name()) {
			if resultJSON, blocked := fm.approveTargetActionToolCall(handler.Name(), callArgs); blocked {
				toolMessage = append(toolMessage, llm.Message{
					Role:       llm.RoleTool,
					Content:    resultJSON,
					ToolCallID: toolCall.ID,
				})
				fm.notifyToolCall("result", toolCall)
				continue
			}
			callArgs = stripTargetActionIntent(callArgs)
		}
		resultJSON := handler.Execute(callArgs)
		toolMessage = append(toolMessage, llm.Message{
			Role:       llm.RoleTool,
			Content:    resultJSON,
			ToolCallID: toolCall.ID,
		})
		fm.notifyToolCall("result", toolCall)
	}
	return message, toolMessage, nil
}

func (fm *ToolManager) prepareToolsForRequest(tools []llm.ToolDef, inputBudget int) []llm.ToolDef {
	if len(tools) == 0 {
		return tools
	}
	core := make([]llm.ToolDef, 0, len(tools))
	mcp := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		if strings.HasPrefix(t.Name, "MCP_") {
			mcp = append(mcp, compactMCPToolDef(t))
			continue
		}
		core = append(core, t)
	}
	// Keep core tools intact. For MCP tools, dynamically cap by token budget.
	sort.SliceStable(mcp, func(i, j int) bool {
		pi := mcpToolPriority(mcp[i].Name)
		pj := mcpToolPriority(mcp[j].Name)
		if pi != pj {
			return pi < pj
		}
		return mcp[i].Name < mcp[j].Name
	})
	toolBudget := inputBudget / 2
	if toolBudget < 8192 {
		toolBudget = 8192
	}
	if toolBudget > 60000 {
		toolBudget = 60000
	}
	coreTokens := estimateToolsTokens(core)
	selectedMCP := make([]llm.ToolDef, 0, len(mcp))
	used := coreTokens
	for _, t := range mcp {
		tk := estimateToolsTokens([]llm.ToolDef{t})
		if used+tk > toolBudget && len(selectedMCP) >= 8 {
			continue
		}
		selectedMCP = append(selectedMCP, t)
		used += tk
	}
	if len(selectedMCP) == 0 && len(mcp) > 0 {
		keep := len(mcp)
		if keep > 8 {
			keep = 8
		}
		selectedMCP = append(selectedMCP, mcp[:keep]...)
	}
	out := make([]llm.ToolDef, 0, len(core)+len(selectedMCP))
	out = append(out, core...)
	out = append(out, selectedMCP...)
	return out
}

func mcpToolPriority(name string) int {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "search"), strings.Contains(n, "query"):
		return 1
	case strings.Contains(n, "repo"), strings.Contains(n, "repository"), strings.Contains(n, "list"):
		return 2
	case strings.Contains(n, "read"), strings.Contains(n, "get"), strings.Contains(n, "file"), strings.Contains(n, "code"):
		return 3
	case strings.Contains(n, "issue"), strings.Contains(n, "pull"), strings.Contains(n, "pr"), strings.Contains(n, "workflow"):
		return 4
	default:
		return 10
	}
}

func compactMCPToolDef(in llm.ToolDef) llm.ToolDef {
	in.Description = truncateString(in.Description, 180)
	if !ShouldCompactMCPTool(in.Name) {
		return in
	}
	// MCP tools often carry very verbose JSON schema/description. Keep a
	// generic object schema to reduce prompt tokens and avoid context overflow.
	properties := map[string]interface{}{}
	required := []string{}
	if shouldRequireTargetActionIntent(in.Name) {
		properties[targetActionIntentParam] = map[string]interface{}{
			"type":        "string",
			"description": "必填。用一句简短中文说明本次 MCP 工具调用想达到什么目的。",
		}
		required = append(required, targetActionIntentParam)
	}
	in.Parameters = map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": true,
	}
	return in
}

func truncateString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max < 16 {
		return s[:max]
	}
	return s[:max] + "...[truncated]"
}

func (fm *ToolManager) prepareMessagesForRequest(messages []llm.Message, tools []llm.ToolDef, inputBudget int, agentName string) []llm.Message {
	if len(messages) <= 3 {
		return messages
	}
	if inputBudget <= 0 {
		return messages
	}
	out := append([]llm.Message(nil), messages...)
	estimate := llm.CountMessagesTokens(out) + estimateToolsTokens(tools)
	for estimate > inputBudget && len(out) > 3 {
		dropEnd := 3 // at least drop oldest conversation message (index=2)
		if out[2].Role == llm.RoleAssistant && len(out[2].ToolCalls) > 0 {
			ids := make(map[string]struct{}, len(out[2].ToolCalls))
			for _, tc := range out[2].ToolCalls {
				ids[tc.ID] = struct{}{}
			}
			for dropEnd < len(out) && out[dropEnd].Role == llm.RoleTool {
				if _, ok := ids[out[dropEnd].ToolCallID]; !ok {
					break
				}
				dropEnd++
			}
		}
		out = append(out[:2], out[dropEnd:]...)
		estimate = llm.CountMessagesTokens(out) + estimateToolsTokens(tools)
	}
	if estimate > inputBudget {
		misc.Debug("%s: request still near token limit after trimming (estimate=%d, budget=%d, tools=%d)", agentName, estimate, inputBudget, len(tools))
	}
	return out
}

func estimateToolsTokens(tools []llm.ToolDef) int {
	total := 0
	for _, t := range tools {
		total += 16 // per-tool structural overhead
		total += llm.CountTokens(t.Name)
		total += llm.CountTokens(t.Description)
		total += llm.EstimateJSONTokens(t.Parameters)
	}
	return total
}

func (fm *ToolManager) estimateInputBudget(agentName string) int {
	section := sectionFromAgentName(agentName)
	budget := misc.GetMaxContext(section, "main_setting")
	if budget > defaultLLMInputHardLimit {
		budget = defaultLLMInputHardLimit
	}
	reserve := int(llm.ConfiguredMaxOutputTokens(section))
	budget -= reserve + 1024 // safety margin
	if budget < 4096 {
		budget = 4096
	}
	return budget
}

func sectionFromAgentName(agentName string) string {
	name := strings.ToLower(strings.TrimSpace(agentName))
	switch {
	case strings.Contains(name, "decision"):
		return "decision"
	case strings.Contains(name, "postexploitation"), strings.Contains(name, "post_exploitation"), strings.Contains(name, "post-exploitation"),
		strings.Contains(name, "remotecontrol"), strings.Contains(name, "remote_control"), strings.Contains(name, "remote-control"):
		return "remote_control"
	case strings.Contains(name, "srccommonagent"), strings.Contains(name, "-src"), strings.Contains(name, "_src"), strings.HasPrefix(name, "src"):
		return "src"
	case strings.Contains(name, "pentest"):
		return "pentest"
	case strings.Contains(name, "android"):
		return "android"
	case strings.Contains(name, "reverse"):
		return "reverse"
	case strings.Contains(name, "general"):
		return "general"
	case strings.Contains(name, "analyze"):
		return "analyze"
	case strings.Contains(name, "verifier"):
		return "verifier"
	case strings.Contains(name, "overview"):
		return "overview"
	case strings.Contains(name, "report"):
		return "report"
	case strings.Contains(name, "ops"):
		return "ops"
	default:
		return "main_setting"
	}
}

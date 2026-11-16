package llm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ContextManagerDebugSnapshot struct {
	MemoryType          string                   `json:"memory_type"`
	ContextID           string                   `json:"context_id,omitempty"`
	SystemPrompt        string                   `json:"system_prompt,omitempty"`
	ExtraSystemPrompt   string                   `json:"extra_system_prompt,omitempty"`
	CompressedSummary   string                   `json:"compressed_summary,omitempty"`
	KeyMessages         map[string][]interface{} `json:"key_messages,omitempty"`
	TaskList            []map[string]string      `json:"task_list,omitempty"`
	MaxHistory          int                      `json:"max_history"`
	MessageSize         int                      `json:"message_size"`
	HasPendingUser      bool                     `json:"has_pending_user_message"`
	LLMBusy             bool                     `json:"llm_busy"`
	WaitingWriters      int                      `json:"waiting_writers"`
	Turns               []Turn                   `json:"turns"`
	FlatMessages        []Message                `json:"flat_messages"`
	RenderedContext     []Message                `json:"rendered_context"`
	RenderedContextSize int                      `json:"rendered_context_tokens"`
}

type SharedContextDebugSnapshot struct {
	MemoryType         string                                 `json:"memory_type"`
	SelectedContextID  string                                 `json:"selected_context_id,omitempty"`
	AvailableContextID []string                               `json:"available_context_ids,omitempty"`
	Contexts           map[string]ContextManagerDebugSnapshot `json:"contexts"`
}

func ExportMemoryDebugSnapshot(memory Memory, contextID string) (interface{}, error) {
	if memory == nil {
		return nil, fmt.Errorf("memory is nil")
	}
	switch m := memory.(type) {
	case *ContextManager:
		return m.ExportDebugSnapshot(contextID), nil
	case *SharedContext:
		return m.ExportDebugSnapshot(contextID)
	default:
		return nil, fmt.Errorf("unsupported memory type: %T", memory)
	}
}

func (cm *ContextManager) ExportDebugSnapshot(contextID string) ContextManagerDebugSnapshot {
	cm.mu.RLock()
	turns := append([]Turn(nil), cm.memory...)
	keyMessages := cloneKeyMessages(cm.envMessage)
	taskList := cloneTaskList(cm.taskList)
	systemPrompt := cm.systemPrompt
	extraSystemPrompt := cm.extraSystemPrompt
	compressedSummary := cm.compressedSummary
	maxHistory := cm.maxHistory
	msgSize := cm.msgSize
	llmBusy := cm.llmBusy
	waitingWriters := cm.waitingWriters
	cm.mu.RUnlock()

	flatMessages := FlattenTurns(turns)
	renderedContext := cm.GetContext(contextID)
	return ContextManagerDebugSnapshot{
		MemoryType:          "ContextManager",
		ContextID:           contextID,
		SystemPrompt:        systemPrompt,
		ExtraSystemPrompt:   extraSystemPrompt,
		CompressedSummary:   compressedSummary,
		KeyMessages:         keyMessages,
		TaskList:            taskList,
		MaxHistory:          maxHistory,
		MessageSize:         msgSize,
		HasPendingUser:      cm.HasPendingUserMessage(),
		LLMBusy:             llmBusy,
		WaitingWriters:      waitingWriters,
		Turns:               turns,
		FlatMessages:        flatMessages,
		RenderedContext:     renderedContext,
		RenderedContextSize: CountMessagesTokens(renderedContext),
	}
}

func (cm *SharedContext) ExportDebugSnapshot(contextID string) (SharedContextDebugSnapshot, error) {
	cm.mu.Lock()
	ids := make([]string, 0, len(cm.Contexts))
	contexts := make(map[string]*ContextManager, len(cm.Contexts))
	for id, mgr := range cm.Contexts {
		ids = append(ids, id)
		contexts[id] = mgr
	}
	cm.mu.Unlock()

	if contextID == "" && len(ids) == 1 {
		contextID = ids[0]
	}
	out := SharedContextDebugSnapshot{
		MemoryType:         "SharedContext",
		SelectedContextID:  contextID,
		AvailableContextID: ids,
		Contexts:           make(map[string]ContextManagerDebugSnapshot, len(ids)),
	}
	if contextID != "" {
		mgr, ok := contexts[contextID]
		if !ok {
			return SharedContextDebugSnapshot{}, fmt.Errorf("context %q not found in SharedContext", contextID)
		}
		out.Contexts[contextID] = mgr.ExportDebugSnapshot(contextID)
		return out, nil
	}
	for id, mgr := range contexts {
		out.Contexts[id] = mgr.ExportDebugSnapshot(id)
	}
	return out, nil
}

func cloneKeyMessages(in map[string][]interface{}) map[string][]interface{} {
	if len(in) == 0 {
		return map[string][]interface{}{}
	}
	out := make(map[string][]interface{}, len(in))
	for key, values := range in {
		cp := make([]interface{}, len(values))
		copy(cp, values)
		out[key] = cp
	}
	return out
}

func cloneTaskList(in []map[string]string) []map[string]string {
	if len(in) == 0 {
		return []map[string]string{}
	}
	out := make([]map[string]string, 0, len(in))
	for _, item := range in {
		cp := make(map[string]string, len(item))
		for k, v := range item {
			cp[k] = v
		}
		out = append(out, cp)
	}
	return out
}

func DecodeJSONStringOrRaw(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var parsed interface{}
	if json.Unmarshal([]byte(raw), &parsed) == nil {
		return parsed
	}
	return raw
}

func CaptureMemoryDebugContext(memory Memory, contextID string, messages []Message) {
	if memory == nil {
		return
	}
	switch m := memory.(type) {
	case *ContextManager:
		m.captureLastLLMContext(contextID, messages)
	case *SharedContext:
		m.captureLastLLMContext(contextID, messages)
	}
}

func ExportMemoryDebugText(memory Memory, contextID string) (string, error) {
	if memory == nil {
		return "", fmt.Errorf("memory is nil")
	}
	switch m := memory.(type) {
	case *ContextManager:
		return m.exportLastLLMContextText(contextID), nil
	case *SharedContext:
		return m.exportLastLLMContextText(contextID)
	default:
		return "", fmt.Errorf("unsupported memory type: %T", memory)
	}
}

func (cm *ContextManager) captureLastLLMContext(contextID string, messages []Message) {
	if cm == nil {
		return
	}
	contextID = strings.TrimSpace(contextID)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.lastLLMContext == nil {
		cm.lastLLMContext = make(map[string][]Message)
	}
	cm.lastLLMContext[contextID] = cloneMessages(messages)
}

func (cm *ContextManager) exportLastLLMContextText(contextID string) string {
	if cm == nil {
		return ""
	}
	contextID = strings.TrimSpace(contextID)
	cm.mu.RLock()
	var captured []Message
	if cm.lastLLMContext != nil {
		captured = cloneMessages(cm.lastLLMContext[contextID])
		if len(captured) == 0 && contextID == "" {
			for _, one := range cm.lastLLMContext {
				if len(one) > 0 {
					captured = cloneMessages(one)
					break
				}
			}
		}
	}
	cm.mu.RUnlock()
	if len(captured) == 0 {
		captured = cm.GetContext(contextID)
	}
	return RenderMessagesForDebugText(captured)
}

func (cm *SharedContext) captureLastLLMContext(contextID string, messages []Message) {
	if cm == nil {
		return
	}
	contextID = strings.TrimSpace(contextID)
	cm.mu.Lock()
	target := cm.Contexts[contextID]
	cm.mu.Unlock()
	if target == nil {
		return
	}
	target.captureLastLLMContext(contextID, messages)
}

func (cm *SharedContext) exportLastLLMContextText(contextID string) (string, error) {
	if cm == nil {
		return "", fmt.Errorf("shared context is nil")
	}
	contextID = strings.TrimSpace(contextID)
	cm.mu.Lock()
	target := cm.Contexts[contextID]
	if target == nil && contextID == "" && len(cm.Contexts) == 1 {
		for id, mgr := range cm.Contexts {
			contextID = id
			target = mgr
			break
		}
	}
	cm.mu.Unlock()
	if target == nil {
		return "", fmt.Errorf("context %q not found in SharedContext", contextID)
	}
	return target.exportLastLLMContextText(contextID), nil
}

func RenderMessagesForDebugText(messages []Message) string {
	messages = cloneMessages(messages)
	var builder strings.Builder
	builder.WriteString("# message_count: ")
	builder.WriteString(strconv.Itoa(len(messages)))
	builder.WriteString("\n# approx_tokens: ")
	builder.WriteString(strconv.Itoa(CountMessagesTokens(messages)))
	builder.WriteString("\n")
	for i, message := range messages {
		builder.WriteString("\n===== MESSAGE ")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(" =====\n")
		builder.WriteString("role: ")
		builder.WriteString(strings.TrimSpace(message.Role))
		builder.WriteString("\n")
		if strings.TrimSpace(message.ToolCallID) != "" {
			builder.WriteString("tool_call_id: ")
			builder.WriteString(strings.TrimSpace(message.ToolCallID))
			builder.WriteString("\n")
		}
		if len(message.ToolCalls) > 0 {
			builder.WriteString("tool_calls:\n")
			for idx, call := range message.ToolCalls {
				builder.WriteString("  - index: ")
				builder.WriteString(strconv.Itoa(idx + 1))
				builder.WriteString("\n")
				builder.WriteString("    id: ")
				builder.WriteString(strings.TrimSpace(call.ID))
				builder.WriteString("\n")
				builder.WriteString("    name: ")
				builder.WriteString(strings.TrimSpace(call.Name))
				builder.WriteString("\n")
				builder.WriteString("    arguments:\n")
				builder.WriteString(indentDebugText(strings.TrimSpace(call.Arguments), "      "))
				builder.WriteString("\n")
			}
		}
		builder.WriteString("content:\n")
		builder.WriteString(indentDebugText(message.Content, "  "))
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n") + "\n"
}

func indentDebugText(raw string, prefix string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	if raw == "" {
		return prefix + "(empty)"
	}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func cloneMessages(in []Message) []Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]Message, len(in))
	copy(out, in)
	for i := range out {
		if len(out[i].ToolCalls) == 0 {
			continue
		}
		calls := make([]ToolCall, len(out[i].ToolCalls))
		copy(calls, out[i].ToolCalls)
		out[i].ToolCalls = calls
	}
	return out
}

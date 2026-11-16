package DecisionBrain

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var systemPrompt = prompts.SharedDecisionBrainRuntimeSystemPrompt()

// ContextBreakdownSection represents the token size of one section in the brain context.
type ContextBreakdownSection struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// ContextBreakdown holds the per-section token breakdown of the brain context.
type ContextBreakdown struct {
	Sections   []ContextBreakdownSection `json:"sections"`
	Total      int                       `json:"total"`
	MaxContext int                       `json:"max_context"`
}

type BrainMemory struct {
	systemPrompt            string
	pinnedMessages          []llm.Message // 永久保留的固定记忆，不参与常规压缩与裁剪
	memory                  []llm.Turn
	mu                      sync.RWMutex
	keyMessage              map[string][]interface{} // 重要的信息，这个信息永远不会被覆盖
	maxHistory              int                      // 历史对话记录最大token数
	compressedSummary       string                   // 压缩后的历史摘要（用于保留关键信息）
	taskContent             string
	eventHandler            func(string, string, int)
	agentRuntimeInfoJson    string
	envInfoJson             string
	exploitIdeaListJson     string
	exploitChainListJson    string
	containerInfoList       string
	noAgentRunning          bool
	userPromptHash          string
	msgSize                 int // 消息大小
	lastUserPrompt          string
	lastLLMContext          []llm.Message
	lastPanelSectionHash    map[string]string
	lastPanelSectionContent map[string]string
	seenAgentRuntime        map[string]bool
	contextBreakdown        ContextBreakdown
	enableExploitFlow       bool
}

type brainMemoryDiskState struct {
	SystemPrompt            string                   `json:"system_prompt"`
	PinnedMessages          []llm.Message            `json:"pinned_messages,omitempty"`
	Memory                  []llm.Turn               `json:"memory,omitempty"`
	KeyMessage              map[string][]interface{} `json:"key_message,omitempty"`
	MaxHistory              *int                     `json:"max_history,omitempty"`
	CompressedSummary       string                   `json:"compressed_summary,omitempty"`
	TaskContent             string                   `json:"task_content,omitempty"`
	AgentRuntimeInfoJSON    string                   `json:"agent_runtime_info_json,omitempty"`
	EnvInfoJSON             string                   `json:"env_info_json,omitempty"`
	ExploitIdeaListJSON     string                   `json:"exploit_idea_list_json,omitempty"`
	ExploitChainListJSON    string                   `json:"exploit_chain_list_json,omitempty"`
	ContainerInfoList       string                   `json:"container_info_list,omitempty"`
	NoAgentRunning          bool                     `json:"no_agent_running"`
	UserPromptHash          string                   `json:"user_prompt_hash,omitempty"`
	MsgSize                 int                      `json:"msg_size"`
	LastUserPrompt          string                   `json:"last_user_prompt,omitempty"`
	LastPanelSectionHash    map[string]string        `json:"last_panel_section_hash,omitempty"`
	LastPanelSectionContent map[string]string        `json:"last_panel_section_content,omitempty"`
	SeenAgentRuntime        map[string]bool          `json:"seen_agent_runtime,omitempty"`
	ContextBreakdown        ContextBreakdown         `json:"context_breakdown"`
	EnableExploitFlow       *bool                    `json:"enable_exploit_flow,omitempty"`
}

func NewBrainMemory() *BrainMemory {
	return &BrainMemory{
		pinnedMessages:          make([]llm.Message, 0),
		memory:                  make([]llm.Turn, 0),
		lastLLMContext:          make([]llm.Message, 0),
		maxHistory:              misc.GetMaxContext("decision"),
		systemPrompt:            systemPrompt,
		keyMessage:              make(map[string][]interface{}),
		lastPanelSectionHash:    map[string]string{},
		lastPanelSectionContent: map[string]string{},
		seenAgentRuntime:        map[string]bool{},
		enableExploitFlow:       true,
	}
}
func (br *BrainMemory) SaveMemoryToFile(filename string) error {
	br.mu.RLock()
	memoryInfoJson, err := json.Marshal(br.diskStateLocked())
	br.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(filename, memoryInfoJson, 0644)
}

func (br *BrainMemory) LoadMemoryFromFile(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var saved brainMemoryDiskState
	if err := json.Unmarshal(content, &saved); err != nil {
		return err
	}
	br.mu.Lock()
	defer br.mu.Unlock()
	if saved.SystemPrompt != "" {
		br.systemPrompt = saved.SystemPrompt
	} else if br.systemPrompt == "" {
		br.systemPrompt = systemPrompt
	}
	br.pinnedMessages = append([]llm.Message(nil), saved.PinnedMessages...)
	br.memory = saved.Memory
	if saved.KeyMessage != nil {
		br.keyMessage = saved.KeyMessage
	} else if br.keyMessage == nil {
		br.keyMessage = make(map[string][]interface{})
	}
	if saved.MaxHistory != nil && *saved.MaxHistory > 0 {
		br.maxHistory = *saved.MaxHistory
	} else if br.maxHistory <= 0 {
		br.maxHistory = misc.GetMaxContext("decision")
	}
	br.compressedSummary = saved.CompressedSummary
	br.taskContent = saved.TaskContent
	br.agentRuntimeInfoJson = saved.AgentRuntimeInfoJSON
	br.envInfoJson = saved.EnvInfoJSON
	br.exploitIdeaListJson = saved.ExploitIdeaListJSON
	br.exploitChainListJson = saved.ExploitChainListJSON
	br.containerInfoList = saved.ContainerInfoList
	br.noAgentRunning = saved.NoAgentRunning
	br.userPromptHash = saved.UserPromptHash
	br.msgSize = saved.MsgSize
	br.lastUserPrompt = saved.LastUserPrompt
	if saved.LastPanelSectionHash != nil {
		br.lastPanelSectionHash = saved.LastPanelSectionHash
	} else if br.lastPanelSectionHash == nil {
		br.lastPanelSectionHash = map[string]string{}
	}
	if saved.LastPanelSectionContent != nil {
		br.lastPanelSectionContent = saved.LastPanelSectionContent
	} else if br.lastPanelSectionContent == nil {
		br.lastPanelSectionContent = map[string]string{}
	}
	if saved.SeenAgentRuntime != nil {
		br.seenAgentRuntime = saved.SeenAgentRuntime
	} else if br.seenAgentRuntime == nil {
		br.seenAgentRuntime = map[string]bool{}
	}
	br.contextBreakdown = saved.ContextBreakdown
	if saved.EnableExploitFlow != nil {
		br.enableExploitFlow = *saved.EnableExploitFlow
	}
	return nil
}

func (br *BrainMemory) SetEventHandler(f func(string, string, int)) {
	br.eventHandler = f
}

func (br *BrainMemory) SetNoAgentRunning(v bool) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.noAgentRunning = v
}

func (br *BrainMemory) SetExploitWorkflowEnabled(enabled bool) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.enableExploitFlow = enabled
}

func (br *BrainMemory) GetLastHash() string {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return br.userPromptHash
}
func (br *BrainMemory) UpdateAgentRuntimeInfo(rJson string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.agentRuntimeInfoJson = rJson
}

func (br *BrainMemory) SetTaskContent(taskContent string) {
	br.taskContent = taskContent
}

func (br *BrainMemory) GetTaskContent() string {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return br.taskContent
}

func (br *BrainMemory) AddMessage(x llm.Message) {
	if len(x.Content) > misc.GetMessageMaximum() {
		x.Content = x.Content[:misc.GetMessageMaximum()] + " ...... (The text exceeds the maximum length of " + strconv.Itoa(misc.GetMessageMaximum()) + " characters and cannot be sent to the LLM—)."
	}
	br.mu.Lock()
	defer br.mu.Unlock()
	// Tool messages are appended to the last turn (which should be an assistant
	// turn with tool_calls). All other messages start a new turn.
	if x.Role == llm.RoleTool && len(br.memory) > 0 {
		last := &br.memory[len(br.memory)-1]
		if last.Role() == llm.RoleAssistant && last.HasToolCalls() {
			last.Messages = append(last.Messages, x)
		} else {
			br.memory = append(br.memory, llm.Turn{Messages: []llm.Message{x}})
		}
	} else {
		br.memory = append(br.memory, llm.Turn{Messages: []llm.Message{x}})
	}
	// Invalidate hash so Start() loop detects the new message and doesn't skip it.
	if x.Role == llm.RoleUser {
		br.userPromptHash = ""
	}
}

func (br *BrainMemory) EnsurePinnedMessages(messages []llm.Message) bool {
	if len(messages) == 0 {
		return false
	}
	br.mu.Lock()
	defer br.mu.Unlock()
	if len(br.pinnedMessages) == 0 {
		br.pinnedMessages = make([]llm.Message, 0, len(messages))
	}
	if containsMessageSequence(br.pinnedMessages, messages) {
		return false
	}
	flat := llm.FlattenTurns(br.memory)
	if filtered, removed := stripMessageSequence(flat, messages); removed {
		br.memory = llm.BuildTurns(filtered)
	}
	br.pinnedMessages = append(br.pinnedMessages, cloneMessages(messages)...)
	br.userPromptHash = ""
	return true
}

const brainMemorySummaryPrefix = "[Memory Summary]"

func (br *BrainMemory) shouldCompressLocked() bool {
	if br.maxHistory <= 0 {
		return false
	}
	return llm.TurnsSize(br.memory) > br.maxHistory
}

// snapshotCompressionCandidateLocked splits memory into old (to be summarized)
// and recent (to keep). Recent turns are selected by size budget: we keep as
// many recent turns as fit within half of maxHistory, ensuring the kept portion
// won't immediately trigger another compression.
func (br *BrainMemory) snapshotCompressionCandidateLocked() (string, []llm.Message, []llm.Turn, bool) {
	if len(br.memory) <= 2 {
		return "", nil, nil, false
	}
	budget := br.maxHistory / 2
	if budget < 4096 {
		budget = 4096
	}
	// Walk backwards to find how many recent turns fit in the budget.
	kept := 0
	size := 0
	for i := len(br.memory) - 1; i >= 0; i-- {
		s := br.memory[i].Size()
		if size+s > budget && kept >= 4 {
			break
		}
		size += s
		kept++
	}
	if kept >= len(br.memory) {
		return "", nil, nil, false
	}
	splitIdx := len(br.memory) - kept
	oldFlat := llm.FlattenTurns(br.memory[:splitIdx])
	recentTurns := make([]llm.Turn, kept)
	copy(recentTurns, br.memory[splitIdx:])
	return br.compressedSummary, oldFlat, recentTurns, true
}

func (br *BrainMemory) CompressIfNeeded(cli llm.Client, model string) error {
	if cli == nil {
		return errors.New("nil openai client")
	}

	br.mu.Lock()
	need := br.shouldCompressLocked()
	br.mu.Unlock()
	if !need {
		return nil
	}

	var existingSummary string
	var oldMessages []llm.Message
	var recentTurns []llm.Turn
	var ok bool
	br.mu.Lock()
	existingSummary, oldMessages, recentTurns, ok = br.snapshotCompressionCandidateLocked()
	br.mu.Unlock()
	if !ok {
		return nil
	}

	newSummary, err := br.summarizeWithLLM(cli, model, existingSummary, oldMessages)
	if err != nil {
		// Fallback: LLM compression failed. Force-drop old turns to prevent
		// unbounded memory growth. Keep only the recent turns.
		misc.Debug("BrainMemory.CompressIfNeeded: LLM summarization failed (%s), falling back to hard truncation", err.Error())
		br.mu.Lock()
		defer br.mu.Unlock()
		_, _, recentFallback, okFallback := br.snapshotCompressionCandidateLocked()
		if okFallback {
			fallbackMsg := brainMemorySummaryPrefix + "\n(Automatic compression failed. Older conversation history has been discarded to stay within context limits.)"
			if br.compressedSummary != "" {
				fallbackMsg += "\nPrevious summary:\n" + br.compressedSummary
			}
			summaryTurn := llm.Turn{Messages: []llm.Message{{Role: llm.RoleUser, Content: fallbackMsg}}}
			br.memory = append([]llm.Turn{summaryTurn}, recentFallback...)
		}
		return err
	}

	br.mu.Lock()
	defer br.mu.Unlock()
	br.compressedSummary = newSummary
	summaryTurn := llm.Turn{Messages: []llm.Message{br.buildSummaryMessageLocked()}}
	br.memory = append([]llm.Turn{summaryTurn}, recentTurns...)
	return nil
}

func (br *BrainMemory) buildSummaryMessageLocked() llm.Message {
	content := brainMemorySummaryPrefix + "\n(This summary was automatically generated by the system to compress earlier conversation history. It is NOT a message you wrote — treat it as reference context.)"
	if br.compressedSummary != "" {
		content = content + "\n" + br.compressedSummary
	}
	return llm.Message{Role: llm.RoleUser, Content: content}
}

func (br *BrainMemory) isCompressedLocked() bool {
	if br.compressedSummary != "" {
		return true
	}
	if len(br.memory) > 0 && len(br.memory[0].Messages) > 0 && strings.HasPrefix(br.memory[0].Messages[0].Content, brainMemorySummaryPrefix) {
		return true
	}
	return false
}

func (br *BrainMemory) IsCompressed() bool {
	br.mu.Lock()
	defer br.mu.Unlock()
	return br.isCompressedLocked()
}

func (br *BrainMemory) summarizeWithLLM(cli llm.Client, model string, existingSummary string, msgs []llm.Message) (string, error) {
	// Build a budget-aware representation of messages to summarize.
	// We allow up to ~60KB of message content for the summarization prompt.
	const maxPromptBytes = 60000
	const maxPerAssistant = 2000
	const maxPerTool = 400
	const maxPerUser = 1500

	trimmed := make([]map[string]string, 0, len(msgs))
	totalBytes := 0
	for _, m := range msgs {
		c := m.Content
		limit := maxPerUser
		switch m.Role {
		case llm.RoleAssistant:
			limit = maxPerAssistant
		case llm.RoleTool:
			limit = maxPerTool
		}
		if len(c) > limit {
			c = c[:limit] + " ...[truncated]"
		}
		if totalBytes+len(c) > maxPromptBytes {
			// Budget exhausted — skip remaining older messages.
			break
		}
		totalBytes += len(c)
		trimmed = append(trimmed, map[string]string{"role": m.Role, "content": c})
	}
	js, _ := json.Marshal(trimmed)

	sys := prompts.MemoryCompressionSystemPrompt()

	user := "Existing summary (may be empty):\n" + existingSummary + "\n\n" +
		"New messages to merge into summary (JSON array of {role,content}):\n" + string(js)

	ms := []llm.Message{
		{Role: llm.RoleSystem, Content: sys},
		{Role: llm.RoleUser, Content: user},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resp, err := llm.RequestLLM(cli, ctx, model, ms, nil)
	if err != nil {
		return "", err
	}
	if resp.Content == "" {
		return "", errors.New("empty summarization response")
	}
	return resp.Content, nil
}

func (br *BrainMemory) GetContentSize(start int) int {
	return llm.TurnsSize(br.memory[start:])
}

func (br *BrainMemory) AddKeyMessage(key string, value interface{}, isAppend bool) {
	br.mu.Lock()
	defer br.mu.Unlock()
	if isAppend {
		existing, exists := br.keyMessage[key]
		if !exists {
			br.keyMessage[key] = []interface{}{value}
		}
		br.keyMessage[key] = append(existing, value)
	} else {
		br.keyMessage[key] = []interface{}{value}
	}
}

func (br *BrainMemory) DeleteKeyMessage(key string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	delete(br.keyMessage, key)
}

func (br *BrainMemory) SetSystemPrompt(x string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.systemPrompt = x
}

func (br *BrainMemory) SetEnvInfo(xJson string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.envInfoJson = xJson
}
func (br *BrainMemory) SetExploitIdeaList(xJson string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.exploitIdeaListJson = xJson
}
func (br *BrainMemory) SetExploitChainList(xJson string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.exploitChainListJson = xJson
}

func (br *BrainMemory) SetContainerListInfo(xJson string) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.containerInfoList = xJson
}

func brainPanelSectionOrder(enableExploitFlow bool) []string {
	if enableExploitFlow {
		return []string{"agent_runtime", "exploit_idea_list", "exploit_chain_list", "env", "containers", "key_info"}
	}
	return []string{"agent_runtime", "env", "containers", "key_info"}
}

func (br *BrainMemory) buildExploitIdeaListLocked() string {
	exploitIdeaList := "ExploitIdea not generated. No analysis-type Agent has been launched or the Agent has not produced any results yet."
	if br.exploitIdeaListJson == "" {
		return exploitIdeaList
	}
	type exploitIdeaBrief struct {
		ExploitIdeaId string `json:"exploitIdeaId"`
		Title         string `json:"title,omitempty"`
		State         string `json:"state"`
		Harm          string `json:"harm"`
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(br.exploitIdeaListJson), &raw); err == nil {
		briefs := make([]exploitIdeaBrief, 0, len(raw))
		for _, item := range raw {
			b := exploitIdeaBrief{}
			if v, ok := item["exploitIdeaId"].(string); ok {
				b.ExploitIdeaId = v
			}
			if v, ok := item["state"].(string); ok {
				b.State = v
			}
			if v, ok := item["harm"].(string); ok {
				b.Harm = v
			}
			if ep, ok := item["exploit_point"].(map[string]interface{}); ok {
				if t, ok := ep["title"].(string); ok {
					b.Title = t
				}
			}
			briefs = append(briefs, b)
		}
		brief, _ := json.Marshal(briefs)
		return string(brief) + "\n[Notice] Use Tool-GetExploitIdeaByIdTool with exploitIdeaId to view full details."
	}
	return br.exploitIdeaListJson
}

func (br *BrainMemory) buildExploitChainListLocked() string {
	ecinfo := "There is currently no attack chain information."
	if br.exploitChainListJson == "" {
		return ecinfo
	}
	var chains []map[string]interface{}
	if err := json.Unmarshal([]byte(br.exploitChainListJson), &chains); err == nil {
		type chainBrief struct {
			Idea     string   `json:"idea"`
			State    string   `json:"state"`
			IdeaIDs  []string `json:"exploit_idea_ids"`
			Evidence string   `json:"evidence,omitempty"`
		}
		briefs := make([]chainBrief, 0, len(chains))
		for _, ch := range chains {
			cb := chainBrief{}
			if v, ok := ch["idea"].(string); ok {
				cb.Idea = v
			}
			if v, ok := ch["state"].(string); ok {
				cb.State = v
			}
			if v, ok := ch["evidence"].(map[string]interface{}); ok {
				if e, ok := v["Evidence"].(string); ok && e != "" {
					if len(e) > 200 {
						e = e[:200] + "...[truncated]"
					}
					cb.Evidence = e
				}
			}
			if ideas, ok := ch["exploit_idea"].([]interface{}); ok {
				for _, idea := range ideas {
					if m, ok := idea.(map[string]interface{}); ok {
						if id, ok := m["exploitIdeaId"].(string); ok {
							cb.IdeaIDs = append(cb.IdeaIDs, id)
						}
					}
				}
			}
			briefs = append(briefs, cb)
		}
		if js, err := json.Marshal(briefs); err == nil {
			return string(js)
		}
	}
	return br.exploitChainListJson
}

func (br *BrainMemory) GetMsgSize() int {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return br.msgSize
}

func (br *BrainMemory) GetContext() []llm.Message {
	br.mu.Lock()
	defer br.mu.Unlock()
	if len(br.systemPrompt) == 0 {
		br.systemPrompt = systemPrompt
	}
	pinned := append([]llm.Message(nil), br.pinnedMessages...)
	flat := llm.FlattenTurns(br.memory)
	messages := make([]llm.Message, 0, len(pinned)+len(flat)+3)
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: br.systemPrompt + "\n\nCurrent time: " + time.Now().Format("2006-01-02 15:04:05"),
	})
	userPrompt := ""
	if br.taskContent != "" {
		// agent_runtime: truncate RUNTask/RUNSummary for agents that have already been shown to LLM.
		agentRuntime := br.agentRuntimeInfoJson
		activeIDs := make([]string, 0)
		doneIDs := make([]string, 0)
		if agentRuntime != "" {
			if br.seenAgentRuntime == nil {
				br.seenAgentRuntime = map[string]bool{}
			}
			var runs []map[string]interface{}
			if err := json.Unmarshal([]byte(agentRuntime), &runs); err == nil {
				for _, one := range runs {
					id, _ := one["AgentID"].(string)
					st := ""
					if s, ok := one["RunState"].(string); ok {
						st = s
					}
					stLower := strings.ToLower(strings.TrimSpace(st))
					if id != "" {
						if stLower == "done" || stLower == "completed" || stLower == "success" {
							doneIDs = append(doneIDs, id)
						} else {
							activeIDs = append(activeIDs, id)
						}
					}
				}

				truncateRuntimeField := func(s string) string {
					b := []byte(s)
					if len(b) > 1200 {
						return string(b[:1200]) + " ...[truncated]"
					}
					return s
				}
				for _, one := range runs {
					id, _ := one["AgentID"].(string)
					if id == "" {
						continue
					}
					if br.seenAgentRuntime[id] {
						if v, ok := one["RUNTask"].(string); ok {
							one["RUNTask"] = truncateRuntimeField(v)
						}
						if v, ok := one["RUNSummary"].(string); ok {
							one["RUNSummary"] = truncateRuntimeField(v)
						}
					}
					// Mark as seen after first inclusion in the panel context.
					br.seenAgentRuntime[id] = true
				}
				if js, err := json.Marshal(runs); err == nil {
					agentRuntime = string(js)
				}
			}
		}

		js1, _ := json.Marshal(br.keyMessage)
		env := "Environment information not generated, Ops-type Agent not running or task not yet completed."
		if br.envInfoJson != "" {
			env = br.envInfoJson
		}
		enableExploitFlow := br.enableExploitFlow
		exploitIdeaList := ""
		if enableExploitFlow {
			exploitIdeaList = br.buildExploitIdeaListLocked()
		}
		cinfo := "No containers are currently running."
		if br.containerInfoList != "" {
			cinfo = br.containerInfoList
		}
		ecinfo := ""
		if enableExploitFlow {
			ecinfo = br.buildExploitChainListLocked()
		}

		envReady := br.envInfoJson != ""
		envWarning := ""
		if envReady {
			envWarning = "\n[WARNING] ENV_READY=true — The test environment has ALREADY been set up successfully. Do NOT schedule any Ops agent to build the environment again. See the 'env' section below for details.\n"
		}
		agentSummary := fmt.Sprintf(
			"agent_runtime_summary:\nNO_AGENT_RUNNING_FLAG: %t\nACTIVE_AGENT_COUNT: %d\nACTIVE_AGENT_IDS: %s\nDONE_AGENT_COUNT: %d\nDONE_AGENT_IDS: %s\nENV_READY: %t%s\n[IMPORTANT] Do NOT infer agent status by skimming agent_runtime JSON. Always trust agent_runtime_summary above.\n",
			br.noAgentRunning,
			len(activeIDs),
			strings.Join(activeIDs, ","),
			len(doneIDs),
			strings.Join(doneIDs, ","),
			envReady,
			envWarning,
		)

		sectionOrder := brainPanelSectionOrder(enableExploitFlow)
		currentSectionHash := map[string]string{
			"agent_runtime": fmt.Sprintf("%x", md5.Sum([]byte(agentRuntime))),
			"env":           fmt.Sprintf("%x", md5.Sum([]byte(env))),
			"containers":    fmt.Sprintf("%x", md5.Sum([]byte(cinfo))),
			"key_info":      fmt.Sprintf("%x", md5.Sum([]byte(string(js1)))),
		}
		currentSectionContent := map[string]string{
			"agent_runtime": agentRuntime,
			"env":           env,
			"containers":    cinfo,
			"key_info":      string(js1),
		}

		// Build the content-only prompt for hash (excludes current_time so time changes don't trigger unnecessary wakeups).
		contentPrompt := fmt.Sprintf("Overall Goal:\n%s\n\n%s\nagent_runtime:\n%s\n\nenv:\n%s\n\ncontainers:\n%s\n\nkey_info:\n%s\n", br.taskContent, agentSummary, agentRuntime, env, cinfo, string(js1))
		if enableExploitFlow {
			currentSectionHash["exploit_idea_list"] = fmt.Sprintf("%x", md5.Sum([]byte(exploitIdeaList)))
			currentSectionHash["exploit_chain_list"] = fmt.Sprintf("%x", md5.Sum([]byte(ecinfo)))
			currentSectionContent["exploit_idea_list"] = exploitIdeaList
			currentSectionContent["exploit_chain_list"] = ecinfo
			contentPrompt = fmt.Sprintf("Overall Goal:\n%s\n\n%s\nagent_runtime:\n%s\n\nenv:\n%s\n\nexploit_idea_list:\n%s\n\ncontainers:\n%s\n\nexploit_chain_list:\n%s\n\nkey_info:\n%s\n", br.taskContent, agentSummary, agentRuntime, env, exploitIdeaList, cinfo, ecinfo, string(js1))
		}
		if br.noAgentRunning {
			if enableExploitFlow {
				contentPrompt = contentPrompt + "\n[ALL AGENTS IDLE] No digital human is currently running. Review the exploit_idea_list and exploit_chain_list above. Ask yourself:\n1. Have I covered enough vulnerability categories for this project's tech stack?\n2. Are there code modules or attack surfaces I haven't explored yet?\n3. Are there exploitIdeas that need verification or combination into chains?\nIf the answer to any of these is YES, schedule more agents NOW. If the current round is meaningfully complete, call Tool-FinishTask with a concise Chinese summary so it can be published to the decision-makers channel. If you do not initiate any tool calls, the system will consider the task stalled."
			} else {
				contentPrompt = contentPrompt + "\n[ALL AGENTS IDLE] No digital human is currently running. Review the current task, the agent_runtime summary, env, containers, and key_info above. If more work is needed, dispatch the appropriate group member now. If another group is required, request assistance. If the current round is meaningfully complete or blocked, call Tool-FinishTask with a concise Chinese summary so it can be published to the decision-makers channel. If you do not initiate any tool calls, the system will consider the task stalled."
			}
		}
		// Include memory length in hash so new chat messages always change the hash.
		hashInput := fmt.Sprintf("pinned_len:%d\nmem_len:%d\n%s", len(br.pinnedMessages), len(br.memory), contentPrompt)
		hash := fmt.Sprintf("%x", md5.Sum([]byte(hashInput)))
		// Prepend current_time to the actual prompt sent to the brain.
		userPrompt = fmt.Sprintf("current_time: %s\n\n%s", time.Now().Format("2006-01-02 15:04:05"), contentPrompt)

		if br.userPromptHash != hash {
			// Build precise changed_sections by comparing per-section hashes.
			if br.lastPanelSectionHash == nil {
				br.lastPanelSectionHash = map[string]string{}
			}
			if br.lastPanelSectionContent == nil {
				br.lastPanelSectionContent = map[string]string{}
			}
			changed := make([]string, 0, len(sectionOrder))
			for _, k := range sectionOrder {
				prev, ok := br.lastPanelSectionHash[k]
				if !ok || prev != currentSectionHash[k] {
					changed = append(changed, k)
				}
			}
			changedJSON, _ := json.Marshal(changed)
			delta := fmt.Sprintf("[StatusPanel Delta]\nchanged_sections=%s\n", string(changedJSON))

			br.lastUserPrompt = userPrompt
			br.lastPanelSectionHash = currentSectionHash
			br.lastPanelSectionContent = currentSectionContent
			userPrompt = delta + "\n" + userPrompt
			//misc.Debug("状态面板变动：\n%s", userPrompt)
			br.userPromptHash = hash
		}
	} else {
		return nil
	}
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: userPrompt,
	})
	messages = append(messages, pinned...)
	messages = append(messages, flat...)
	messages = llm.SanitizeToolCallMessages(messages)

	// Hard limit: if total tokens exceed maxHistory, drop oldest conversation
	// messages after the pinned prefix until under limit.
	if br.maxHistory > 0 {
		historyStart := 2 + len(pinned)
		for llm.CountMessagesTokens(messages) > br.maxHistory && len(messages) > historyStart+1 {
			dropEnd := historyStart + 1
			if messages[historyStart].Role == llm.RoleAssistant && len(messages[historyStart].ToolCalls) > 0 {
				ids := make(map[string]bool)
				for _, tc := range messages[historyStart].ToolCalls {
					ids[tc.ID] = true
				}
				for dropEnd < len(messages) && messages[dropEnd].Role == llm.RoleTool && ids[messages[dropEnd].ToolCallID] {
					dropEnd++
				}
			}
			messages = append(messages[:historyStart], messages[dropEnd:]...)
		}
	}

	// Re-surface unanswered user chat messages at the end of the message list.
	// Without this, chat messages get buried under tool calls and eventually
	// compressed away, causing the brain to silently ignore user input.
	var unanswered []string
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role == llm.RoleAssistant && strings.Contains(m.Content, "<UserMessage>") {
			// Found an assistant reply with UserMessage — all earlier chat messages are answered.
			break
		}
		if m.Role == llm.RoleUser && (strings.Contains(m.Content, "[Team Chat from ") || strings.Contains(m.Content, "[BrainMessage from ")) {
			unanswered = append(unanswered, m.Content)
		}
	}
	if len(unanswered) > 0 {
		// Reverse to preserve chronological order.
		for i, j := 0, len(unanswered)-1; i < j; i, j = i+1, j-1 {
			unanswered[i], unanswered[j] = unanswered[j], unanswered[i]
		}
		reminder := "[REMINDER] The user sent you the following chat message(s) that you have NOT yet replied to. You MUST reply using <UserMessage>your reply</UserMessage> NOW:\n"
		for _, msg := range unanswered {
			reminder += "\n" + msg + "\n"
		}
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: reminder,
		})
	}

	br.msgSize = llm.CountMessagesTokens(messages)

	// Compute per-section token breakdown for UI display.
	sections := []ContextBreakdownSection{
		{Name: "system_prompt", Tokens: llm.CountTokens(br.systemPrompt)},
		{Name: "task_goal", Tokens: llm.CountTokens(br.taskContent)},
	}
	if br.lastPanelSectionContent != nil {
		for _, k := range brainPanelSectionOrder(br.enableExploitFlow) {
			if c, ok := br.lastPanelSectionContent[k]; ok {
				sections = append(sections, ContextBreakdownSection{Name: k, Tokens: llm.CountTokens(c)})
			}
		}
	}
	// Conversation history tokens = total - system prompt - status panel
	panelTokens := 0
	for _, s := range sections {
		panelTokens += s.Tokens
	}
	historyTokens := br.msgSize - panelTokens
	if historyTokens < 0 {
		historyTokens = 0
	}
	sections = append(sections, ContextBreakdownSection{Name: "conversation_history", Tokens: historyTokens})
	br.contextBreakdown = ContextBreakdown{
		Sections:   sections,
		Total:      br.msgSize,
		MaxContext: br.maxHistory,
	}

	return messages
}

// GetContextBreakdown returns the latest per-section token breakdown.
func (br *BrainMemory) GetContextBreakdown() ContextBreakdown {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return br.contextBreakdown
}

func (br *BrainMemory) diskStateLocked() brainMemoryDiskState {
	maxHistory := br.maxHistory
	enableExploitFlow := br.enableExploitFlow
	return brainMemoryDiskState{
		SystemPrompt:            br.systemPrompt,
		PinnedMessages:          append([]llm.Message(nil), br.pinnedMessages...),
		Memory:                  append([]llm.Turn(nil), br.memory...),
		KeyMessage:              br.keyMessage,
		MaxHistory:              &maxHistory,
		CompressedSummary:       br.compressedSummary,
		TaskContent:             br.taskContent,
		AgentRuntimeInfoJSON:    br.agentRuntimeInfoJson,
		EnvInfoJSON:             br.envInfoJson,
		ExploitIdeaListJSON:     br.exploitIdeaListJson,
		ExploitChainListJSON:    br.exploitChainListJson,
		ContainerInfoList:       br.containerInfoList,
		NoAgentRunning:          br.noAgentRunning,
		UserPromptHash:          br.userPromptHash,
		MsgSize:                 br.msgSize,
		LastUserPrompt:          br.lastUserPrompt,
		LastPanelSectionHash:    br.lastPanelSectionHash,
		LastPanelSectionContent: br.lastPanelSectionContent,
		SeenAgentRuntime:        br.seenAgentRuntime,
		ContextBreakdown:        br.contextBreakdown,
		EnableExploitFlow:       &enableExploitFlow,
	}
}

func cloneMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		cloned := msg
		if len(msg.ToolCalls) > 0 {
			cloned.ToolCalls = append([]llm.ToolCall(nil), msg.ToolCalls...)
		}
		out = append(out, cloned)
	}
	return out
}

func containsMessageSequence(messages []llm.Message, target []llm.Message) bool {
	if len(target) == 0 || len(messages) < len(target) {
		return false
	}
	for start := 0; start <= len(messages)-len(target); start++ {
		if sameMessageSequence(messages[start:start+len(target)], target) {
			return true
		}
	}
	return false
}

func stripMessageSequence(messages []llm.Message, target []llm.Message) ([]llm.Message, bool) {
	if len(target) == 0 || len(messages) < len(target) {
		return append([]llm.Message(nil), messages...), false
	}
	out := make([]llm.Message, 0, len(messages))
	removed := false
	for i := 0; i < len(messages); {
		if i <= len(messages)-len(target) && sameMessageSequence(messages[i:i+len(target)], target) {
			removed = true
			i += len(target)
			continue
		}
		out = append(out, messages[i])
		i++
	}
	return out, removed
}

func sameMessageSequence(left []llm.Message, right []llm.Message) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !sameMessage(left[i], right[i]) {
			return false
		}
	}
	return true
}

func sameMessage(left llm.Message, right llm.Message) bool {
	if left.Role != right.Role || left.Content != right.Content || left.ToolCallID != right.ToolCallID {
		return false
	}
	if len(left.ToolCalls) != len(right.ToolCalls) {
		return false
	}
	for i := range left.ToolCalls {
		if left.ToolCalls[i] != right.ToolCalls[i] {
			return false
		}
	}
	return true
}

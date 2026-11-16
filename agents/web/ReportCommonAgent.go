package web

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"AIxVuln/toolCalling"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type ReportCommonAgent struct {
	memory       llm.Memory
	client       *toolCalling.ToolManager
	task         *taskManager.Task
	id           string
	config       *ReportCommonAgentConfig
	state        string
	stateHandler func(string)
	profile      AgentProfile
	taskChan     chan TaskAssignment
}
type ReportCommonAgentConfig struct {
	ReportType     string `json:"reportType"`
	TaskContent    string `json:"task_content"`
	ExploitIdeaId  string `json:"exploit_idea_id"`
	ExploitChainId string `json:"exploit_chain_id"`
}

func (c *ReportCommonAgent) GetTask() *taskManager.Task {
	return c.task
}

func (c *ReportCommonAgent) GetMemory() llm.Memory {
	return c.memory
}
func (c *ReportCommonAgent) GetId() string {
	return c.id
}
func (c *ReportCommonAgent) SetId(id string) {
	c.id = id
}
func (c *ReportCommonAgent) GetState() string {
	return c.state
}

func (c *ReportCommonAgent) GetProfile() AgentProfile {
	return c.profile
}

func (c *ReportCommonAgent) SetProfile(p AgentProfile) {
	c.profile = p
}
func (c *ReportCommonAgent) SetState(state string) {
	c.state = state
	if c.stateHandler != nil {
		c.stateHandler(state)
	}
}
func (c *ReportCommonAgent) SetKeyMessage(k map[string][]interface{}) {
	c.memory.SetKeyMessage(k, c.task.GetTaskId())
}

func (c *ReportCommonAgent) SetMemory(m llm.Memory) {
	c.memory = m
}
func (c *ReportCommonAgent) SetEnvInfo(k map[string]interface{}) {
	c.task.SetEnvInfo(k)
}

func (c *ReportCommonAgent) AssignTask(assignment TaskAssignment) {
	c.taskChan <- assignment
}

func NewReportCommonAgent(task *taskManager.Task, argsJson string) (Agent, error) {
	task.SetAgentName("ReportCommonAgent")
	// Use a combined system prompt that covers both report types.
	// The specific report template will be injected per task in executeTask.
	agentToolName := "Agent-Report-ReportCommonAgent"
	systemPrompt := prompts.ReportCommonAgentSystemPrompt(CommonSystemPromptForAgent(agentToolName))
	var memory llm.Memory
	if task.GetMemory() == nil {
		memory = llm.NewContextManager("report")
		memory.SetEventHandler(task.GetEventHandler())
		task.SetMemory(memory)
	} else {
		memory = task.GetMemory()
	}
	b := BuildAgentWithMemoryForAgent(task, memory, systemPrompt, ReportToolFactories("analyzer"), agentToolName)
	toolCalling.RegisterConfiguredMCPProfilesForAgentTypeWithTask(b.Client, task, agentToolName)
	agent := ReportCommonAgent{memory: b.Memory, client: b.Client, task: task, state: "Not Running", taskChan: make(chan TaskAssignment, 1)}
	agent.SetId(agent.Name() + "-" + uuid.New().String())
	return &agent, nil
}

func (c *ReportCommonAgent) StartTask(ctx context.Context) *StartResp {
	for {
		var assignment TaskAssignment
		select {
		case <-ctx.Done():
			return &StartResp{Err: ctx.Err()}
		case assignment = <-c.taskChan:
		}
		resp := c.executeTask(ctx, assignment)
		if assignment.DoneCb != nil {
			assignment.DoneCb(resp)
		}
		if ctx.Err() != nil {
			return resp
		}
	}
}

func (c *ReportCommonAgent) executeTask(ctx context.Context, assignment TaskAssignment) *StartResp {
	// Reset memory: compress previous session into a summary before starting new task.
	model := misc.GetConfigValueDefault("report", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
	if err := c.memory.ResetMemoryWithSummary(llm.GetResponsesClient("report", "main_setting"), model); err != nil {
		misc.Debug("%s: ResetMemoryWithSummary error (non-fatal): %s", c.Name(), err.Error())
	}

	config := &ReportCommonAgentConfig{}
	if err := json.Unmarshal([]byte(assignment.ArgsJson), config); err != nil {
		return &StartResp{Err: err}
	}
	c.config = config
	reportType := strings.ToLower(strings.TrimSpace(config.ReportType))
	switch reportType {
	case "analyzer", "verifier":
	default:
		reportType = "verifier"
	}
	config.ReportType = reportType

	// Build task content: fetch full exploit data by ID if provided.
	taskContent := strings.TrimSpace(config.TaskContent)
	if eid := strings.TrimSpace(config.ExploitIdeaId); eid != "" {
		c.task.SetCurrVulnId(eid)
		getter := c.task.GetExploitIdeaGetter()
		if getter == nil {
			return &StartResp{Err: fmt.Errorf("exploit idea getter not configured")}
		}
		idea, err := getter(eid)
		if err != nil {
			return &StartResp{Err: fmt.Errorf("failed to fetch exploitIdea %s: %w", eid, err)}
		}
		ideaJson, _ := json.Marshal(idea)
		if reportType == "analyzer" {
			taskContent = fmt.Sprintf("Write an analysis-only vulnerability report for this exploitIdea (unverified): %s", string(ideaJson))
		} else {
			taskContent = fmt.Sprintf("Write a vulnerability report for this exploitIdea (verified): %s", string(ideaJson))
		}
		if extra := strings.TrimSpace(config.TaskContent); extra != "" {
			taskContent += "\n\nAdditional instructions: " + extra
		}
	} else if cid := strings.TrimSpace(config.ExploitChainId); cid != "" {
		c.task.SetCurrVulnId(cid)
		getter := c.task.GetExploitChainGetter()
		if getter == nil {
			return &StartResp{Err: fmt.Errorf("exploit chain getter not configured")}
		}
		chain, err := getter(cid)
		if err != nil {
			return &StartResp{Err: fmt.Errorf("failed to fetch exploitChain %s: %w", cid, err)}
		}
		chainJson, _ := json.Marshal(chain)
		if reportType == "analyzer" {
			taskContent = fmt.Sprintf("Write an analysis-only vulnerability report for this exploitChain (unverified): %s", string(chainJson))
		} else {
			taskContent = fmt.Sprintf("Write a vulnerability report for this exploitChain (verified): %s", string(chainJson))
		}
		if extra := strings.TrimSpace(config.TaskContent); extra != "" {
			taskContent += "\n\nAdditional instructions: " + extra
		}
	}
	if taskContent == "" {
		return &StartResp{Err: fmt.Errorf("either exploit_idea_id, exploit_chain_id, or task_content is required")}
	}
	if strings.TrimSpace(config.ExploitIdeaId) == "" && strings.TrimSpace(config.ExploitChainId) == "" {
		c.task.SetCurrVulnId("")
	}

	// Inject report template based on report type (loaded from data/.reportTemplate/).
	templateName := "analyze"
	if reportType == "verifier" {
		templateName = "verifier"
	}
	reportTemplate := "\n" + misc.GetReportTemplate(templateName)

	fullTaskContent := taskContent + "\n" + reportTemplate
	tl := []map[string]string{{"TaskContent": fullTaskContent}}
	c.task.SetTaskList(tl)

	c.memory.AddMessage(&llm.MessageX{
		Msg:       llm.Message{Role: llm.RoleUser, Content: "[New Task Assigned]\n" + fullTaskContent},
		Shared:    false,
		ContextId: c.task.GetTaskId(),
	})

	c.SetState("Running")
	defer func() { c.SetState("Done") }()
	if len(c.task.GetTaskList()) < 2 {
		c.client.RemoveTool("TaskListTool")
	}
	var summary string
	client := llm.GetResponsesClient("report", "main_setting")
	contextLimitRecoveries := 0
	for {
		select {
		case <-ctx.Done():
			return &StartResp{Err: ctx.Err()}
		default:
		}
		var eventLog string
		c.memory.UnlockForLLM()
		msgList := c.memory.GetContext(c.task.GetTaskId())
		if msgList == nil {
			c.memory.LockForLLM()
			return &StartResp{Err: fmt.Errorf("agent task not set")}
		}
		c.memory.LockForLLM()
		debugLastMessages(c.profile.PersonaName, msgList)
		llm.CaptureMemoryDebugContext(c.memory, c.task.GetTaskId(), msgList)
		c.client.SetToolCallObserver(func(stage string, toolCall llm.ToolCall) {
			c.task.EmitAgentFeed(c.GetId(), "AgentToolCall", map[string]interface{}{
				"stage":      stage,
				"toolCallID": toolCall.ID,
				"name":       toolCall.Name,
			})
		})
		assistantMessage, toolMessage, err := c.client.ToolCallRequestWithLabel(ctx, client, msgList, model, c.Name(), c.profile.PersonaName, c.task.GetProjectName())
		if err != nil {
			c.memory.UnlockForLLM()
			if llm.IsContextLimitError(err) && contextLimitRecoveries < 2 {
				contextLimitRecoveries++
				beforeTokens := llm.CountMessagesTokens(msgList)
				if compressErr := c.memory.CompressIfNeeded(client, model); compressErr != nil {
					misc.Debug("%s: context-limit compression error: %s", c.Name(), compressErr.Error())
				}
				afterTokens := llm.CountMessagesTokens(c.memory.GetContext(c.task.GetTaskId()))
				misc.Debug("%s: context limit hit, compressed memory and retrying (attempt=%d before=%d after=%d err=%s)", c.Name(), contextLimitRecoveries, beforeTokens, afterTokens, err.Error())
				continue
			}
			return &StartResp{Err: err}
		}
		contextLimitRecoveries = 0
		c.task.EmitAgentFeed(c.GetId(), "AgentMessage", map[string]interface{}{
			"role":    assistantMessage.Role,
			"content": assistantMessage.Content,
		})
		misc.Debug("[%s] 报告编写者响应: %s 消息大小: %d", c.profile.PersonaName, assistantMessage.Content, c.GetMemory().GetMsgSize(c.task.GetTaskId()))

		eventLog = eventLog + "assistant: " + assistantMessage.Content + "\n"
		var index = 0
		if len(assistantMessage.ToolCalls) > 0 {
			for _, tool := range assistantMessage.ToolCalls {
				misc.Debug("报告编写者tool： %s -- %s", tool.Name, tool.Arguments)
				c.task.EmitAgentFeed(c.GetId(), "AgentToolCall", map[string]interface{}{
					"stage":      "call",
					"toolCallID": tool.ID,
					"name":       tool.Name,
					"arguments":  tool.Arguments,
				})
				eventLog = eventLog + "ToolCalling: " + tool.Name + " args: " + tool.Arguments + "\n"
				eventLog = eventLog + "ToolResult: " + toolMessage[index].Content + "\n"
				c.task.EmitAgentFeed(c.GetId(), "AgentToolCall", map[string]interface{}{
					"stage":      "result",
					"toolCallID": tool.ID,
					"name":       tool.Name,
					"result":     toolMessage[index].Content,
				})
				index++
			}
		}
		_ = c.task.EventLog(eventLog)
		msg := &llm.MessageX{Msg: assistantMessage, Shared: false, ContextId: c.task.GetTaskId()}
		c.memory.AddMessage(msg)
		if len(toolMessage) > 0 {
			if s, ok := extractAgentFinishSummary(toolMessage); ok {
				summary = s
				for _, message := range toolMessage {
					msgTool := &llm.MessageX{Msg: message, Shared: false, ContextId: c.task.GetTaskId()}
					c.memory.AddMessage(msgTool)
				}
				c.memory.UnlockForLLM()
				break
			}
			for _, message := range toolMessage {
				msgTool := &llm.MessageX{Msg: message, Shared: false, ContextId: c.task.GetTaskId()}
				c.memory.AddMessage(msgTool)
			}
		} else {
			if c.memory.HasPendingUserMessage() {
				misc.Debug("%s: pending user message detected, continuing loop", c.Name())
				c.memory.UnlockForLLM()
				continue
			}
			misc.Debug("%s: 空响应（无工具调用），发送提醒继续", c.Name())
			c.memory.UnlockForLLM()
			c.memory.AddMessage(&llm.MessageX{Msg: llm.Message{Role: llm.RoleUser, Content: "Please continue your task. If you have finished, call AgentFinishTool with a summary."}, Shared: false, ContextId: c.task.GetTaskId()})
			continue
		}
		if err := c.memory.CompressIfNeeded(client, model); err != nil {
			misc.Debug("%s: memory compress error: %s", c.Name(), err.Error())
		}
		c.memory.UnlockForLLM()
	}
	return &StartResp{Err: nil, Memory: c.memory, EvnInfo: c.task.GetEnvInfo(), Summary: summary}
}

func (c *ReportCommonAgent) Name() string {
	return "ReportCommonAgent"
}
func (c *ReportCommonAgent) SetStateHandler(f func(string)) {
	c.stateHandler = f
}

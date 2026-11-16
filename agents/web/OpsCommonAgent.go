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

	"github.com/google/uuid"
)

type OpsCommonAgent struct {
	memory       llm.Memory
	client       *toolCalling.ToolManager
	task         *taskManager.Task
	id           string
	state        string
	stateHandler func(string)
	profile      AgentProfile
	taskChan     chan TaskAssignment
}
type OpsCommonAgentConfig struct {
	TaskContent string `json:"task_content"`
}

func (c *OpsCommonAgent) GetTask() *taskManager.Task {
	return c.task
}

func (c *OpsCommonAgent) GetMemory() llm.Memory {
	return c.memory
}
func (c *OpsCommonAgent) GetId() string {
	return c.id
}
func (c *OpsCommonAgent) SetId(id string) {
	c.id = id
}
func (c *OpsCommonAgent) GetState() string {
	return c.state
}

func (c *OpsCommonAgent) GetProfile() AgentProfile {
	return c.profile
}

func (c *OpsCommonAgent) SetProfile(p AgentProfile) {
	c.profile = p
}
func (c *OpsCommonAgent) SetState(state string) {
	c.state = state
	if c.stateHandler != nil {
		c.stateHandler(state)
	}
}
func (c *OpsCommonAgent) SetMemory(m llm.Memory) {
	c.memory = m
}
func (c *OpsCommonAgent) SetKeyMessage(k map[string][]interface{}) {
	c.memory.SetKeyMessage(k, c.task.GetTaskId())
}
func (c *OpsCommonAgent) SetEnvInfo(k map[string]interface{}) {
	c.task.SetEnvInfo(k)
}

func (c *OpsCommonAgent) AssignTask(assignment TaskAssignment) {
	c.taskChan <- assignment
}

func NewOpsCommonAgent(task *taskManager.Task, argsJson string) (Agent, error) {
	task.SetAgentName("OpsCommonAgent")
	agentToolName := "Agent-Ops-OpsCommonAgent"
	systemPrompt := prompts.OpsCommonAgentSystemPrompt(CommonSystemPromptForAgent(agentToolName))
	var memory llm.Memory
	if task.GetMemory() == nil {
		memory = llm.NewContextManager("ops")
		memory.SetEventHandler(task.GetEventHandler())
		task.SetMemory(memory)
	} else {
		memory = task.GetMemory()
	}
	b := BuildAgentWithMemoryForAgent(task, memory, systemPrompt, OpsToolFactories(), agentToolName)
	toolCalling.RegisterConfiguredMCPProfilesForAgentTypeWithTask(b.Client, task, agentToolName)
	agent := OpsCommonAgent{memory: b.Memory, client: b.Client, task: task, state: "Not Running", taskChan: make(chan TaskAssignment, 1)}
	agent.SetId(agent.Name() + "-" + uuid.New().String())
	return &agent, nil
}

func (c *OpsCommonAgent) StartTask(ctx context.Context) *StartResp {
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

func (c *OpsCommonAgent) executeTask(ctx context.Context, assignment TaskAssignment) *StartResp {
	config := &OpsCommonAgentConfig{}
	if err := json.Unmarshal([]byte(assignment.ArgsJson), config); err != nil {
		return &StartResp{Err: err}
	}
	taskContent := config.TaskContent
	tl := []map[string]string{{"TaskContent": taskContent}}
	c.task.SetTaskList(tl)

	c.memory.AddMessage(&llm.MessageX{
		Msg:       llm.Message{Role: llm.RoleUser, Content: "[New Task Assigned]\n" + taskContent},
		Shared:    false,
		ContextId: c.task.GetTaskId(),
	})

	c.SetState("Running")
	defer func() { c.SetState("Done") }()
	if len(c.task.GetTaskList()) < 2 {
		c.client.RemoveTool("TaskListTool")
	}
	var summary string
	model := misc.GetConfigValueDefault("ops", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
	client := llm.GetResponsesClient("ops", "main_setting")
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
		misc.Debug("[%s] 运维者响应: %s 消息大小: %d", c.profile.PersonaName, assistantMessage.Content, c.GetMemory().GetMsgSize(c.task.GetTaskId()))

		eventLog = eventLog + "assistant: " + assistantMessage.Content + "\n"
		var index = 0
		if len(assistantMessage.ToolCalls) > 0 {
			for _, tool := range assistantMessage.ToolCalls {
				misc.Debug("运维者tool： %s -- %s", tool.Name, tool.Arguments)
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

func (c *OpsCommonAgent) Name() string {
	return "OpsCommonAgent"
}
func (c *OpsCommonAgent) SetStateHandler(f func(string)) {
	c.stateHandler = f
}

package binary

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

type AndroidCommonAgent struct {
	memory       llm.Memory
	client       *toolCalling.ToolManager
	task         *taskManager.Task
	id           string
	config       *AndroidCommonAgentConfig
	state        string
	stateHandler func(string)
	profile      AgentProfile
	taskChan     chan TaskAssignment
}

type AndroidCommonAgentConfig struct {
	TaskContent string `json:"task_content"`
	ApkFilePath string `json:"apk_file_path"`
}

func (c *AndroidCommonAgent) GetTask() *taskManager.Task {
	return c.task
}
func (c *AndroidCommonAgent) GetMemory() llm.Memory {
	return c.memory
}
func (c *AndroidCommonAgent) GetId() string {
	return c.id
}
func (c *AndroidCommonAgent) SetId(id string) {
	c.id = id
}
func (c *AndroidCommonAgent) GetState() string {
	return c.state
}

func (c *AndroidCommonAgent) GetProfile() AgentProfile {
	return c.profile
}

func (c *AndroidCommonAgent) SetProfile(p AgentProfile) {
	c.profile = p
}
func (c *AndroidCommonAgent) SetState(state string) {
	c.state = state
	if c.stateHandler != nil {
		c.stateHandler(state)
	}
}
func (c *AndroidCommonAgent) SetMemory(m llm.Memory) {
	c.memory = m
}
func (c *AndroidCommonAgent) SetEnvInfo(k map[string]interface{}) {
	c.task.SetEnvInfo(k)
}
func (c *AndroidCommonAgent) SetKeyMessage(k map[string][]interface{}) {
	c.memory.SetKeyMessage(k, c.task.GetTaskId())
}

func (c *AndroidCommonAgent) AssignTask(assignment TaskAssignment) {
	c.taskChan <- assignment
}

func NewAndroidCommonAgent(task *taskManager.Task, argsJson string) (Agent, error) {
	task.SetAgentName("AndroidCommonAgent")
	agentToolName := "Agent-Binary-AndroidCommonAgent"
	systemPrompt := prompts.AndroidCommonAgentSystemPrompt(CommonSystemPromptForAgent(agentToolName))
	var memory llm.Memory
	if task.GetMemory() == nil {
		memory = llm.NewContextManager("android")
		memory.SetEventHandler(task.GetEventHandler())
		task.SetMemory(memory)
	} else {
		memory = task.GetMemory()
	}
	b := BuildAgentWithMemoryForAgent(task, memory, systemPrompt, ReverseToolFactories(), agentToolName)
	toolCalling.RegisterConfiguredMCPProfilesForAgentTypeWithTask(b.Client, task, agentToolName)
	b.Memory.AddKeyMessage(&llm.EnvMessageX{
		Key: "BinaryHostWorkspace",
		Content: map[string]interface{}{
			"sandbox_source_root": misc.SandboxSourceDir,
			"mcp_path_prefix":     "aix-mcp://",
			"note":                "MCP runs on the host. When you want MCP-side tools to open project files, always reference them as aix-mcp://<path-relative-to-/workdir/sourceCodeDir>. When a tool argument needs the file CONTENT itself instead of a path, use <aixfile>relative/path</aixfile>. The bridge resolves aix-mcp:// to host paths and expands <aixfile> tags to real file content before execution.",
		},
		AppendEnv: false,
		ContextId: task.GetTaskId(),
		NotShared: true,
	})
	agent := AndroidCommonAgent{memory: b.Memory, client: b.Client, task: task, state: "Not Running", taskChan: make(chan TaskAssignment, 1)}
	agent.SetId(agent.Name() + "-" + uuid.New().String())
	return &agent, nil
}

func (c *AndroidCommonAgent) StartTask(ctx context.Context) *StartResp {
	for {
		// Wait for a task assignment or context cancellation.
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
		// If context was cancelled during execution, exit the outer loop.
		if ctx.Err() != nil {
			return resp
		}
	}
}

func (c *AndroidCommonAgent) executeTask(ctx context.Context, assignment TaskAssignment) *StartResp {
	config := &AndroidCommonAgentConfig{}
	if err := json.Unmarshal([]byte(assignment.ArgsJson), config); err != nil {
		return &StartResp{Err: err}
	}
	c.config = config

	// Per-task quota state must be reset on every assignment.
	c.task.ResetExploitIdeaSubmitted()
	c.task.SetExploitIdeaQuota(0)

	// Inject task content as a user message.
	taskContent := config.TaskContent
	if path := strings.TrimSpace(config.ApkFilePath); path != "" {
		taskContent += fmt.Sprintf("\nTarget APK file path: %s", path)
		c.task.SetCurrVulnId("android:" + path)
	} else {
		c.task.SetCurrVulnId("binary-android")
	}
	defer c.task.SetCurrVulnId("")

	tl := []map[string]string{{"TaskContent": taskContent}}
	c.task.SetTaskList(tl)

	c.memory.AddMessage(&llm.MessageX{
		Msg:       llm.Message{Role: llm.RoleUser, Content: "[New Task Assigned]\n" + taskContent},
		Shared:    false,
		ContextId: c.task.GetTaskId(),
	})

	c.SetState("Running")
	defer func() {
		c.SetState("Done")
	}()
	if len(c.task.GetTaskList()) < 2 {
		c.client.RemoveTool("TaskListTool")
	}
	var summary string
	model := misc.GetConfigValueDefault("android", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
	client := llm.GetResponsesClient("android", "main_setting")
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
		misc.Debug("[%s] 逆向者响应: %s 消息大小: %d", c.profile.PersonaName, assistantMessage.Content, c.GetMemory().GetMsgSize(c.task.GetTaskId()))
		eventLog = eventLog + "assistant: " + assistantMessage.Content + "\n"
		var index = 0
		if len(assistantMessage.ToolCalls) > 0 {
			for _, tool := range assistantMessage.ToolCalls {
				misc.Debug("逆向者tool：%s -- %s", tool.Name, tool.Arguments)
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
			// Check if AgentFinishTool was called.
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
			// Empty response with no tool calls — do NOT exit, nudge the LLM to continue.
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

func (c *AndroidCommonAgent) Name() string {
	return "AndroidCommonAgent"
}

func (c *AndroidCommonAgent) SetStateHandler(f func(string)) {
	c.stateHandler = f
}

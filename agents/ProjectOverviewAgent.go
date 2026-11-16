package agents

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

type ProjectOverviewAgent struct {
	memory       llm.Memory
	client       *toolCalling.ToolManager
	task         *taskManager.Task
	id           string
	state        string
	stateHandler func(string)
	profile      AgentProfile
	taskChan     chan TaskAssignment
}

const overviewMCPAgentType = "Agent-General-GeneralCommonAgent"

type projectOverviewAgentConfig struct {
	TaskContent string `json:"task_content"`
}

func (c *ProjectOverviewAgent) GetTask() *taskManager.Task {
	return c.task
}

func (c *ProjectOverviewAgent) GetMemory() llm.Memory {
	return c.memory
}

func (c *ProjectOverviewAgent) GetId() string {
	return c.id
}

func (c *ProjectOverviewAgent) SetId(id string) {
	c.id = id
}

func (c *ProjectOverviewAgent) GetState() string {
	return c.state
}

func (c *ProjectOverviewAgent) GetProfile() AgentProfile {
	return c.profile
}

func (c *ProjectOverviewAgent) SetProfile(p AgentProfile) {
	c.profile = p
}

func (c *ProjectOverviewAgent) SetState(state string) {
	c.state = state
	if c.stateHandler != nil {
		c.stateHandler(state)
	}
}

func (c *ProjectOverviewAgent) SetMemory(m llm.Memory) {
	c.memory = m
}

func (c *ProjectOverviewAgent) SetKeyMessage(k map[string][]interface{}) {
	c.memory.SetKeyMessage(k, c.task.GetTaskId())
}

func (c *ProjectOverviewAgent) SetEnvInfo(k map[string]interface{}) {
	c.task.SetEnvInfo(k)
}

func (c *ProjectOverviewAgent) Name() string {
	return "ProjectOverviewAgent"
}

func (c *ProjectOverviewAgent) SetStateHandler(f func(string)) {
	c.stateHandler = f
}

func (c *ProjectOverviewAgent) AssignTask(assignment TaskAssignment) {
	c.taskChan <- assignment
}

func NewProjectOverviewAgent(task *taskManager.Task, argsJson string) (Agent, error) {
	task.SetAgentName("ProjectOverviewAgent")
	systemPrompt := projectOverviewSystemPrompt()

	memory := llm.NewContextManager("overview")
	memory.SetEventHandler(task.GetEventHandler())
	task.SetMemory(memory)
	tl := []map[string]string{{"TaskContent": projectOverviewTaskContentFromArgs(argsJson)}}
	task.SetTaskList(tl)
	b := BuildAgentWithMemoryForAgent(task, memory, systemPrompt, OverviewToolFactories(), "Agent-General-ProjectOverviewAgent")
	toolCalling.RegisterConfiguredMCPProfilesForAgentTypeWithTask(b.Client, task, overviewMCPAgentType)
	agent := ProjectOverviewAgent{memory: b.Memory, client: b.Client, task: task, state: "Not Running", taskChan: make(chan TaskAssignment, 1)}
	agent.SetId(agent.Name() + "-" + uuid.New().String())
	return &agent, nil
}

func projectOverviewSystemPrompt() string {
	return prompts.ProjectOverviewAgentSystemPrompt(CommonSystemPromptForAgent("Agent-General-ProjectOverviewAgent"))
}

func projectOverviewTaskContent() string {
	return "Inspect the current project workspace and produce a concise structured overview. Prefer source-code inspection when files exist; if /workdir/sourceCodeDir is empty or too sparse to classify the target, use lightweight command-based reconnaissance and fingerprinting against any target information provided in the task or workspace instead of skipping the overview. If the task mainly provides a URL and no source code, a lightweight curl of the homepage plus basic fingerprinting/server-info checks is usually enough for the overview; the URL's scheme/host/port is already the explicit entrypoint, so do not scan ports for URL targets. Only consider a small conservative port check when the task provides a bare IP/hostname without an explicit URL or port."
}

func projectOverviewTaskContentFromArgs(argsJson string) string {
	var cfg projectOverviewAgentConfig
	if strings.TrimSpace(argsJson) != "" {
		_ = json.Unmarshal([]byte(argsJson), &cfg)
	}
	return buildProjectOverviewTaskContent(strings.TrimSpace(cfg.TaskContent))
}

func buildProjectOverviewTaskContent(projectTaskContent string) string {
	base := projectOverviewTaskContent()
	projectTaskContent = strings.TrimSpace(projectTaskContent)
	if projectTaskContent == "" {
		return base
	}
	return base + "\n\nCurrent project task/context:\n" + projectTaskContent + "\n\nUse the concrete target information above as first-class input for the overview. Do not ignore provided URLs, host:port entrypoints, challenge descriptions, filenames, or artifact hints just because /workdir/sourceCodeDir is empty."
}

func (c *ProjectOverviewAgent) StartTask(ctx context.Context) *StartResp {
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

func (c *ProjectOverviewAgent) executeTask(ctx context.Context, assignment TaskAssignment) *StartResp {
	taskContent := projectOverviewTaskContentFromArgs(assignment.ArgsJson)
	c.task.SetTaskList([]map[string]string{{"TaskContent": taskContent}})
	c.memory.AddMessage(&llm.MessageX{
		Msg:       llm.Message{Role: llm.RoleUser, Content: "[New Task Assigned]\n" + taskContent},
		Shared:    false,
		ContextId: c.task.GetTaskId(),
	})

	c.SetState("Running")
	defer func() { c.SetState("Done") }()
	c.client.RemoveTool("TaskListTool")
	var summary string
	model := misc.GetConfigValueDefault("overview", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
	client := llm.GetResponsesClient("overview", "main_setting")
	contextLimitRecoveries := 0
	for {
		select {
		case <-ctx.Done():
			return &StartResp{Err: ctx.Err()}
		default:
		}
		msgList := c.memory.GetContext(c.task.GetTaskId())
		if msgList == nil {
			return &StartResp{Err: fmt.Errorf("agent task not set")}
		}
		llm.CaptureMemoryDebugContext(c.memory, c.task.GetTaskId(), msgList)
		c.client.SetToolCallObserver(func(stage string, toolCall llm.ToolCall) {
			c.task.EmitAgentFeed(c.GetId(), "AgentToolCall", map[string]interface{}{
				"stage":      stage,
				"toolCallID": toolCall.ID,
				"name":       toolCall.Name,
			})
		})
		assistantMessage, toolMessage, err := c.client.ToolCallRequestWithLabel(ctx, client, msgList, model, c.Name(), "ProjectOverview", c.task.GetProjectName())
		if err != nil {
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
		misc.Debug("[ProjectOverview] 响应: %s", assistantMessage.Content)
		msg := &llm.MessageX{Msg: assistantMessage, Shared: false, ContextId: c.task.GetTaskId()}
		c.memory.AddMessage(msg)
		if len(toolMessage) > 0 {
			if s, ok := extractAgentFinishSummary(toolMessage); ok {
				summary = s
				for _, message := range toolMessage {
					msgTool := &llm.MessageX{Msg: message, Shared: false, ContextId: c.task.GetTaskId()}
					c.memory.AddMessage(msgTool)
				}
				break
			}
			for _, message := range toolMessage {
				msgTool := &llm.MessageX{Msg: message, Shared: false, ContextId: c.task.GetTaskId()}
				c.memory.AddMessage(msgTool)
			}
		} else {
			misc.Debug("[ProjectOverview] 空响应，发送提醒继续")
			c.memory.AddMessage(&llm.MessageX{Msg: llm.Message{Role: llm.RoleUser, Content: "Please continue your task. If you have finished, call AgentFinishTool with a summary."}, Shared: false, ContextId: c.task.GetTaskId()})
			continue
		}
		if err := c.memory.CompressIfNeeded(client, model); err != nil {
			misc.Debug("%s: memory compress error: %s", c.Name(), err.Error())
		}
	}
	return &StartResp{Err: nil, Memory: c.memory, Summary: summary}
}

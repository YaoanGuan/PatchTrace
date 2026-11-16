package toolCalling

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type toolUsageGuideRequest struct {
	Question     string
	SessionID    string
	CloseSession bool
	Scope        string
}

var toolUsageGuideAnswerFunc = defaultToolUsageGuideAnswer

type toolUsageGuideAgent struct {
	task        *taskManager.Task
	memory      llm.Memory
	client      *ToolManager
	contextID   string
	projectName string
}

func newToolUsageGuideAgent(task *taskManager.Task) *toolUsageGuideAgent {
	contextID := "tool-usage-guide-" + uuid.New().String()
	memory := llm.NewContextManager("general")
	if task != nil && task.GetEventHandler() != nil {
		memory.SetEventHandler(task.GetEventHandler())
	}
	memory.SetSystemPrompt(&llm.SystemPromptX{
		SystemPrompt: toolUsageGuideSystemPrompt(),
		ContextId:    contextID,
	})
	initializeEphemeralLookupTask(memory, contextID, "Answer another digital human's tool-usage question by using only the provided resource/docs lookup tools, then finish with AgentFinishTool.")
	client := NewToolManager()
	if task != nil && task.GetSm() != nil {
		client.SetSourceRoot(task.GetSourceCodePath())
	}
	client.Register(NewListResourceDocsTool(task))
	client.Register(NewSearchResourceDocsTool(task))
	client.Register(NewReadResourceDocTool(task))
	client.Register(NewAgentFinishTool(task))
	projectName := ""
	if task != nil {
		projectName = task.GetProjectName()
	}
	return &toolUsageGuideAgent{
		task:        task,
		memory:      memory,
		client:      client,
		contextID:   contextID,
		projectName: projectName,
	}
}

func toolUsageGuideSystemPrompt() string {
	return prompts.ToolUsageGuideSystemPrompt()
}

func runToolUsageGuideAgent(task *taskManager.Task, req toolUsageGuideRequest) (toolUsageGuideResult, error) {
	result, err := toolUsageGuideAnswerFunc(task, req)
	if strings.TrimSpace(req.Question) != "" || err != nil {
		appendQuestionAnswerLog(task, "runToolUsageGuideAgent", req.Question, result.Answer, err)
	}
	return result, err
}

func (a *toolUsageGuideAgent) answer(question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", fmt.Errorf("question is required")
	}
	a.memory.AddMessage(&llm.MessageX{
		ContextId: a.contextID,
		Msg: llm.Message{
			Role:    llm.RoleUser,
			Content: "请回答这个工具使用问题，并只基于 resource/docs 中查到的内容作答：\n" + question,
		},
	})
	model := misc.GetConfigValueDefault("general", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
	cli := llm.GetResponsesClient("general", "main_setting")
	ctx, cancel := context.WithTimeout(context.Background(), ephemeralKnowledgeAgentTimeout)
	defer cancel()
	for turn := 0; turn < ephemeralKnowledgeAgentMaxTurns; turn++ {
		msgList := a.memory.GetContext(a.contextID)
		if msgList == nil {
			return "", fmt.Errorf("tool usage guide memory not initialized")
		}
		assistantMessage, toolMessages, err := a.client.ToolCallRequestWithLabel(ctx, cli, msgList, model, "ToolUsageGuideAgent", "ToolUsageGuideAgent", a.projectName)
		if err != nil {
			return "", err
		}
		a.memory.AddMessage(&llm.MessageX{ContextId: a.contextID, Msg: assistantMessage})
		if len(toolMessages) == 0 {
			a.memory.AddMessage(&llm.MessageX{
				ContextId: a.contextID,
				Msg: llm.Message{
					Role:    llm.RoleUser,
					Content: "不要空转。继续使用 docs 工具查找，或者在确认文档不足后调用 AgentFinishTool 给出简洁中文答复。",
				},
			})
			continue
		}
		if summary, ok := extractToolUsageGuideSummary(toolMessages); ok {
			return summary, nil
		}
		for _, message := range toolMessages {
			a.memory.AddMessage(&llm.MessageX{ContextId: a.contextID, Msg: message})
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("tool usage guide helper reached max turns without finishing")
}

func extractToolUsageGuideSummary(toolMessages []llm.Message) (string, bool) {
	for _, message := range toolMessages {
		if strings.HasPrefix(message.Content, AgentFinishMarker) {
			return strings.TrimSpace(strings.TrimPrefix(message.Content, AgentFinishMarker)), true
		}
	}
	return "", false
}

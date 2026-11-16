package toolCalling

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const ephemeralKnowledgeAgentTimeout = 75 * time.Second
const ephemeralKnowledgeAgentMaxTurns = 4

type pocKnowledgeRequest struct {
	Question     string
	SessionID    string
	CloseSession bool
	Scope        string
}

type pocKnowledgeResult struct {
	Answer    string
	SessionID string
	Created   bool
	Closed    bool
}

type pocKnowledgeAgent struct {
	task        *taskManager.Task
	memory      llm.Memory
	client      *ToolManager
	contextID   string
	projectName string
}

func newPocKnowledgeAgent(task *taskManager.Task) *pocKnowledgeAgent {
	contextID := "poc-knowledge-" + uuid.New().String()
	memory := llm.NewContextManager("general")
	if task != nil && task.GetEventHandler() != nil {
		memory.SetEventHandler(task.GetEventHandler())
	}
	memory.SetSystemPrompt(&llm.SystemPromptX{
		SystemPrompt: pocKnowledgeSystemPrompt(),
		ContextId:    contextID,
	})
	initializeEphemeralLookupTask(memory, contextID, "Answer another digital human's PoC / exploit knowledge question by using only the provided PoC knowledge lookup tools, then finish with AgentFinishTool.")
	client := NewToolManager()
	if task != nil && task.GetSm() != nil {
		client.SetSourceRoot(task.GetSourceCodePath())
	}
	client.Register(NewListPocKnowledgeDocsTool(task))
	client.Register(NewSearchPocKnowledgeDocsTool(task))
	client.Register(NewReadPocKnowledgeDocTool(task))
	client.Register(NewAgentFinishTool(task))
	projectName := ""
	if task != nil {
		projectName = task.GetProjectName()
	}
	return &pocKnowledgeAgent{
		task:        task,
		memory:      memory,
		client:      client,
		contextID:   contextID,
		projectName: projectName,
	}
}

func pocKnowledgeSystemPrompt() string {
	return prompts.PocKnowledgeSystemPrompt()
}

func runPocKnowledgeAgent(task *taskManager.Task, req pocKnowledgeRequest) (pocKnowledgeResult, error) {
	return pocKnowledgeAnswerFunc(task, req)
}

func (a *pocKnowledgeAgent) answer(question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", fmt.Errorf("question is required")
	}
	a.memory.AddMessage(&llm.MessageX{
		ContextId: a.contextID,
		Msg: llm.Message{
			Role:    llm.RoleUser,
			Content: "请仅基于 resource/pocs 本地知识库中查到的内容回答这个问题：\n" + question,
		},
	})
	model := misc.GetConfigValueDefault("general", "MODEL", misc.GetConfigValueRequired("main_setting", "MODEL"))
	cli := llm.GetResponsesClient("general", "main_setting")
	ctx, cancel := context.WithTimeout(context.Background(), ephemeralKnowledgeAgentTimeout)
	defer cancel()
	for turn := 0; turn < ephemeralKnowledgeAgentMaxTurns; turn++ {
		msgList := a.memory.GetContext(a.contextID)
		if msgList == nil {
			return "", fmt.Errorf("poc knowledge memory not initialized")
		}
		assistantMessage, toolMessages, err := a.client.ToolCallRequestWithLabel(ctx, cli, msgList, model, "PocKnowledgeAgent", "PocKnowledgeAgent", a.projectName)
		if err != nil {
			return "", err
		}
		a.memory.AddMessage(&llm.MessageX{ContextId: a.contextID, Msg: assistantMessage})
		if len(toolMessages) == 0 {
			a.memory.AddMessage(&llm.MessageX{
				ContextId: a.contextID,
				Msg: llm.Message{
					Role:    llm.RoleUser,
					Content: "不要空转。继续使用 PoC 知识库工具检索，或者在确认资料不足后调用 AgentFinishTool 给出简洁中文答复。",
				},
			})
			continue
		}
		if summary, ok := extractPocKnowledgeSummary(toolMessages); ok {
			return summary, nil
		}
		for _, message := range toolMessages {
			a.memory.AddMessage(&llm.MessageX{ContextId: a.contextID, Msg: message})
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("poc knowledge helper reached max turns without finishing")
}

func extractPocKnowledgeSummary(toolMessages []llm.Message) (string, bool) {
	for _, message := range toolMessages {
		if strings.HasPrefix(message.Content, AgentFinishMarker) {
			return strings.TrimSpace(strings.TrimPrefix(message.Content, AgentFinishMarker)), true
		}
	}
	return "", false
}

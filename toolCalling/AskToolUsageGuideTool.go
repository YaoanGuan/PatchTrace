package toolCalling

import (
	"AIxVuln/taskManager"
	"strings"
)

type AskToolUsageGuideTool struct {
	task         *taskManager.Task
	sessionScope string
}

var runToolUsageGuideQuery = runToolUsageGuideAgent

func NewAskToolUsageGuideTool(task *taskManager.Task) *AskToolUsageGuideTool {
	return &AskToolUsageGuideTool{task: task}
}

func NewAskToolUsageGuideToolWithScope(task *taskManager.Task, sessionScope string) *AskToolUsageGuideTool {
	return &AskToolUsageGuideTool{task: task, sessionScope: strings.TrimSpace(sessionScope)}
}

func (h *AskToolUsageGuideTool) Name() string {
	return "AskToolUsageGuideTool"
}

func (h *AskToolUsageGuideTool) Description() string {
	return "Use this when the blocker is HOW to use a system tool, MCP capability, packaged helper, or workflow. The first call starts a reusable docs-guide session and returns session_id; reuse that session_id for follow-up questions, and set close_session=true when you are done."
}

func (h *AskToolUsageGuideTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": "The current tool/MCP/workflow usage question. Required unless you are only closing an existing docs-guide session.",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional existing docs-guide session_id returned by a previous AskToolUsageGuideTool call. Use it for follow-up questions so the helper keeps context.",
			},
			"close_session": map[string]interface{}{
				"type":        "boolean",
				"description": "Set true to close the docs-guide session after answering, or to close an existing session without asking a new question.",
			},
		},
	}
}

func (h *AskToolUsageGuideTool) Execute(parameters map[string]interface{}) string {
	question := strings.TrimSpace(stringParam(parameters["question"]))
	sessionID := strings.TrimSpace(stringParam(parameters["session_id"]))
	closeSession := boolParam(parameters["close_session"])
	switch {
	case question == "" && !closeSession:
		return Fail("Missing 'question' parameter")
	case question == "" && closeSession && sessionID == "":
		return Fail("Missing 'session_id' parameter when closing a session without a question")
	}
	result, err := runToolUsageGuideQuery(h.task, toolUsageGuideRequest{
		Question:     question,
		SessionID:    sessionID,
		CloseSession: closeSession,
		Scope:        h.sessionScope,
	})
	if err != nil {
		return Fail("tool usage guide failed: " + err.Error())
	}
	return Success(map[string]interface{}{
		"answer":          result.Answer,
		"session_id":      result.SessionID,
		"created":         result.Created,
		"closed":          result.Closed,
		"destroyed":       result.Closed,
		"session_active":  result.SessionID != "" && !result.Closed,
		"ephemeral_agent": false,
	})
}

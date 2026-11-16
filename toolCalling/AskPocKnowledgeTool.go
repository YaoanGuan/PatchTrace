package toolCalling

import (
	"AIxVuln/taskManager"
	"strings"
)

type AskPocKnowledgeTool struct {
	task         *taskManager.Task
	sessionScope string
}

var runPocKnowledgeQuery = runPocKnowledgeAgent
var runPocKnowledgeFallback = fallbackPocKnowledgeAnswer

func NewAskPocKnowledgeTool(task *taskManager.Task) *AskPocKnowledgeTool {
	return &AskPocKnowledgeTool{task: task}
}

func NewAskPocKnowledgeToolWithScope(task *taskManager.Task, sessionScope string) *AskPocKnowledgeTool {
	return &AskPocKnowledgeTool{task: task, sessionScope: strings.TrimSpace(sessionScope)}
}

func (h *AskPocKnowledgeTool) Name() string {
	return "AskPocKnowledgeTool"
}

func (h *AskPocKnowledgeTool) Description() string {
	return "Use this first when you identify a target product/technology or need offensive technique references such as exploit ideas, bypasses, in-memory code injection, shellcode loading, reflective loading, persistence, or PoC notes. The first call starts a reusable PoC-knowledge session and returns session_id; reuse that session_id for follow-up knowledge questions, and set close_session=true when you are done."
}

func (h *AskPocKnowledgeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": "The current product/technology/vulnerability/PoC knowledge question. Required unless you are only closing an existing PoC-knowledge session.",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional existing PoC-knowledge session_id returned by a previous AskPocKnowledgeTool call. Use it for follow-up questions so the helper keeps context.",
			},
			"close_session": map[string]interface{}{
				"type":        "boolean",
				"description": "Set true to close the PoC-knowledge session after answering, or to close an existing session without asking a new question.",
			},
		},
	}
}

func (h *AskPocKnowledgeTool) Execute(parameters map[string]interface{}) string {
	question := strings.TrimSpace(stringParam(parameters["question"]))
	sessionID := strings.TrimSpace(stringParam(parameters["session_id"]))
	closeSession := boolParam(parameters["close_session"])
	switch {
	case question == "" && !closeSession:
		return Fail("Missing 'question' parameter")
	case question == "" && closeSession && sessionID == "":
		return Fail("Missing 'session_id' parameter when closing a session without a question")
	}
	result, err := runPocKnowledgeQuery(h.task, pocKnowledgeRequest{
		Question:     question,
		SessionID:    sessionID,
		CloseSession: closeSession,
		Scope:        h.sessionScope,
	})
	if err != nil {
		fallbackAllowed := question != "" && sessionID == "" && !closeSession
		fallbackAnswer, fallbackErr := runPocKnowledgeFallback(question)
		if fallbackAllowed && fallbackErr == nil && strings.TrimSpace(fallbackAnswer) != "" {
			appendQuestionAnswerLog(h.task, "AskPocKnowledgeTool", question, fallbackAnswer, err)
			return Success(map[string]interface{}{
				"answer":           fallbackAnswer,
				"session_id":       "",
				"created":          false,
				"closed":           true,
				"session_active":   false,
				"ephemeral_agent":  false,
				"destroyed":        true,
				"fallback_mode":    "direct_search",
				"fallback_warning": "poc knowledge helper failed; returned a direct local-search summary instead",
			})
		}
		appendQuestionAnswerLog(h.task, "AskPocKnowledgeTool", question, "", err)
		return Fail("poc knowledge lookup failed: " + err.Error())
	}
	appendQuestionAnswerLog(h.task, "AskPocKnowledgeTool", question, result.Answer, nil)
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

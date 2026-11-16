package toolCalling

import (
	"AIxVuln/taskManager"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAskPocKnowledgeToolReturnsSessionPayload(t *testing.T) {
	oldRunner := runPocKnowledgeQuery
	oldFallback := runPocKnowledgeFallback
	oldQLogPath := qLogPath
	t.Cleanup(func() {
		runPocKnowledgeQuery = oldRunner
		runPocKnowledgeFallback = oldFallback
		qLogPath = oldQLogPath
	})
	qLogPath = filepath.Join(t.TempDir(), "q.log")

	runPocKnowledgeQuery = func(_ *taskManager.Task, req pocKnowledgeRequest) (pocKnowledgeResult, error) {
		if req.Question != "需要 Shiro 相关漏洞和可利用线索" {
			t.Fatalf("unexpected question: %q", req.Question)
		}
		if req.SessionID != "" {
			t.Fatalf("unexpected session id: %q", req.SessionID)
		}
		if req.CloseSession {
			t.Fatalf("unexpected close_session=true")
		}
		return pocKnowledgeResult{
			Answer:    "结论：命中 Shiro rememberMe 反序列化资料。",
			SessionID: "pks-session-test",
			Created:   true,
		}, nil
	}

	tool := NewAskPocKnowledgeTool(nil)
	resp := tool.Execute(map[string]interface{}{
		"question": "需要 Shiro 相关漏洞和可利用线索",
	})

	var payload struct {
		Success bool                   `json:"success"`
		Error   string                 `json:"error"`
		Result  map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success, got error: %s", payload.Error)
	}
	if payload.Result["answer"] != "结论：命中 Shiro rememberMe 反序列化资料。" {
		t.Fatalf("unexpected answer payload: %+v", payload.Result)
	}
	if payload.Result["session_id"] != "pks-session-test" {
		t.Fatalf("expected session_id in payload, got %+v", payload.Result)
	}
	if payload.Result["created"] != true {
		t.Fatalf("expected created=true, got %+v", payload.Result)
	}
	if payload.Result["destroyed"] != false {
		t.Fatalf("expected destroyed=false, got %+v", payload.Result)
	}
	if payload.Result["session_active"] != true {
		t.Fatalf("expected session_active=true, got %+v", payload.Result)
	}

	logBody, err := os.ReadFile(qLogPath)
	if err != nil {
		t.Fatalf("read q.log: %v", err)
	}
	logText := string(logBody)
	if !strings.Contains(logText, "\"tool\":\"AskPocKnowledgeTool\"") {
		t.Fatalf("expected AskPocKnowledgeTool log entry, got: %s", logText)
	}
	if !strings.Contains(logText, "需要 Shiro 相关漏洞和可利用线索") {
		t.Fatalf("expected question in q.log, got: %s", logText)
	}
	if !strings.Contains(logText, "结论：命中 Shiro rememberMe 反序列化资料。") {
		t.Fatalf("expected answer in q.log, got: %s", logText)
	}
}

func TestAskPocKnowledgeToolFallsBackToDirectSearchSummary(t *testing.T) {
	oldRunner := runPocKnowledgeQuery
	oldFallback := runPocKnowledgeFallback
	t.Cleanup(func() {
		runPocKnowledgeQuery = oldRunner
		runPocKnowledgeFallback = oldFallback
	})

	runPocKnowledgeQuery = func(_ *taskManager.Task, req pocKnowledgeRequest) (pocKnowledgeResult, error) {
		if req.Question != "PHPShe CMS RCE SQL injection file upload vulnerability CVE exploit POC getshell" {
			t.Fatalf("unexpected question: %q", req.Question)
		}
		return pocKnowledgeResult{}, context.DeadlineExceeded
	}
	runPocKnowledgeFallback = func(question string) (string, error) {
		if !strings.Contains(question, "PHPShe") {
			t.Fatalf("unexpected fallback question: %q", question)
		}
		return "结论：兜底检索已返回本地知识库命中摘要。", nil
	}

	tool := NewAskPocKnowledgeTool(nil)
	resp := tool.Execute(map[string]interface{}{
		"question": "PHPShe CMS RCE SQL injection file upload vulnerability CVE exploit POC getshell",
	})

	var payload struct {
		Success bool                   `json:"success"`
		Error   string                 `json:"error"`
		Result  map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success, got error: %s", payload.Error)
	}
	if payload.Result["fallback_mode"] != "direct_search" {
		t.Fatalf("expected direct_search fallback, got %+v", payload.Result)
	}
	if payload.Result["answer"] != "结论：兜底检索已返回本地知识库命中摘要。" {
		t.Fatalf("unexpected fallback answer payload: %+v", payload.Result)
	}
}

type fakePocKnowledgeResponder struct {
	name      string
	questions []string
}

func (f *fakePocKnowledgeResponder) answer(question string) (string, error) {
	f.questions = append(f.questions, question)
	return fmt.Sprintf("%s:%d:%s", f.name, len(f.questions), question), nil
}

func TestRunPocKnowledgeAgentReusesSession(t *testing.T) {
	oldAnswerFunc := pocKnowledgeAnswerFunc
	oldSessionStore := pocKnowledgeSessions
	oldResponderFactory := newPocKnowledgeResponder
	oldQLogPath := qLogPath
	t.Cleanup(func() {
		pocKnowledgeAnswerFunc = oldAnswerFunc
		pocKnowledgeSessions = oldSessionStore
		newPocKnowledgeResponder = oldResponderFactory
		qLogPath = oldQLogPath
	})

	qLogPath = filepath.Join(t.TempDir(), "q.log")
	pocKnowledgeSessions = newPocKnowledgeSessionStore(time.Hour, time.Now)
	pocKnowledgeAnswerFunc = defaultPocKnowledgeAnswer

	var responders []*fakePocKnowledgeResponder
	newPocKnowledgeResponder = func(_ *taskManager.Task) pocKnowledgeResponder {
		responder := &fakePocKnowledgeResponder{
			name: fmt.Sprintf("responder-%d", len(responders)+1),
		}
		responders = append(responders, responder)
		return responder
	}

	first, err := runPocKnowledgeAgent(nil, pocKnowledgeRequest{Question: "第一问"})
	if err != nil {
		t.Fatalf("first run returned error: %v", err)
	}
	if !first.Created {
		t.Fatalf("expected first result to create a session, got %+v", first)
	}
	if first.SessionID == "" {
		t.Fatalf("expected session id in first result")
	}

	second, err := runPocKnowledgeAgent(nil, pocKnowledgeRequest{
		SessionID: first.SessionID,
		Question:  "第二问",
	})
	if err != nil {
		t.Fatalf("second run returned error: %v", err)
	}
	if second.Created {
		t.Fatalf("expected second result to reuse the session, got %+v", second)
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("expected reused session id, first=%q second=%q", first.SessionID, second.SessionID)
	}
	if len(responders) != 1 {
		t.Fatalf("expected exactly one responder, got %d", len(responders))
	}
	if got := strings.Join(responders[0].questions, "|"); got != "第一问|第二问" {
		t.Fatalf("expected both questions on the same responder, got %q", got)
	}

	closeResult, err := runPocKnowledgeAgent(nil, pocKnowledgeRequest{
		SessionID:    first.SessionID,
		Question:     "第三问",
		CloseSession: true,
	})
	if err != nil {
		t.Fatalf("close run returned error: %v", err)
	}
	if !closeResult.Closed {
		t.Fatalf("expected session to close, got %+v", closeResult)
	}
	if len(responders[0].questions) != 3 {
		t.Fatalf("expected third question before close, got %+v", responders[0].questions)
	}

	_, err = runPocKnowledgeAgent(nil, pocKnowledgeRequest{
		SessionID: first.SessionID,
		Question:  "第四问",
	})
	if err == nil || !strings.Contains(err.Error(), "not found or expired") {
		t.Fatalf("expected not found or expired error after close, got %v", err)
	}
}

func TestRunPocKnowledgeAgentIsolatesByScope(t *testing.T) {
	oldAnswerFunc := pocKnowledgeAnswerFunc
	oldSessionStore := pocKnowledgeSessions
	oldResponderFactory := newPocKnowledgeResponder
	t.Cleanup(func() {
		pocKnowledgeAnswerFunc = oldAnswerFunc
		pocKnowledgeSessions = oldSessionStore
		newPocKnowledgeResponder = oldResponderFactory
	})

	pocKnowledgeSessions = newPocKnowledgeSessionStore(time.Hour, time.Now)
	pocKnowledgeAnswerFunc = defaultPocKnowledgeAnswer
	newPocKnowledgeResponder = func(_ *taskManager.Task) pocKnowledgeResponder {
		return &fakePocKnowledgeResponder{name: "isolated"}
	}

	first, err := runPocKnowledgeAgent(nil, pocKnowledgeRequest{
		Question: "caller-a-1",
		Scope:    "caller-a",
	})
	if err != nil {
		t.Fatalf("first run returned error: %v", err)
	}

	_, err = runPocKnowledgeAgent(nil, pocKnowledgeRequest{
		SessionID: first.SessionID,
		Question:  "caller-b-1",
		Scope:     "caller-b",
	})
	if err == nil || !strings.Contains(err.Error(), "not found or expired") {
		t.Fatalf("expected scope isolation error, got %v", err)
	}
}

func TestNewPocKnowledgeAgentInitializesTaskList(t *testing.T) {
	agent := newPocKnowledgeAgent(nil)

	msgList := agent.memory.GetContext(agent.contextID)
	if msgList == nil {
		t.Fatalf("expected initialized context")
	}
	if len(msgList) < 2 {
		t.Fatalf("expected system and task messages, got %d", len(msgList))
	}
	if !strings.Contains(msgList[1].Content, "PoC / exploit knowledge question") {
		t.Fatalf("unexpected task prompt: %q", msgList[1].Content)
	}
}

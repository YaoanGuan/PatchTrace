package toolCalling

import (
	"AIxVuln/taskManager"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunToolUsageGuideAgentWritesQLog(t *testing.T) {
	oldAnswerFunc := toolUsageGuideAnswerFunc
	oldQLogPath := qLogPath
	t.Cleanup(func() {
		toolUsageGuideAnswerFunc = oldAnswerFunc
		qLogPath = oldQLogPath
	})
	qLogPath = filepath.Join(t.TempDir(), "q.log")
	toolUsageGuideAnswerFunc = func(_ *taskManager.Task, req toolUsageGuideRequest) (toolUsageGuideResult, error) {
		if req.Question != "jadx MCP 怎么传 aix-mcp 路径？" {
			t.Fatalf("unexpected question: %q", req.Question)
		}
		return toolUsageGuideResult{
			Answer:    "使用 aix-mcp://xxx.apk 作为文件参数。",
			SessionID: "tool-usage-guide-session-test",
			Created:   true,
		}, nil
	}

	result, err := runToolUsageGuideAgent(nil, toolUsageGuideRequest{Question: "jadx MCP 怎么传 aix-mcp 路径？"})
	if err != nil {
		t.Fatalf("runToolUsageGuideAgent returned error: %v", err)
	}
	if result.Answer != "使用 aix-mcp://xxx.apk 作为文件参数。" {
		t.Fatalf("unexpected answer: %q", result.Answer)
	}

	logBody, err := os.ReadFile(qLogPath)
	if err != nil {
		t.Fatalf("read q.log: %v", err)
	}
	logText := string(logBody)
	if !strings.Contains(logText, "\"tool\":\"runToolUsageGuideAgent\"") {
		t.Fatalf("expected runToolUsageGuideAgent log entry, got: %s", logText)
	}
	if !strings.Contains(logText, "jadx MCP 怎么传 aix-mcp 路径？") {
		t.Fatalf("expected question in q.log, got: %s", logText)
	}
	if !strings.Contains(logText, "使用 aix-mcp://xxx.apk 作为文件参数。") {
		t.Fatalf("expected answer in q.log, got: %s", logText)
	}
}

type fakeToolUsageGuideResponder struct {
	name      string
	questions []string
}

func (f *fakeToolUsageGuideResponder) answer(question string) (string, error) {
	f.questions = append(f.questions, question)
	return fmt.Sprintf("%s:%d:%s", f.name, len(f.questions), question), nil
}

func TestRunToolUsageGuideAgentReusesSession(t *testing.T) {
	oldAnswerFunc := toolUsageGuideAnswerFunc
	oldSessionStore := toolUsageGuideSessions
	oldResponderFactory := newToolUsageGuideResponder
	oldQLogPath := qLogPath
	t.Cleanup(func() {
		toolUsageGuideAnswerFunc = oldAnswerFunc
		toolUsageGuideSessions = oldSessionStore
		newToolUsageGuideResponder = oldResponderFactory
		qLogPath = oldQLogPath
	})

	qLogPath = filepath.Join(t.TempDir(), "q.log")
	toolUsageGuideSessions = newToolUsageGuideSessionStore(time.Hour, time.Now)
	toolUsageGuideAnswerFunc = defaultToolUsageGuideAnswer

	var responders []*fakeToolUsageGuideResponder
	newToolUsageGuideResponder = func(_ *taskManager.Task) toolUsageGuideResponder {
		responder := &fakeToolUsageGuideResponder{
			name: fmt.Sprintf("responder-%d", len(responders)+1),
		}
		responders = append(responders, responder)
		return responder
	}

	first, err := runToolUsageGuideAgent(nil, toolUsageGuideRequest{Question: "第一问"})
	if err != nil {
		t.Fatalf("first run returned error: %v", err)
	}
	if !first.Created {
		t.Fatalf("expected first result to create a session, got %+v", first)
	}
	if first.SessionID == "" {
		t.Fatalf("expected session id in first result")
	}
	if first.Closed {
		t.Fatalf("expected first session to remain open, got %+v", first)
	}

	second, err := runToolUsageGuideAgent(nil, toolUsageGuideRequest{
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

	closeResult, err := runToolUsageGuideAgent(nil, toolUsageGuideRequest{
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

	_, err = runToolUsageGuideAgent(nil, toolUsageGuideRequest{
		SessionID: first.SessionID,
		Question:  "第四问",
	})
	if err == nil || !strings.Contains(err.Error(), "not found or expired") {
		t.Fatalf("expected not found or expired error after close, got %v", err)
	}
}

func TestRunToolUsageGuideAgentIsolatesByScope(t *testing.T) {
	oldAnswerFunc := toolUsageGuideAnswerFunc
	oldSessionStore := toolUsageGuideSessions
	oldResponderFactory := newToolUsageGuideResponder
	t.Cleanup(func() {
		toolUsageGuideAnswerFunc = oldAnswerFunc
		toolUsageGuideSessions = oldSessionStore
		newToolUsageGuideResponder = oldResponderFactory
	})

	toolUsageGuideSessions = newToolUsageGuideSessionStore(time.Hour, time.Now)
	toolUsageGuideAnswerFunc = defaultToolUsageGuideAnswer
	newToolUsageGuideResponder = func(_ *taskManager.Task) toolUsageGuideResponder {
		return &fakeToolUsageGuideResponder{name: "isolated"}
	}

	first, err := runToolUsageGuideAgent(nil, toolUsageGuideRequest{
		Question: "caller-a-1",
		Scope:    "caller-a",
	})
	if err != nil {
		t.Fatalf("first run returned error: %v", err)
	}

	_, err = runToolUsageGuideAgent(nil, toolUsageGuideRequest{
		SessionID: first.SessionID,
		Question:  "caller-b-1",
		Scope:     "caller-b",
	})
	if err == nil || !strings.Contains(err.Error(), "not found or expired") {
		t.Fatalf("expected scope isolation error, got %v", err)
	}
}

func TestNewToolUsageGuideAgentInitializesTaskList(t *testing.T) {
	agent := newToolUsageGuideAgent(nil)

	msgList := agent.memory.GetContext(agent.contextID)
	if msgList == nil {
		t.Fatalf("expected initialized context")
	}
	if len(msgList) < 2 {
		t.Fatalf("expected system and task messages, got %d", len(msgList))
	}
	if !strings.Contains(msgList[1].Content, "tool-usage question") {
		t.Fatalf("unexpected task prompt: %q", msgList[1].Content)
	}
}

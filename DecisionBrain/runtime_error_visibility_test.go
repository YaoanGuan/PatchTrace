package DecisionBrain

import (
	"AIxVuln/agents"
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type runtimeTestAgent struct {
	id      string
	state   string
	profile agents.AgentProfile
	task    *taskManager.Task
	memory  llm.Memory
}

func (a *runtimeTestAgent) Name() string { return "runtime-test-agent" }
func (a *runtimeTestAgent) StartTask(ctx context.Context) *agents.StartResp {
	return &agents.StartResp{}
}
func (a *runtimeTestAgent) AssignTask(assignment agents.TaskAssignment) {}
func (a *runtimeTestAgent) GetMemory() llm.Memory                       { return a.memory }
func (a *runtimeTestAgent) SetMemory(mem llm.Memory)                    { a.memory = mem }
func (a *runtimeTestAgent) GetTask() *taskManager.Task                  { return a.task }
func (a *runtimeTestAgent) SetKeyMessage(k map[string][]interface{})    {}
func (a *runtimeTestAgent) GetId() string                               { return a.id }
func (a *runtimeTestAgent) SetId(id string)                             { a.id = id }
func (a *runtimeTestAgent) GetState() string                            { return a.state }
func (a *runtimeTestAgent) GetProfile() agents.AgentProfile             { return a.profile }
func (a *runtimeTestAgent) SetProfile(p agents.AgentProfile)            { a.profile = p }
func (a *runtimeTestAgent) SetEnvInfo(map[string]interface{})           {}
func (a *runtimeTestAgent) SetStateHandler(func(string))                {}
func (a *runtimeTestAgent) SetState(state string)                       { a.state = state }

func TestAgentRuntimeGetRunInfoUsesErrorFallback(t *testing.T) {
	task := &taskManager.Task{}
	task.SetTaskList([]map[string]string{{"TaskContent": "inspect target"}})
	agent := &runtimeTestAgent{
		id:    "agent-1",
		state: "Error",
		task:  task,
	}
	runtime := &AgentRuntime{
		agent:     agent,
		Resp:      &agents.StartResp{Err: errors.New("llm request failed")},
		startedAt: time.Unix(1710000000, 0),
		done:      true,
	}

	info := runtime.GetRunInfo()
	if got := info["RUNSummary"]; got != "失败：llm request failed" {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if got := info["RUNError"]; got != "llm request failed" {
		t.Fatalf("unexpected error: %#v", got)
	}
}

func TestAgentRunDonePublishesErrorAndMarksRuntimeState(t *testing.T) {
	agent := &runtimeTestAgent{
		id:    "agent-2",
		state: "Running",
	}
	ws := make(chan string, 4)
	db := &DecisionBrain{
		projectName:       "proj",
		webOutputChan:     ws,
		runAgentList:      []*AgentRuntime{{agent: agent, Resp: &agents.StartResp{}, startedAt: time.Now()}},
		memory:            NewBrainMemory(),
		lastSummary:       make(map[string]string),
		lastTask:          make(map[string]string),
		lastError:         make(map[string]string),
		digitalHumanPool:  make(map[string][]digitalHumanEntry),
		digitalHumanBusy:  make(map[string]digitalHumanBusyInfo),
		digitalHumanQueue: make(map[string][]digitalHumanQueuedReq),
	}

	db.AgentRunDone(agent.GetId(), &agents.StartResp{Err: errors.New("context limit recovery failed")})

	if got := agent.GetState(); got != "Error" {
		t.Fatalf("expected agent state Error, got %q", got)
	}

	raw := <-ws
	var msg WebMsg
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal websocket msg: %v", err)
	}
	if msg.Type != "AgentDone" {
		t.Fatalf("unexpected websocket type: %s", msg.Type)
	}
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected websocket payload type: %T", msg.Data)
	}
	if got := data["Resp"]; got != "失败：context limit recovery failed" {
		t.Fatalf("unexpected websocket summary: %#v", got)
	}
	if got := data["Error"]; got != "context limit recovery failed" {
		t.Fatalf("unexpected websocket error: %#v", got)
	}

	runs := db.GetAgentRuntimeList()
	if len(runs) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(runs))
	}
	if got := runs[0]["RunState"]; got != "Error" {
		t.Fatalf("unexpected run state: %#v", got)
	}
	if got := runs[0]["RUNError"]; got != "context limit recovery failed" {
		t.Fatalf("unexpected run error: %#v", got)
	}
}

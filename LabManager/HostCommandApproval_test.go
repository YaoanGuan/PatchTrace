package LabManager

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHostCommandApprovalApproveAndRun(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "workdir")
	sourceDir := filepath.Join(workDir, "sourceCodeDir")
	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	manager := newHostCommandApprovalManager("lab-test", func() string { return workDir }, func() string { return sourceDir }, func() string { return projectDir }, nil)
	resultCh := make(chan taskManager.HostCommandResult, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), taskManager.TaskActor{
			PersonaName:   "顾凌川",
			GroupID:       binaryGroupID,
			GroupName:     binaryGroupName,
			AgentToolName: "Agent-Binary-ReverseCommonAgent",
		}, taskManager.HostCommandRequest{
			Command: "printf approved",
			Reason:  "need host test",
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	var requestID string
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot()
		if len(snapshot) > 0 {
			requestID = snapshot[0].ID
			if snapshot[0].Status != "pending_approval" {
				t.Fatalf("expected pending_approval, got %s", snapshot[0].Status)
			}
			if snapshot[0].Workdir != workDir {
				t.Fatalf("expected snapshot workdir %q, got %q", workDir, snapshot[0].Workdir)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("approval request was not created")
	}
	if err := manager.Decide(requestID, true, "允许执行", "用户"); err != nil {
		t.Fatalf("approve request: %v", err)
	}
	select {
	case result := <-resultCh:
		if result.Status != "completed" {
			t.Fatalf("expected completed, got %s", result.Status)
		}
		if result.Workdir != workDir {
			t.Fatalf("expected result workdir %q, got %q", workDir, result.Workdir)
		}
		if result.Output != "approved" {
			t.Fatalf("unexpected output: %q", result.Output)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for approved request result")
	}
}

func TestHostCommandApprovalReject(t *testing.T) {
	tmp := t.TempDir()
	manager := newHostCommandApprovalManager("lab-test", func() string { return tmp }, func() string { return filepath.Join(tmp, "sourceCodeDir") }, func() string { return tmp }, nil)
	resultCh := make(chan taskManager.HostCommandResult, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), taskManager.TaskActor{
			PersonaName: "温舒然",
			GroupName:   webGroupName,
		}, taskManager.HostCommandRequest{
			Command: "printf should-not-run",
			Reason:  "need host test",
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	var requestID string
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot()
		if len(snapshot) > 0 {
			requestID = snapshot[0].ID
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("approval request was not created")
	}
	if err := manager.Decide(requestID, false, "不允许执行", "用户"); err != nil {
		t.Fatalf("reject request: %v", err)
	}
	select {
	case result := <-resultCh:
		if result.Status != "rejected" {
			t.Fatalf("expected rejected, got %s", result.Status)
		}
		if result.Approved {
			t.Fatal("rejected request should not be approved")
		}
		if result.Error == "" {
			t.Fatal("expected rejection error message")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rejected request result")
	}
}

func TestHostCommandApprovalMultiplePendingRequests(t *testing.T) {
	tmp := t.TempDir()
	manager := newHostCommandApprovalManager("lab-test", func() string { return tmp }, func() string { return filepath.Join(tmp, "sourceCodeDir") }, func() string { return tmp }, nil)
	resultCh := make(chan taskManager.HostCommandResult, 2)

	go func() {
		resultCh <- manager.Request(context.Background(), taskManager.TaskActor{
			PersonaName:   "甲",
			GroupName:     binaryGroupName,
			AgentToolName: "Agent-Binary-ReverseCommonAgent",
		}, taskManager.HostCommandRequest{
			Command: "printf first",
			Reason:  "need host test #1",
		})
	}()
	go func() {
		resultCh <- manager.Request(context.Background(), taskManager.TaskActor{
			PersonaName:   "乙",
			GroupName:     webGroupName,
			AgentToolName: "Agent-General-GeneralCommonAgent",
		}, taskManager.HostCommandRequest{
			Command: "printf second",
			Reason:  "need host test #2",
		})
	}()

	var pending []hostCommandApprovalRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot()
		if len(snapshot) >= 2 {
			pending = snapshot[:2]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending approvals, got %d", len(pending))
	}
	for _, item := range pending {
		if item.Status != "pending_approval" {
			t.Fatalf("expected pending_approval, got %s", item.Status)
		}
	}

	if err := manager.Decide(pending[0].ID, true, "允许 #1", "用户"); err != nil {
		t.Fatalf("approve first request: %v", err)
	}
	if err := manager.Decide(pending[1].ID, false, "拒绝 #2", "用户"); err != nil {
		t.Fatalf("reject second request: %v", err)
	}

	gotCompleted := false
	gotRejected := false
	timeout := time.After(3 * time.Second)
	for !(gotCompleted && gotRejected) {
		select {
		case result := <-resultCh:
			switch result.Status {
			case "completed":
				gotCompleted = true
			case "rejected":
				gotRejected = true
			default:
				t.Fatalf("unexpected result status: %s", result.Status)
			}
		case <-timeout:
			t.Fatal("timed out waiting for multiple request results")
		}
	}
}

func TestHostCommandApprovalCancelledWhilePending(t *testing.T) {
	tmp := t.TempDir()
	manager := newHostCommandApprovalManager("lab-test", func() string { return tmp }, func() string { return filepath.Join(tmp, "sourceCodeDir") }, func() string { return tmp }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan taskManager.HostCommandResult, 1)

	go func() {
		resultCh <- manager.Request(ctx, taskManager.TaskActor{
			PersonaName:   "丙",
			GroupName:     binaryGroupName,
			AgentToolName: "Agent-Binary-AndroidCommonAgent",
		}, taskManager.HostCommandRequest{
			Command: "printf never-run",
			Reason:  "need host test cancel",
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	var requestID string
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot()
		if len(snapshot) > 0 {
			requestID = snapshot[0].ID
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("approval request was not created")
	}

	cancel()

	select {
	case result := <-resultCh:
		if result.Status != "cancelled" {
			t.Fatalf("expected cancelled, got %s", result.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cancelled request result")
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot()
		if len(snapshot) > 0 && snapshot[0].ID == requestID {
			if snapshot[0].Status != "cancelled" {
				t.Fatalf("expected cancelled snapshot status, got %s", snapshot[0].Status)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cancelled approval request not found in snapshot")
}

func TestHostCommandApprovalAutoApproveByConfig(t *testing.T) {
	oldConfig := misc.GetConfigValueDefault("misc", "AUTO_MODE", "")
	misc.ClearRuntimeAutomationMode()
	if err := misc.SetConfigValue("misc", "AUTO_MODE", misc.AutomationModeFull); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "AUTO_MODE", oldConfig)
		misc.ClearRuntimeAutomationMode()
	})

	tmp := t.TempDir()
	manager := newHostCommandApprovalManager("lab-test", func() string { return tmp }, func() string { return filepath.Join(tmp, "sourceCodeDir") }, func() string { return tmp }, nil)
	result := manager.Request(context.Background(), taskManager.TaskActor{
		PersonaName:   "自动批准测试",
		GroupName:     binaryGroupName,
		AgentToolName: "Agent-Binary-ReverseCommonAgent",
	}, taskManager.HostCommandRequest{
		Command: "printf auto-approved",
		Reason:  "need host test auto mode",
	})
	if result.Status != "completed" {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if result.DecisionNote == "" || result.DecisionNote != "全自动模式已自动批准宿主机命令执行（设置项）" {
		t.Fatalf("expected auto decision note, got %+v", result)
	}
	if result.Output != "auto-approved" {
		t.Fatalf("unexpected output: %+v", result)
	}
	snapshot := manager.Snapshot()
	if len(snapshot) == 0 {
		t.Fatal("expected approval snapshot item")
	}
	if snapshot[0].Reviewer != "系统·全自动模式" {
		t.Fatalf("expected auto reviewer, got %+v", snapshot[0])
	}
	if snapshot[0].DecisionNote != "全自动模式已自动批准宿主机命令执行（设置项）" {
		t.Fatalf("expected auto decision note in snapshot, got %+v", snapshot[0])
	}
	if snapshot[0].Status == "pending_approval" {
		t.Fatalf("expected no pending approval in full auto mode, got %+v", snapshot[0])
	}
}

func TestDecisionApprovalAutoApproveByFullMode(t *testing.T) {
	oldConfig := misc.GetConfigValueDefault("misc", "AUTO_MODE", "")
	misc.ClearRuntimeAutomationMode()
	if err := misc.SetConfigValue("misc", "AUTO_MODE", misc.AutomationModeFull); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "AUTO_MODE", oldConfig)
		misc.ClearRuntimeAutomationMode()
	})

	tmp := t.TempDir()
	manager := newHostCommandApprovalManager("lab-test", func() string { return tmp }, func() string { return filepath.Join(tmp, "sourceCodeDir") }, func() string { return tmp }, nil)
	result := manager.RequestDecision(context.Background(), taskManager.TaskActor{
		PersonaName:   "自动批准测试",
		GroupName:     pentestGroupName,
		AgentToolName: "Agent-Pentest-PentestCommonAgent",
	}, taskManager.DecisionApprovalRequest{
		Kind:   taskManager.DecisionApprovalKindTargetAction,
		Title:  "高风险目标操作审批",
		Target: "target.example",
		Action: "执行一次会修改数据的验证动作",
		Reason: "确认漏洞影响",
		Impact: "可能修改目标数据",
	})
	if result.Status != "approved" || !result.Approved {
		t.Fatalf("expected auto-approved decision, got %+v", result)
	}
	if result.DecisionNote != "全自动模式已自动批准该决策事项（设置项）" {
		t.Fatalf("expected generic auto decision note, got %+v", result)
	}
	snapshot := manager.Snapshot()
	if len(snapshot) == 0 {
		t.Fatal("expected decision snapshot item")
	}
	if snapshot[0].Status == "pending_approval" || snapshot[0].Reviewer != "系统·全自动模式" {
		t.Fatalf("expected auto-approved snapshot, got %+v", snapshot[0])
	}
	if snapshot[0].Kind != taskManager.DecisionApprovalKindTargetAction || snapshot[0].Action == "" || snapshot[0].Impact == "" {
		t.Fatalf("expected generic decision fields in snapshot, got %+v", snapshot[0])
	}
}

func TestAbandonIdeaDecisionAutoAbandonsByFullMode(t *testing.T) {
	oldConfig := misc.GetConfigValueDefault("misc", "AUTO_MODE", "")
	misc.ClearRuntimeAutomationMode()
	if err := misc.SetConfigValue("misc", "AUTO_MODE", misc.AutomationModeFull); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "AUTO_MODE", oldConfig)
		misc.ClearRuntimeAutomationMode()
	})

	tmp := t.TempDir()
	manager := newHostCommandApprovalManager("lab-test", func() string { return tmp }, func() string { return filepath.Join(tmp, "sourceCodeDir") }, func() string { return tmp }, nil)
	result := manager.RequestDecision(context.Background(), taskManager.TaskActor{
		PersonaName:   "自动放弃测试",
		GroupName:     pentestGroupName,
		AgentToolName: "Agent-Pentest-PentestCommonAgent",
	}, taskManager.DecisionApprovalRequest{
		Kind:   taskManager.DecisionApprovalKindAbandonIdea,
		Title:  "是否放弃 SSRF 思路",
		Target: "target.example",
		Action: "SSRF 到云元数据接口",
		Reason: "已尝试多个入口和协议绕过，均无响应证据",
		Impact: "可能错过 SSRF 攻击面",
	})
	if result.Status != "approved" || !result.Approved {
		t.Fatalf("expected auto-abandoned decision, got %+v", result)
	}
	if result.DecisionNote != "全自动模式已自动放弃该思路（设置项）" {
		t.Fatalf("expected abandon auto decision note, got %+v", result)
	}
	snapshot := manager.Snapshot()
	if len(snapshot) == 0 || snapshot[0].Kind != taskManager.DecisionApprovalKindAbandonIdea {
		t.Fatalf("expected abandon idea snapshot, got %+v", snapshot)
	}
}

func TestHostCommandApprovalSnapshotKeepsActiveAndLastTwentyHistory(t *testing.T) {
	manager := newHostCommandApprovalManager("lab-test", func() string { return t.TempDir() }, nil, nil, nil)
	manager.mu.Lock()
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("history-%02d", i)
		manager.items[id] = &hostCommandApprovalState{record: hostCommandApprovalRecord{
			ID:        id,
			Kind:      taskManager.DecisionApprovalKindTargetAction,
			Title:     "历史决策",
			Action:    id,
			Reason:    "test",
			Status:    "approved",
			CreatedAt: fmt.Sprintf("2026-04-27 10:00:%02d", i),
			UpdatedAt: fmt.Sprintf("2026-04-27 10:00:%02d", i),
		}}
		manager.order = append(manager.order, id)
	}
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("active-%02d", i)
		manager.items[id] = &hostCommandApprovalState{record: hostCommandApprovalRecord{
			ID:        id,
			Kind:      taskManager.DecisionApprovalKindHostCommand,
			Title:     "待处理决策",
			Action:    id,
			Reason:    "test",
			Status:    "pending_approval",
			CreatedAt: fmt.Sprintf("2026-04-27 10:01:%02d", i),
			UpdatedAt: fmt.Sprintf("2026-04-27 10:01:%02d", i),
		}}
		manager.order = append(manager.order, id)
	}
	manager.compactHistoryLocked()
	manager.mu.Unlock()

	snapshot := manager.Snapshot()
	if len(snapshot) != maxDecisionHistoryItems+2 {
		t.Fatalf("expected active items plus last 20 history records, got %d", len(snapshot))
	}
	seen := map[string]bool{}
	for _, item := range snapshot {
		seen[item.ID] = true
	}
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("history-%02d", i)
		if seen[id] {
			t.Fatalf("old history item %s should be removed from snapshot", id)
		}
	}
	for i := 5; i < 25; i++ {
		id := fmt.Sprintf("history-%02d", i)
		if !seen[id] {
			t.Fatalf("recent history item %s should remain in snapshot", id)
		}
	}
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("active-%02d", i)
		if !seen[id] {
			t.Fatalf("active item %s should remain even beyond history window", id)
		}
	}
	manager.mu.RLock()
	stored := len(manager.order)
	manager.mu.RUnlock()
	if stored != maxDecisionHistoryItems+2 {
		t.Fatalf("expected compacted storage size %d, got %d", maxDecisionHistoryItems+2, stored)
	}
}

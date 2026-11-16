package LabManager

import (
	"AIxVuln/agents"
	"AIxVuln/dockerManager"
	"AIxVuln/taskManager"
	"testing"
	"time"
)

func prepareBinaryGroupTestRuntime(labName string) {
	taskManager.SetServiceManager(labName, &dockerManager.ServiceManager{})
	taskManager.SetGoroutineChan(labName, make(chan func(), 8))
}

func TestPrepareBrainForMessageLockedRestartsFinishedBrainWithoutDeadlock(t *testing.T) {
	bg := newBinarySecurityGroup("binary-test-lab", "test task", nil, nil)
	prepareBinaryGroupTestRuntime("binary-test-lab")
	bg.brain = bg.newDecisionBrain()
	bg.brain.RestoreBrainFinished(true)
	bg.brainFinished = true

	done := make(chan struct{})
	errCh := make(chan string, 1)

	go func() {
		bg.mu.Lock()
		defer bg.mu.Unlock()

		brain := bg.prepareBrainForMessageLocked()
		if brain == nil {
			errCh <- "prepareBrainForMessageLocked returned nil brain"
			return
		}
		if brain.IsBrainFinished() {
			errCh <- "decision brain should be resumable after prepare"
			return
		}
		if bg.brainFinished {
			errCh <- "binary group brainFinished flag should be cleared after prepare"
			return
		}
		close(done)
	}()

	select {
	case errMsg := <-errCh:
		t.Fatal(errMsg)
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("prepareBrainForMessageLocked deadlocked while resuming a finished brain")
	}
}

func TestBinaryGroupListMemoryDebugTargetsDoesNotCreateBrainAndIncludesFallbackMembers(t *testing.T) {
	bg := newBinarySecurityGroup("binary-debug-lab", "test task", nil, nil)
	prepareBinaryGroupTestRuntime("binary-debug-lab")
	bg.brainConfig.DigitalHumanPool = map[string][]agents.AgentProfile{}

	targets := bg.ListMemoryDebugTargets()
	if bg.brain != nil {
		t.Fatal("expected ListMemoryDebugTargets to avoid creating binary decision brain")
	}

	foundMembers := make(map[string]bool)
	for _, target := range targets {
		foundMembers[target.PersonaName] = true
	}
	for _, member := range bg.roster {
		if !foundMembers[member.PersonaName] {
			t.Fatalf("expected fallback debug target for member %s, got %+v", member.PersonaName, targets)
		}
	}
}

func TestBinaryGroupExportMemoryDebugCreatesBrainOnDemand(t *testing.T) {
	bg := newBinarySecurityGroup("binary-export-lab", "test task", nil, nil)
	prepareBinaryGroupTestRuntime("binary-export-lab")

	exported, err := bg.ExportMemoryDebug(binaryGroupID + "::decision_brain")
	if err != nil {
		t.Fatalf("ExportMemoryDebug returned error: %v", err)
	}
	if bg.brain == nil {
		t.Fatal("expected ExportMemoryDebug to create binary decision brain on demand")
	}
	if exported == nil {
		t.Fatal("expected non-nil export")
	}
	if exported.Target.TargetID != binaryGroupID+"::decision_brain" {
		t.Fatalf("unexpected target id: %s", exported.Target.TargetID)
	}
}

func TestBinaryGroupNotifyDecisionBrainEventDoesNotDeadlockOnWaitWake(t *testing.T) {
	bg := newBinarySecurityGroup("binary-notify-lab", "test task", nil, nil)
	prepareBinaryGroupTestRuntime("binary-notify-lab")
	bg.SetWaitObserver(func(bool) {
		_ = bg.IsRunning()
	})
	bg.brain = bg.newDecisionBrain()
	bg.brainRunning = true
	_ = bg.brain.WaitTool()

	done := make(chan struct{})
	go func() {
		bg.NotifyDecisionBrainEvent("wake up")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("NotifyDecisionBrainEvent deadlocked while the wait callback re-read binary group state")
	}
}

func TestBinaryGroupSnapshotIncludesDigitalHumans(t *testing.T) {
	bg := newBinarySecurityGroup("binary-snapshot-lab", "test task", nil, nil)
	prepareBinaryGroupTestRuntime("binary-snapshot-lab")

	snapshot := bg.Snapshot()
	roster, ok := snapshot["digitalHumans"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected digitalHumans roster map in snapshot, got %#v", snapshot["digitalHumans"])
	}
	if len(roster) == 0 {
		t.Fatalf("expected non-empty digitalHumans roster in snapshot, got %#v", roster)
	}
}

package LabManager

import (
	"strings"
	"testing"
)

func TestTotalBrainMemoryDebugTargetAndPersistence(t *testing.T) {
	labName := "lab-total-brain-memory-" + strings.ReplaceAll(t.Name(), "/", "-")
	prepareBinaryGroupTestRuntime(labName)

	cfg := LabConfig{
		LabID:               labName,
		LabName:             labName,
		SourceCodeDir:       t.TempDir(),
		SkipProjectOverview: true,
		TaskContent:         "test task",
	}
	lm, err := NewLabManager(cfg)
	if err != nil {
		t.Fatalf("NewLabManager returned error: %v", err)
	}

	foundTarget := false
	for _, target := range lm.GetMemoryDebugTargets() {
		if target.TargetID == totalBrainMemoryDebugTargetID() {
			foundTarget = true
			if !target.Available {
				t.Fatalf("expected total brain debug target to be available: %+v", target)
			}
		}
	}
	if !foundTarget {
		t.Fatalf("expected total brain debug target %q", totalBrainMemoryDebugTargetID())
	}

	lm.emitDecisionBrainMessage(lm.binaryGroup, "已完成阶段分析，等待总脑协调下一步。")

	exported, err := lm.ExportMemoryDebug(totalBrainMemoryDebugTargetID())
	if err != nil {
		t.Fatalf("ExportMemoryDebug returned error: %v", err)
	}
	if !strings.Contains(exported.Content, "已完成阶段分析") {
		t.Fatalf("expected exported total brain context to include decision-makers message, got: %s", exported.Content)
	}

	prepareBinaryGroupTestRuntime(labName)
	restored, err := NewLabManager(cfg)
	if err != nil {
		t.Fatalf("restored NewLabManager returned error: %v", err)
	}
	exported, err = restored.ExportMemoryDebug(totalBrainMemoryDebugTargetID())
	if err != nil {
		t.Fatalf("restored ExportMemoryDebug returned error: %v", err)
	}
	if !strings.Contains(exported.Content, "已完成阶段分析") {
		t.Fatalf("expected restored exported total brain context to include persisted message, got: %s", exported.Content)
	}
}

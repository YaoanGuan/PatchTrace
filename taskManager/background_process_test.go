package taskManager

import (
	"strings"
	"testing"
	"time"
)

func TestHostBackgroundProcessReadWriteAndExit(t *testing.T) {
	projectName := "bg-host-readwrite"
	workdir := t.TempDir()
	t.Cleanup(func() {
		CloseBackgroundProcesses(projectName)
	})

	snapshot, err := StartHostBackgroundProcess(BackgroundProcessStartOptions{
		ProjectName: projectName,
		OwnerTaskID: "task-1",
		Command:     "cat",
		Workdir:     workdir,
	})
	if err != nil {
		t.Fatalf("StartHostBackgroundProcess returned error: %v", err)
	}
	if snapshot.SessionID == "" {
		t.Fatal("expected session id")
	}
	if !strings.HasPrefix(snapshot.SessionID, "hst-") {
		t.Fatalf("expected short host session id, got %q", snapshot.SessionID)
	}
	if len(snapshot.SessionID) > 16 {
		t.Fatalf("expected short session id, got %q", snapshot.SessionID)
	}

	if _, err := WriteBackgroundProcess(projectName, snapshot.SessionID, "task-1", "hello-background\n", false); err != nil {
		t.Fatalf("WriteBackgroundProcess returned error: %v", err)
	}

	readResult, err := ReadBackgroundProcess(projectName, snapshot.SessionID[:8], "task-1", 0, 0, 16*1024, 2000)
	if err != nil {
		t.Fatalf("ReadBackgroundProcess returned error: %v", err)
	}
	if !strings.Contains(readResult.Stdout, "hello-background") {
		t.Fatalf("expected stdout to contain written content, got %q", readResult.Stdout)
	}
	if readResult.WaitEvent != "output" {
		t.Fatalf("expected wait_event=output, got %q", readResult.WaitEvent)
	}

	if _, err := WriteBackgroundProcess(projectName, snapshot.SessionID, "task-1", "", true); err != nil {
		t.Fatalf("closing stdin returned error: %v", err)
	}

	exitResult, err := ReadBackgroundProcess(projectName, snapshot.SessionID, "task-1", readResult.StdoutOffset, readResult.StderrOffset, 16*1024, 3000)
	if err != nil {
		t.Fatalf("waiting for process exit returned error: %v", err)
	}
	if exitResult.Session.Running {
		t.Fatalf("expected process to exit, status=%s", exitResult.Session.Status)
	}
	if exitResult.Session.Status != "completed" {
		t.Fatalf("expected completed status, got %s", exitResult.Session.Status)
	}
}

func TestHostBackgroundProcessUserTerminationVisible(t *testing.T) {
	projectName := "bg-host-user-kill"
	workdir := t.TempDir()
	t.Cleanup(func() {
		CloseBackgroundProcesses(projectName)
	})

	snapshot, err := StartHostBackgroundProcess(BackgroundProcessStartOptions{
		ProjectName: projectName,
		OwnerTaskID: "task-2",
		Command:     "sleep 30",
		Workdir:     workdir,
	})
	if err != nil {
		t.Fatalf("StartHostBackgroundProcess returned error: %v", err)
	}

	if _, err := TerminateBackgroundProcess(projectName, snapshot.SessionID, "task-2", "user", "killed by user test", "TERM"); err != nil {
		t.Fatalf("TerminateBackgroundProcess returned error: %v", err)
	}

	stateResult, err := ReadBackgroundProcess(projectName, snapshot.SessionID, "task-2", 0, 0, 16*1024, 1000)
	if err != nil {
		t.Fatalf("ReadBackgroundProcess after terminate returned error: %v", err)
	}
	if stateResult.Session.TerminatedBy != "user" {
		t.Fatalf("expected terminated_by=user, got %q", stateResult.Session.TerminatedBy)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		finalResult, err := ReadBackgroundProcess(projectName, snapshot.SessionID, "task-2", stateResult.StdoutOffset, stateResult.StderrOffset, 16*1024, 500)
		if err != nil {
			t.Fatalf("ReadBackgroundProcess waiting for final termination returned error: %v", err)
		}
		if !finalResult.Session.Running {
			if finalResult.Session.Status != "terminated" {
				t.Fatalf("expected terminated status, got %s", finalResult.Session.Status)
			}
			if finalResult.Session.TerminationReason == "" {
				t.Fatal("expected termination reason to be preserved")
			}
			return
		}
		stateResult = finalResult
	}
	t.Fatal("timed out waiting for terminated background process")
}

func TestHostBackgroundProcessCanBeControlledAcrossTasks(t *testing.T) {
	projectName := "bg-host-shared-across-tasks"
	workdir := t.TempDir()
	t.Cleanup(func() {
		CloseBackgroundProcesses(projectName)
	})

	snapshot, err := StartHostBackgroundProcess(BackgroundProcessStartOptions{
		ProjectName: projectName,
		OwnerTaskID: "task-a",
		Command:     "cat",
		Workdir:     workdir,
	})
	if err != nil {
		t.Fatalf("StartHostBackgroundProcess returned error: %v", err)
	}

	if _, err := WriteBackgroundProcess(projectName, snapshot.SessionID, "task-b", "shared-session\n", false); err != nil {
		t.Fatalf("WriteBackgroundProcess from another task returned error: %v", err)
	}

	readResult, err := ReadBackgroundProcess(projectName, snapshot.SessionID, "task-c", 0, 0, 16*1024, 2000)
	if err != nil {
		t.Fatalf("ReadBackgroundProcess from another task returned error: %v", err)
	}
	if !strings.Contains(readResult.Stdout, "shared-session") {
		t.Fatalf("expected shared stdout content, got %q", readResult.Stdout)
	}

	if _, err := WriteBackgroundProcess(projectName, snapshot.SessionID, "task-d", "", true); err != nil {
		t.Fatalf("closing stdin from another task returned error: %v", err)
	}

	exitResult, err := ReadBackgroundProcess(projectName, snapshot.SessionID, "task-e", readResult.StdoutOffset, readResult.StderrOffset, 16*1024, 3000)
	if err != nil {
		t.Fatalf("waiting for exit from another task returned error: %v", err)
	}
	if exitResult.Session.Running {
		t.Fatalf("expected shared process to exit, status=%s", exitResult.Session.Status)
	}
}

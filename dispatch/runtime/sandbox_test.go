package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeExecProject records the arguments a DockerSandboxRuntime passes to its
// injected exec primitive and returns canned output.
type fakeExecProject struct {
	containerID string
	command     string
	workdir     string
	timeout     time.Duration
	calls       int
}

func TestDockerSandboxRuntimeMapsExecResults(t *testing.T) {
	record := &fakeExecProject{}
	exec := func(_ context.Context, containerID, command, workdir string, timeout time.Duration) (string, string, int, error) {
		record.containerID = containerID
		record.command = command
		record.workdir = workdir
		record.timeout = timeout
		record.calls++
		return "out line\n", "err line\n", 3, nil
	}
	resolve := func(_ context.Context, projectID string) (string, error) {
		if projectID != "p1" {
			t.Fatalf("resolve project = %q, want p1", projectID)
		}
		return "container-1", nil
	}
	rt := NewDockerSandboxRuntime(exec, resolve)

	res, err := rt.Exec(context.Background(), "p1", ExecRequest{
		Command: "cat /tmp/poc",
		Workdir: "/workspace/src",
		Timeout: 7 * time.Second,
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if res.Stdout != "out line\n" || res.Stderr != "err line\n" || res.ExitCode != 3 || res.Err != nil {
		t.Fatalf("Exec result = %+v, want the injected stdout/stderr/exit code", res)
	}
	if record.calls != 1 || record.containerID != "container-1" ||
		record.command != "cat /tmp/poc" || record.workdir != "/workspace/src" ||
		record.timeout != 7*time.Second {
		t.Fatalf("exec call = %+v, want container-1/cat /tmp/poc//workspace/src/7s", record)
	}
}

func TestDockerSandboxRuntimeMapsExecError(t *testing.T) {
	wantErr := errors.New("docker attach failed")
	exec := func(context.Context, string, string, string, time.Duration) (string, string, int, error) {
		return "partial", "", -1, wantErr
	}
	resolve := func(context.Context, string) (string, error) { return "container-1", nil }
	rt := NewDockerSandboxRuntime(exec, resolve)

	res, err := rt.Exec(context.Background(), "p1", ExecRequest{Command: "true"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Exec err = %v, want %v", err, wantErr)
	}
	if !errors.Is(res.Err, wantErr) {
		t.Fatalf("Exec result Err = %v, want %v", res.Err, wantErr)
	}
	if res.ExitCode != -1 || res.Stdout != "partial" {
		t.Fatalf("Exec result = %+v, want the partial output and failure exit code", res)
	}
}

func TestDockerSandboxRuntimeAppliesDefaultTimeout(t *testing.T) {
	got := time.Duration(0)
	exec := func(_ context.Context, _, _, _ string, timeout time.Duration) (string, string, int, error) {
		got = timeout
		return "", "", 0, nil
	}
	resolve := func(context.Context, string) (string, error) { return "container-1", nil }
	rt := NewDockerSandboxRuntime(exec, resolve)

	if _, err := rt.Exec(context.Background(), "p1", ExecRequest{Command: "true"}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got != DefaultExecTimeout {
		t.Fatalf("exec timeout = %v, want the default %v", got, DefaultExecTimeout)
	}
}

func TestDockerSandboxRuntimeSurfacesResolveAndConfigErrors(t *testing.T) {
	t.Run("resolve error", func(t *testing.T) {
		called := false
		exec := func(context.Context, string, string, string, time.Duration) (string, string, int, error) {
			called = true
			return "", "", 0, nil
		}
		resolve := func(context.Context, string) (string, error) { return "", errors.New("project has no sandbox") }
		rt := NewDockerSandboxRuntime(exec, resolve)

		_, err := rt.Exec(context.Background(), "p1", ExecRequest{Command: "true"})
		if err == nil || !strings.Contains(err.Error(), "project has no sandbox") {
			t.Fatalf("Exec err = %v, want the resolver failure", err)
		}
		if called {
			t.Fatal("a resolver failure must not call exec")
		}
	})

	t.Run("empty container", func(t *testing.T) {
		resolve := func(context.Context, string) (string, error) { return "   ", nil }
		rt := NewDockerSandboxRuntime(func(context.Context, string, string, string, time.Duration) (string, string, int, error) {
			return "", "", 0, nil
		}, resolve)

		if _, err := rt.Exec(context.Background(), "p1", ExecRequest{Command: "true"}); err == nil {
			t.Fatal("an empty container id must be an error")
		}
	})

	t.Run("missing exec", func(t *testing.T) {
		rt := NewDockerSandboxRuntime(nil, func(context.Context, string) (string, error) { return "c", nil })
		if _, err := rt.Exec(context.Background(), "p1", ExecRequest{Command: "true"}); err == nil {
			t.Fatal("a runtime without an exec primitive must error")
		}
	})

	t.Run("missing resolver", func(t *testing.T) {
		rt := NewDockerSandboxRuntime(func(context.Context, string, string, string, time.Duration) (string, string, int, error) {
			return "", "", 0, nil
		}, nil)
		if _, err := rt.Exec(context.Background(), "p1", ExecRequest{Command: "true"}); err == nil {
			t.Fatal("a runtime without a container resolver must error")
		}
	})
}

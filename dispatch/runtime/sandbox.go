// Package runtime owns the sandbox execution boundary used by explore tasks.
// The interface keeps the task package independent of any concrete container
// implementation so unit tests can run fully offline.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DefaultExecTimeout bounds one sandbox command when the caller does not set
// ExecRequest.Timeout.
const DefaultExecTimeout = 2 * time.Minute

// ExecRequest is one command to run inside a project's sandbox.
type ExecRequest struct {
	Command string
	Workdir string
	Timeout time.Duration
}

// ExecResult is the observed outcome of one sandbox command. Err carries a
// transport-level failure (the command did not run) and is distinct from a
// non-zero ExitCode (the command ran and failed).
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// SandboxRuntime executes commands inside a project's sandbox.
type SandboxRuntime interface {
	Exec(ctx context.Context, projectID string, req ExecRequest) (ExecResult, error)
}

// ExecFunc is the low-level container exec primitive. dockerManager's
// (*DockerManager).ExecCapture has exactly this signature, so dispatcher wiring
// can inject the method value directly.
type ExecFunc func(ctx context.Context, containerID, command, workdir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)

// ContainerResolver maps a graph project id to the container that hosts its
// sandbox. It is injected so this package never imports taskManager or Web.
type ContainerResolver func(ctx context.Context, projectID string) (containerID string, err error)

// DockerSandboxRuntime is the Docker-backed SandboxRuntime seam. It resolves
// the project's container and runs the command through the injected exec
// primitive, preserving the caller context, workdir, demultiplexed streams and
// exit code.
type DockerSandboxRuntime struct {
	exec    ExecFunc
	resolve ContainerResolver
}

var _ SandboxRuntime = (*DockerSandboxRuntime)(nil)

// NewDockerSandboxRuntime builds a Docker-backed runtime from an exec primitive
// (typically (*dockerManager.DockerManager).ExecCapture) and a project ->
// container resolver.
func NewDockerSandboxRuntime(exec ExecFunc, resolve ContainerResolver) *DockerSandboxRuntime {
	return &DockerSandboxRuntime{exec: exec, resolve: resolve}
}

// Exec resolves the project's container, applies the default timeout when the
// request leaves it unset, and runs the command. A transport failure is
// returned both in ExecResult.Err and as the method error; a non-zero exit code
// is a successful Exec with ExitCode set.
func (r *DockerSandboxRuntime) Exec(ctx context.Context, projectID string, req ExecRequest) (ExecResult, error) {
	if r == nil || r.exec == nil {
		return ExecResult{ExitCode: -1}, errors.New("dispatch/runtime: docker sandbox runtime has no exec primitive")
	}
	if r.resolve == nil {
		return ExecResult{ExitCode: -1}, errors.New("dispatch/runtime: docker sandbox runtime has no container resolver")
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return ExecResult{ExitCode: -1}, errors.New("dispatch/runtime: sandbox command is empty")
	}
	containerID, err := r.resolve(ctx, projectID)
	if err != nil {
		return ExecResult{ExitCode: -1}, fmt.Errorf("dispatch/runtime: resolve sandbox for project %s: %w", projectID, err)
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return ExecResult{ExitCode: -1}, fmt.Errorf("dispatch/runtime: project %s has no sandbox container", projectID)
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultExecTimeout
	}
	stdout, stderr, exitCode, err := r.exec(ctx, containerID, command, strings.TrimSpace(req.Workdir), timeout)
	result := ExecResult{Stdout: stdout, Stderr: stderr, ExitCode: exitCode, Err: err}
	if err != nil {
		return result, fmt.Errorf("dispatch/runtime: sandbox exec in project %s: %w", projectID, err)
	}
	return result, nil
}

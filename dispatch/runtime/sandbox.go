// Package runtime owns the sandbox execution boundary used by explore tasks.
// The interface keeps the task package independent of any concrete container
// implementation so unit tests can run fully offline.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotImplemented is returned by the Docker-backed runtime: the current
// dockerManager exec surface cannot honour the ExecRequest contract without
// losing information (see DockerSandboxRuntime).
var ErrNotImplemented = errors.New("dispatch/runtime: docker sandbox runtime is not implemented")

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

// DockerSandboxRuntime is the Docker-backed SandboxRuntime seam.
//
// It deliberately reports ErrNotImplemented. A faithful implementation needs:
//   - a projectID -> container mapping (dockerManager only takes a container
//     ID; the registry lives in taskManager, outside this package's allowed
//     imports);
//   - caller context propagation (dockerManager.DockerExec derives its own
//     context.Background with a seconds-only timeout);
//   - a working directory (DockerExec has no ExecOptions.WorkingDir);
//   - separate stdout/stderr and an exit code (DockerExec runs Tty:true and
//     returns one merged string).
//
// Wrapping it would silently drop ctx cancellation, the workdir and the stream
// split, which is worse than failing loudly. Dispatcher wiring can supply any
// SandboxRuntime implementation that satisfies the interface.
type DockerSandboxRuntime struct{}

var _ SandboxRuntime = (*DockerSandboxRuntime)(nil)

// NewDockerSandboxRuntime returns the (currently unimplemented) Docker runtime.
func NewDockerSandboxRuntime() *DockerSandboxRuntime { return &DockerSandboxRuntime{} }

// Exec always returns ErrNotImplemented.
func (r *DockerSandboxRuntime) Exec(context.Context, string, ExecRequest) (ExecResult, error) {
	return ExecResult{}, fmt.Errorf("%w: dockerManager cannot exec by project with a context, workdir, split streams and an exit code", ErrNotImplemented)
}

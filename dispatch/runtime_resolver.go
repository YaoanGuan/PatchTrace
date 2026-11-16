package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dockerManager"
	"AIxVuln/taskManager"
)

// NewTaskManagerResolver returns the production project -> sandbox container
// resolver. taskManager holds the per-lab sandbox registry populated by the
// project lifecycle, and each Sandbox carries the Docker container id that
// runtime.DockerSandboxRuntime executes commands in.
func NewTaskManagerResolver() runtime.ContainerResolver {
	return containerResolver(taskManager.GetSandbox)
}

// containerResolver is the injectable form. It never returns an empty
// container id: a missing sandbox or a sandbox without a container is a clear
// error so explore runs fail loudly instead of executing nowhere.
func containerResolver(lookup func(string) (*dockerManager.Sandbox, error)) runtime.ContainerResolver {
	return func(ctx context.Context, projectID string) (string, error) {
		if lookup == nil {
			return "", errors.New("dispatch: container resolver has no sandbox lookup")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		sandbox, err := lookup(projectID)
		if err != nil {
			return "", fmt.Errorf("dispatch: resolve sandbox for project %s: %w", projectID, err)
		}
		if sandbox == nil {
			return "", fmt.Errorf("dispatch: project %s has no sandbox", projectID)
		}
		containerID := strings.TrimSpace(sandbox.ContainerId)
		if containerID == "" {
			return "", fmt.Errorf("dispatch: project %s sandbox has no container id", projectID)
		}
		return containerID, nil
	}
}

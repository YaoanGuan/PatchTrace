package dockerManager

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ExecCapture validates its inputs before touching the Docker daemon, so these
// unit tests stay fully offline (the manager has no client).
func TestExecCaptureRejectsInvalidInput(t *testing.T) {
	dm := &DockerManager{}
	cases := []struct {
		name        string
		containerID string
		command     string
		want        string
	}{
		{name: "empty container", containerID: "  ", command: "true", want: "container"},
		{name: "empty command", containerID: "cid", command: "   ", want: "command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := dm.ExecCapture(context.Background(), tc.containerID, tc.command, "", time.Second)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ExecCapture err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestExecCaptureRejectsNilManager(t *testing.T) {
	var dm *DockerManager
	if _, _, _, err := dm.ExecCapture(context.Background(), "cid", "true", "", time.Second); err == nil {
		t.Fatal("a nil manager must be an error")
	}
}

package dockerManager

import (
	"AIxVuln/misc"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type BackgroundExecHandle struct {
	ExecID string
	Resp   *types.HijackedResponse
}

func (dm *DockerManager) DockerExecBackground(containerID string, cmd []string) (*BackgroundExecHandle, error) {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := dm.cli.ContainerExecCreate(context.Background(), containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("创建后台 exec 失败: %w", err)
	}

	resp, err := dm.cli.ContainerExecAttach(context.Background(), execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("连接后台 exec 失败: %w", err)
	}

	return &BackgroundExecHandle{
		ExecID: execID.ID,
		Resp:   &resp,
	}, nil
}

func (dm *DockerManager) DockerExecInspect(execID string) (container.ExecInspect, error) {
	inspect, err := dm.cli.ContainerExecInspect(context.Background(), execID)
	if err != nil {
		return container.ExecInspect{}, fmt.Errorf("检查 exec 状态失败: %w", err)
	}
	return inspect, nil
}

func (dm *DockerManager) DockerKillPIDInContainer(containerID string, pid int, signal string) error {
	if strings.TrimSpace(containerID) == "" {
		return fmt.Errorf("empty container id")
	}
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	sig := normalizeKillSignal(signal)
	cmd := []string{"sh", "-lc", fmt.Sprintf("kill -%s -- -%d 2>/dev/null || kill -%s %d", sig, pid, sig, pid)}
	if _, err := dm.DockerExec(containerID, cmd, 10); err != nil {
		return err
	}
	return nil
}

func (s *Sandbox) StartBackgroundCommand(command string, workdir string) (*BackgroundExecHandle, error) {
	wrapped := wrapSandboxBackgroundCommand(command, workdir)
	return s.dm.DockerExecBackground(s.ContainerId, []string{"sh", "-lc", wrapped})
}

func (s *Sandbox) InspectBackgroundCommand(execID string) (container.ExecInspect, error) {
	return s.dm.DockerExecInspect(execID)
}

func (s *Sandbox) SignalBackgroundPID(pid int, signal string) error {
	return s.dm.DockerKillPIDInContainer(s.ContainerId, pid, signal)
}

func wrapSandboxBackgroundCommand(command string, workdir string) string {
	command = strings.TrimSpace(command)
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		workdir = misc.SandboxWorkDir
	}
	workdir = quoteShellSingle(workdir)
	quoted := quoteShellSingle(command)
	return "cd " + workdir + " && printf '__AIX_BG_PID__%s\\n' \"$$\"; exec sh -lc " + quoted
}

func quoteShellSingle(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", `'"'"'`) + "'"
}

func normalizeKillSignal(signal string) string {
	switch strings.ToLower(strings.TrimSpace(signal)) {
	case "kill", "sigkill", "9":
		return "KILL"
	case "int", "interrupt", "sigint", "2":
		return "INT"
	case "term", "terminate", "sigterm", "15", "":
		return "TERM"
	default:
		if _, err := strconv.Atoi(signal); err == nil {
			return signal
		}
		return "TERM"
	}
}

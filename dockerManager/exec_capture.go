package dockerManager

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// ExecCapture runs one shell command inside containerID and captures its
// demultiplexed stdout/stderr and exit code.
//
// Unlike DockerExec it honours the caller's context and an optional timeout,
// supports a working directory, and never merges the two streams. The second
// return value is nil when the command ran; a non-zero exit code is data, not
// an error. Transport failures (create/attach/read/inspect) are returned as
// errors with an exit code of -1.
func (dm *DockerManager) ExecCapture(ctx context.Context, containerID, command, workdir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if dm == nil {
		return "", "", -1, fmt.Errorf("执行 exec 失败: DockerManager 未初始化")
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return "", "", -1, fmt.Errorf("执行 exec 失败: container id 为空")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", "", -1, fmt.Errorf("执行 exec 失败: command 为空")
	}
	if dm.cli == nil {
		return "", "", -1, fmt.Errorf("执行 exec 失败: DockerManager 未初始化")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-lc", command},
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   strings.TrimSpace(workdir),
	}
	created, err := dm.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", "", -1, fmt.Errorf("创建 exec 失败: %w", err)
	}
	resp, err := dm.cli.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", "", -1, fmt.Errorf("连接 exec 失败: %w", err)
	}
	defer resp.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, resp.Reader)
		done <- copyErr
	}()

	select {
	case <-ctx.Done():
		// Unblock StdCopy before reading the buffers so the race detector sees
		// no concurrent access; the partial output is still returned.
		resp.Close()
		<-done
		return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("命令执行被取消或超时: %w", ctx.Err())
	case copyErr := <-done:
		if copyErr != nil {
			return "", "", -1, fmt.Errorf("读取 exec 输出失败: %w", copyErr)
		}
	}

	inspect, err := dm.cli.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("检查 exec 状态失败: %w", err)
	}
	return stdoutBuf.String(), stderrBuf.String(), inspect.ExitCode, nil
}

//go:build !windows

package taskManager

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

func backgroundCommandSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func signalBackgroundProcess(process *os.Process, signal string) error {
	if process == nil {
		return fmt.Errorf("process is nil")
	}
	sig := unixSignalFromString(signal)
	if err := syscall.Kill(-process.Pid, sig); err == nil {
		return nil
	} else if !isNoSuchProcessErr(err) {
		if err2 := process.Signal(sig); err2 == nil || isNoSuchProcessErr(err2) {
			return nil
		}
		return err
	}
	return nil
}

func unixSignalFromString(signal string) syscall.Signal {
	switch strings.ToLower(strings.TrimSpace(signal)) {
	case "", "term", "terminate", "sigterm", "15":
		return syscall.SIGTERM
	case "kill", "sigkill", "9":
		return syscall.SIGKILL
	case "int", "interrupt", "sigint", "2":
		return syscall.SIGINT
	default:
		return syscall.SIGTERM
	}
}

func isNoSuchProcessErr(err error) bool {
	if err == nil {
		return false
	}
	return err == syscall.ESRCH || strings.Contains(strings.ToLower(err.Error()), "no such process")
}

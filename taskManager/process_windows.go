//go:build windows

package taskManager

import (
	"fmt"
	"os"
	"syscall"
)

func backgroundCommandSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func signalBackgroundProcess(process *os.Process, signal string) error {
	if process == nil {
		return fmt.Errorf("process is nil")
	}
	return process.Kill()
}

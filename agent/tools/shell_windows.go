//go:build windows

package tools

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func killProcessGroup(pid int, _ interface{}) error {
	process, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	return syscall.TerminateProcess(process, 1)
}

func sigterm() syscall.Signal {
	return syscall.SIGTERM
}

func sigkill() syscall.Signal {
	return syscall.SIGKILL
}

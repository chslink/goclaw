//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func killProcessGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func sigterm() syscall.Signal {
	return syscall.SIGTERM
}

func sigkill() syscall.Signal {
	return syscall.SIGKILL
}

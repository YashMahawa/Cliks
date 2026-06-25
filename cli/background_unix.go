//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func prepareBackgroundCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func processLooksAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

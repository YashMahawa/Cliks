//go:build !windows

package main

import (
	"os"
	"syscall"
)

func tuiExitSignals() []os.Signal {
	return []os.Signal{syscall.SIGHUP, syscall.SIGTERM}
}

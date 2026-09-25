//go:build !windows

package main

import "testing"

func TestNonWindowsQuickEditNoOp(t *testing.T) {
	disableQuickEditForTerminalCapture()
	restoreQuickEditConsoleMode()
	forceRestoreQuickEditConsoleMode()
}

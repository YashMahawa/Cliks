//go:build !windows

package main

func disableQuickEditForTerminalCapture() {}
func restoreQuickEditConsoleMode() {}
func forceRestoreQuickEditConsoleMode() {}

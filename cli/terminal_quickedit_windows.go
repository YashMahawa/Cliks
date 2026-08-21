//go:build windows

package main

import (
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

const (
	enableQuickEditMode uint32 = 0x0040
	enableExtendedFlags uint32 = 0x0080
)

var (
	quickEditMu           sync.Mutex
	quickEditStored       bool
	quickEditOriginalMode uint32
	quickEditActiveCount  int
)

func getConsoleInputHandle() (windows.Handle, bool) {
	handle, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err == nil && handle != windows.InvalidHandle {
		var mode uint32
		if err := windows.GetConsoleMode(handle, &mode); err == nil {
			return handle, true
		}
	}
	stdinHandle := windows.Handle(os.Stdin.Fd())
	if stdinHandle != windows.InvalidHandle {
		var mode uint32
		if err := windows.GetConsoleMode(stdinHandle, &mode); err == nil {
			return stdinHandle, true
		}
	}
	return windows.InvalidHandle, false
}

// disableQuickEditForTerminalCapture detects and stores the pre-existing console mode state
// immediately before starting a terminal capture session, and dynamically disables QuickEdit mode.
func disableQuickEditForTerminalCapture() {
	quickEditMu.Lock()
	defer quickEditMu.Unlock()

	handle, ok := getConsoleInputHandle()
	if !ok {
		return
	}

	var currentMode uint32
	if err := windows.GetConsoleMode(handle, &currentMode); err != nil {
		return
	}

	if !quickEditStored {
		quickEditOriginalMode = currentMode
		quickEditStored = true
	}
	quickEditActiveCount++

	disabledMode := (currentMode &^ enableQuickEditMode) | enableExtendedFlags
	_ = windows.SetConsoleMode(handle, disabledMode)
}

// restoreQuickEditConsoleMode automatically restores the pre-existing console state on session teardown.
func restoreQuickEditConsoleMode() {
	quickEditMu.Lock()
	defer quickEditMu.Unlock()

	if !quickEditStored {
		return
	}

	if quickEditActiveCount > 0 {
		quickEditActiveCount--
	}

	if quickEditActiveCount == 0 {
		if handle, ok := getConsoleInputHandle(); ok {
			restoredMode := quickEditOriginalMode | enableExtendedFlags
			_ = windows.SetConsoleMode(handle, restoredMode)
		}
		quickEditStored = false
	}
}

// forceRestoreQuickEditConsoleMode forces immediate restoration of stored console mode state (e.g. on crash recovery or repair).
func forceRestoreQuickEditConsoleMode() {
	quickEditMu.Lock()
	defer quickEditMu.Unlock()

	if !quickEditStored {
		return
	}

	if handle, ok := getConsoleInputHandle(); ok {
		restoredMode := quickEditOriginalMode | enableExtendedFlags
		_ = windows.SetConsoleMode(handle, restoredMode)
	}
	quickEditActiveCount = 0
	quickEditStored = false
}

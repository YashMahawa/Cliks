//go:build windows

package main

import (
	"testing"
)

func TestQuickEditModeStateManagement(t *testing.T) {
	quickEditMu.Lock()
	quickEditStored = false
	quickEditOriginalMode = 0
	quickEditActiveCount = 0
	quickEditMu.Unlock()

	fakeOriginalMode := uint32(0x01f5)

	quickEditMu.Lock()
	quickEditOriginalMode = fakeOriginalMode
	quickEditStored = true
	quickEditActiveCount = 1
	quickEditMu.Unlock()

	restoreQuickEditConsoleMode()

	quickEditMu.Lock()
	if quickEditStored {
		t.Fatalf("expected quickEditStored to be false after restore")
	}
	if quickEditActiveCount != 0 {
		t.Fatalf("expected quickEditActiveCount to be 0, got %d", quickEditActiveCount)
	}
	quickEditMu.Unlock()
}

func TestQuickEditModeNestedActiveCount(t *testing.T) {
	quickEditMu.Lock()
	quickEditStored = false
	quickEditOriginalMode = 0
	quickEditActiveCount = 0
	quickEditMu.Unlock()

	disableQuickEditForTerminalCapture()
	disableQuickEditForTerminalCapture()

	quickEditMu.Lock()
	stored := quickEditStored
	count := quickEditActiveCount
	quickEditMu.Unlock()

	if stored && count != 2 {
		t.Fatalf("expected active count 2, got %d", count)
	}

	restoreQuickEditConsoleMode()

	quickEditMu.Lock()
	countAfterFirstRestore := quickEditActiveCount
	storedAfterFirstRestore := quickEditStored
	quickEditMu.Unlock()

	if stored {
		if !storedAfterFirstRestore {
			t.Fatalf("expected quickEditStored to remain true when active count > 0")
		}
		if countAfterFirstRestore != 1 {
			t.Fatalf("expected active count 1, got %d", countAfterFirstRestore)
		}
	}

	forceRestoreQuickEditConsoleMode()

	quickEditMu.Lock()
	if quickEditStored {
		t.Fatalf("expected quickEditStored to be false after force restore")
	}
	if quickEditActiveCount != 0 {
		t.Fatalf("expected quickEditActiveCount to be 0, got %d", quickEditActiveCount)
	}
	quickEditMu.Unlock()
}

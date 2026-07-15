//go:build windows

package main

import "testing"

func TestCopyWindowsUTF16TruncatesAndTerminates(t *testing.T) {
	buffer := make([]uint16, 8)
	copyWindowsUTF16(buffer, "Notifications are definitely longer")
	if buffer[len(buffer)-1] != 0 {
		t.Fatal("notification text was not null terminated")
	}
	if buffer[0] != 'N' {
		t.Fatalf("first rune = %d", buffer[0])
	}
}

func TestCopyWindowsUTF16DoesNotSplitEmoji(t *testing.T) {
	buffer := make([]uint16, 4)
	copyWindowsUTF16(buffer, "ab👋tail")
	if buffer[2] != 0 || buffer[3] != 0 {
		t.Fatalf("split surrogate pair: %#v", buffer)
	}
}

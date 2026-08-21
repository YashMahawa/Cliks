//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxAutostartXDGFallback(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Enable autostart (systemd unavailable in test container -> falls back to XDG autostart)
	msg, err := linuxAutostart("enable", "CLIK-123456")
	if err != nil {
		t.Fatalf("linuxAutostart enable failed: %v", err)
	}
	if !strings.Contains(msg, "Cliks autostart enabled") {
		t.Fatalf("unexpected enable message: %s", msg)
	}

	desktopPath := filepath.Join(tempDir, "autostart", "cliks.desktop")
	data, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("expected XDG autostart file at %s, error: %v", desktopPath, err)
	}

	content := string(data)
	if !strings.Contains(content, "[Desktop Entry]") {
		t.Fatalf("missing [Desktop Entry] header in %s", content)
	}
	if !strings.Contains(content, "CLIKS_AUTOSTART_TEAM=CLIK-123456") {
		t.Fatalf("missing CLIKS_AUTOSTART_TEAM env in %s", content)
	}
	if !strings.Contains(content, "CLIKS_RUN_MODE=boot") {
		t.Fatalf("missing CLIKS_RUN_MODE env in %s", content)
	}

	// Status check
	statusMsg, err := linuxAutostart("status", "CLIK-123456")
	if err != nil {
		t.Fatalf("linuxAutostart status failed: %v", err)
	}
	if !strings.Contains(statusMsg, "XDG desktop autostart entry") {
		t.Fatalf("expected status to mention XDG desktop autostart entry, got: %s", statusMsg)
	}

	// Disable check
	disableMsg, err := linuxAutostart("disable", "CLIK-123456")
	if err != nil {
		t.Fatalf("linuxAutostart disable failed: %v", err)
	}
	if !strings.Contains(disableMsg, "disabled") {
		t.Fatalf("unexpected disable message: %s", disableMsg)
	}

	if _, err := os.Stat(desktopPath); !os.IsNotExist(err) {
		t.Fatalf("expected desktop entry %s to be removed after disable", desktopPath)
	}
}

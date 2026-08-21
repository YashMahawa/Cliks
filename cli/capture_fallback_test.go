package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingCaptureHelperReturnsOffMode(t *testing.T) {
	// Point helper path to non-existent location
	t.Setenv("CLIKS_CAPTURE_HELPER", filepath.Join(t.TempDir(), "nonexistent-helper"))

	capture := newActivityCapture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state := capture.start(ctx, SharingConfig{Keyboard: true, Mouse: true}, "isolated")
	if state.Mode != "off" {
		t.Fatalf("expected capture mode 'off' when helper is missing, got %q", state.Mode)
	}
	if state.PermissionHint == "" {
		t.Fatalf("expected non-empty PermissionHint when helper is missing")
	}
}

func TestInstallScriptContainsSwiftCompilerCheck(t *testing.T) {
	content, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatalf("failed to read install.sh: %v", err)
	}
	script := string(content)
	if !containsString(script, "swiftc") {
		t.Fatalf("install.sh missing swiftc check")
	}
	if !containsString(script, "xcode-select --install") {
		t.Fatalf("install.sh missing xcode-select --install hint")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && searchSubstr(s, substr))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

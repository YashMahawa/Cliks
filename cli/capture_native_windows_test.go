//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestWindowsCaptureHelperCommandEnvOverride(t *testing.T) {
	t.Setenv("CLIKS_CAPTURE_HELPER", "custom-helper.exe")
	exe, args := windowsCaptureHelperCommand()
	if exe != "custom-helper.exe" {
		t.Fatalf("exe = %q, want custom-helper.exe", exe)
	}
	if len(args) != 1 || args[0] != "--stdio" {
		t.Fatalf("args = %#v, want [--stdio]", args)
	}
}

func TestWindowsCaptureHelperCommandDefault(t *testing.T) {
	t.Setenv("CLIKS_CAPTURE_HELPER", "")
	exe, args := windowsCaptureHelperCommand()
	if exe == "" {
		t.Fatal("exe should not be empty")
	}
	if len(args) != 1 || args[0] != "--stdio" {
		t.Fatalf("args = %#v, want [--stdio]", args)
	}
}

func TestWindowsGlobalHookPermissionHint(t *testing.T) {
	hint := globalHookPermissionHint()
	if !strings.Contains(hint, "Raw Input IPC helper") {
		t.Fatalf("unexpected permission hint: %q", hint)
	}
}

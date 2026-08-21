//go:build darwin

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDarwinCaptureHelperHandshakeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	mockHelper := filepath.Join(tmpDir, "cliks-capture")
	script := `#!/bin/sh
echo "ready"
echo "k"
echo "l"
sleep 5
`
	if err := os.WriteFile(mockHelper, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock helper: %v", err)
	}
	t.Setenv("CLIKS_CAPTURE_HELPER", mockHelper)

	capture := newActivityCapture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state := capture.startGlobalHook(ctx, SharingConfig{Keyboard: true, Mouse: true}, "auto")
	if state.Mode != "macos-isolated-app" {
		t.Fatalf("state.Mode = %q, want macos-isolated-app (hint: %q)", state.Mode, state.PermissionHint)
	}

	wants := []LocalActivityEvent{
		{Kind: "keyboard"},
		{Kind: "mouse", Button: "left"},
	}
	for _, want := range wants {
		select {
		case got := <-capture.Events:
			if got.Kind != want.Kind || got.Button != want.Button {
				t.Fatalf("got event kind=%q button=%q, want kind=%q button=%q", got.Kind, got.Button, want.Kind, want.Button)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event kind=%q button=%q", want.Kind, want.Button)
		}
	}
	capture.stop()
}

func TestDarwinCaptureHelperHandshakeFailureWithStderr(t *testing.T) {
	tmpDir := t.TempDir()
	mockHelper := filepath.Join(tmpDir, "cliks-capture")
	script := `#!/bin/sh
echo "Cliks Capture needs Input Monitoring permission." >&2
exit 2
`
	if err := os.WriteFile(mockHelper, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock helper: %v", err)
	}
	t.Setenv("CLIKS_CAPTURE_HELPER", mockHelper)

	capture := newActivityCapture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state := capture.startGlobalHook(ctx, SharingConfig{Keyboard: true, Mouse: true}, "auto")
	if state.Mode != "off" {
		t.Fatalf("state.Mode = %q, want off when helper fails handshake", state.Mode)
	}
	if !strings.Contains(state.PermissionHint, "Cliks Capture needs Input Monitoring permission.") {
		t.Fatalf("state.PermissionHint = %q, want stderr denial message", state.PermissionHint)
	}
}

func TestDarwinCaptureHelperHandshakeTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	mockHelper := filepath.Join(tmpDir, "cliks-capture")
	script := `#!/bin/sh
# Sleep without emitting ready token
sleep 10
`
	if err := os.WriteFile(mockHelper, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock helper: %v", err)
	}
	t.Setenv("CLIKS_CAPTURE_HELPER", mockHelper)

	capture := newActivityCapture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	state := capture.startGlobalHook(ctx, SharingConfig{Keyboard: true, Mouse: true}, "auto")
	duration := time.Since(start)

	if state.Mode != "off" {
		t.Fatalf("state.Mode = %q, want off on handshake timeout", state.Mode)
	}
	if duration > 5*time.Second {
		t.Fatalf("handshake timeout took %v, expected ~3s", duration)
	}
	if !strings.Contains(state.PermissionHint, "timed out") {
		t.Fatalf("PermissionHint = %q, expected timeout message", state.PermissionHint)
	}
}

func TestDarwinCaptureHelperReady(t *testing.T) {
	tmpDir := t.TempDir()

	// Ready mock
	readyHelper := filepath.Join(tmpDir, "ready-capture")
	if err := os.WriteFile(readyHelper, []byte("#!/bin/sh\necho 'ready'\nsleep 5\n"), 0755); err != nil {
		t.Fatalf("failed to create ready helper: %v", err)
	}
	t.Setenv("CLIKS_CAPTURE_HELPER", readyHelper)
	if !macCaptureHelperReady() {
		t.Fatalf("macCaptureHelperReady() = false, want true for mock emitting ready")
	}

	// Unready mock
	unreadyHelper := filepath.Join(tmpDir, "unready-capture")
	if err := os.WriteFile(unreadyHelper, []byte("#!/bin/sh\necho 'Cliks Capture needs Input Monitoring permission.' >&2\nexit 2\n"), 0755); err != nil {
		t.Fatalf("failed to create unready helper: %v", err)
	}
	t.Setenv("CLIKS_CAPTURE_HELPER", unreadyHelper)
	if macCaptureHelperReady() {
		t.Fatalf("macCaptureHelperReady() = true, want false for mock failing permission")
	}
}

func TestDarwinDoctorEvaluatesHelperBundleNotTerminal(t *testing.T) {
	tmpDir := t.TempDir()
	mockHelper := filepath.Join(tmpDir, "cliks-capture")
	if err := os.WriteFile(mockHelper, []byte("#!/bin/sh\necho 'ready'\nsleep 5\n"), 0755); err != nil {
		t.Fatalf("failed to create mock helper: %v", err)
	}
	t.Setenv("CLIKS_CAPTURE_HELPER", mockHelper)

	trusted, reason := macInputMonitoringTrusted()
	if !trusted {
		t.Fatalf("macInputMonitoringTrusted() = false, want true when helper bundle emits ready")
	}
	if !strings.Contains(reason, "Cliks Capture.app helper bundle") {
		t.Fatalf("reason = %q, want helper bundle reference", reason)
	}

	var report doctorReport
	appendPlatformCaptureChecks(&report, false)
	foundTrustCheck := false
	for _, check := range report.checks {
		if check.label == "Input Monitoring trust" && strings.Contains(check.status, "granted") {
			foundTrustCheck = true
		}
	}
	if !foundTrustCheck {
		t.Fatalf("appendPlatformCaptureChecks report checks = %+v, expected granted Input Monitoring trust", report.checks)
	}
}

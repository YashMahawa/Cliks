//go:build darwin && !cgo

package main

import (
	"context"
	"testing"
)

func TestDarwinNoCGODirectCaptureFallback(t *testing.T) {
	capture := newActivityCapture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sharing := SharingConfig{Keyboard: true, Mouse: true}
	state := capture.startDirectGlobalHook(ctx, sharing)
	if state.Mode == "macos-event-tap" {
		t.Fatalf("expected non-CGO build to not use macos-event-tap, got mode=%q", state.Mode)
	}
}

func TestDarwinNoCGODiagnostics(t *testing.T) {
	_ = macListenEventAccessAllowed()
	_ = requestMacListenEventAccess()
	hint := globalHookPermissionHint()
	if hint == "" {
		t.Fatalf("expected non-empty permission hint")
	}
}

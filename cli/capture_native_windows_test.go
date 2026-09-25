//go:build windows

package main

import (
	"context"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsNativeCallbacksEmitOnlyAllowedActivityKinds(t *testing.T) {
	capture := newActivityCapture()
	capture.ctx, capture.cancel = context.WithCancel(context.Background())
	defer capture.cancel()
	windowsNativeCaptureLock.Lock()
	dispatch := newNativeCaptureDispatcher(capture)
	windowsNativeCapture = &windowsCaptureSession{dispatch: dispatch, sharing: SharingConfig{Keyboard: true, Mouse: true}}
	windowsNativeCaptureLock.Unlock()
	defer func() {
		windowsNativeCaptureLock.Lock()
		windowsNativeCapture = nil
		windowsNativeCaptureLock.Unlock()
		dispatch.stop()
	}()

	lowLevelKeyboardCallback(hcAction, wmKeyDown, 0)
	lowLevelMouseCallback(hcAction, wmLButtonDown, 0)
	lowLevelMouseCallback(hcAction, wmRButtonDown, 0)
	lowLevelMouseCallback(hcAction, 0x020A, 0) // wheel must not count

	wants := []LocalActivityEvent{
		{Kind: "keyboard"},
		{Kind: "mouse", Button: "left"},
		{Kind: "mouse", Button: "right"},
	}
	for _, want := range wants {
		select {
		case got := <-capture.Events:
			if got.Kind != want.Kind || got.Button != want.Button {
				t.Fatalf("event = %#v, want kind=%q button=%q", got, want.Kind, want.Button)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for kind=%q button=%q", want.Kind, want.Button)
		}
	}
	select {
	case extra := <-capture.Events:
		t.Fatalf("unexpected extra event: %#v", extra)
	default:
	}
}

func TestWindowsHookProbeResponseSuppressesUserEvents(t *testing.T) {
	capture := newActivityCapture()
	capture.ctx, capture.cancel = context.WithCancel(context.Background())
	defer capture.cancel()

	windowsNativeCaptureLock.Lock()
	dispatch := newNativeCaptureDispatcher(capture)
	session := &windowsCaptureSession{
		dispatch:     dispatch,
		sharing:      SharingConfig{Keyboard: true, Mouse: true},
		keyboardHook: 1,
		mouseHook:    2,
	}
	windowsNativeCapture = session
	windowsNativeCaptureLock.Unlock()
	defer func() {
		windowsNativeCaptureLock.Lock()
		windowsNativeCapture = nil
		windowsNativeCaptureLock.Unlock()
		dispatch.stop()
	}()

	kbdInfo := kbdllHookStruct{DwExtraInfo: probeExtraInfo}
	lowLevelKeyboardCallback(hcAction, wmKeyDown, uintptr(unsafe.Pointer(&kbdInfo)))

	msInfo := msllHookStruct{DwExtraInfo: probeExtraInfo}
	lowLevelMouseCallback(hcAction, wmLButtonDown, uintptr(unsafe.Pointer(&msInfo)))

	// Probes should update probe timestamps
	if !session.checkVitality(time.Second) {
		t.Fatalf("expected session vitality to be OK after probe response")
	}

	// No activity events should be emitted for probe events
	select {
	case event := <-capture.Events:
		t.Fatalf("unexpected event emitted from probe: %#v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWindowsHookSupervisorEvictionAndStructuredLogging(t *testing.T) {
	clearDiagnosticEvents()

	session := &windowsCaptureSession{
		sharing:      SharingConfig{Keyboard: true, Mouse: true},
		keyboardHook: 1,
		mouseHook:    2,
	}

	// Stale probe responses -> checkVitality fails
	oldTime := time.Now().Add(-10 * time.Second)
	session.lastKeyboardProbeResponse = oldTime
	session.lastMouseProbeResponse = oldTime
	session.lastUserActivity = oldTime

	if session.checkVitality(2 * time.Second) {
		t.Fatalf("expected checkVitality to fail for stale timestamps")
	}

	// Simulate logging eviction event
	logDiagnosticEvent(structuredDiagnosticEvent{
		Event:    "hook_eviction",
		Platform: "windows",
		Hook:     "windows-native",
		Reason:   "OS hook timeout or elevated window transition (UIPI)",
	})

	events := getDiagnosticEvents()
	if len(events) == 0 {
		t.Fatalf("expected diagnostic event to be logged")
	}

	ev := events[len(events)-1]
	if ev.Event != "hook_eviction" || ev.Platform != "windows" || ev.Hook != "windows-native" {
		t.Fatalf("unexpected diagnostic event: %#v", ev)
	}
}

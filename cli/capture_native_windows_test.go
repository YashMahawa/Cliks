//go:build windows

package main

import (
	"context"
	"sync/atomic"
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

func TestWindowsNativeProbeFiltering(t *testing.T) {
	capture := newActivityCapture()
	capture.ctx, capture.cancel = context.WithCancel(context.Background())
	defer capture.cancel()

	dispatch := newNativeCaptureDispatcher(capture)
	defer dispatch.stop()

	session := &windowsCaptureSession{
		dispatch: dispatch,
		sharing:  SharingConfig{Keyboard: true, Mouse: true},
	}

	windowsNativeCaptureLock.Lock()
	windowsNativeCapture = session
	windowsNativeCaptureLock.Unlock()
	defer func() {
		windowsNativeCaptureLock.Lock()
		windowsNativeCapture = nil
		windowsNativeCaptureLock.Unlock()
	}()

	kbdStruct := kbdLLHookStruct{DwExtraInfo: hookProbeMagic}
	msStruct := msLLHookStruct{DwExtraInfo: hookProbeMagic}

	beforeNano := time.Now().UnixNano()

	lowLevelKeyboardCallback(hcAction, wmKeyDown, uintptr(unsafe.Pointer(&kbdStruct)))
	lowLevelMouseCallback(hcAction, wmLButtonDown, uintptr(unsafe.Pointer(&msStruct)))

	afterNano := atomic.LoadInt64(&session.lastCallbackNano)
	if afterNano < beforeNano {
		t.Fatalf("lastCallbackNano was not updated on probe ACK")
	}

	// Verify no activity events were emitted from probes
	select {
	case event := <-capture.Events:
		t.Fatalf("unexpected event emitted from probe input: %#v", event)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event received
	}
}

func TestExponentialBackoffDelay(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 500 * time.Millisecond},
		{1, 500 * time.Millisecond},
		{2, 1000 * time.Millisecond},
		{3, 2000 * time.Millisecond},
		{4, 4000 * time.Millisecond},
		{5, 8000 * time.Millisecond},
		{6, 10000 * time.Millisecond},
	}

	for _, tt := range tests {
		got := exponentialBackoffDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("exponentialBackoffDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestWindowsWatchdogStateTransitions(t *testing.T) {
	capture := newActivityCapture()
	capture.ctx, capture.cancel = context.WithCancel(context.Background())
	defer capture.cancel()

	var stateHistory []CaptureState
	capture.OnStateChange = func(cs CaptureState) {
		stateHistory = append(stateHistory, cs)
	}

	dispatch := newNativeCaptureDispatcher(capture)
	defer dispatch.stop()

	session := &windowsCaptureSession{
		dispatch: dispatch,
		sharing:  SharingConfig{Keyboard: true, Mouse: true},
		capture:  capture,
	}

	// Simulate handleHookDetachment recovery transition (cancel context to skip backoff sleep)
	testCtx, testCancel := context.WithCancel(capture.ctx)
	testCancel()
	session.handleHookDetachment(testCtx, capture, 0, session.sharing)

	if len(stateHistory) == 0 || stateHistory[len(stateHistory)-1].Mode != "recovering" {
		t.Fatalf("expected state mode 'recovering', got %#v", stateHistory)
	}

	// Simulate max failures exceeded
	session.rehookFailures = maxRehookAttempts
	session.handleHookDetachment(capture.ctx, capture, 0, session.sharing)

	if len(stateHistory) < 2 || stateHistory[len(stateHistory)-1].Mode != "off" {
		t.Fatalf("expected state mode 'off' after max attempts, got %#v", stateHistory)
	}
}

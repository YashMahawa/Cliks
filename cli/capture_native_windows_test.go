//go:build windows

package main

import (
	"context"
	"testing"
	"time"
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

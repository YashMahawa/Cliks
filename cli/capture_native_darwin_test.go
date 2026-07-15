//go:build darwin

package main

import (
	"context"
	"testing"
	"time"
)

func TestDarwinNativeCallbackEmitsOnlyAllowedActivityKinds(t *testing.T) {
	capture := newActivityCapture()
	capture.ctx, capture.cancel = context.WithCancel(context.Background())
	defer capture.cancel()
	token := uint64(42)
	darwinCaptureMu.Lock()
	dispatch := newNativeCaptureDispatcher(capture)
	darwinCaptures[token] = &darwinCaptureSession{dispatch: dispatch, sharing: SharingConfig{Keyboard: true, Mouse: true}}
	darwinCaptureMu.Unlock()
	defer func() {
		darwinCaptureMu.Lock()
		delete(darwinCaptures, token)
		darwinCaptureMu.Unlock()
		dispatch.stop()
	}()

	emitDarwinCaptureEvent(token, 1, 0)
	emitDarwinCaptureEvent(token, 2, 1)
	emitDarwinCaptureEvent(token, 2, 2)
	emitDarwinCaptureEvent(token, 2, 3)

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

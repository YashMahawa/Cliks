package main

import (
	"context"
	"testing"
	"time"
)

func TestNativeCaptureDispatcherPreservesBurstOrder(t *testing.T) {
	capture := newActivityCapture()
	capture.ctx, capture.cancel = context.WithCancel(context.Background())
	defer capture.cancel()
	dispatch := newNativeCaptureDispatcher(capture)
	defer dispatch.stop()
	for i := 0; i < 1500; i++ {
		dispatch.push(LocalActivityEvent{Kind: "keyboard", At: time.UnixMilli(int64(i))})
	}
	for i := 0; i < 1500; i++ {
		select {
		case event := <-capture.Events:
			if event.At.UnixMilli() != int64(i) {
				t.Fatalf("event %d arrived at %d", i, event.At.UnixMilli())
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out at event %d", i)
		}
	}
}

func TestStructuredDiagnosticEventLogging(t *testing.T) {
	clearDiagnosticEvents()

	logDiagnosticEvent(structuredDiagnosticEvent{
		Event:    "hook_eviction",
		Platform: "windows",
		Hook:     "windows-native",
		Reason:   "OS hook timeout or elevated window transition (UIPI)",
	})

	logDiagnosticEvent(structuredDiagnosticEvent{
		Event:    "hook_recovery_success",
		Platform: "windows",
		Hook:     "windows-native",
		Attempt:  1,
		DelayMS:  1000,
	})

	events := getDiagnosticEvents()
	if len(events) != 2 {
		t.Fatalf("expected 2 diagnostic events, got %d", len(events))
	}

	if events[0].Event != "hook_eviction" || events[0].Reason == "" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].Event != "hook_recovery_success" || events[1].Attempt != 1 || events[1].DelayMS != 1000 {
		t.Fatalf("unexpected second event: %#v", events[1])
	}

	clearDiagnosticEvents()
	if len(getDiagnosticEvents()) != 0 {
		t.Fatalf("expected 0 events after clear")
	}
}

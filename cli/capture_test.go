package main

import (
	"context"
	"testing"
	"time"
)

func TestEvdevRetryDelayBacksOffAndCaps(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}
	for index, expected := range want {
		if got := evdevRetryDelay(index + 1); got != expected {
			t.Fatalf("retry %d = %s, want %s", index+1, got, expected)
		}
	}
}

func TestCaptureBackpressureDoesNotDropBurstEvents(t *testing.T) {
	capture := newActivityCapture()
	for i := 0; i < cap(capture.Events); i++ {
		capture.Events <- LocalActivityEvent{Kind: "keyboard"}
	}
	done := make(chan struct{})
	go func() {
		capture.emit(LocalActivityEvent{Kind: "mouse", Button: "left"})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("emit returned while the queue was still full")
	case <-time.After(20 * time.Millisecond):
	}
	<-capture.Events
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emit did not resume after queue space became available")
	}
}

func TestSessionBackpressureStopsOnCancellation(t *testing.T) {
	controller := newSessionController(defaultConfig(), StartOptions{}, nil)
	for i := 0; i < cap(controller.local); i++ {
		controller.local <- LocalActivityEvent{Kind: "keyboard"}
	}
	done := make(chan struct{})
	go func() {
		controller.recordLocalActivity(LocalActivityEvent{Kind: "keyboard"})
		close(done)
	}()
	controller.cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recordLocalActivity did not stop after cancellation")
	}
	if controller.ctx.Err() != context.Canceled {
		t.Fatalf("context error = %v, want canceled", controller.ctx.Err())
	}
}

func TestTouchpadTapDetectorSingleFingerTap(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evAbs, absX, 1200, now)
	detector.handle(evAbs, absY, 900, now)
	if detector.handle(evKey, btnTouch, 1, now) != "" {
		t.Fatal("touch start should not emit")
	}
	if button := detector.handle(evKey, btnTouch, 0, now.Add(90*time.Millisecond)); button != "left" {
		t.Fatalf("short single-finger tap emitted %q, want left", button)
	}
}

func TestTouchpadTapDetectorTwoFingerTapIsRightClick(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evKey, btnToolDouble, 1, now)
	detector.handle(evKey, btnTouch, 1, now)
	if button := detector.handle(evKey, btnTouch, 0, now.Add(90*time.Millisecond)); button != "right" {
		t.Fatalf("short two-finger tap emitted %q, want right", button)
	}
}

func TestTouchpadTapDetectorDoesNotDuplicatePhysicalButton(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evKey, btnTouch, 1, now)
	detector.handle(evKey, btnLeft, 1, now.Add(20*time.Millisecond))
	if button := detector.handle(evKey, btnTouch, 0, now.Add(90*time.Millisecond)); button != "" {
		t.Fatalf("physical click should suppress tap heuristic, got %q", button)
	}
}

func TestTouchpadTapDetectorDoesNotEmitOnTouchStart(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	if button := detector.handle(evKey, btnTouch, 1, now); button != "" {
		t.Fatalf("touch start emitted %q", button)
	}
}

func TestTouchpadTapDetectorIgnoresThreeFingerTap(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evKey, btnTouch, 1, now)
	detector.handle(evKey, btnToolTriple, 1, now.Add(20*time.Millisecond))
	if button := detector.handle(evKey, btnTouch, 0, now.Add(90*time.Millisecond)); button != "" {
		t.Fatalf("three-finger tap should not emit, got %q", button)
	}
}

func TestTouchpadTapDetectorIgnoresThreeFingerStateBeforeTouch(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evKey, btnToolTriple, 1, now)
	detector.handle(evKey, btnTouch, 1, now.Add(10*time.Millisecond))
	if button := detector.handle(evKey, btnTouch, 0, now.Add(90*time.Millisecond)); button != "" {
		t.Fatalf("pre-touch three-finger state should not emit, got %q", button)
	}
}

func TestHandleEvdevEmitsPhysicalLeftClick(t *testing.T) {
	capture := newActivityCapture()
	detector := &touchpadTapDetector{}
	capture.handleEvdev(evdevEventChunk(evKey, btnLeft, 1), SharingConfig{Mouse: true}, detector)
	select {
	case event := <-capture.Events:
		if event.Kind != "mouse" || event.Button != "left" {
			t.Fatalf("event = %+v, want left mouse click", event)
		}
	default:
		t.Fatal("expected physical left click event")
	}
}

func TestTouchpadTapDetectorIgnoresMovement(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evAbs, absX, 100, now)
	detector.handle(evAbs, absY, 100, now)
	detector.handle(evKey, btnTouch, 1, now)
	detector.handle(evAbs, absX, 500, now.Add(50*time.Millisecond))
	if button := detector.handle(evKey, btnTouch, 0, now.Add(120*time.Millisecond)); button != "" {
		t.Fatalf("moving touch should not emit, got %q", button)
	}
}

func TestTouchpadTapDetectorIgnoresLongPress(t *testing.T) {
	detector := &touchpadTapDetector{}
	now := time.Unix(100, 0)
	detector.handle(evKey, btnTouch, 1, now)
	if button := detector.handle(evKey, btnTouch, 0, now.Add(600*time.Millisecond)); button != "" {
		t.Fatalf("long press should not emit, got %q", button)
	}
}

func evdevEventChunk(eventType uint16, code uint16, value int32) []byte {
	chunk := make([]byte, 24)
	chunk[16] = byte(eventType)
	chunk[17] = byte(eventType >> 8)
	chunk[18] = byte(code)
	chunk[19] = byte(code >> 8)
	chunk[20] = byte(value)
	chunk[21] = byte(value >> 8)
	chunk[22] = byte(value >> 16)
	chunk[23] = byte(value >> 24)
	return chunk
}

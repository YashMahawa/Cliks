package main

import (
	"testing"
	"time"
)

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

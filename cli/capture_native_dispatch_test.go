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

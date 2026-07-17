//go:build darwin

package main

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include "capture_native_darwin.h"
*/
import "C"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

var (
	darwinCaptureToken atomic.Uint64
	darwinCaptureMu    sync.RWMutex
	darwinCaptures     = map[uint64]*darwinCaptureSession{}
)

type darwinCaptureSession struct {
	dispatch *nativeCaptureDispatcher
	sharing  SharingConfig
}

func (c *ActivityCapture) startDirectGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	if !macListenEventAccessAllowed() {
		return CaptureState{Mode: "off", PermissionHint: globalHookPermissionHint()}
	}
	token := darwinCaptureToken.Add(1)
	darwinCaptureMu.Lock()
	dispatch := newNativeCaptureDispatcher(c)
	darwinCaptures[token] = &darwinCaptureSession{dispatch: dispatch, sharing: sharing}
	darwinCaptureMu.Unlock()

	handle := C.cliks_event_tap_create(C.uintptr_t(token))
	if handle == nil {
		darwinCaptureMu.Lock()
		delete(darwinCaptures, token)
		darwinCaptureMu.Unlock()
		dispatch.stop()
		return CaptureState{Mode: "off", PermissionHint: globalHookPermissionHint()}
	}
	go func() {
		runDone := make(chan struct{})
		stopDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				C.cliks_event_tap_stop(handle)
			case <-runDone:
			}
			close(stopDone)
		}()
		C.cliks_event_tap_run(handle)
		if ctx.Err() == nil && c.cancel != nil {
			c.cancel()
		}
		close(runDone)
		<-stopDone
		C.cliks_event_tap_destroy(handle)
		darwinCaptureMu.Lock()
		delete(darwinCaptures, token)
		darwinCaptureMu.Unlock()
		dispatch.stop()
	}()
	return CaptureState{Mode: "macos-event-tap", PermissionHint: platformStartupCaptureNotice()}
}

//export cliksCaptureDarwinEvent
func cliksCaptureDarwinEvent(token C.uintptr_t, kind C.int, button C.int) {
	emitDarwinCaptureEvent(uint64(token), int(kind), int(button))
}

func emitDarwinCaptureEvent(token uint64, kind int, button int) {
	darwinCaptureMu.RLock()
	session := darwinCaptures[token]
	darwinCaptureMu.RUnlock()
	if session == nil {
		return
	}
	now := time.Now()
	switch kind {
	case 1:
		if session.sharing.Keyboard {
			session.dispatch.push(LocalActivityEvent{Kind: "keyboard", At: now})
		}
	case 2:
		if !session.sharing.Mouse {
			return
		}
		mouseButton := ""
		if button == 1 {
			mouseButton = "left"
		} else if button == 2 {
			mouseButton = "right"
		}
		if mouseButton != "" {
			session.dispatch.push(LocalActivityEvent{Kind: "mouse", Button: mouseButton, At: now})
		}
	}
}

func globalHookPermissionHint() string {
	return "macOS blocked direct capture. This compatibility mode requires Input Monitoring for the terminal or launcher. Prefer isolated capture from cliks setup."
}

func macListenEventAccessAllowed() bool {
	return C.cliks_event_tap_access_allowed() == 1
}

func requestMacListenEventAccess() bool {
	return C.cliks_event_tap_request_access() == 1
}

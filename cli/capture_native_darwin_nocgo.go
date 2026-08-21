//go:build darwin && !cgo

package main

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
	return CaptureState{Mode: "off", PermissionHint: globalHookPermissionHint()}
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
	return false
}

func requestMacListenEventAccess() bool {
	return false
}

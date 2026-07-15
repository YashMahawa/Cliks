//go:build windows

package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL  = 13
	whMouseLL     = 14
	hcAction      = 0
	wmKeyDown     = 0x0100
	wmSysKeyDown  = 0x0104
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmQuit        = 0x0012
	pmNoRemove    = 0x0000
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procSetWindowsHookExW    = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx  = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx       = user32.NewProc("CallNextHookEx")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procPeekMessageW         = user32.NewProc("PeekMessageW")
	procPostThreadMessageW   = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadID   = kernel32.NewProc("GetCurrentThreadId")
	procGetModuleHandleW     = kernel32.NewProc("GetModuleHandleW")
	windowsKeyboardCallback  = syscall.NewCallback(lowLevelKeyboardCallback)
	windowsMouseCallback     = syscall.NewCallback(lowLevelMouseCallback)
	windowsNativeCaptureLock sync.RWMutex
	windowsNativeCapture     *windowsCaptureSession
)

type windowsCaptureSession struct {
	dispatch *nativeCaptureDispatcher
	sharing  SharingConfig
}

type windowsPoint struct {
	X int32
	Y int32
}

type windowsMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   windowsPoint
	Private uint32
}

type windowsHookStart struct {
	threadID uint32
	err      error
}

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	ready := make(chan windowsHookStart, 1)
	go c.runWindowsHooks(ctx, sharing, ready)
	select {
	case <-ctx.Done():
		return CaptureState{Mode: "off", PermissionHint: ctx.Err().Error()}
	case result := <-ready:
		if result.err != nil {
			return CaptureState{Mode: "off", PermissionHint: "Windows native capture could not start: " + result.err.Error()}
		}
		return CaptureState{Mode: "windows-native"}
	case <-time.After(3 * time.Second):
		if c.cancel != nil {
			c.cancel()
		}
		return CaptureState{Mode: "off", PermissionHint: "Windows native capture timed out while starting."}
	}
}

func (c *ActivityCapture) runWindowsHooks(ctx context.Context, sharing SharingConfig, ready chan<- windowsHookStart) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	threadID, _, _ := procGetCurrentThreadID.Call()
	var message windowsMessage
	// Force creation of this thread's message queue before another goroutine can post WM_QUIT.
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmNoRemove)

	module, _, _ := procGetModuleHandleW.Call(0)
	keyboardHook := uintptr(0)
	mouseHook := uintptr(0)
	if sharing.Keyboard {
		keyboardHook, _, _ = procSetWindowsHookExW.Call(whKeyboardLL, windowsKeyboardCallback, module, 0)
		if keyboardHook == 0 {
			ready <- windowsHookStart{err: fmt.Errorf("SetWindowsHookExW keyboard hook failed")}
			return
		}
	}
	if sharing.Mouse {
		mouseHook, _, _ = procSetWindowsHookExW.Call(whMouseLL, windowsMouseCallback, module, 0)
		if mouseHook == 0 {
			if keyboardHook != 0 {
				procUnhookWindowsHookEx.Call(keyboardHook)
			}
			ready <- windowsHookStart{err: fmt.Errorf("SetWindowsHookExW mouse hook failed")}
			return
		}
	}
	defer func() {
		if keyboardHook != 0 {
			procUnhookWindowsHookEx.Call(keyboardHook)
		}
		if mouseHook != 0 {
			procUnhookWindowsHookEx.Call(mouseHook)
		}
		windowsNativeCaptureLock.Lock()
		session := windowsNativeCapture
		windowsNativeCapture = nil
		windowsNativeCaptureLock.Unlock()
		if session != nil {
			session.dispatch.stop()
		}
	}()

	windowsNativeCaptureLock.Lock()
	windowsNativeCapture = &windowsCaptureSession{dispatch: newNativeCaptureDispatcher(c), sharing: sharing}
	windowsNativeCaptureLock.Unlock()
	ready <- windowsHookStart{threadID: uint32(threadID)}

	go func(id uint32) {
		<-ctx.Done()
		procPostThreadMessageW.Call(uintptr(id), wmQuit, 0, 0)
	}(uint32(threadID))

	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			if ctx.Err() == nil && c.cancel != nil {
				c.cancel()
			}
			return
		}
	}
}

func lowLevelKeyboardCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code == hcAction && (wParam == wmKeyDown || wParam == wmSysKeyDown) {
		emitWindowsNativeEvent("keyboard", "")
	}
	result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return result
}

func lowLevelMouseCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code == hcAction {
		switch wParam {
		case wmLButtonDown:
			emitWindowsNativeEvent("mouse", "left")
		case wmRButtonDown:
			emitWindowsNativeEvent("mouse", "right")
		}
	}
	result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return result
}

func emitWindowsNativeEvent(kind string, button string) {
	windowsNativeCaptureLock.RLock()
	session := windowsNativeCapture
	windowsNativeCaptureLock.RUnlock()
	if session == nil || (kind == "keyboard" && !session.sharing.Keyboard) || (kind == "mouse" && !session.sharing.Mouse) {
		return
	}
	session.dispatch.push(LocalActivityEvent{Kind: kind, Button: button, At: time.Now()})
}

func globalHookPermissionHint() string {
	return "Capture may pause only while an Administrator window is focused (Windows security)."
}

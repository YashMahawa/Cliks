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

	wmUser   = 0x0400
	wmRehook = wmUser + 1

	probeExtraInfo = uintptr(0x434C494B) // 'CLIK'

	inputMouse      = 0
	inputKeyboard   = 1
	mouseEventfMove = 0x0001
	vkF24           = 0x87
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
	procSendInput            = user32.NewProc("SendInput")
	procGetCurrentThreadID   = kernel32.NewProc("GetCurrentThreadId")
	procGetModuleHandleW     = kernel32.NewProc("GetModuleHandleW")
	windowsKeyboardCallback  = syscall.NewCallback(lowLevelKeyboardCallback)
	windowsMouseCallback     = syscall.NewCallback(lowLevelMouseCallback)
	windowsNativeCaptureLock sync.RWMutex
	windowsNativeCapture     *windowsCaptureSession
)

type windowsCaptureSession struct {
	dispatch                  *nativeCaptureDispatcher
	sharing                   SharingConfig
	threadID                  uint32
	keyboardHook              uintptr
	mouseHook                 uintptr
	module                    uintptr
	lastKeyboardProbeResponse time.Time
	lastMouseProbeResponse    time.Time
	lastUserActivity          time.Time
	vitalityMu                sync.RWMutex
	evicted                   bool
	recoveryAttempt           int
}

func (s *windowsCaptureSession) recordProbeResponse(kind string) {
	s.vitalityMu.Lock()
	defer s.vitalityMu.Unlock()
	now := time.Now()
	if kind == "keyboard" {
		s.lastKeyboardProbeResponse = now
	} else if kind == "mouse" {
		s.lastMouseProbeResponse = now
	}
}

func (s *windowsCaptureSession) recordUserActivity(kind string) {
	s.vitalityMu.Lock()
	defer s.vitalityMu.Unlock()
	now := time.Now()
	s.lastUserActivity = now
	if kind == "keyboard" {
		s.lastKeyboardProbeResponse = now
	} else if kind == "mouse" {
		s.lastMouseProbeResponse = now
	}
}

func (s *windowsCaptureSession) checkVitality(threshold time.Duration) bool {
	s.vitalityMu.RLock()
	defer s.vitalityMu.RUnlock()
	now := time.Now()
	kbOK := !s.sharing.Keyboard || now.Sub(s.lastKeyboardProbeResponse) <= threshold || now.Sub(s.lastUserActivity) <= threshold
	msOK := !s.sharing.Mouse || now.Sub(s.lastMouseProbeResponse) <= threshold || now.Sub(s.lastUserActivity) <= threshold
	handlesOK := (!s.sharing.Keyboard || s.keyboardHook != 0) && (!s.sharing.Mouse || s.mouseHook != 0)
	return kbOK && msOK && handlesOK
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

type kbdllHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msllHookStruct struct {
	Pt          windowsPoint
	MouseData   uint32
	Flags       uint32
	Time        uint32
	_           uint32
	DwExtraInfo uintptr
}

type win32KeybdInput struct {
	InputType   uint32
	_           uint32
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	_           uint32
	DwExtraInfo uintptr
	_           [8]byte
}

type win32MouseInput struct {
	InputType   uint32
	_           uint32
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	_           uint32
	DwExtraInfo uintptr
}

type windowsHookStart struct {
	threadID uint32
	err      error
}

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig, mode string) CaptureState {
	_ = mode // Windows hooks require no special cross-application permission.
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

	session := &windowsCaptureSession{
		dispatch:                  newNativeCaptureDispatcher(c),
		sharing:                   sharing,
		threadID:                  uint32(threadID),
		keyboardHook:              keyboardHook,
		mouseHook:                 mouseHook,
		module:                    module,
		lastKeyboardProbeResponse: time.Now(),
		lastMouseProbeResponse:    time.Now(),
		lastUserActivity:          time.Now(),
	}

	defer func() {
		if session.keyboardHook != 0 {
			procUnhookWindowsHookEx.Call(session.keyboardHook)
		}
		if session.mouseHook != 0 {
			procUnhookWindowsHookEx.Call(session.mouseHook)
		}
		windowsNativeCaptureLock.Lock()
		windowsNativeCapture = nil
		windowsNativeCaptureLock.Unlock()
		if session != nil {
			session.dispatch.stop()
		}
	}()

	windowsNativeCaptureLock.Lock()
	windowsNativeCapture = session
	windowsNativeCaptureLock.Unlock()

	ready <- windowsHookStart{threadID: uint32(threadID)}

	go func(id uint32) {
		<-ctx.Done()
		procPostThreadMessageW.Call(uintptr(id), wmQuit, 0, 0)
	}(uint32(threadID))

	// Start background supervisor loop
	go c.runHookSupervisor(ctx, session)

	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			if ctx.Err() == nil && c.cancel != nil {
				c.cancel()
			}
			return
		}
		if message.Message == wmRehook {
			rehookWindowsHooks(session)
		}
	}
}

func rehookWindowsHooks(session *windowsCaptureSession) {
	if session == nil {
		return
	}
	session.vitalityMu.Lock()
	defer session.vitalityMu.Unlock()

	if session.keyboardHook != 0 {
		procUnhookWindowsHookEx.Call(session.keyboardHook)
		session.keyboardHook = 0
	}
	if session.mouseHook != 0 {
		procUnhookWindowsHookEx.Call(session.mouseHook)
		session.mouseHook = 0
	}

	if session.sharing.Keyboard {
		kb, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, windowsKeyboardCallback, session.module, 0)
		session.keyboardHook = kb
	}
	if session.sharing.Mouse {
		ms, _, _ := procSetWindowsHookExW.Call(whMouseLL, windowsMouseCallback, session.module, 0)
		session.mouseHook = ms
	}
}

func (c *ActivityCapture) runHookSupervisor(ctx context.Context, session *windowsCaptureSession) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check vitality with 2.5s window
			if session.checkVitality(2500 * time.Millisecond) {
				if session.evicted {
					session.evicted = false
					session.recoveryAttempt = 0
				}
				continue
			}

			// Perform synthetic vitality probe
			sendVitalityProbe(session.sharing)
			time.Sleep(100 * time.Millisecond)

			if session.checkVitality(2500 * time.Millisecond) {
				if session.evicted {
					session.evicted = false
					session.recoveryAttempt = 0
				}
				continue
			}

			// Vitality probe failed -> Detached / Evicted!
			if !session.evicted {
				session.evicted = true
				logDiagnosticEvent(structuredDiagnosticEvent{
					Event:     "hook_eviction",
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Platform:  "windows",
					Hook:      "windows-native",
					Reason:    "OS hook timeout or elevated window transition (UIPI)",
				})
			}

			// Attempt re-registration with exponential backoff & randomized jitter
			session.recoveryAttempt++
			attempt := session.recoveryAttempt
			delay := randomizedRetryDelay(attempt)

			// Request re-hook on the message loop thread
			procPostThreadMessageW.Call(uintptr(session.threadID), wmRehook, 0, 0)

			time.Sleep(150 * time.Millisecond)
			sendVitalityProbe(session.sharing)
			time.Sleep(100 * time.Millisecond)

			if session.checkVitality(2500 * time.Millisecond) {
				// Recovery succeeded!
				session.evicted = false
				session.recoveryAttempt = 0
				logDiagnosticEvent(structuredDiagnosticEvent{
					Event:     "hook_recovery_success",
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Platform:  "windows",
					Hook:      "windows-native",
					Attempt:   attempt,
					DelayMS:   delay.Milliseconds(),
				})
			} else {
				// Recovery failed on this attempt
				logDiagnosticEvent(structuredDiagnosticEvent{
					Event:     "hook_recovery_failed",
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Platform:  "windows",
					Hook:      "windows-native",
					Attempt:   attempt,
					Reason:    "SetWindowsHookExW or vitality probe check failed",
					DelayMS:   delay.Milliseconds(),
				})
				// Wait for backoff delay before continuing next loop
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}
		}
	}
}

func sendVitalityProbe(sharing SharingConfig) bool {
	if procSendInput.Find() != nil {
		return false
	}
	sentAny := false
	if sharing.Keyboard {
		input := win32KeybdInput{
			InputType:   inputKeyboard,
			WVk:         vkF24,
			DwFlags:     0,
			Time:        0,
			DwExtraInfo: probeExtraInfo,
		}
		res, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
		if res > 0 {
			sentAny = true
		}
	}
	if sharing.Mouse {
		input := win32MouseInput{
			InputType:   inputMouse,
			Dx:          0,
			Dy:          0,
			MouseData:   0,
			DwFlags:     mouseEventfMove,
			Time:        0,
			DwExtraInfo: probeExtraInfo,
		}
		res, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
		if res > 0 {
			sentAny = true
		}
	}
	return sentAny
}

func lowLevelKeyboardCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code == hcAction {
		info := (*kbdllHookStruct)(unsafe.Pointer(lParam))
		if info != nil && info.DwExtraInfo == probeExtraInfo {
			windowsNativeCaptureLock.RLock()
			session := windowsNativeCapture
			windowsNativeCaptureLock.RUnlock()
			if session != nil {
				session.recordProbeResponse("keyboard")
			}
			result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
			return result
		}
		if wParam == wmKeyDown || wParam == wmSysKeyDown {
			windowsNativeCaptureLock.RLock()
			session := windowsNativeCapture
			windowsNativeCaptureLock.RUnlock()
			if session != nil {
				session.recordUserActivity("keyboard")
			}
			emitWindowsNativeEvent("keyboard", "")
		}
	}
	result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return result
}

func lowLevelMouseCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code == hcAction {
		info := (*msllHookStruct)(unsafe.Pointer(lParam))
		if info != nil && info.DwExtraInfo == probeExtraInfo {
			windowsNativeCaptureLock.RLock()
			session := windowsNativeCapture
			windowsNativeCaptureLock.RUnlock()
			if session != nil {
				session.recordProbeResponse("mouse")
			}
			result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
			return result
		}
		switch wParam {
		case wmLButtonDown:
			windowsNativeCaptureLock.RLock()
			session := windowsNativeCapture
			windowsNativeCaptureLock.RUnlock()
			if session != nil {
				session.recordUserActivity("mouse")
			}
			emitWindowsNativeEvent("mouse", "left")
		case wmRButtonDown:
			windowsNativeCaptureLock.RLock()
			session := windowsNativeCapture
			windowsNativeCaptureLock.RUnlock()
			if session != nil {
				session.recordUserActivity("mouse")
			}
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


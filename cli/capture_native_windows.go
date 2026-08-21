//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL      = 13
	whMouseLL         = 14
	hcAction          = 0
	wmKeyDown         = 0x0100
	wmSysKeyDown      = 0x0104
	wmLButtonDown     = 0x0201
	wmRButtonDown     = 0x0204
	wmQuit            = 0x0012
	wmUserRehook      = 0x0401
	pmNoRemove        = 0x0000
	hookProbeMagic    = 0x434C494B // 'CLIK' in ASCII
	maxRehookAttempts = 5
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	advapi32                     = windows.NewLazySystemDLL("advapi32.dll")
	procSetWindowsHookExW        = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx           = user32.NewProc("CallNextHookEx")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procPeekMessageW             = user32.NewProc("PeekMessageW")
	procPostThreadMessageW       = user32.NewProc("PostThreadMessageW")
	procSendInput                = user32.NewProc("SendInput")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetCurrentThreadID       = kernel32.NewProc("GetCurrentThreadId")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procOpenProcessToken         = advapi32.NewProc("OpenProcessToken")
	procGetTokenInformation      = advapi32.NewProc("GetTokenInformation")

	windowsKeyboardCallback  = syscall.NewCallback(lowLevelKeyboardCallback)
	windowsMouseCallback     = syscall.NewCallback(lowLevelMouseCallback)
	windowsNativeCaptureLock sync.RWMutex
	windowsNativeCapture     *windowsCaptureSession
)

type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msLLHookStruct struct {
	Pt          windowsPoint
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type tokenElevationStruct struct {
	TokenIsElevated uint32
}

type windowsCaptureSession struct {
	dispatch         *nativeCaptureDispatcher
	sharing          SharingConfig
	capture          *ActivityCapture
	lastCallbackNano int64 // atomic UnixNano
	threadID         uint32
	keyboardHook     uintptr
	mouseHook        uintptr
	rehookFailures   int
	uipiPaused       bool
	isRecovering     bool
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
		c.updateState(CaptureState{Mode: "windows-native"})
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
		dispatch:         newNativeCaptureDispatcher(c),
		sharing:          sharing,
		capture:          c,
		threadID:         uint32(threadID),
		keyboardHook:     keyboardHook,
		mouseHook:        mouseHook,
		lastCallbackNano: time.Now().UnixNano(),
	}

	windowsNativeCaptureLock.Lock()
	windowsNativeCapture = session
	windowsNativeCaptureLock.Unlock()

	defer func() {
		if keyboardHook != 0 {
			procUnhookWindowsHookEx.Call(keyboardHook)
		}
		if mouseHook != 0 {
			procUnhookWindowsHookEx.Call(mouseHook)
		}
		windowsNativeCaptureLock.Lock()
		windowsNativeCapture = nil
		windowsNativeCaptureLock.Unlock()
		session.dispatch.stop()
	}()

	ready <- windowsHookStart{threadID: uint32(threadID)}

	go session.runWatchdog(ctx)

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
		if message.Message == wmUserRehook {
			rehookWindowsHooks(module, sharing, &keyboardHook, &mouseHook, session)
		}
	}
}

func rehookWindowsHooks(module uintptr, sharing SharingConfig, kbdHook *uintptr, msHook *uintptr, session *windowsCaptureSession) {
	if *kbdHook != 0 {
		procUnhookWindowsHookEx.Call(*kbdHook)
		*kbdHook = 0
	}
	if *msHook != 0 {
		procUnhookWindowsHookEx.Call(*msHook)
		*msHook = 0
	}

	var newKbdHook, newMsHook uintptr
	if sharing.Keyboard {
		newKbdHook, _, _ = procSetWindowsHookExW.Call(whKeyboardLL, windowsKeyboardCallback, module, 0)
		if newKbdHook == 0 {
			return
		}
	}
	if sharing.Mouse {
		newMsHook, _, _ = procSetWindowsHookExW.Call(whMouseLL, windowsMouseCallback, module, 0)
		if newMsHook == 0 {
			if newKbdHook != 0 {
				procUnhookWindowsHookEx.Call(newKbdHook)
			}
			return
		}
	}

	*kbdHook = newKbdHook
	*msHook = newMsHook

	windowsNativeCaptureLock.Lock()
	session.keyboardHook = newKbdHook
	session.mouseHook = newMsHook
	atomic.StoreInt64(&session.lastCallbackNano, time.Now().UnixNano())
	windowsNativeCaptureLock.Unlock()
}

func recordHookCallbackTime() {
	windowsNativeCaptureLock.RLock()
	session := windowsNativeCapture
	windowsNativeCaptureLock.RUnlock()
	if session != nil {
		atomic.StoreInt64(&session.lastCallbackNano, time.Now().UnixNano())
	}
}

func lowLevelKeyboardCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code == hcAction {
		recordHookCallbackTime()
		if lParam != 0 {
			kbd := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
			if kbd.DwExtraInfo == hookProbeMagic {
				result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
				return result
			}
		}
		if wParam == wmKeyDown || wParam == wmSysKeyDown {
			emitWindowsNativeEvent("keyboard", "")
		}
	}
	result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return result
}

func lowLevelMouseCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code == hcAction {
		recordHookCallbackTime()
		if lParam != 0 {
			ms := (*msLLHookStruct)(unsafe.Pointer(lParam))
			if ms.DwExtraInfo == hookProbeMagic {
				result, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
				return result
			}
		}
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

func (s *windowsCaptureSession) runWatchdog(ctx context.Context) {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkHealthAndProbing(ctx)
		}
	}
}

func (s *windowsCaptureSession) checkHealthAndProbing(ctx context.Context) {
	windowsNativeCaptureLock.Lock()
	if windowsNativeCapture != s {
		windowsNativeCaptureLock.Unlock()
		return
	}
	sharing := s.sharing
	capture := s.capture
	threadID := s.threadID
	uipiWasPaused := s.uipiPaused
	windowsNativeCaptureLock.Unlock()

	// 1. Check UIPI Foreground Window state
	if isForegroundWindowElevated() {
		windowsNativeCaptureLock.Lock()
		s.uipiPaused = true
		windowsNativeCaptureLock.Unlock()
		if capture != nil {
			capture.updateState(CaptureState{
				Mode:           "uipi-paused",
				PermissionHint: "Capture paused while Administrator window is focused (UIPI).",
			})
		}
		return
	}

	// Foreground window is standard. If previously UIPI paused, restore state to active.
	if uipiWasPaused {
		windowsNativeCaptureLock.Lock()
		s.uipiPaused = false
		windowsNativeCaptureLock.Unlock()
		if capture != nil {
			capture.updateState(CaptureState{
				Mode:           "windows-native",
				PermissionHint: "",
			})
		}
	}

	// 2. Check Hook Health via Probe
	lastNano := atomic.LoadInt64(&s.lastCallbackNano)
	lastAt := time.Unix(0, lastNano)

	// If no activity in the last 2 seconds, send a probe input
	if time.Since(lastAt) > 2*time.Second {
		if !sendHookProbeInput(sharing.Keyboard, sharing.Mouse) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(250 * time.Millisecond):
		}

		newNano := atomic.LoadInt64(&s.lastCallbackNano)
		if newNano == lastNano {
			// Hook did not receive probe -> DETACHED!
			s.handleHookDetachment(ctx, capture, threadID, sharing)
			return
		}
	}

	// If probe succeeded or recent activity occurred, restore state to active if recovering
	windowsNativeCaptureLock.Lock()
	if s.isRecovering {
		s.isRecovering = false
		s.rehookFailures = 0
		if capture != nil {
			capture.updateState(CaptureState{
				Mode:           "windows-native",
				PermissionHint: "",
			})
		}
	}
	windowsNativeCaptureLock.Unlock()
}

func (s *windowsCaptureSession) handleHookDetachment(ctx context.Context, capture *ActivityCapture, threadID uint32, sharing SharingConfig) {
	windowsNativeCaptureLock.Lock()
	s.isRecovering = true
	s.rehookFailures++
	currentFailures := s.rehookFailures
	windowsNativeCaptureLock.Unlock()

	if currentFailures > maxRehookAttempts {
		if capture != nil {
			capture.updateState(CaptureState{
				Mode:           "off",
				PermissionHint: "Windows low-level hook registration failed after multiple recovery attempts.",
			})
		}
		return
	}

	if capture != nil {
		capture.updateState(CaptureState{
			Mode:           "recovering",
			PermissionHint: fmt.Sprintf("Restoring dropped Windows low-level input hooks (attempt %d/%d)...", currentFailures, maxRehookAttempts),
		})
	}

	// Calculate exponential backoff delay before posting re-hook message
	backoff := exponentialBackoffDelay(currentFailures)
	select {
	case <-ctx.Done():
		return
	case <-time.After(backoff):
	}

	// Post message to OS thread to re-register hooks
	procPostThreadMessageW.Call(uintptr(threadID), wmUserRehook, 0, 0)
}

func exponentialBackoffDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 500 * time.Millisecond
	}
	delay := 500 * time.Millisecond * time.Duration(1<<(attempt-1))
	if delay > 10*time.Second {
		delay = 10 * time.Second
	}
	return delay
}

func sendHookProbeInput(keyboard bool, mouse bool) bool {
	if keyboard {
		if unsafe.Sizeof(uintptr(0)) == 8 {
			var inp [40]byte
			*(*uint32)(unsafe.Pointer(&inp[0])) = 1        // INPUT_KEYBOARD
			*(*uint16)(unsafe.Pointer(&inp[8])) = 0        // wVk = 0
			*(*uint32)(unsafe.Pointer(&inp[12])) = 0x0002 // KEYEVENTF_KEYUP
			*(*uintptr)(unsafe.Pointer(&inp[24])) = hookProbeMagic
			ret, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp[0])), uintptr(unsafe.Sizeof(inp)))
			return ret > 0
		} else {
			var inp [28]byte
			*(*uint32)(unsafe.Pointer(&inp[0])) = 1        // INPUT_KEYBOARD
			*(*uint16)(unsafe.Pointer(&inp[4])) = 0        // wVk = 0
			*(*uint32)(unsafe.Pointer(&inp[8])) = 0x0002  // KEYEVENTF_KEYUP
			*(*uintptr)(unsafe.Pointer(&inp[16])) = hookProbeMagic
			ret, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp[0])), uintptr(unsafe.Sizeof(inp)))
			return ret > 0
		}
	} else if mouse {
		if unsafe.Sizeof(uintptr(0)) == 8 {
			var inp [40]byte
			*(*uint32)(unsafe.Pointer(&inp[0])) = 0        // INPUT_MOUSE
			*(*uint32)(unsafe.Pointer(&inp[20])) = 0x0001 // MOUSEEVENTF_MOVE
			*(*uintptr)(unsafe.Pointer(&inp[32])) = hookProbeMagic
			ret, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp[0])), uintptr(unsafe.Sizeof(inp)))
			return ret > 0
		} else {
			var inp [28]byte
			*(*uint32)(unsafe.Pointer(&inp[0])) = 0        // INPUT_MOUSE
			*(*uint32)(unsafe.Pointer(&inp[16])) = 0x0001 // MOUSEEVENTF_MOVE
			*(*uintptr)(unsafe.Pointer(&inp[24])) = hookProbeMagic
			ret, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp[0])), uintptr(unsafe.Sizeof(inp)))
			return ret > 0
		}
	}
	return false
}

func isForegroundWindowElevated() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid <= 4 || pid == uint32(os.Getpid()) {
		return false
	}
	// PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	hProcess, _, err := procOpenProcess.Call(0x1000, 0, uintptr(pid))
	if hProcess == 0 {
		if err != nil && isAccessDenied(err) {
			return true
		}
		return false
	}
	defer procCloseHandle.Call(hProcess)

	var hToken uintptr
	// TOKEN_QUERY = 0x0008
	ret, _, err := procOpenProcessToken.Call(hProcess, 0x0008, uintptr(unsafe.Pointer(&hToken)))
	if ret == 0 {
		if err != nil && isAccessDenied(err) {
			return true
		}
		return false
	}
	defer procCloseHandle.Call(hToken)

	var elevation tokenElevationStruct
	var returnLength uint32
	// TokenElevation = 20
	ret, _, _ = procGetTokenInformation.Call(
		hToken,
		20,
		uintptr(unsafe.Pointer(&elevation)),
		uintptr(unsafe.Sizeof(elevation)),
		uintptr(unsafe.Pointer(&returnLength)),
	)
	if ret == 0 {
		return false
	}
	return elevation.TokenIsElevated != 0
}

func isAccessDenied(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == windows.ERROR_ACCESS_DENIED
	}
	return strings.Contains(strings.ToLower(err.Error()), "access is denied")
}

func globalHookPermissionHint() string {
	return "Capture may pause only while an Administrator window is focused (Windows security)."
}

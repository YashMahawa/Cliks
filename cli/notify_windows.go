//go:build windows

package main

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	nimAdd         = 0x00000000
	nimModify      = 0x00000001
	nimDelete      = 0x00000002
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	nifInfo        = 0x00000010
	niifInfo       = 0x00000001
	niifNoSound    = 0x00000010
	wmApp          = 0x8000
	idiApplication = 32512
)

var (
	windowsShell32             = windows.NewLazySystemDLL("shell32.dll")
	windowsUser32Notify        = windows.NewLazySystemDLL("user32.dll")
	windowsKernel32Notify      = windows.NewLazySystemDLL("kernel32.dll")
	procShellNotifyIconW       = windowsShell32.NewProc("Shell_NotifyIconW")
	procCreateWindowExW        = windowsUser32Notify.NewProc("CreateWindowExW")
	procDestroyWindow          = windowsUser32Notify.NewProc("DestroyWindow")
	procLoadIconW              = windowsUser32Notify.NewProc("LoadIconW")
	procTranslateMessage       = windowsUser32Notify.NewProc("TranslateMessage")
	procDispatchMessageW       = windowsUser32Notify.NewProc("DispatchMessageW")
	procGetModuleHandleWNotify = windowsKernel32Notify.NewProc("GetModuleHandleW")
)

type windowsNotifyIconData struct {
	Size             uint32
	Window           windows.Handle
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             windows.Handle
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	ItemGUID         windows.GUID
	BalloonIcon      windows.Handle
}

func sendNativeNotification(title string, body string, sound bool) error {
	className, _ := windows.UTF16PtrFromString("STATIC")
	windowName, _ := windows.UTF16PtrFromString("Cliks Notifications")
	module, _, _ := procGetModuleHandleWNotify.Call(0)
	messageWindow := ^uintptr(2) // HWND_MESSAGE
	window, _, createErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0, 0, 0, 0, 0,
		messageWindow,
		0,
		module,
		0,
	)
	if window == 0 {
		return fmt.Errorf("Windows notification window could not be created: %v", createErr)
	}
	defer procDestroyWindow.Call(window)

	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	data := windowsNotifyIconData{
		Size:            uint32(unsafe.Sizeof(windowsNotifyIconData{})),
		Window:          windows.Handle(window),
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmApp + 1,
		Icon:            windows.Handle(icon),
	}
	copyWindowsUTF16(data.Tip[:], "Cliks")
	if result, _, callErr := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&data))); result == 0 {
		return fmt.Errorf("Windows notification icon could not be registered: %v", callErr)
	}
	defer procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&data)))

	data.Flags = nifInfo
	data.InfoFlags = niifInfo
	if !sound {
		data.InfoFlags |= niifNoSound
	}
	copyWindowsUTF16(data.InfoTitle[:], title)
	copyWindowsUTF16(data.Info[:], body)
	if result, _, callErr := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&data))); result == 0 {
		return fmt.Errorf("Windows notification could not be shown: %v", callErr)
	}
	// Pump the hidden owner's message queue while Windows presents the banner.
	// Incoming signals call this off the session loop, so it never delays relay IO.
	deadline := time.Now().Add(1500 * time.Millisecond)
	var message windowsMessage
	for time.Now().Before(deadline) {
		for {
			result, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), window, 0, 0, 1)
			if result == 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
		}
		time.Sleep(15 * time.Millisecond)
	}
	return nil
}

func copyWindowsUTF16(destination []uint16, value string) {
	if len(destination) == 0 {
		return
	}
	encoded, _ := windows.UTF16FromString(value)
	if len(encoded) > len(destination) {
		payload := len(destination) - 1
		// Do not split an emoji/supplementary rune's UTF-16 surrogate pair.
		if payload > 0 && encoded[payload-1] >= 0xD800 && encoded[payload-1] <= 0xDBFF {
			payload--
		}
		encoded = append(encoded[:payload:payload], 0)
	}
	copy(destination, encoded)
}

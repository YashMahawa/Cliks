//go:build windows

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ridInput  = 0x10000003
	ridHeader = 0x10000005

	rimTypeMouse    = 0
	rimTypeKeyboard = 1

	ridevInputSink = 0x00000100

	wmInput       = 0x00FF
	wmKeyDown     = 0x0100
	wmSysKeyDown  = 0x0104
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmQuit        = 0x0012

	riMouseLeftButtonDown  = 0x0001
	riMouseRightButtonDown = 0x0004
)

var (
	user32                      = windows.NewLazySystemDLL("user32.dll")
	kernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procGetMessageW             = user32.NewProc("GetMessageW")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procUnregisterClassW        = user32.NewProc("UnregisterClassW")
	procRegisterRawInputDevices = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData         = user32.NewProc("GetRawInputData")
	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")
)

type rawInputHeader struct {
	Type   uint32
	Size   uint32
	Device uintptr
	WParam uintptr
}

type rawKeybd struct {
	MakeCode         uint16
	Flags            uint16
	Reserved         uint16
	VKey             uint16
	Message          uint32
	ExtraInformation uint32
}

type rawMouse struct {
	Flags            uint16
	ButtonFlags      uint16
	ButtonData       uint16
	RawButtons       uint32
	LastX            int32
	LastY            int32
	ExtraInformation uint32
}

type rawInput struct {
	Header rawInputHeader
	Data   [32]byte
}

type rawInputDevice struct {
	UsagePage uint16
	Usage     uint16
	Flags     uint32
	Target    uintptr
}

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
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

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig, mode string) CaptureState {
	_ = mode
	helperExe, helperArgs := windowsCaptureHelperCommand()
	cmd := exec.CommandContext(ctx, helperExe, helperArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not create stdout pipe for Windows capture helper."}
	}
	if err := cmd.Start(); err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not start Windows capture helper process: " + err.Error()}
	}

	reader := bufio.NewReader(stdout)
	readyChan := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			readyChan <- fmt.Errorf("reading ready token: %w", err)
			return
		}
		if strings.TrimSpace(line) != "ready" {
			readyChan <- fmt.Errorf("unexpected initial token: %q", line)
			return
		}
		readyChan <- nil
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return CaptureState{Mode: "off", PermissionHint: ctx.Err().Error()}
	case err := <-readyChan:
		if err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return CaptureState{Mode: "off", PermissionHint: "Windows capture helper failed readiness check: " + err.Error()}
		}
	case <-time.After(180 * time.Millisecond):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return CaptureState{Mode: "off", PermissionHint: "Windows capture helper timed out during readiness probe."}
	}

	go func() {
		defer cmd.Wait()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			switch strings.TrimSpace(scanner.Text()) {
			case "k":
				if sharing.Keyboard {
					c.emit(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
				}
			case "l":
				if sharing.Mouse {
					c.emit(LocalActivityEvent{Kind: "mouse", Button: "left", At: time.Now()})
				}
			case "r":
				if sharing.Mouse {
					c.emit(LocalActivityEvent{Kind: "mouse", Button: "right", At: time.Now()})
				}
			}
		}
	}()

	return CaptureState{
		Mode:           "windows-isolated-helper",
		PermissionHint: "Windows capture helper active using Raw Input across normal and elevated windows.",
	}
}

func windowsCaptureHelperCommand() (string, []string) {
	if helper := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_HELPER")); helper != "" {
		return helper, []string{"--stdio"}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "cliks-capture.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, []string{"--stdio"}
		}
		return exe, []string{"capture-helper", "--stdio"}
	}
	return "cliks-capture.exe", []string{"--stdio"}
}

func runWindowsCaptureHelper(args []string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className, _ := windows.UTF16PtrFromString("CliksCaptureClass")
	windowName, _ := windows.UTF16PtrFromString("CliksCaptureWindow")

	module, _, _ := procGetModuleHandleW.Call(0)

	wndProc := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
		if msg == wmInput {
			var raw rawInput
			size := uint32(unsafe.Sizeof(raw))
			res, _, _ := procGetRawInputData.Call(
				lParam,
				uintptr(ridInput),
				uintptr(unsafe.Pointer(&raw)),
				uintptr(unsafe.Pointer(&size)),
				uintptr(unsafe.Sizeof(rawInputHeader{})),
			)
			if int32(res) >= 0 {
				if raw.Header.Type == rimTypeKeyboard {
					keybd := (*rawKeybd)(unsafe.Pointer(&raw.Data[0]))
					if keybd.Message == wmKeyDown || keybd.Message == wmSysKeyDown {
						fmt.Println("k")
					}
				} else if raw.Header.Type == rimTypeMouse {
					mouse := (*rawMouse)(unsafe.Pointer(&raw.Data[0]))
					if mouse.ButtonFlags&riMouseLeftButtonDown != 0 {
						fmt.Println("l")
					}
					if mouse.ButtonFlags&riMouseRightButtonDown != 0 {
						fmt.Println("r")
					}
				}
			}
		}
		res, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return res
	})

	var wc wndClassExW
	wc.Size = uint32(unsafe.Sizeof(wc))
	wc.WndProc = wndProc
	wc.Instance = module
	wc.ClassName = className

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwndMsgVal := uintptr(unsafe.Pointer(uintptr(0) - 3)) // HWND_MESSAGE

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		hwndMsgVal,
		0,
		module,
		0,
	)

	devices := []rawInputDevice{
		{
			UsagePage: 0x01,
			Usage:     0x06,
			Flags:     ridevInputSink,
			Target:    hwnd,
		},
		{
			UsagePage: 0x01,
			Usage:     0x02,
			Flags:     ridevInputSink,
			Target:    hwnd,
		},
	}

	res, _, err := procRegisterRawInputDevices.Call(
		uintptr(unsafe.Pointer(&devices[0])),
		uintptr(len(devices)),
		uintptr(unsafe.Sizeof(devices[0])),
	)
	if res == 0 {
		return fmt.Errorf("RegisterRawInputDevices failed: %v", err)
	}

	fmt.Println("ready")

	go func() {
		buf := make([]byte, 1)
		_, _ = os.Stdin.Read(buf)
		os.Exit(0)
	}()

	var msg windowsMessage
	for {
		res, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(res) <= 0 {
			break
		}
	}

	if hwnd != 0 {
		procDestroyWindow.Call(hwnd)
	}
	procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), module)
	return nil
}

func globalHookPermissionHint() string {
	return "Raw Input IPC helper captures keyboard/mouse activity across normal and elevated windows without pauses."
}


package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"golang.org/x/term"
)

type LocalActivityEvent struct {
	Kind   string
	At     time.Time
	Button string
}

type CaptureState struct {
	Mode           string
	PermissionHint string
}

type ActivityCapture struct {
	Events           chan LocalActivityEvent
	cancel           context.CancelFunc
	mu               sync.Mutex
	terminalOldState *term.State
}

func newActivityCapture() *ActivityCapture {
	return &ActivityCapture{Events: make(chan LocalActivityEvent, 256)}
}

func (c *ActivityCapture) start(parent context.Context, sharing SharingConfig, mode string) CaptureState {
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	if !sharing.Keyboard && !sharing.Mouse {
		return CaptureState{Mode: "off"}
	}
	if mode == "terminal" {
		if err := c.startTerminal(ctx, sharing); err != nil {
			return CaptureState{Mode: "off", PermissionHint: err.Error()}
		}
		return CaptureState{Mode: "terminal"}
	}
	if runtime.GOOS == "linux" {
		if state := c.startEvdev(ctx, sharing); state.Mode == "evdev" || mode == "evdev" {
			return state
		}
	}
	if mode == "evdev" {
		return CaptureState{Mode: "off", PermissionHint: "Linux evdev capture is only available on Linux desktops with readable /dev/input/event* devices."}
	}
	if runtime.GOOS == "darwin" {
		return CaptureState{Mode: "off", PermissionHint: "macOS global capture will use native Event Tap in a future Go build. For now use: cliks start --terminal --self"}
	}
	if runtime.GOOS == "windows" {
		return CaptureState{Mode: "off", PermissionHint: "Windows global capture will use native low-level hooks in a future Go build. For now use: cliks start --terminal --self"}
	}
	return CaptureState{Mode: "off", PermissionHint: "Global capture is not available in this environment. Try: cliks start --terminal --self"}
}

func (c *ActivityCapture) stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.restoreTerminal()
}

func (c *ActivityCapture) startTerminal(ctx context.Context, sharing SharingConfig) error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("terminal capture needs an interactive terminal")
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.terminalOldState = oldState
	c.mu.Unlock()
	if sharing.Mouse {
		fmt.Print("\x1b[?1000h\x1b[?1006h")
	}
	go func() {
		defer c.restoreTerminal()
		buf := make([]byte, 256)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n == 0 {
				continue
			}
			text := string(buf[:n])
			if text == "\x03" {
				_ = os.Stdin.Close()
				return
			}
			withoutMouse := terminalMousePattern.ReplaceAllStringFunc(text, func(match string) string {
				if !sharing.Mouse {
					return ""
				}
				button := terminalMouseButton(match)
				if button != "" {
					c.emit(LocalActivityEvent{Kind: "mouse", At: time.Now(), Button: button})
				}
				return ""
			})
			if sharing.Keyboard && withoutMouse != "" {
				c.emit(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
			}
		}
	}()
	return nil
}

func (c *ActivityCapture) restoreTerminal() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.terminalOldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), c.terminalOldState)
		c.terminalOldState = nil
		fmt.Print("\x1b[?1000l\x1b[?1006l")
	}
}

func (c *ActivityCapture) startEvdev(ctx context.Context, sharing SharingConfig) CaptureState {
	if runtime.GOOS != "linux" {
		return CaptureState{Mode: "off"}
	}
	devices, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(devices) == 0 {
		return CaptureState{Mode: "off", PermissionHint: "Linux global capture could not find /dev/input/event* devices."}
	}
	opened := 0
	hint := ""
	for _, device := range devices {
		file, err := os.Open(device)
		if err != nil {
			if os.IsPermission(err) {
				hint = "Linux global capture needs permission to read /dev/input/event*. Add your user to the input group, then log out/in: sudo usermod -aG input $USER"
			}
			continue
		}
		opened++
		go c.readEvdev(ctx, file, sharing)
	}
	if opened == 0 {
		if hint == "" {
			hint = "Linux global capture could not open /dev/input/event*. Try: sudo usermod -aG input $USER, then log out and back in."
		}
		return CaptureState{Mode: "off", PermissionHint: hint}
	}
	return CaptureState{Mode: "evdev", PermissionHint: hint}
}

func (c *ActivityCapture) readEvdev(ctx context.Context, file *os.File, sharing SharingConfig) {
	defer file.Close()
	buf := make([]byte, 24*32)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}
		c.handleEvdev(buf[:n], sharing)
	}
}

func (c *ActivityCapture) handleEvdev(chunk []byte, sharing SharingConfig) {
	eventSize := 0
	if len(chunk)%24 == 0 {
		eventSize = 24
	} else if len(chunk)%16 == 0 {
		eventSize = 16
	}
	if eventSize == 0 {
		return
	}
	for offset := 0; offset+eventSize <= len(chunk); offset += eventSize {
		eventType := binary.LittleEndian.Uint16(chunk[offset+eventSize-8:])
		code := binary.LittleEndian.Uint16(chunk[offset+eventSize-6:])
		value := int32(binary.LittleEndian.Uint32(chunk[offset+eventSize-4:]))
		if eventType != 1 || value != 1 {
			continue
		}
		if isMouseButtonCode(code) {
			if sharing.Mouse {
				c.emit(LocalActivityEvent{Kind: "mouse", At: time.Now(), Button: mouseButtonFromEvdevCode(code)})
			}
			continue
		}
		if sharing.Keyboard && isKeyboardKeyCode(code) {
			c.emit(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
		}
	}
}

func (c *ActivityCapture) emit(event LocalActivityEvent) {
	select {
	case c.Events <- event:
	default:
	}
}

func repairTerminal() {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print("\x1b[?1000l\x1b[?1006l")
	}
}

var terminalMousePattern = regexp.MustCompile(`\x1b\[<(\d+);(\d+);(\d+)([mM])`)

func terminalMouseButton(match string) string {
	parts := terminalMousePattern.FindStringSubmatch(match)
	if len(parts) != 5 || parts[4] != "M" {
		return ""
	}
	var code int
	_, _ = fmt.Sscanf(parts[1], "%d", &code)
	if code&32 != 0 || code&64 != 0 {
		return ""
	}
	switch code & 3 {
	case 0:
		return "left"
	case 2:
		return "right"
	default:
		return ""
	}
}

func isMouseButtonCode(code uint16) bool {
	return code == 0x110 || code == 0x111
}

func isKeyboardKeyCode(code uint16) bool {
	return code >= 1 && code <= 0xff
}

func mouseButtonFromEvdevCode(code uint16) string {
	if code == 0x110 {
		return "left"
	}
	if code == 0x111 {
		return "right"
	}
	return "unknown"
}

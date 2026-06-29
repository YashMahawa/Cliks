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

const (
	evSyn = 0
	evKey = 1
	evAbs = 3

	btnLeft       = 0x110
	btnRight      = 0x111
	btnTouch      = 0x14a
	btnToolFinger = 0x145
	btnToolDouble = 0x14d
	btnToolTriple = 0x14e
	btnToolQuad   = 0x14f

	absX           = 0x00
	absY           = 0x01
	absMTPositionX = 0x35
	absMTPositionY = 0x36
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
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.Mutex
	terminalOldState *term.State
}

func newActivityCapture() *ActivityCapture {
	return &ActivityCapture{Events: make(chan LocalActivityEvent, 1024), ctx: context.Background()}
}

func (c *ActivityCapture) start(parent context.Context, sharing SharingConfig, mode string) CaptureState {
	ctx, cancel := context.WithCancel(parent)
	c.ctx = ctx
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
	touchpad := &touchpadTapDetector{}
	consecutiveErrors := 0
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
			consecutiveErrors++
			timer := time.NewTimer(evdevRetryDelay(consecutiveErrors))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		consecutiveErrors = 0
		c.handleEvdev(buf[:n], sharing, touchpad)
	}
}

func evdevRetryDelay(consecutiveErrors int) time.Duration {
	return randomizedRetryDelay(consecutiveErrors)
}

func (c *ActivityCapture) handleEvdev(chunk []byte, sharing SharingConfig, touchpad *touchpadTapDetector) {
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

		now := time.Now()
		if sharing.Mouse && touchpad != nil {
			if button := touchpad.handle(eventType, code, value, now); button != "" {
				c.emit(LocalActivityEvent{Kind: "mouse", At: now, Button: button})
			}
		}

		if eventType != evKey || value != 1 {
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
	case <-c.ctx.Done():
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
	return code == btnLeft || code == btnRight
}

func isKeyboardKeyCode(code uint16) bool {
	return code >= 1 && code <= 0xff
}

func mouseButtonFromEvdevCode(code uint16) string {
	if code == btnLeft {
		return "left"
	}
	if code == btnRight {
		return "right"
	}
	return "unknown"
}

type touchpadTapDetector struct {
	touching          bool
	fingerCount       int
	tooManyFingers    bool
	buttonDuringTouch bool
	startedAt         time.Time
	startX            int32
	startY            int32
	x                 int32
	y                 int32
	havePos           bool
}

func (t *touchpadTapDetector) handle(eventType uint16, code uint16, value int32, now time.Time) string {
	switch eventType {
	case evAbs:
		switch code {
		case absX, absMTPositionX:
			t.x = value
			if !t.touching {
				t.startX = value
			}
			t.havePos = true
		case absY, absMTPositionY:
			t.y = value
			if !t.touching {
				t.startY = value
			}
			t.havePos = true
		}
	case evKey:
		switch code {
		case btnLeft, btnRight:
			if value > 0 && t.touching {
				t.buttonDuringTouch = true
			}
		case btnToolDouble:
			if value > 0 {
				t.fingerCount = maxInt(t.fingerCount, 2)
			}
		case btnToolTriple, btnToolQuad:
			if value > 0 {
				t.tooManyFingers = true
			}
		case btnToolFinger:
			if value == 1 && !t.touching {
				t.fingerCount = maxInt(t.fingerCount, 1)
				t.tooManyFingers = false
				t.buttonDuringTouch = false
			}
		case btnTouch:
			if value == 1 {
				t.touching = true
				if t.fingerCount == 0 {
					t.fingerCount = 1
				}
				t.buttonDuringTouch = false
				t.startedAt = now
				t.startX = t.x
				t.startY = t.y
				return ""
			}
			if value == 0 && t.touching {
				button := t.tapButton(now)
				t.touching = false
				t.fingerCount = 0
				t.tooManyFingers = false
				t.buttonDuringTouch = false
				return button
			}
		}
	}
	return ""
}

func (t *touchpadTapDetector) tapButton(now time.Time) string {
	if t.tooManyFingers || t.buttonDuringTouch || t.startedAt.IsZero() {
		return ""
	}
	if now.Sub(t.startedAt) > 260*time.Millisecond {
		return ""
	}
	if !t.havePos {
		return t.buttonForFingerCount()
	}
	const maxTapTravel = 90
	if abs32(t.x-t.startX) > maxTapTravel || abs32(t.y-t.startY) > maxTapTravel {
		return ""
	}
	return t.buttonForFingerCount()
}

func (t *touchpadTapDetector) buttonForFingerCount() string {
	if t.fingerCount == 2 {
		return "right"
	}
	return "left"
}

func abs32(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}

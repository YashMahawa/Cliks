//go:build linux

package main

import (
	"bufio"
	"context"
	"net"
	"os"
	"strings"
	"time"
)

func linuxCaptureSocket() string {
	if value := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_SOCKET")); value != "" {
		return value
	}
	return "/run/cliks/capture.sock"
}

func (c *ActivityCapture) startLinuxCaptureHelper(ctx context.Context, sharing SharingConfig) CaptureState {
	conn, err := net.DialTimeout("unix", linuxCaptureSocket(), 1200*time.Millisecond)
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Isolated Linux capture is not ready. Run cliks setup. Direct /dev/input access is available only as the explicitly less-safe fallback: cliks set capture.mode direct"}
	}
	go func() {
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			switch scanner.Text() {
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
	return CaptureState{Mode: "linux-isolated-helper", PermissionHint: "Isolated helper sends only keyboard, left-click, and right-click activity kinds."}
}

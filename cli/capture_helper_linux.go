//go:build linux

package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func linuxCaptureSocket() string {
	if value := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_SOCKET")); value != "" {
		return value
	}
	return "/run/cliks/capture.sock"
}

func linuxCaptureTokenPath() string {
	if value := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_TOKEN_FILE")); value != "" {
		return value
	}
	socket := linuxCaptureSocket()
	return filepath.Join(filepath.Dir(socket), "token")
}

func (c *ActivityCapture) startLinuxCaptureHelper(ctx context.Context, sharing SharingConfig) CaptureState {
	tokenPath := linuxCaptureTokenPath()
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Isolated Linux capture token not found. Run cliks setup."}
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return CaptureState{Mode: "off", PermissionHint: "Isolated Linux capture token is empty."}
	}

	conn, err := net.DialTimeout("unix", linuxCaptureSocket(), 1200*time.Millisecond)
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Isolated Linux capture is not ready. Run cliks setup. Direct /dev/input access is available only as the explicitly less-safe fallback: cliks set capture.mode direct"}
	}

	_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := io.WriteString(conn, token+"\n"); err != nil {
		_ = conn.Close()
		return CaptureState{Mode: "off", PermissionHint: "Failed to send authorization token to capture service."}
	}

	reader := bufio.NewReader(conn)
	resp, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(resp) != "ready" {
		_ = conn.Close()
		return CaptureState{Mode: "off", PermissionHint: "Capture service rejected handshake token."}
	}
	_ = conn.SetDeadline(time.Time{})

	go func() {
		defer conn.Close()
		scanner := bufio.NewScanner(reader)
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

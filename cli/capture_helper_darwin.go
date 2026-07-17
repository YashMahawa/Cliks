//go:build darwin

package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig, mode string) CaptureState {
	if mode == "direct" {
		return c.startDirectGlobalHook(ctx, sharing)
	}
	helper := macCaptureHelperPath()
	if helper == "" {
		return CaptureState{Mode: "off", PermissionHint: "Cliks Capture.app is missing. Run cliks setup or reinstall. You can temporarily opt into the less-safe terminal permission with: cliks set capture.mode direct"}
	}
	cmd := exec.CommandContext(ctx, helper, "--stdio")
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not start Cliks Capture.app. Run cliks setup; direct compatibility mode remains available in Capture safety."}
	}
	go func() {
		defer cmd.Wait()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
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
	return CaptureState{Mode: "macos-isolated-app", PermissionHint: "Input Monitoring belongs to Cliks Capture.app, not your terminal. Remove it later in System Settings at any time."}
}

func macCaptureHelperPath() string {
	candidates := []string{strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_HELPER"))}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Applications", "Cliks Capture.app", "Contents", "MacOS", "cliks-capture"))
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "Cliks Capture.app", "Contents", "MacOS", "cliks-capture"))
	}
	for _, candidate := range candidates {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func macCaptureHelperReady() bool {
	helper := macCaptureHelperPath()
	if helper == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper, "--stdio")
	if err := cmd.Start(); err != nil {
		return false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return false
	case <-time.After(350 * time.Millisecond):
		_ = cmd.Process.Kill()
		return true
	}
}

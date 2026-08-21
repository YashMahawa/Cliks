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
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not open Cliks Capture.app output. Run cliks setup."}
	}
	if err := cmd.Start(); err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not start Cliks Capture.app. Run cliks setup; direct compatibility mode remains available in Capture safety."}
	}
	scanner := bufio.NewScanner(stdout)
	ready := make(chan bool, 1)
	go func() {
		defer cmd.Wait()
		if !scanner.Scan() || scanner.Text() != "ready" {
			ready <- false
			return
		}
		ready <- true
		for scanner.Scan() {
			switch scanner.Text() {
			case "k":
				if c.getSharing().Keyboard {
					c.emit(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
				}
			case "l":
				if c.getSharing().Mouse {
					c.emit(LocalActivityEvent{Kind: "mouse", Button: "left", At: time.Now()})
				}
			case "r":
				if c.getSharing().Mouse {
					c.emit(LocalActivityEvent{Kind: "mouse", Button: "right", At: time.Now()})
				}
			}
		}
	}()
	select {
	case ok := <-ready:
		if !ok {
			return CaptureState{Mode: "off", PermissionHint: "Cliks Capture could not start its input tap. Allow Cliks Capture.app in Input Monitoring, then restart Cliks."}
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		return CaptureState{Mode: "off", PermissionHint: "Cliks Capture is waiting for Input Monitoring. Approve the macOS prompt, then restart Cliks."}
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return CaptureState{Mode: "off", PermissionHint: "Capture stopped."}
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper, "--stdio")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready <- scanner.Scan() && scanner.Text() == "ready"
	}()
	select {
	case ok := <-ready:
		return ok
	case <-ctx.Done():
		return false
	}
}

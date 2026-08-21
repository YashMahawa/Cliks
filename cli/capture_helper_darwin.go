//go:build darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig, mode string) CaptureState {
	if mode == "direct" {
		return c.startDirectGlobalHook(ctx, sharing)
	}
	helper := macCaptureHelperPath()
	if helper == "" {
		return CaptureState{Mode: "off", PermissionHint: "Cliks Capture.app helper bundle is missing or unlinked. Run cliks setup or reinstall Cliks. Direct mode is available: cliks set capture.mode direct"}
	}
	cmd := exec.CommandContext(ctx, helper, "--stdio")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not create stdout pipe for Cliks Capture.app: " + err.Error()}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not create stderr pipe for Cliks Capture.app: " + err.Error()}
	}
	if err := cmd.Start(); err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Could not start Cliks Capture.app: " + err.Error()}
	}

	var stderrBuf bytes.Buffer
	var stderrMu sync.Mutex
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			if stderrBuf.Len() > 0 {
				stderrBuf.WriteString("\n")
			}
			stderrBuf.WriteString(line)
			stderrMu.Unlock()
		}
	}()

	readyChan := make(chan struct{})
	cmdDone := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		firstLine := true
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if firstLine {
				firstLine = false
				if line == "ready" {
					close(readyChan)
					continue
				}
			}
			switch line {
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

	go func() {
		cmdDone <- cmd.Wait()
	}()

	launchTimeout := 3 * time.Second
	select {
	case <-readyChan:
		return CaptureState{Mode: "macos-isolated-app", PermissionHint: "Input Monitoring belongs to Cliks Capture.app, not your terminal. Remove it later in System Settings at any time."}
	case err := <-cmdDone:
		stderrMu.Lock()
		stderrStr := strings.TrimSpace(stderrBuf.String())
		stderrMu.Unlock()
		if stderrStr == "" {
			if err != nil {
				stderrStr = fmt.Sprintf("Cliks Capture helper process exited unexpectedly (%v).", err)
			} else {
				stderrStr = "Cliks Capture helper process exited without emitting ready token."
			}
		}
		return CaptureState{Mode: "off", PermissionHint: stderrStr}
	case <-time.After(launchTimeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		stderrMu.Lock()
		stderrStr := strings.TrimSpace(stderrBuf.String())
		stderrMu.Unlock()
		if stderrStr == "" {
			stderrStr = "Cliks Capture initialization timed out waiting for ready token."
		}
		return CaptureState{Mode: "off", PermissionHint: stderrStr}
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return CaptureState{Mode: "off", PermissionHint: "Capture cancelled during startup."}
	}
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
	readyChan := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "ready" {
				close(readyChan)
			}
		}
	}()
	select {
	case <-readyChan:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return true
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return false
	}
}

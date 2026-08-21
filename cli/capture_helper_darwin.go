//go:build darwin

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const macCaptureHelperExpectedBundleID = "io.cliks.capture"

var (
	macHelperOverrideWarningMu sync.Mutex
	macHelperOverrideWarning   string

	macCodeSignatureVerifier = defaultMacCodeSignatureVerifier
)

func setMacHelperOverrideWarning(msg string) {
	macHelperOverrideWarningMu.Lock()
	defer macHelperOverrideWarningMu.Unlock()
	macHelperOverrideWarning = msg
}

func getMacHelperOverrideWarning() string {
	macHelperOverrideWarningMu.Lock()
	defer macHelperOverrideWarningMu.Unlock()
	return macHelperOverrideWarning
}

func defaultMacCodeSignatureVerifier(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codesign", "-v", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr == "" {
			outStr = err.Error()
		}
		return fmt.Errorf("code signature invalid: %s", outStr)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	cmd2 := exec.CommandContext(ctx2, "codesign", "-dv", path)
	out2, _ := cmd2.CombinedOutput()
	out2Str := string(out2)

	identifier := parseBundleIdentifierFromCodesignOutput(out2Str)
	if identifier == "" {
		identifier = parseBundleIdentifierFromInfoPlist(path)
	}

	if identifier == "" {
		return fmt.Errorf("bundle identifier missing")
	}
	if identifier != macCaptureHelperExpectedBundleID {
		return fmt.Errorf("bundle identifier mismatch: expected %q, got %q", macCaptureHelperExpectedBundleID, identifier)
	}

	return nil
}

func parseBundleIdentifierFromCodesignOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Identifier=") {
			return strings.TrimPrefix(line, "Identifier=")
		}
	}
	return ""
}

func parseBundleIdentifierFromInfoPlist(execPath string) string {
	dir := filepath.Dir(execPath)
	if filepath.Base(dir) == "MacOS" {
		infoPlistPath := filepath.Join(filepath.Dir(dir), "Info.plist")
		data, err := os.ReadFile(infoPlistPath)
		if err == nil {
			content := string(data)
			if idx := strings.Index(content, "<key>CFBundleIdentifier</key>"); idx != -1 {
				rest := content[idx:]
				if strIdx := strings.Index(rest, "<string>"); strIdx != -1 {
					rest = rest[strIdx+len("<string>"):]
					if endIdx := strings.Index(rest, "</string>"); endIdx != -1 {
						return strings.TrimSpace(rest[:endIdx])
					}
				}
			}
		}
	}
	return ""
}

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig, mode string) CaptureState {
	if mode == "direct" {
		return c.startDirectGlobalHook(ctx, sharing)
	}
	helper := macCaptureHelperPath()
	if helper == "" {
		return c.startDirectGlobalHook(ctx, sharing)
	}
	cmd := exec.CommandContext(ctx, helper, "--stdio")
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return c.startDirectGlobalHook(ctx, sharing)
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
	envOverride := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_HELPER"))
	if envOverride != "" {
		if info, err := os.Stat(envOverride); err != nil || info.IsDir() {
			warn := fmt.Sprintf("CLIKS_CAPTURE_HELPER override %q does not exist or is not a valid binary file", envOverride)
			setMacHelperOverrideWarning(warn)
			fmt.Fprintf(os.Stderr, "Security warning: %s. Falling back to standard installed bundle.\n", warn)
		} else if err := macCodeSignatureVerifier(envOverride); err != nil {
			warn := fmt.Sprintf("CLIKS_CAPTURE_HELPER override %q rejected: %v", envOverride, err)
			setMacHelperOverrideWarning(warn)
			fmt.Fprintf(os.Stderr, "Security warning: %s. Falling back to standard installed bundle.\n", warn)
		} else {
			setMacHelperOverrideWarning("")
			return envOverride
		}
	} else {
		setMacHelperOverrideWarning("")
	}

	var stdCandidates []string
	if home, err := os.UserHomeDir(); err == nil {
		stdCandidates = append(stdCandidates, filepath.Join(home, "Applications", "Cliks Capture.app", "Contents", "MacOS", "cliks-capture"))
	}
	if executable, err := os.Executable(); err == nil {
		stdCandidates = append(stdCandidates, filepath.Join(filepath.Dir(executable), "Cliks Capture.app", "Contents", "MacOS", "cliks-capture"))
	}

	for _, candidate := range stdCandidates {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				if err := macCodeSignatureVerifier(candidate); err == nil {
					return candidate
				} else {
					fmt.Fprintf(os.Stderr, "Security warning: Standard helper bundle %q failed security validation: %v.\n", candidate, err)
				}
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

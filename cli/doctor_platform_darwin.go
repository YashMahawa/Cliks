//go:build darwin

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	trusted, method := macAccessibilityTrusted()
	if trusted {
		report.checks = append(report.checks, doctorCheck{"macOS Accessibility", "allowed (" + method + ")"})
	} else if method == "probe-failed" {
		report.checks = append(report.checks, doctorCheck{"macOS Accessibility", "unknown — run capture-test"})
		report.issues = append(report.issues, doctorIssue{
			title:  "Verify Accessibility permission",
			detail: "Cliks could not confirm Accessibility access. Global capture needs your terminal app allowed under Privacy & Security.",
			commands: []string{
				"Open System Settings > Privacy & Security > Accessibility",
				"Allow your terminal app (Terminal, iTerm, Warp, VS Code, etc.)",
				"cliks capture-test",
			},
		})
	} else {
		report.checks = append(report.checks, doctorCheck{"macOS Accessibility", "not confirmed (" + method + ")"})
		report.issues = append(report.issues, doctorIssue{
			title:  "Allow Accessibility permission",
			detail: "macOS global input capture needs Accessibility permission for the terminal app that launches Cliks.",
			commands: []string{
				"Open System Settings > Privacy & Security > Accessibility",
				"Allow your terminal app",
				"cliks capture-test",
			},
		})
	}

	if thorough {
		// Self-test only for explicit doctor runs — never block session startup.
		probe := probeGlobalCapture(1500 * time.Millisecond)
		report.checks = append(report.checks, doctorCheck{"Capture backend probe", probe})
		if strings.Contains(probe, "off") || strings.Contains(probe, "failed") {
			report.issues = append(report.issues, doctorIssue{
				title:  "Global capture probe did not stay active",
				detail: "The background hook did not report a healthy capture mode. Grant Accessibility, then re-test.",
				commands: []string{"cliks capture-test", "cliks start --terminal --self"},
			})
		}
	}
	report.recommendation = []string{"Recommended run command:", "cliks start"}
}

func macAccessibilityTrusted() (bool, string) {
	// Prefer a short AppleScript/osascript check when available; otherwise rely on capture probe.
	cmd := exec.Command("osascript", "-e", `tell application "System Events" to get UI elements enabled`)
	out, err := cmd.Output()
	if err == nil {
		value := strings.TrimSpace(strings.ToLower(string(out)))
		if value == "true" {
			return true, "system events"
		}
		if value == "false" {
			return false, "system events"
		}
	}
	return false, "probe-failed"
}

func probeGlobalCapture(timeout time.Duration) string {
	cfg := loadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	capture := newActivityCapture()
	state := capture.start(ctx, cfg.Sharing, "auto")
	// Give the hook a brief moment to settle, then stop cleanly.
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	capture.stop()
	if state.Mode == "" || state.Mode == "off" {
		if state.PermissionHint != "" {
			return "failed — " + truncate(state.PermissionHint, 80)
		}
		return "failed"
	}
	return fmt.Sprintf("ok (%s)", state.Mode)
}

func platformStartupCaptureNotice() string {
	trusted, _ := macAccessibilityTrusted()
	if trusted {
		return ""
	}
	return "macOS: if capture is quiet, allow Accessibility for this terminal, then run cliks capture-test."
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max-1] + "…"
}

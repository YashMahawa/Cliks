//go:build darwin

package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	trusted, method := macInputMonitoringTrusted()
	if trusted {
		report.checks = append(report.checks, doctorCheck{"macOS Input Monitoring", "allowed (" + method + ")"})
	} else {
		report.checks = append(report.checks, doctorCheck{"macOS Input Monitoring", "not allowed (" + method + ")"})
		report.issues = append(report.issues, doctorIssue{
			title:  "Allow Input Monitoring permission",
			detail: "Cliks uses Apple's listen-only Event Tap. Enable Cliks or the terminal that launches it under Input Monitoring; switching terminal apps may require enabling the new app too.",
			commands: []string{
				"Open System Settings > Privacy & Security > Input Monitoring",
				"Allow Cliks or the terminal app you use to start it",
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
				title:    "Global capture probe did not stay active",
				detail:   "The native Event Tap did not report a healthy capture mode. Grant Input Monitoring, restart Cliks, then re-test.",
				commands: []string{"cliks capture-test", "cliks start --terminal --self"},
			})
		}
	}
	report.recommendation = []string{"Recommended run command:", "cliks start"}
}

func macInputMonitoringTrusted() (bool, string) {
	if macListenEventAccessAllowed() {
		return true, "CoreGraphics preflight"
	}
	return false, "CoreGraphics preflight"
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
	trusted, _ := macInputMonitoringTrusted()
	if trusted {
		return ""
	}
	return "macOS: allow Input Monitoring for Cliks or the terminal that launches it. Restart Cliks after enabling it, then run cliks capture-test."
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max-1] + "…"
}

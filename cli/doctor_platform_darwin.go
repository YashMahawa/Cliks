//go:build darwin

package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	helper := macCaptureHelperPath()
	if helper != "" {
		report.checks = append(report.checks, doctorCheck{"Isolated capture app", "installed (" + helper + ")"})
	} else {
		report.checks = append(report.checks, doctorCheck{"Isolated capture app", "missing / unlinked"})
		report.issues = append(report.issues, doctorIssue{
			title:  "Install isolated macOS capture helper bundle",
			detail: "Cliks Capture.app helper bundle is missing or unlinked. Run cliks setup or reinstall Cliks.",
			commands: []string{
				"cliks setup",
				"Re-run the Cliks curl installer",
				"Open System Settings → Privacy & Security → Input Monitoring (x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_ListenEvent)",
			},
		})
	}

	trusted, reason := macInputMonitoringTrusted()
	if trusted {
		report.checks = append(report.checks, doctorCheck{"Input Monitoring trust", "granted (" + reason + ")"})
	} else {
		report.checks = append(report.checks, doctorCheck{"Input Monitoring trust", "denied or uninitialized (" + reason + ")"})
		if helper != "" {
			report.issues = append(report.issues, doctorIssue{
				title:  "Grant Input Monitoring permission to Cliks Capture.app",
				detail: "Input Monitoring is required for Cliks Capture.app. Grant permission to the helper bundle in System Settings.",
				commands: []string{
					"Open System Settings → Privacy & Security → Input Monitoring (x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_ListenEvent)",
					"cliks setup",
					"cliks capture-test",
				},
			})
		}
	}

	if thorough {
		// Self-test only for explicit doctor runs — never block session startup.
		probe := probeGlobalCapture(2500 * time.Millisecond)
		report.checks = append(report.checks, doctorCheck{"Capture backend probe", probe})
		if strings.Contains(probe, "off") || strings.Contains(probe, "failed") {
			if helper == "" {
				report.issues = append(report.issues, doctorIssue{
					title:  "Capture backend probe failed: helper bundle missing",
					detail: "The capture probe failed because Cliks Capture.app is missing. Reinstall Cliks or run cliks setup.",
					commands: []string{"cliks setup", "Re-run the Cliks curl installer"},
				})
			} else if !trusted {
				// Covered by Input Monitoring issue
			} else {
				report.issues = append(report.issues, doctorIssue{
					title:    "Global capture probe did not stay active",
					detail:   "The native Event Tap did not report a healthy capture mode. Grant Input Monitoring, restart Cliks, then re-test.",
					commands: []string{"cliks capture-test", "cliks start --terminal --self"},
				})
			}
		}
	}
	report.recommendation = []string{"Recommended run command:", "cliks start"}
}

func macInputMonitoringTrusted() (bool, string) {
	helper := macCaptureHelperPath()
	if helper == "" {
		return false, "Cliks Capture.app helper bundle missing"
	}
	if macCaptureHelperReady() {
		return true, "Cliks Capture.app helper bundle"
	}
	return false, "Cliks Capture.app helper bundle"
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
	return "macOS: install and allow Cliks Capture.app under Input Monitoring. Keep terminal permission off unless using the labeled direct fallback."
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max-1] + "…"
}

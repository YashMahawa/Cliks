//go:build windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	elevated, detail := windowsElevationStatus()
	if elevated {
		report.checks = append(report.checks, doctorCheck{"Windows elevation", "yes (Administrator)"})
		report.checks = append(report.checks, doctorCheck{"UIPI risk", "low for elevated apps"})
	} else {
		report.checks = append(report.checks, doctorCheck{"Windows elevation", "no (standard user)"})
		report.checks = append(report.checks, doctorCheck{"UIPI risk", "high while elevated apps are focused"})
		if detail != "" {
			report.checks = append(report.checks, doctorCheck{"Elevation check", detail})
		}
		// Tip only — not a blocking issue. Everyday apps work without user action.
		report.checks = append(report.checks, doctorCheck{"Elevated privilege warning", "capture pauses while elevated (Administrator) windows are focused"})
	}
	if thorough {
		vitality, probe := probeWindowsNativeCaptureVitality()
		report.checks = append(report.checks, doctorCheck{"Hook vitality", vitality})
		report.checks = append(report.checks, doctorCheck{"Capture backend probe", probe})
		if strings.Contains(probe, "failed") || strings.Contains(vitality, "fail") {
			report.issues = append(report.issues, doctorIssue{
				title:    "Windows native hook vitality failure",
				detail:   "Low-level keyboard/mouse hooks are detached or unresponsive. System security (UIPI) or OS callback timeouts may have evicted hooks.",
				commands: []string{"cliks capture-test"},
			})
		}
	}
	report.recommendation = []string{"Recommended run command:", "cliks start"}
}

func probeWindowsNativeCaptureVitality() (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	capture := newActivityCapture()
	state := capture.start(ctx, SharingConfig{Keyboard: true, Mouse: true}, "auto")
	defer capture.stop()

	if state.Mode != "windows-native" {
		return "fail (hooks unavailable)", fmt.Sprintf("failed (%s)", valuePlain(state.PermissionHint, "native hooks unavailable"))
	}

	windowsNativeCaptureLock.RLock()
	session := windowsNativeCapture
	windowsNativeCaptureLock.RUnlock()

	if session == nil {
		return "fail (session missing)", "failed (session missing)"
	}

	sendVitalityProbe(SharingConfig{Keyboard: true, Mouse: true})
	time.Sleep(100 * time.Millisecond)

	if session.checkVitality(1 * time.Second) {
		return "pass (hooks attached and responding)", "ok (windows-native)"
	}
	return "fail (detached or unresponsive hooks detected)", "failed (vitality probe failed: hooks detached or unresponsive)"
}

func probeWindowsNativeCapture() string {
	_, probe := probeWindowsNativeCaptureVitality()
	return probe
}

func windowsElevationStatus() (bool, string) {
	// Prefer a pure PowerShell role check; fall back to whoami /groups.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)")
	out, err := cmd.Output()
	if err == nil {
		value := strings.TrimSpace(strings.ToLower(string(out)))
		if value == "true" {
			return true, "powershell"
		}
		if value == "false" {
			return false, "powershell"
		}
	}
	cmd = exec.Command("whoami", "/groups")
	out, err = cmd.Output()
	if err != nil {
		return false, "unable to query elevation"
	}
	text := strings.ToLower(string(out))
	// S-1-16-12288 is High Mandatory Level (elevated).
	if strings.Contains(text, "s-1-16-12288") || strings.Contains(text, "high mandatory level") {
		return true, "whoami"
	}
	return false, "whoami"
}

func platformStartupCaptureNotice() string {
	elevated, _ := windowsElevationStatus()
	if elevated {
		return ""
	}
	// UIPI is silent at the OS level — surface it so users know capture is not broken.
	return "Windows tip: capture may pause while an Administrator window is focused (UIPI); it resumes on normal apps."
}

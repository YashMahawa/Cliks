//go:build windows

package main

import (
	"os/exec"
	"strings"
)

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	_ = thorough
	elevated, detail := windowsElevationStatus()
	if elevated {
		report.checks = append(report.checks, doctorCheck{"Windows elevation", "yes (Administrator)"})
		report.checks = append(report.checks, doctorCheck{"UIPI risk", "low for elevated apps"})
		report.recommendation = []string{"Recommended run command:", "cliks start"}
		return
	}
	report.checks = append(report.checks, doctorCheck{"Windows elevation", "no (standard user)"})
	report.checks = append(report.checks, doctorCheck{"UIPI risk", "high while elevated apps are focused"})
	if detail != "" {
		report.checks = append(report.checks, doctorCheck{"Elevation check", detail})
	}
	report.issues = append(report.issues, doctorIssue{
		title:  "Background capture can pause on elevated windows",
		detail: "Windows User Interface Privilege Isolation (UIPI) blocks standard-user hooks when an Administrator window (Task Manager, elevated terminals, installers) is focused. Capture resumes when focus returns to a normal window.",
		commands: []string{
			"Keep Cliks running as a normal user for day-to-day work",
			"If you need capture while elevated apps are focused, relaunch an elevated terminal and run: cliks start",
			"cliks capture-test",
		},
	})
	report.recommendation = []string{"Recommended run command:", "cliks start"}
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
	return "Windows: running as a standard user. Capture may pause while elevated apps are focused (UIPI)."
}

//go:build linux

package main

import (
	"fmt"
	"os"
	"strings"
)

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	_ = thorough
	input := linuxInputStatus()
	report.checks = append(report.checks, doctorCheck{"Linux input devices", yesNo(input.hasInputDir)})
	if input.hasInputDir {
		report.checks = append(report.checks,
			doctorCheck{"Readable event devices", fmt.Sprintf("%d/%d", input.readableCount, input.eventCount)},
			doctorCheck{"Active input group", yesNo(input.inputGroupActive)},
		)
	}

	wayland := os.Getenv("WAYLAND_DISPLAY") != "" || strings.Contains(strings.ToLower(os.Getenv("XDG_SESSION_TYPE")), "wayland")
	sandbox := os.Getenv("FLATPAK_ID") != "" || os.Getenv("container") != ""
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		sandbox = true
	}
	if wayland {
		report.checks = append(report.checks, doctorCheck{"Session type", "wayland"})
	}
	if sandbox {
		report.checks = append(report.checks, doctorCheck{"Sandbox", "detected"})
	}

	switch {
	case !input.hasInputDir:
		detail := "No /dev/input here (common in containers or SSH). On your normal desktop, run cliks setup."
		if sandbox {
			detail = "Sandboxed session cannot access input devices. Use Cliks in a normal desktop terminal."
		}
		report.issues = append(report.issues, doctorIssue{"Use a desktop session for ambient capture", detail, []string{"cliks setup", "cliks start --terminal --self"}})
	case input.eventCount == 0:
		report.issues = append(report.issues, doctorIssue{"No input devices found", "Open Cliks from a real desktop session (not a remote shell).", []string{"cliks setup"}})
	case input.readableCount == 0:
		commands := []string{"cliks setup", "sudo usermod -aG input " + input.username, "Log out and back in once"}
		detail := "One-time permission so Cliks can sense activity kinds (never key values). Easiest: cliks setup"
		if wayland || sandbox {
			detail += " On some Wayland/sandbox setups you may need a normal host session."
		}
		report.issues = append(report.issues, doctorIssue{"Allow background capture (one-time)", detail, commands})
	}

	if input.readableCount > 0 {
		report.recommendation = []string{"Recommended run command:", "cliks start --evdev"}
	} else {
		report.recommendation = []string{"Recommended local test:", "cliks start --terminal --self"}
	}
}

func platformStartupCaptureNotice() string {
	input := linuxInputStatus()
	if input.readableCount > 0 {
		return ""
	}
	if os.Getenv("FLATPAK_ID") != "" || os.Getenv("container") != "" {
		return "Linux: sandboxed session — global capture may be blocked. Prefer host install or cliks start --terminal --self."
	}
	if input.hasInputDir && input.readableCount == 0 {
		return "Linux: cannot read /dev/input yet. Join the input group or use terminal capture."
	}
	return ""
}

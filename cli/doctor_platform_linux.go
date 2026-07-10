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
		detail := "Cliks cannot see /dev/input. This is normal in containers, SSH sessions, and locked-down environments."
		if sandbox {
			detail = "Sandboxed session cannot access /dev/input. Install Cliks on the host desktop session, or use terminal capture."
		}
		report.issues = append(report.issues, doctorIssue{"Global capture is unavailable here", detail, []string{"Use a normal desktop terminal", "cliks start --terminal --self"}})
	case input.eventCount == 0:
		report.issues = append(report.issues, doctorIssue{"No input event devices found", "Cliks found /dev/input, but no /dev/input/event* devices.", []string{"ls -l /dev/input", "Try again from the real desktop session"}})
	case input.readableCount == 0:
		commands := []string{"sudo usermod -aG input " + input.username, "Log out and back in, or reboot", "cliks doctor"}
		detail := "Linux global capture needs permission to read /dev/input/event*. Cliks still sends only event type and timing, never key values."
		if wayland || sandbox {
			detail += " Wayland and sandboxed sessions may still block device access even after joining the input group."
			commands = append(commands, "cliks start --terminal --self")
		}
		report.issues = append(report.issues, doctorIssue{"Allow Cliks to read input events", detail, commands})
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

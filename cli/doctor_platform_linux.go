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
	hs := probeLinuxCaptureHelper()
	isolated := hs.ready && hs.seat != "muted"

	if hs.ready {
		switch hs.seat {
		case "muted":
			report.checks = append(report.checks, doctorCheck{"Isolated capture helper", "muted (seat ownership check failed)"})
		case "fallback":
			report.checks = append(report.checks, doctorCheck{"Isolated capture helper", "yes (fallback owner verification)"})
		default:
			report.checks = append(report.checks, doctorCheck{"Isolated capture helper", "yes"})
		}
	} else {
		report.checks = append(report.checks, doctorCheck{"Isolated capture helper", yesNo(false)})
	}

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
	case hs.ready && hs.seat == "muted":
		detail := "The capture helper is connected, but input capture is muted because seat ownership verification failed or your user session is not active on the seat."
		report.issues = append(report.issues, doctorIssue{"Activate active user session or check seat status", detail, []string{"Switch to active user session or check seat ownership with loginctl", "cliks setup"}})
	case !isolated:
		commands := []string{"cliks setup", "Re-run the Cliks installer"}
		detail := "Install the isolated helper so the Cliks client and unrelated user programs never receive raw input-device access."
		if wayland || sandbox {
			detail += " On some Wayland/sandbox setups you may need a normal host session."
		}
		report.issues = append(report.issues, doctorIssue{"Install private background capture", detail, commands})
	}

	if isolated {
		report.recommendation = []string{"Recommended run command:", "cliks start"}
	} else {
		report.recommendation = []string{"Recommended local test:", "cliks start --terminal --self"}
	}
}

func platformStartupCaptureNotice() string {
	input := linuxInputStatus()
	hs := probeLinuxCaptureHelper()
	if hs.ready && hs.seat == "muted" {
		return "Linux: capture helper is connected but muted (seat ownership check failed)."
	}
	if hs.ready && hs.seat != "muted" {
		return ""
	}
	if os.Getenv("FLATPAK_ID") != "" || os.Getenv("container") != "" {
		return "Linux: sandboxed session — global capture may be blocked. Prefer host install or cliks start --terminal --self."
	}
	if input.hasInputDir && input.readableCount == 0 {
		return "Linux: isolated capture helper is not ready. Run cliks setup. Direct input-group access is a labeled compatibility fallback only."
	}
	return ""
}

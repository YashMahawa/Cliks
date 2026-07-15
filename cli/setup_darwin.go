//go:build darwin

package main

import (
	"os/exec"
)

func platformCaptureSetup() []setupStep {
	steps := []setupStep{}
	trusted, method := macInputMonitoringTrusted()
	if trusted {
		steps = append(steps, setupStep{
			title:  "Background capture",
			status: "ok",
			detail: "macOS Input Monitoring is allowed (" + method + "). The native Event Tap is ready.",
		})
		return steps
	}

	// Ask through Apple's API first so macOS registers the responsible app,
	// then open the exact pane so the remaining action is one toggle.
	_ = requestMacListenEventAccess()
	opened := openMacInputMonitoringSettings()
	detail := "macOS needs Input Monitoring so Cliks can sense keyboard/mouse activity kinds (never what you type)."
	if opened {
		detail += " System Settings was opened for you — enable Cliks or the terminal you use to start it (Terminal / iTerm / Warp / VS Code), restart Cliks, then run: cliks setup"
	} else {
		detail += " Open System Settings → Privacy & Security → Input Monitoring, enable Cliks or its terminal, restart Cliks, then run: cliks setup"
	}
	steps = append(steps, setupStep{
		title:   "Background capture",
		status:  "action",
		detail:  detail,
		command: "Open System Settings → Privacy & Security → Input Monitoring",
	})
	steps = append(steps, setupStep{
		title:  "Meanwhile",
		status: "tip",
		detail: "You can still join a room. If capture is quiet, grant Input Monitoring and restart Cliks once.",
	})
	return steps
}

func openMacInputMonitoringSettings() bool {
	// Try modern Settings URLs first, then classic preference pane.
	targets := []string{
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_ListenEvent",
		"x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent",
		"/System/Library/PreferencePanes/Security.prefPane",
	}
	for _, target := range targets {
		cmd := exec.Command("open", target)
		if err := cmd.Start(); err == nil {
			return true
		}
	}
	return false
}

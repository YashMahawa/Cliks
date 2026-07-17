//go:build darwin

package main

import (
	"os/exec"
)

func platformCaptureSetup() []setupStep {
	steps := []setupStep{}
	helper := macCaptureHelperPath()
	ready := helper != "" && macCaptureHelperReady()
	if ready {
		steps = append(steps, setupStep{
			title:  "Private background capture",
			status: "ok",
			detail: "Cliks Capture.app is installed and allowed. Input Monitoring is isolated from your terminal.",
		})
	} else if helper != "" {
		steps = append(steps, setupStep{title: "Private background capture", status: "action", detail: "Enable only Cliks Capture in Input Monitoring, then restart Cliks. Do not enable Terminal, iTerm, Warp, or VS Code.", command: "Open System Settings → Privacy & Security → Input Monitoring"})
	} else {
		steps = append(steps, setupStep{
			title: "Private background capture", status: "action",
			detail:  "Cliks Capture.app is missing. Reinstall Cliks to add the isolated open-source helper.",
			command: "Re-run the Cliks curl installer",
		})
	}
	opened := false
	if !ready {
		opened = openMacInputMonitoringSettings()
	}
	detail := "If unsigned Cliks Capture is blocked, use Privacy & Security → Open Anyway once. This is Apple's warning for community builds, not a request to disable Gatekeeper."
	if opened {
		detail += " Settings was opened for you."
	}
	steps = append(steps, setupStep{
		title:  "Community build note",
		status: "tip",
		detail: detail,
	})
	steps = append(steps, setupStep{
		title:  "Compatibility fallback",
		status: "tip",
		detail: "If the helper fails, direct mode can temporarily use your terminal's Input Monitoring permission: cliks set capture.mode direct. This is less safe. Disable that terminal permission when done, then run: cliks set capture.mode isolated",
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

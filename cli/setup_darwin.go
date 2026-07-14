//go:build darwin

package main

import (
	"os/exec"
)

func platformCaptureSetup() []setupStep {
	steps := []setupStep{}
	trusted, method := macAccessibilityTrusted()
	if trusted {
		steps = append(steps, setupStep{
			title:  "Background capture",
			status: "ok",
			detail: "macOS Accessibility is allowed (" + method + "). Global capture is ready.",
		})
		return steps
	}

	// Open Settings so non-tech users only need to flip one switch.
	opened := openMacAccessibilitySettings()
	detail := "macOS needs one permission so Cliks can sense keyboard/mouse activity kinds (never what you type)."
	if opened {
		detail += " System Settings was opened for you — enable the terminal you use to start Cliks (Terminal / iTerm / Warp / VS Code). Permission is per app: switch terminals → enable the new one too. Then run: cliks setup"
	} else {
		detail += " Open System Settings → Privacy & Security → Accessibility, enable the terminal that launches Cliks (per-app if you switch apps), then run: cliks setup"
	}
	steps = append(steps, setupStep{
		title:  "Background capture",
		status: "action",
		detail: detail,
		command: "Open System Settings → Privacy & Security → Accessibility",
	})
	steps = append(steps, setupStep{
		title:  "Meanwhile",
		status: "tip",
		detail: "You can still join a room. If capture is quiet, grant Accessibility and restart Cliks once.",
	})
	return steps
}

func openMacAccessibilitySettings() bool {
	// Try modern Settings URLs first, then classic preference pane.
	targets := []string{
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_Accessibility",
		"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility",
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

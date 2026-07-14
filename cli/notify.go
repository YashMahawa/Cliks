package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func notifyWave(cfg CliksConfig, sender string) error {
	if !cfg.Notifications.Enabled || cfg.PresenceStatus == "focus" || cfg.PresenceStatus == "dnd" {
		return nil
	}
	sender = strings.TrimSpace(sender)
	if sender == "" {
		sender = "A teammate"
	}
	return sendNativeNotification("Cliks", sender+" waved to you", cfg.Notifications.Sound)
}

func nativeNotificationStatus() (bool, string) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("osascript"); err == nil {
			return true, "macOS Notification Center"
		}
		return false, "osascript is unavailable"
	case "windows":
		if _, err := exec.LookPath("powershell"); err == nil {
			return true, "Windows toast notifications"
		}
		return false, "Windows PowerShell is unavailable"
	case "linux":
		if _, err := exec.LookPath("notify-send"); err == nil {
			return true, "desktop notifications (notify-send)"
		}
		return false, "install libnotify / notify-send"
	default:
		return false, "native notifications are not supported on this platform"
	}
}

func runNotificationTest() error {
	ready, detail := nativeNotificationStatus()
	if !ready {
		return errors.New(detail)
	}
	cfg := loadConfig()
	if err := sendNativeNotification("Cliks", "Notifications are ready — waves can reach you here.", cfg.Notifications.Sound); err != nil {
		return err
	}
	return nil
}

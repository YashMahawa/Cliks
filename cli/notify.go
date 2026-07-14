package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func notifyReaction(cfg CliksConfig, sender string, reaction string) error {
	if !cfg.Notifications.Enabled || cfg.PresenceStatus == "focus" || cfg.PresenceStatus == "dnd" {
		return nil
	}
	title, body := reactionNotificationContent(sender, reaction)
	return sendNativeNotification(title, body, cfg.Notifications.Sound)
}

func reactionNotificationContent(sender string, reaction string) (string, string) {
	sender = strings.TrimSpace(sender)
	if sender == "" {
		sender = "A teammate"
	}
	return sender + " " + reactionGlyph(reaction) + " " + reactionPhrase(reaction), "Cliks quick signal"
}

func notifyWave(cfg CliksConfig, sender string) error {
	return notifyReaction(cfg, sender, "wave")
}

func reactionPhrase(reaction string) string {
	switch reaction {
	case "wave":
		return "Hey there!"
	case "nice":
		return "Nice work!"
	case "coffee":
		return "Coffee time?"
	case "celebrate":
		return "That deserves a celebration!"
	case "break":
		return "Let’s take a break."
	case "focus":
		return "Going into focus mode."
	default:
		return "Sent a quick signal."
	}
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
		if isTermuxRuntime() {
			if _, err := exec.LookPath("termux-notification"); err == nil {
				return true, "Termux notification service"
			}
			return false, "install Termux:API and run: pkg install termux-api"
		}
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
	title, body := reactionNotificationContent("Mira", "wave")
	if err := sendNativeNotification(title, "Example: "+body, cfg.Notifications.Sound); err != nil {
		return err
	}
	return nil
}

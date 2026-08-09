package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

var reactionNotificationSender = sendNativeNotification

func notifyReaction(cfg CliksConfig, sender string, reaction string) error {
	if !cfg.Notifications.Enabled || cfg.Listening.Muted || cfg.PresenceStatus == "focus" || cfg.PresenceStatus == "dnd" {
		return nil
	}
	title, body := reactionNotificationContent(sender, reaction)
	// Cliks plays the optional signal cue through its own spatial audio engine.
	// Keep the native banner visual so the OS cannot add a second, jarring chime.
	return reactionNotificationSender(title, body, false)
}

func reactionNotificationContent(sender string, reaction string) (string, string) {
	sender = strings.TrimSpace(sender)
	if sender == "" {
		sender = "A teammate"
	}
	return sender + " sent " + reactionGlyph(reaction), reactionPhrase(reaction)
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
		return true, "Windows Notification Center (built in)"
	case "linux":
		if isTermuxRuntime() {
			if _, err := exec.LookPath("termux-notification"); err == nil {
				return true, "Termux notification service"
			}
			return false, "install Termux:API and run: pkg install termux-api"
		}
		if _, err := exec.LookPath("notify-send"); err == nil {
			if nativeNotificationPlatformReady() {
				return true, "desktop notifications (notify-send + active notification service)"
			}
			return false, "notify-send is installed, but no desktop notification service is running"
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
	title, body := reactionNotificationContent("Mira", "wave")
	if err := sendNativeNotification(title, "Example: "+body, false); err != nil {
		return err
	}
	return nil
}

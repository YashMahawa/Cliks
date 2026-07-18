//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func desktopNotificationEnv() []string {
	env := os.Environ()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		return env
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" && os.Getuid() >= 0 {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	busPath := filepath.Join(runtimeDir, "bus")
	if runtimeDir != "" {
		if _, err := os.Stat(busPath); err == nil {
			env = append(env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+busPath)
		}
	}
	return env
}

func nativeNotificationPlatformReady() bool {
	if isTermuxRuntime() {
		_, err := exec.LookPath("termux-notification")
		return err == nil
	}
	if path, err := exec.LookPath("busctl"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, "--user", "--no-pager", "list")
		cmd.Env = desktopNotificationEnv()
		if output, err := cmd.Output(); err == nil && notificationBusListHasProvider(string(output)) {
			return true
		}
	}
	if path, err := exec.LookPath("gdbus"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, "call", "--session", "--dest", "org.freedesktop.DBus", "--object-path", "/org/freedesktop/DBus", "--method", "org.freedesktop.DBus.NameHasOwner", "org.freedesktop.Notifications")
		cmd.Env = desktopNotificationEnv()
		if output, err := cmd.Output(); err == nil && strings.Contains(strings.ToLower(string(output)), "true") {
			return true
		}
	}
	return false
}

func notificationBusListHasProvider(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "org.freedesktop.Notifications ") {
			return true
		}
	}
	return false
}

func repairNativeNotificationService() (bool, string) {
	if isTermuxRuntime() || nativeNotificationPlatformReady() {
		return false, ""
	}
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return false, ""
	}
	for _, service := range []string{"caelestia-quickshell.service", "mako.service", "dunst.service", "swaync.service"} {
		check := exec.Command(systemctl, "--user", "is-enabled", service)
		check.Env = desktopNotificationEnv()
		if check.Run() != nil {
			continue
		}
		start := exec.Command(systemctl, "--user", "start", service)
		start.Env = desktopNotificationEnv()
		if start.Run() == nil {
			for attempt := 0; attempt < 20; attempt++ {
				if nativeNotificationPlatformReady() {
					return true, "Started " + strings.TrimSuffix(service, ".service") + " so native banners can appear."
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	return false, ""
}

func sendNativeNotification(title string, body string, sound bool) error {
	if isTermuxRuntime() {
		path, err := exec.LookPath("termux-notification")
		if err != nil {
			return fmt.Errorf("native notifications need Termux:API (pkg install termux-api)")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		args := []string{"--title", title, "--content", body, "--group", "cliks"}
		if sound {
			args = append(args, "--sound")
		}
		return exec.CommandContext(ctx, path, args...).Run()
	}
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return fmt.Errorf("native notifications need notify-send")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []string{"--app-name=Cliks", "--urgency=normal"}
	if !sound {
		args = append(args, "--hint=boolean:suppress-sound:true")
	}
	args = append(args, title, body)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = desktopNotificationEnv()
	if output, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("notify-send could not reach the desktop notification service: %s", detail)
		}
		return fmt.Errorf("notify-send could not reach the desktop notification service: %w", err)
	}
	if sound {
		if player, err := exec.LookPath("canberra-gtk-play"); err == nil {
			_ = exec.CommandContext(ctx, player, "-i", "message-new-instant").Run()
		}
	}
	return nil
}

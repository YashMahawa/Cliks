//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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
	cmd.Env = os.Environ()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" && os.Getuid() >= 0 {
			runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		}
		busPath := filepath.Join(runtimeDir, "bus")
		if runtimeDir != "" {
			if _, err := os.Stat(busPath); err == nil {
				cmd.Env = append(cmd.Env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+busPath)
			}
		}
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("notify-send could not reach the desktop notification service: %w", err)
	}
	if sound {
		if player, err := exec.LookPath("canberra-gtk-play"); err == nil {
			_ = exec.CommandContext(ctx, player, "-i", "message-new-instant").Run()
		}
	}
	return nil
}

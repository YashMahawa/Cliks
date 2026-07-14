//go:build linux

package main

import (
	"context"
	"fmt"
	"os/exec"
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
	if err := exec.CommandContext(ctx, path, args...).Run(); err != nil {
		return err
	}
	if sound {
		if player, err := exec.LookPath("canberra-gtk-play"); err == nil {
			_ = exec.CommandContext(ctx, player, "-i", "message-new-instant").Run()
		}
	}
	return nil
}

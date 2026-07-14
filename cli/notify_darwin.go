//go:build darwin

package main

import (
	"context"
	"os/exec"
	"time"
)

func sendNativeNotification(title string, body string, sound bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	script := `on run argv
display notification (item 2 of argv) with title (item 1 of argv)
end run`
	if sound {
		script = `on run argv
display notification (item 2 of argv) with title (item 1 of argv) sound name "Glass"
end run`
	}
	return exec.CommandContext(ctx, "osascript", "-e", script, title, body).Run()
}

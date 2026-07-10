//go:build darwin || windows

package main

import (
	"context"
	"runtime"
	"time"

	hook "github.com/robotn/gohook"
)

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	evChan := hook.Start()
	if evChan == nil {
		return CaptureState{
			Mode:           "off",
			PermissionHint: globalHookPermissionHint(),
		}
	}

	go func() {
		defer hook.End()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-evChan:
				if !ok {
					return
				}

				if sharing.Keyboard && ev.Kind == hook.KeyDown {
					c.emit(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
				}

				if sharing.Mouse && ev.Kind == hook.MouseDown {
					button := "unknown"
					if ev.Button == hook.MouseMap["left"] {
						button = "left"
					} else if ev.Button == hook.MouseMap["right"] {
						button = "right"
					}
					if button != "unknown" {
						c.emit(LocalActivityEvent{Kind: "mouse", At: time.Now(), Button: button})
					}
				}
			}
		}
	}()

	// Do not surface scary permission text when hooks started cleanly.
	// Soft tips only appear if capture stays silent or doctor/setup is run.
	hint := ""
	if notice := platformStartupCaptureNotice(); notice != "" && runtime.GOOS == "darwin" {
		// macOS Accessibility is the one OS dialog users may still need.
		hint = notice
	}
	return CaptureState{
		Mode:           "background",
		PermissionHint: hint,
	}
}

func globalHookPermissionHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "If capture is quiet: System Settings → Privacy & Security → Accessibility → enable your terminal, then cliks setup"
	case "windows":
		return "Capture may pause only while an Administrator window is focused (Windows security)."
	default:
		return "If capture is quiet, run: cliks setup"
	}
}

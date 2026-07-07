//go:build darwin || windows

package main

import (
	"context"
	"time"

	hook "github.com/robotn/gohook"
)

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	evChan := hook.Start()

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

	return CaptureState{
		Mode:           "background",
		PermissionHint: "If capture is not working, please ensure the application has Accessibility (macOS) or Input permissions.",
	}
}

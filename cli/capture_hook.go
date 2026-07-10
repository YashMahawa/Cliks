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

	hint := globalHookPermissionHint()
	if notice := platformStartupCaptureNotice(); notice != "" {
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
		return "If capture is quiet, allow Accessibility for this terminal (System Settings → Privacy & Security → Accessibility), then run: cliks capture-test"
	case "windows":
		return "If capture pauses on elevated windows, that is UIPI. Capture resumes on normal windows, or relaunch Cliks elevated if needed."
	default:
		return "If capture is not working, check input permissions and run: cliks doctor"
	}
}

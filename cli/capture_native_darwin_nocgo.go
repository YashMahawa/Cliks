//go:build darwin && !cgo

package main

import (
	"context"
)

func (c *ActivityCapture) startDirectGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	return c.startGlobalHook(ctx, sharing, "isolated")
}

func macListenEventAccessAllowed() bool {
	return macCaptureHelperReady()
}

func requestMacListenEventAccess() bool {
	return macCaptureHelperReady()
}

func globalHookPermissionHint() string {
	return "macOS non-CGO build relies on Cliks Capture.app. Run cliks setup or reinstall."
}

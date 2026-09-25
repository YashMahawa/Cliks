//go:build darwin && !cgo

package main

import "context"

func (c *ActivityCapture) startDirectGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	return CaptureState{Mode: "off", PermissionHint: "Direct global hook requires CGO on macOS."}
}

func macListenEventAccessAllowed() bool {
	return false
}

func requestMacListenEventAccess() bool {
	return false
}

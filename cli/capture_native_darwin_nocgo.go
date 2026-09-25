//go:build darwin && !cgo

package main

import "context"

func (c *ActivityCapture) startDirectGlobalHook(ctx context.Context, sharing SharingConfig) CaptureState {
	return CaptureState{Mode: "off", PermissionHint: "Direct CGO global hook is disabled in this build."}
}

func macListenEventAccessAllowed() bool {
	return false
}

func requestMacListenEventAccess() bool {
	return false
}

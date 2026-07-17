//go:build !darwin && !windows

package main

import (
	"context"
)

func (c *ActivityCapture) startGlobalHook(ctx context.Context, sharing SharingConfig, mode string) CaptureState {
	_ = mode
	return CaptureState{Mode: "off", PermissionHint: "Global hook capture is not supported on this OS."}
}

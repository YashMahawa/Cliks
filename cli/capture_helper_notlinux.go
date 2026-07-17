//go:build !linux

package main

import "context"

func (c *ActivityCapture) startLinuxCaptureHelper(ctx context.Context, sharing SharingConfig) CaptureState {
	return CaptureState{Mode: "off"}
}

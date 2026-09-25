//go:build !darwin && !windows

package main

func newBuiltInAudioPlayer() *audioPlayer {
	return nil
}

func probeBuiltInAudioBackend() ProbeResult {
	return ProbeResult{
		Name:      "built-in",
		Available: false,
		State:     DriverStateOff,
		Mode:      "built-in",
	}
}

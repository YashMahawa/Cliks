//go:build !darwin && !windows

package main

func newBuiltInAudioPlayer() *audioPlayer {
	return nil
}

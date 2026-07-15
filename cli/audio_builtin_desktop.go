//go:build darwin || windows

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

var (
	builtInAudioOnce sync.Once
	builtInAudioCtx  *oto.Context
	builtInAudioErr  error
)

func newBuiltInAudioPlayer() *audioPlayer {
	return &audioPlayer{
		Command:       "built-in",
		Spatial:       true,
		VolumeCapable: true,
		Play:          playBuiltInAudio,
	}
}

func builtInAudioContext() (*oto.Context, error) {
	builtInAudioOnce.Do(func() {
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate:   44100,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
			BufferSize:   40 * time.Millisecond,
		})
		if err != nil {
			builtInAudioErr = err
			return
		}
		select {
		case <-ready:
			builtInAudioCtx = ctx
		case <-time.After(1500 * time.Millisecond):
			builtInAudioErr = fmt.Errorf("built-in audio device timed out")
		}
	})
	return builtInAudioCtx, builtInAudioErr
}

func playBuiltInAudio(ctx context.Context, job playbackJob) error {
	audioCtx, err := builtInAudioContext()
	if err != nil || audioCtx == nil {
		if err == nil {
			err = fmt.Errorf("built-in audio device is unavailable")
		}
		return err
	}
	data, err := os.ReadFile(job.File)
	if err != nil {
		return err
	}
	pcm, sampleRate, err := stereoPCMFromMonoWAV(data, job.Gain, job.Pan)
	if err != nil {
		return err
	}
	if sampleRate != 44100 {
		return fmt.Errorf("built-in audio expected 44100 Hz, got %d", sampleRate)
	}
	player := audioCtx.NewPlayer(bytes.NewReader(pcm))
	defer player.Close()
	player.Play()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !player.IsPlaying() {
			return player.Err()
		}
		select {
		case <-ctx.Done():
			player.Pause()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

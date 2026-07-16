//go:build darwin || windows

package main

import (
	"context"
	"io"
	"time"
)

type loopingPCMReader struct {
	data   []byte
	offset int
}

func (r *loopingPCMReader) Read(target []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	written := 0
	for written < len(target) {
		count := copy(target[written:], r.data[r.offset:])
		written += count
		r.offset = (r.offset + count) % len(r.data)
	}
	return written, nil
}

func playAmbient(ctx context.Context, mode string, volume float64) error {
	audioCtx, err := builtInAudioContext()
	if err != nil {
		return err
	}
	pcm, err := ambientStereoPCM(mode)
	if err != nil {
		return err
	}
	player := audioCtx.NewPlayer(&loopingPCMReader{data: pcm})
	player.SetVolume(clamp(volume, 0, 0.6))
	defer player.Close()
	player.Play()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			player.Pause()
			return nil
		case <-ticker.C:
			if err := player.Err(); err != nil {
				return err
			}
		}
	}
}

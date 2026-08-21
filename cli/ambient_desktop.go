//go:build darwin || windows

package main

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"sync"
	"time"
)

type loopingPCMReader struct {
	mu     sync.Mutex
	data   []byte
	offset int
	volume float64
}

func (r *loopingPCMReader) setVolume(vol float64) {
	r.mu.Lock()
	r.volume = clamp(vol, 0, 1)
	r.mu.Unlock()
}

func (r *loopingPCMReader) Read(target []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	r.mu.Lock()
	vol := clamp(r.volume, 0, 1)
	written := 0
	targetLen := len(target)
	dataLen := len(r.data) - (len(r.data) % 2)
	if dataLen == 0 {
		r.mu.Unlock()
		return 0, io.EOF
	}
	for written < targetLen {
		availData := dataLen - r.offset
		toCopy := targetLen - written
		if toCopy > availData {
			toCopy = availData
		}
		toCopy -= toCopy % 2
		if toCopy == 0 {
			break
		}
		for i := 0; i < toCopy; i += 2 {
			sample := int16(binary.LittleEndian.Uint16(r.data[r.offset+i : r.offset+i+2]))
			scaled := int16(math.Round(clamp(float64(sample)*vol, -32768, 32767)))
			binary.LittleEndian.PutUint16(target[written+i:written+i+2], uint16(scaled))
		}
		written += toCopy
		r.offset = (r.offset + toCopy) % dataLen
	}
	r.mu.Unlock()
	return written, nil
}

func supportsDynamicAmbientVolume() bool {
	return true
}

func playAmbient(ctx context.Context, mode string, volume float64, volChan <-chan float64) error {
	audioCtx, err := builtInAudioContext()
	if err != nil {
		return err
	}
	pcm, err := ambientStereoPCM(mode)
	if err != nil {
		return err
	}
	reader := &loopingPCMReader{
		data:   pcm,
		volume: clamp(volume, 0, 1),
	}
	player := audioCtx.NewPlayer(reader)
	player.SetVolume(1.0)
	defer player.Close()
	player.Play()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			player.Pause()
			return nil
		case v := <-volChan:
			reader.setVolume(v)
		case <-ticker.C:
			if err := player.Err(); err != nil {
				return err
			}
		}
	}
}

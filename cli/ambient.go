package main

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
)

type ambientController struct {
	parent context.Context
	mu     sync.Mutex
	mode   string
	volume float64
	cancel context.CancelFunc
	done   chan struct{}
}

func newAmbientController(parent context.Context) *ambientController {
	return &ambientController{parent: parent}
}

func (a *ambientController) update(listening ListeningConfig) {
	mode := listening.Ambient
	volume := listening.AmbientVolume
	if listening.Muted {
		mode = "off"
	}
	a.mu.Lock()
	if mode == a.mode && math.Abs(volume-a.volume) < 0.001 {
		a.mu.Unlock()
		return
	}
	oldCancel, oldDone := a.cancel, a.done
	a.cancel, a.done = nil, nil
	a.mode, a.volume = mode, volume
	a.mu.Unlock()
	if oldCancel != nil {
		oldCancel()
		<-oldDone
	}
	if mode == "off" {
		return
	}
	ctx, cancel := context.WithCancel(a.parent)
	done := make(chan struct{})
	a.mu.Lock()
	if a.mode != mode || math.Abs(a.volume-volume) >= 0.001 {
		a.mu.Unlock()
		cancel()
		close(done)
		return
	}
	a.cancel, a.done = cancel, done
	a.mu.Unlock()
	go func() {
		defer close(done)
		_ = playAmbient(ctx, mode, volume)
	}()
}

func (a *ambientController) close() {
	a.mu.Lock()
	cancel, done := a.cancel, a.done
	a.cancel, a.done = nil, nil
	a.mode = "off"
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func ambientLabel(mode string) string {
	switch mode {
	case "rain":
		return "rain window"
	case "cafe":
		return "cafe hum"
	case "deep":
		return "deep focus"
	default:
		return "off"
	}
}

func nextAmbient(mode string, delta int) string {
	values := []string{"off", "rain", "cafe", "deep"}
	index := 0
	for i, value := range values {
		if mode == value {
			index = i
			break
		}
	}
	index = (index + delta + len(values)) % len(values)
	return values[index]
}

func ambientStereoPCM(mode string, seconds int) []byte {
	const sampleRate = 44100
	if seconds < 1 {
		seconds = 1
	}
	frames := sampleRate * seconds
	pcm := make([]byte, frames*4)
	seed := uint32(0xC11C5001)
	brown := 0.0
	for i := 0; i < frames; i++ {
		seed = seed*1664525 + 1013904223
		white := float64(int32(seed)) / float64(math.MaxInt32)
		brown = clamp(brown*0.985+white*0.015, -1, 1)
		t := float64(i) / sampleRate
		var sample float64
		switch mode {
		case "rain":
			sample = white*0.13 + brown*0.20
		case "cafe":
			sample = brown*0.32 + math.Sin(2*math.Pi*96*t)*0.018 + math.Sin(2*math.Pi*143*t)*0.012
		case "deep":
			sample = math.Sin(2*math.Pi*55*t)*0.22 + math.Sin(2*math.Pi*82.5*t)*0.08 + brown*0.045
		}
		value := int16(clamp(sample, -1, 1) * 32767)
		offset := i * 4
		binary.LittleEndian.PutUint16(pcm[offset:offset+2], uint16(value))
		binary.LittleEndian.PutUint16(pcm[offset+2:offset+4], uint16(value))
	}
	return pcm
}

func pcmWAV(stereoPCM []byte) []byte {
	data := make([]byte, 44+len(stereoPCM))
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+len(stereoPCM)))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 2)
	binary.LittleEndian.PutUint32(data[24:28], 44100)
	binary.LittleEndian.PutUint32(data[28:32], 44100*4)
	binary.LittleEndian.PutUint16(data[32:34], 4)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(stereoPCM)))
	copy(data[44:], stereoPCM)
	return data
}

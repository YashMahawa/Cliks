package main

import (
	"context"
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
	case "fire":
		return "fireside"
	case "cafe":
		return "coffee house"
	case "cloud":
		return "cloud drift"
	case "contemplation":
		return "contemplation"
	case "downtempo":
		return "night drive"
	default:
		return "off"
	}
}

func nextAmbient(mode string, delta int) string {
	index := 0
	for i, value := range ambientModes {
		if mode == value {
			index = i
			break
		}
	}
	index = (index + delta + len(ambientModes)) % len(ambientModes)
	return ambientModes[index]
}

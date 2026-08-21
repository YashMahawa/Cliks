package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func requireMPV(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv executable not found on PATH; skipping mpv pool test")
	}
}

func getTestSoundFile(t *testing.T) string {
	root, err := soundsRoot()
	if err != nil {
		t.Fatalf("soundsRoot error: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, "keyboard", "*.wav"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no sound files found in %s", root)
	}
	return matches[0]
}

func TestMPVPlayerPoolInitializationAndCapacity(t *testing.T) {
	requireMPV(t)

	pool, err := newMPVPlayerPool("")
	if err != nil {
		t.Fatalf("newMPVPlayerPool failed: %v", err)
	}
	defer pool.Close()

	if len(pool.workers) != mpvPoolSize {
		t.Fatalf("pool worker count = %d, want %d", len(pool.workers), mpvPoolSize)
	}

	for i, w := range pool.workers {
		if w == nil || w.isDead() {
			t.Fatalf("worker %d is nil or dead on init", i)
		}
	}
}

func TestMPVPlayerPoolPlaybackAndRealtimeAdjustments(t *testing.T) {
	requireMPV(t)

	pool, err := newMPVPlayerPool("")
	if err != nil {
		t.Fatalf("newMPVPlayerPool failed: %v", err)
	}
	defer pool.Close()

	soundFile := getTestSoundFile(t)
	ctx := context.Background()

	// Play multiple sounds with different spatial and gain parameters
	adjustments := []struct {
		gain float64
		pan  float64
	}{
		{0.8, -0.7},
		{0.5, 0.0},
		{0.3, 0.7},
		{0.9, -0.2},
	}

	for _, adj := range adjustments {
		err := pool.Play(ctx, playbackJob{
			File: soundFile,
			Gain: adj.gain,
			Pan:  adj.pan,
		})
		if err != nil {
			t.Fatalf("pool.Play failed for gain=%.1f, pan=%.1f: %v", adj.gain, adj.pan, err)
		}
	}
}

func TestMPVPlayerPoolAutomaticWorkerReplacement(t *testing.T) {
	requireMPV(t)

	pool, err := newMPVPlayerPool("")
	if err != nil {
		t.Fatalf("newMPVPlayerPool failed: %v", err)
	}
	defer pool.Close()

	soundFile := getTestSoundFile(t)
	ctx := context.Background()

	// Force kill worker at index 0
	targetWorker := pool.workers[0]
	if targetWorker.cmd != nil && targetWorker.cmd.Process != nil {
		_ = targetWorker.cmd.Process.Kill()
	}

	// Give OS time to process signal
	time.Sleep(50 * time.Millisecond)

	// Play multiple jobs to trigger selection and replacement of dead worker
	for i := 0; i < mpvPoolSize*2; i++ {
		err := pool.Play(ctx, playbackJob{
			File: soundFile,
			Gain: 0.5,
			Pan:  0.0,
		})
		if err != nil {
			t.Fatalf("pool.Play failed after worker kill on iteration %d: %v", i, err)
		}
	}

	// Verify all workers in pool are alive now
	for i, w := range pool.workers {
		if w == nil || w.isDead() {
			t.Fatalf("worker %d is dead after automatic replacement recovery", i)
		}
	}
}

func TestMPVPlayerPoolConcurrentPlaybacks(t *testing.T) {
	requireMPV(t)

	pool, err := newMPVPlayerPool("")
	if err != nil {
		t.Fatalf("newMPVPlayerPool failed: %v", err)
	}
	defer pool.Close()

	soundFile := getTestSoundFile(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	concurrentCount := 12

	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			pan := -0.8 + float64(idx)*0.15
			err := pool.Play(ctx, playbackJob{
				File: soundFile,
				Gain: 0.6,
				Pan:  pan,
			})
			if err != nil {
				t.Errorf("concurrent Play failed for goroutine %d: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestMPVPlayerPoolCleanShutdown(t *testing.T) {
	requireMPV(t)

	pool, err := newMPVPlayerPool("")
	if err != nil {
		t.Fatalf("newMPVPlayerPool failed: %v", err)
	}

	workers := append([]*mpvWorker(nil), pool.workers...)
	pool.Close()

	for i, w := range workers {
		if !w.isDead() {
			t.Fatalf("worker %d is still alive after pool.Close()", i)
		}
	}
}

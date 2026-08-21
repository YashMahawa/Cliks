package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupMockAudioPlayer(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	mpvPath := filepath.Join(dir, "mpv")
	_ = os.WriteFile(mpvPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDoctorReportDetectsDisconnectedAudioDevice(t *testing.T) {
	setupMockAudioPlayer(t)

	cfg := defaultConfig()
	cfg.Listening.AudioDevice = "missing-headphones-xyz"

	report := buildDoctorReport(cfg)

	foundCheck := false
	for _, check := range report.checks {
		if check.label == "Audio output" && strings.Contains(check.status, "disconnected") {
			foundCheck = true
			break
		}
	}
	if !foundCheck {
		t.Errorf("Expected doctor report check for 'Audio output' to indicate disconnected, checks: %+v", report.checks)
	}

	foundIssue := false
	for _, issue := range report.issues {
		if strings.Contains(issue.title, "unavailable") || strings.Contains(issue.detail, "missing-headphones-xyz") {
			foundIssue = true
			if len(issue.commands) == 0 || issue.commands[0] != "cliks set audio.device default" {
				t.Errorf("Expected issue corrective command 'cliks set audio.device default', got: %v", issue.commands)
			}
			break
		}
	}
	if !foundIssue {
		t.Errorf("Expected doctor issue for disconnected audio device, issues: %+v", report.issues)
	}
}

func TestAudioEngineFallbackToDefaultOnPlaybackFailure(t *testing.T) {
	setupMockAudioPlayer(t)

	listening := ListeningConfig{
		AudioDevice: "missing-device-abc",
		Volume:      0.5,
		Density:     1.0,
	}

	engine := newAudioEngine(listening)
	defer engine.Close()

	// Mock audioCommandRunner to simulate failure on missing-device-abc and success on default
	originalRunner := audioCommandRunner
	defer func() { audioCommandRunner = originalRunner }()

	var callsMu sync.Mutex
	calls := []string{}

	audioCommandRunner = func(ctx context.Context, player *audioPlayer, job playbackJob) error {
		callsMu.Lock()
		calls = append(calls, job.Device)
		callsMu.Unlock()

		if job.Device == "missing-device-abc" {
			return errors.New("device not found")
		}
		return nil
	}

	// Trigger audio playback
	engine.enqueueScaled(RemoteActivityEvent{Kind: "keyboard"}, peerPlacement{Pan: 0, Distance: 1, Warmth: 1}, 1.0)

	// Wait briefly for worker to process job and fallback
	deadline := time.Now().Add(2 * time.Second)
	fallbackOccurred := false
	for time.Now().Before(deadline) {
		engine.mu.Lock()
		device := engine.listening.AudioDevice
		engine.mu.Unlock()
		if device == "" {
			fallbackOccurred = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !fallbackOccurred {
		t.Fatalf("Expected AudioEngine to fall back to default audio device (AudioDevice == ''), got %q", engine.listening.AudioDevice)
	}

	callsMu.Lock()
	defer callsMu.Unlock()
	if len(calls) < 2 {
		t.Fatalf("Expected at least 2 play calls (initial failed call + retried default call), got %v", calls)
	}
	if calls[0] != "missing-device-abc" {
		t.Errorf("First call device = %q, want 'missing-device-abc'", calls[0])
	}
	if calls[1] != "" && calls[1] != "default" {
		t.Errorf("Retried call device = %q, want default ('')", calls[1])
	}
}

func TestRuntimeAudioWarningsAreThrottled(t *testing.T) {
	setupMockAudioPlayer(t)

	listening := ListeningConfig{
		AudioDevice: "missing-device-xyz",
		Volume:      0.5,
	}

	engine := newAudioEngine(listening)
	defer engine.Close()

	engine.mu.Lock()
	engine.warned = false
	engine.mu.Unlock()

	// Call warnFallbackOnce twice
	engine.warnFallbackOnce("missing-device-xyz")
	engine.warnFallbackOnce("missing-device-xyz")

	engine.mu.Lock()
	warned := engine.warned
	engine.mu.Unlock()

	if !warned {
		t.Errorf("Expected engine.warned to be true after warnFallbackOnce")
	}
}

func TestSetAudioDeviceWarnsOnMissingHardware(t *testing.T) {
	setupMockAudioPlayer(t)

	cfg := defaultConfig()

	// Capture stderr
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ok, err := applyConfigSetting(&cfg, "audio.device", "disconnected-headset-999")
	w.Close()
	os.Stderr = origStderr

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if err != nil {
		t.Fatalf("Unexpected error from applyConfigSetting: %v", err)
	}
	if ok {
		t.Fatalf("Expected reconnectRequired = false")
	}
	if cfg.Listening.AudioDevice != "disconnected-headset-999" {
		t.Errorf("Expected AudioDevice = 'disconnected-headset-999', got %q", cfg.Listening.AudioDevice)
	}
	if !strings.Contains(output, "Warning") || !strings.Contains(output, "disconnected-headset-999") {
		t.Errorf("Expected warning on stderr for disconnected device, got output: %q", output)
	}
}

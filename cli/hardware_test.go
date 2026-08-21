package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockFailingInputProvider struct {
	name string
	hint string
}

func (m *mockFailingInputProvider) Name() string { return m.name }

func (m *mockFailingInputProvider) Probe(ctx context.Context) ProbeResult {
	return ProbeResult{
		Name:           m.name,
		Available:      false,
		State:          DriverStateFailed,
		Mode:           "off",
		PermissionHint: m.hint,
		Err:            errors.New(m.hint),
	}
}

func (m *mockFailingInputProvider) Start(ctx context.Context, sharing SharingConfig, events chan<- LocalActivityEvent) (CaptureState, error) {
	return CaptureState{Mode: "off", PermissionHint: m.hint}, errors.New(m.hint)
}

func (m *mockFailingInputProvider) Stop() error { return nil }

type mockWorkingInputProvider struct {
	name string
	mode string
}

func (m *mockWorkingInputProvider) Name() string { return m.name }

func (m *mockWorkingInputProvider) Probe(ctx context.Context) ProbeResult {
	return ProbeResult{
		Name:      m.name,
		Available: true,
		State:     DriverStateDegraded,
		Mode:      m.mode,
	}
}

func (m *mockWorkingInputProvider) Start(ctx context.Context, sharing SharingConfig, events chan<- LocalActivityEvent) (CaptureState, error) {
	events <- LocalActivityEvent{Kind: "keyboard", At: time.Now()}
	return CaptureState{Mode: m.mode}, nil
}

func (m *mockWorkingInputProvider) Stop() error { return nil }

func TestInputCaptureFallbackPreservesBufferingAndDegrades(t *testing.T) {
	capture := newActivityCapture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	failing := &mockFailingInputProvider{name: "failing-global-hook", hint: "Missing global input permissions."}
	working := &mockWorkingInputProvider{name: "terminal-fallback", mode: "terminal"}

	providers := []InputCaptureProvider{failing, working}
	var hints []string
	var activeState CaptureState

	for _, provider := range providers {
		probeRes := provider.Probe(ctx)
		state, err := provider.Start(ctx, SharingConfig{Keyboard: true, Mouse: true}, capture.Events)
		if err == nil && state.Mode != "off" {
			if len(hints) > 0 {
				state.PermissionHint = strings.Join(hints, " ") + " " + state.PermissionHint
			}
			activeState = state
			break
		}
		if state.PermissionHint != "" {
			hints = append(hints, state.PermissionHint)
		} else if probeRes.PermissionHint != "" {
			hints = append(hints, probeRes.PermissionHint)
		}
	}

	if activeState.Mode != "terminal" {
		t.Fatalf("activeState mode = %q, want terminal", activeState.Mode)
	}
	if !strings.Contains(activeState.PermissionHint, "Missing global input permissions") {
		t.Fatalf("permission hint missing failure detail: %q", activeState.PermissionHint)
	}

	select {
	case event := <-capture.Events:
		if event.Kind != "keyboard" {
			t.Fatalf("event kind = %q, want keyboard", event.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("event buffer dropped during provider fallback")
	}
}

func TestProbeHardwareSubsystemReportsStatus(t *testing.T) {
	cfg := defaultConfig()
	status := ProbeHardwareSubsystem(context.Background(), cfg)

	if status.InputProbes == nil {
		t.Fatal("expected InputProbes list")
	}
	if status.AudioProbes == nil {
		t.Fatal("expected AudioProbes list")
	}
}

func TestAudioBackendFallbackSelection(t *testing.T) {
	engine := newAudioEngine(ListeningConfig{})
	defer engine.Close()

	fallback := engine.tryFallbackAudioPlayer("nonexistent-player")
	if fallback == nil && hasCommand("afplay") {
		t.Fatal("expected fallback audio player on macOS")
	}
}

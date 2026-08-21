package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func isolateHotReloadConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	return root
}

func TestCLICommandsEnqueueReloadConfig(t *testing.T) {
	isolateHotReloadConfig(t)

	// 1. cmdSet (non-endpoint change)
	if err := cmdSet([]string{"share.keyboard", "off"}); err != nil {
		t.Fatalf("cmdSet failed: %v", err)
	}
	var commands []localSessionCommand
	consumeSessionCommands(func(cmd localSessionCommand) { commands = append(commands, cmd) })
	if len(commands) != 1 || commands[0].Type != "reload_config" {
		t.Fatalf("commands = %+v, want 1 reload_config command", commands)
	}

	// 2. cmdPreset
	if err := cmdPreset([]string{"social"}); err != nil {
		t.Fatalf("cmdPreset failed: %v", err)
	}
	commands = nil
	consumeSessionCommands(func(cmd localSessionCommand) { commands = append(commands, cmd) })
	if len(commands) != 1 || commands[0].Type != "reload_config" {
		t.Fatalf("commands = %+v, want 1 reload_config command", commands)
	}

	// 3. cmdNickname
	if err := cmdNickname([]string{"HotReloadUser"}); err != nil {
		t.Fatalf("cmdNickname failed: %v", err)
	}
	commands = nil
	consumeSessionCommands(func(cmd localSessionCommand) { commands = append(commands, cmd) })
	if len(commands) != 1 || commands[0].Type != "reload_config" {
		t.Fatalf("commands = %+v, want 1 reload_config command", commands)
	}
}

func TestSessionControllerHotReloadsPrivacySettings(t *testing.T) {
	isolateHotReloadConfig(t)

	// Initial config: keyboard and mouse sharing enabled
	cfg := defaultConfig()
	cfg.Sharing.Keyboard = true
	cfg.Sharing.Mouse = true
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	controller := newSessionController(cfg, StartOptions{}, instance)
	defer controller.stop()

	// Initially, keyboard and mouse events should be recorded
	controller.recordLocalActivity(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
	select {
	case evt := <-controller.local:
		if evt.Kind != "keyboard" {
			t.Fatalf("expected keyboard event, got %v", evt.Kind)
		}
	default:
		t.Fatal("expected keyboard event to be recorded")
	}

	// Now disable keyboard sharing via CLI
	if err := cmdSet([]string{"share.keyboard", "off"}); err != nil {
		t.Fatal(err)
	}

	// Run reload in session or wait for commandLoop to consume command
	go controller.commandLoop()

	// Wait up to 500ms for session controller to detect and process reload_config
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		controller.mu.Lock()
		kbSharing := controller.cfg.Sharing.Keyboard
		controller.mu.Unlock()
		if !kbSharing {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	controller.mu.Lock()
	kbSharing := controller.cfg.Sharing.Keyboard
	controller.mu.Unlock()
	if kbSharing {
		t.Fatal("expected keyboard sharing to be false within 500ms")
	}

	// Record keyboard activity - must be halted immediately
	controller.recordLocalActivity(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
	select {
	case evt := <-controller.local:
		t.Fatalf("expected no keyboard event when sharing is off, got %v", evt)
	default:
		// Success: halted
	}

	// Mouse activity should still be recorded
	controller.recordLocalActivity(LocalActivityEvent{Kind: "mouse", Button: "left", At: time.Now()})
	select {
	case evt := <-controller.local:
		if evt.Kind != "mouse" {
			t.Fatalf("expected mouse event, got %v", evt.Kind)
		}
	default:
		t.Fatal("expected mouse event to be recorded")
	}

	// Resume keyboard sharing via CLI
	if err := cmdSet([]string{"share.keyboard", "on"}); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		controller.mu.Lock()
		kbSharing = controller.cfg.Sharing.Keyboard
		controller.mu.Unlock()
		if kbSharing {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	controller.mu.Lock()
	kbSharing = controller.cfg.Sharing.Keyboard
	controller.mu.Unlock()
	if !kbSharing {
		t.Fatal("expected keyboard sharing to be true within 500ms")
	}

	// Keyboard activity should now be recorded again
	controller.recordLocalActivity(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
	select {
	case evt := <-controller.local:
		if evt.Kind != "keyboard" {
			t.Fatalf("expected keyboard event after resuming, got %v", evt.Kind)
		}
	default:
		t.Fatal("expected keyboard event to be recorded after resuming")
	}
}

func TestSessionControllerHotReloadsAudioSettings(t *testing.T) {
	isolateHotReloadConfig(t)

	cfg := defaultConfig()
	cfg.Listening.Volume = 0.8
	cfg.Listening.Spatial = true
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	controller := newSessionController(cfg, StartOptions{}, instance)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller.audio = newAudioEngineWithContext(ctx, cfg.Listening)
	defer controller.stop()

	go controller.commandLoop()

	// Update volume and spatial audio via CLI
	if err := cmdSet([]string{"volume", "0.25", "hear.spatial", "off"}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		controller.mu.Lock()
		vol := controller.cfg.Listening.Volume
		spatial := controller.cfg.Listening.Spatial
		controller.mu.Unlock()
		if vol == 0.25 && !spatial {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	controller.mu.Lock()
	vol := controller.cfg.Listening.Volume
	spatial := controller.cfg.Listening.Spatial
	controller.mu.Unlock()

	if vol != 0.25 {
		t.Fatalf("volume = %v, want 0.25", vol)
	}
	if spatial {
		t.Fatalf("spatial = %v, want false", spatial)
	}

	controller.audio.mu.Lock()
	audioVol := controller.audio.listening.Volume
	audioSpatial := controller.audio.listening.Spatial
	controller.audio.mu.Unlock()

	if audioVol != 0.25 {
		t.Fatalf("audio engine volume = %v, want 0.25", audioVol)
	}
	if audioSpatial {
		t.Fatalf("audio engine spatial = %v, want false", audioSpatial)
	}
}

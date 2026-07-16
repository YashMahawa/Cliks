package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSoloDeskUsesLocalSimulationConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.Solo = SoloConfig{People: 6, Keyboard: true, Mouse: false}
	model := newSoloModel(cfg)
	defer model.audio.Close()
	if got := len(model.state.Peers); got != 6 {
		t.Fatalf("peers = %d, want 6", got)
	}
	if model.state.ConnectionStatus != "offline · private" || model.state.CaptureMode != "simulation" {
		t.Fatalf("Solo Desk unexpectedly resembles a live session: %#v", model.state)
	}
	view := model.View()
	for _, want := range []string{"Solo Desk", "OFFLINE", "No network. Nothing", "is sent."} {
		if !strings.Contains(view, want) {
			t.Fatalf("solo view missing %q:\n%s", want, view)
		}
	}
}

func TestSoloControlsPersistWithoutTeamOrSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	model := newSoloModel(defaultConfig())
	defer model.audio.Close()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	got := updated.(soloModel)
	if got.cfg.Solo.People != 5 {
		t.Fatalf("people = %d, want 5", got.cfg.Solo.People)
	}
	if saved := loadConfig(); saved.Solo.People != 5 {
		t.Fatalf("saved people = %d, want 5", saved.Solo.People)
	}
	got.now = time.Now()
	got.spark()
	if len(got.state.RecentPeerActivity) == 0 {
		t.Fatal("Wake the room did not create local activity")
	}
}

func TestSoloCanKeepBothActivitySoundsOff(t *testing.T) {
	cfg := defaultConfig()
	cfg.Solo = SoloConfig{People: 3, Keyboard: false, Mouse: false}
	normalizeConfig(&cfg)
	if cfg.Solo.Keyboard || cfg.Solo.Mouse {
		t.Fatalf("normalization re-enabled Solo activity sounds: %#v", cfg.Solo)
	}
}

func TestSoloShowsAndPersistsIndependentSoundLevels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	model := newSoloModel(cfg)
	defer model.audio.Close()
	model.width, model.height = 120, 36
	for _, want := range []string{"Master", "Keyboard level", "Click level", "Room tone level"} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("solo view missing %q", want)
		}
	}
	before := model.cfg.Solo.MouseVolume
	updated, _ := model.activate("mouse-quieter")
	got := updated.(soloModel)
	if got.cfg.Solo.MouseVolume >= before {
		t.Fatalf("mouse volume did not decrease: before=%v after=%v", before, got.cfg.Solo.MouseVolume)
	}
	if saved := loadConfig(); saved.Solo.MouseVolume != got.cfg.Solo.MouseVolume {
		t.Fatalf("saved mouse volume=%v, want %v", saved.Solo.MouseVolume, got.cfg.Solo.MouseVolume)
	}
}

func TestSoloNarrowLayoutKeepsControlsVisibleAndClickable(t *testing.T) {
	cfg := defaultConfig()
	model := newSoloModel(cfg)
	defer model.audio.Close()
	model.width, model.height = 68, 42
	view := model.View()
	x, y := renderedTextPosition(t, view, "[ Keyboard")
	if action := model.hit(x+2, y); action != "keyboard" {
		t.Fatalf("narrow Keyboard hit = %q, want keyboard at %d,%d; regions=%+v\n%s", action, x+2, y, model.hitRegions(), ansi.Strip(view))
	}
	if !strings.Contains(view, "Room tone level") {
		t.Fatal("narrow Solo layout hid independent volume controls")
	}
}

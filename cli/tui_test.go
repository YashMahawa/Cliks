package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeClickOutsideHighlightedRowDoesNotActivate(t *testing.T) {
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := homeModel{
		cfg:       cfg,
		mode:      "home",
		cursor:    2,
		mouseOver: false,
	}

	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 1, Y: 0})
	got := updated.(homeModel)
	if cmd != nil {
		t.Fatalf("outside click returned command")
	}
	if got.mode != "home" {
		t.Fatalf("mode = %q, want home", got.mode)
	}
	if got.action != "" {
		t.Fatalf("action = %q, want none", got.action)
	}
	if got.cursor != model.cursor {
		t.Fatalf("cursor = %d, want %d", got.cursor, model.cursor)
	}
}

func TestPreferencesClickOutsideRowDoesNotChangeSetting(t *testing.T) {
	cfg := defaultConfig()
	model := homeModel{
		cfg:            cfg,
		mode:           "preferences",
		settingsCursor: 2,
		mouseOver:      false,
	}

	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 1, Y: 0})
	got := updated.(homeModel)
	if cmd != nil {
		t.Fatalf("outside preferences click returned command")
	}
	if got.cfg.Listening.Muted != cfg.Listening.Muted {
		t.Fatalf("muted = %v, want %v", got.cfg.Listening.Muted, cfg.Listening.Muted)
	}
	if got.settingsCursor != model.settingsCursor {
		t.Fatalf("settingsCursor = %d, want %d", got.settingsCursor, model.settingsCursor)
	}
}

func TestKeepRunningToggleWithoutActiveSessionDoesNotStart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := homeModel{
		cfg:    cfg,
		mode:   "home",
		cursor: 1,
	}

	updated, cmd := model.activate()
	got := updated.(homeModel)
	if cmd != nil {
		t.Fatalf("keep-running toggle returned command")
	}
	if got.action != "" {
		t.Fatalf("action = %q, want none", got.action)
	}
	if !got.cfg.KeepRunning {
		t.Fatalf("KeepRunning = false, want true")
	}
	if saved := loadConfig(); !saved.KeepRunning {
		t.Fatalf("saved KeepRunning = false, want true")
	}
}

func TestLiveEscapeReturnsToHomeInsteadOfStopping(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(sessionModel)
	if cmd == nil {
		t.Fatalf("escape did not request quit/back transition")
	}
	if got.exit != sessionExitBack {
		t.Fatalf("exit = %q, want back", got.exit)
	}
}

func TestLiveControlHoverAndClickBack(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)
	model.width = 120
	y := model.controlsContentY()

	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseMotion, X: 4, Y: y})
	got := updated.(sessionModel)
	if cmd != nil {
		t.Fatalf("hover returned command")
	}
	if got.buttonHover != 0 {
		t.Fatalf("buttonHover = %d, want 0", got.buttonHover)
	}

	updated, cmd = got.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 4, Y: y})
	got = updated.(sessionModel)
	if cmd == nil {
		t.Fatalf("back click did not request transition")
	}
	if got.exit != sessionExitBack {
		t.Fatalf("exit = %q, want back", got.exit)
	}
}

func TestLiveTabOpensUnifiedPreferences(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(sessionModel)
	if got.mode != "settings" {
		t.Fatalf("mode = %q, want settings", got.mode)
	}
	view := got.View()
	if !strings.Contains(view, "Dynamic circle") || !strings.Contains(view, "Keep Running") {
		t.Fatalf("settings view does not include unified preference rows:\n%s", view)
	}
}

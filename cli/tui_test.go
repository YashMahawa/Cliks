package main

import (
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

package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeymapFallbackDefaults(t *testing.T) {
	cfg := defaultConfig()
	// empty keymap should fall back to standard defaults
	cfg.Keymap = KeymapConfig{}

	if got := getBinding(cfg, "up"); got != "up, k" {
		t.Fatalf("up binding = %q, want 'up, k'", got)
	}
	if got := getBinding(cfg, "down"); got != "down, j" {
		t.Fatalf("down binding = %q, want 'down, j'", got)
	}
	if got := getBinding(cfg, "select"); got != "enter, space" {
		t.Fatalf("select binding = %q, want 'enter, space'", got)
	}
	if got := getBinding(cfg, "help"); got != "?" {
		t.Fatalf("help binding = %q, want '?'", got)
	}
}

func TestCustomKeymapRemapping(t *testing.T) {
	cfg := defaultConfig()
	cfg.Keymap.Up = "w"
	cfg.Keymap.Down = "s"
	cfg.Keymap.Left = "a"
	cfg.Keymap.Right = "d"

	if !keyMatches("w", getBinding(cfg, "up")) {
		t.Fatal("keyMatches('w', up) failed for custom binding")
	}
	if keyMatches("k", getBinding(cfg, "up")) {
		t.Fatal("keyMatches('k', up) should be false when overridden with 'w'")
	}
	if !keyMatches("s", getBinding(cfg, "down")) {
		t.Fatal("keyMatches('s', down) failed for custom binding")
	}
}

func TestDynamicShortcutHelpView(t *testing.T) {
	cfg := defaultConfig()
	cfg.Keymap.Up = "w"
	cfg.Keymap.Down = "s"
	cfg.Keymap.Help = "h"

	view := shortcutHelpView("home", 80, cfg)
	if !strings.Contains(view, "w, s") {
		t.Fatalf("shortcutHelpView missing customized navigation keys 'w, s':\n%s", view)
	}
	if !strings.Contains(view, "Press h or") {
		t.Fatalf("shortcutHelpView missing customized help key 'h':\n%s", view)
	}
}

func TestApplyConfigSettingKeymaps(t *testing.T) {
	cfg := defaultConfig()
	changed, err := applyConfigSetting(&cfg, "keymap.up", "w")
	if err != nil || changed {
		t.Fatalf("applyConfigSetting(keymap.up) err=%v changed=%v", err, changed)
	}
	if cfg.Keymap.Up != "w" {
		t.Fatalf("cfg.Keymap.Up = %q, want 'w'", cfg.Keymap.Up)
	}
}

func TestCustomKeymapNavigationInHomeModel(t *testing.T) {
	cfg := defaultConfig()
	cfg.Keymap.Down = "s"
	model := homeModel{
		cfg:    cfg,
		mode:   "home",
		cursor: 0,
	}

	// Pressing 's' should navigate down because Down is remapped to 's'
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(homeModel)
	if got.cursor != 1 {
		t.Fatalf("after pressing remapped key 's', cursor = %d, want 1", got.cursor)
	}
}

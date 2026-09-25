package main

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func isolateWizardConfig(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
}

func TestSettingCatalogCategorizationCompleteness(t *testing.T) {
	if len(settingCategories) != 5 {
		t.Fatalf("expected 5 setting categories, got %d", len(settingCategories))
	}

	seenKeys := make(map[string]bool)
	for _, cat := range settingCategories {
		items := settingsForCategory(cat.ID)
		if len(items) == 0 {
			t.Fatalf("category %s has no settings assigned", cat.Name)
		}
		for _, item := range items {
			if item.Key == "" || item.Label == "" || item.ControlType == "" {
				t.Fatalf("setting in category %s missing required metadata: %+v", cat.Name, item)
			}
			seenKeys[item.Key] = true
		}
	}

	if len(seenKeys) < len(configSettingCatalog) {
		t.Fatalf("not all catalog settings categorized: seen %d, catalog %d", len(seenKeys), len(configSettingCatalog))
	}
}

func TestCmdSetZeroArgsLaunchesWizardWithoutError(t *testing.T) {
	isolateWizardConfig(t)
	// cmdSet with 0 args in non-interactive environment should complete cleanly
	if err := cmdSet([]string{}); err != nil {
		t.Fatalf("cmdSet([]) returned error: %v", err)
	}
}

func TestCmdSetPositionalBypassesWizard(t *testing.T) {
	isolateWizardConfig(t)
	if err := cmdSet([]string{"theme", "ocean"}); err != nil {
		t.Fatalf("cmdSet positional failed: %v", err)
	}
	cfg := loadConfig()
	if cfg.Theme != "ocean" {
		t.Fatalf("expected theme ocean, got %s", cfg.Theme)
	}
}

func TestWizardCategoryNavigation(t *testing.T) {
	cfg := defaultConfig()
	m := newSettingsWizardModel(cfg)

	if m.currentCategory().ID != "audio" {
		t.Fatalf("expected initial category audio, got %s", m.currentCategory().ID)
	}

	// Press 2 to jump to Privacy category
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(settingsWizardModel)
	if m.currentCategory().ID != "privacy" {
		t.Fatalf("expected category privacy after key '2', got %s", m.currentCategory().ID)
	}

	// Press ] to jump to next category (Appearance)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = updated.(settingsWizardModel)
	if m.currentCategory().ID != "appearance" {
		t.Fatalf("expected category appearance after key ']', got %s", m.currentCategory().ID)
	}

	// Press [ to jump back to Privacy
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = updated.(settingsWizardModel)
	if m.currentCategory().ID != "privacy" {
		t.Fatalf("expected category privacy after key '[', got %s", m.currentCategory().ID)
	}
}

func TestWizardWidgetToggleAndSliderControls(t *testing.T) {
	isolateWizardConfig(t)
	cfg := defaultConfig()
	m := newSettingsWizardModel(cfg)

	// Category 0 (Audio): hear.muted is a toggle setting
	// Find index of hear.muted in audio category items
	items := m.currentItems()
	mutedIdx := -1
	for i, item := range items {
		if item.Key == "hear.muted" {
			mutedIdx = i
			break
		}
	}
	if mutedIdx < 0 {
		t.Fatal("hear.muted not found in audio category")
	}

	m.itemIndex = mutedIdx
	m.focusArea = 1 // Settings list focus

	// Toggle hear.muted using Right arrow
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(settingsWizardModel)
	if !m.cfg.Listening.Muted {
		t.Fatal("expected hear.muted to be toggled to true")
	}

	// Find volume slider in audio category
	volIdx := -1
	for i, item := range items {
		if item.Key == "volume" {
			volIdx = i
			break
		}
	}
	if volIdx < 0 {
		t.Fatal("volume not found in audio category")
	}

	m.itemIndex = volIdx
	initialVol := m.cfg.Listening.Volume

	// Decrement volume using Left arrow
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(settingsWizardModel)
	if m.cfg.Listening.Volume >= initialVol {
		t.Fatalf("expected volume to decrease from %f, got %f", initialVol, m.cfg.Listening.Volume)
	}
}

func TestWizardWidgetSelectPickerCycling(t *testing.T) {
	isolateWizardConfig(t)
	cfg := defaultConfig()
	m := newSettingsWizardModel(cfg)

	// Jump to category 3 (Appearance)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updated.(settingsWizardModel)

	// Find theme setting
	items := m.currentItems()
	themeIdx := -1
	for i, item := range items {
		if item.Key == "theme" {
			themeIdx = i
			break
		}
	}
	if themeIdx < 0 {
		t.Fatal("theme not found in appearance category")
	}

	m.itemIndex = themeIdx
	m.focusArea = 1

	initialTheme := m.cfg.Theme
	// Cycle theme with Right arrow
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(settingsWizardModel)
	if m.cfg.Theme == initialTheme {
		t.Fatalf("expected theme to cycle away from %s, got %s", initialTheme, m.cfg.Theme)
	}
}

func TestWizardRejectsInvalidInputPriorToSave(t *testing.T) {
	isolateWizardConfig(t)
	cfg := defaultConfig()
	m := newSettingsWizardModel(cfg)

	// Jump to category 5 (System)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m = updated.(settingsWizardModel)

	// Attempt to set invalid api.url directly via applyVal
	err := m.applyVal("api.url", "invalid-url")
	if err == nil {
		t.Fatal("expected invalid api.url to fail validation")
	}
	if !m.isError {
		t.Fatal("expected model isError to be true after invalid input")
	}
	if m.cfg.APIURL == "invalid-url" {
		t.Fatal("invalid URL was applied to model state")
	}
}

func TestWizardSaveAndApplyPersistsAndTriggersReload(t *testing.T) {
	isolateWizardConfig(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Acquire mock running session
	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	cfg := defaultConfig()
	m := newSettingsWizardModel(cfg)

	// Change theme and server URL
	_ = m.applyVal("theme", "ocean")
	_ = m.applyVal("api.url", "https://self-hosted.example")

	// Save and apply
	cmd := m.saveAndApply()
	if cmd == nil {
		t.Fatal("expected saveAndApply to return tea.Quit cmd")
	}

	// Verify settings were saved to disk
	savedCfg := loadConfig()
	if savedCfg.Theme != "ocean" || savedCfg.APIURL != "https://self-hosted.example" {
		t.Fatalf("saved config mismatch: %+v", savedCfg)
	}

	// Verify session reload command was enqueued
	var commands []localSessionCommand
	consumeSessionCommands(func(cmd localSessionCommand) { commands = append(commands, cmd) })
	if len(commands) != 1 || commands[0].Type != "reload_connection" {
		t.Fatalf("expected 1 reload_connection command, got %+v", commands)
	}
}

func TestWizardViewRendering(t *testing.T) {
	cfg := defaultConfig()
	m := newSettingsWizardModel(cfg)
	viewOutput := m.View()

	if viewOutput == "" {
		t.Fatal("View() output was empty")
	}
	if !containsString(viewOutput, "Cliks Settings Wizard") {
		t.Fatalf("View() output missing title bar: %s", viewOutput)
	}
	if !containsString(viewOutput, "Audio & Sound") {
		t.Fatalf("View() output missing active category: %s", viewOutput)
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(haystack) > 0 && os.Getenv("GO_TEST") == "" && stringContains(haystack, needle))
}

func stringContains(a, b string) bool {
	for i := 0; i+len(b) <= len(a); i++ {
		if a[i:i+len(b)] == b {
			return true
		}
	}
	return false
}

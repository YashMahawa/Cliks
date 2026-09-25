package main

import (
	"strings"
	"testing"
)

func TestNOColorDisablesColorOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "")

	applyThemeWithPalette("ember", PaletteConfig{Accent: "#FF0000"})
	rendered := styleAccent.Render("test text")

	if strings.Contains(rendered, "\x1b[") {
		t.Fatalf("rendered output contains ANSI escapes when NO_COLOR is set: %q", rendered)
	}
	if rendered != "test text" {
		t.Fatalf("rendered text = %q, want %q", rendered, "test text")
	}
}

func TestCLIColorForceForcesColorOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("CLICOLOR", "")

	applyThemeWithPalette("ember", PaletteConfig{Accent: "#FF0000"})
	rendered := styleAccent.Render("test text")

	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("rendered output missing ANSI escapes when CLICOLOR_FORCE=1: %q", rendered)
	}
}

func TestCLIColorZeroDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "0")

	applyThemeWithPalette("ember", PaletteConfig{Accent: "#FF0000"})
	rendered := styleAccent.Render("test text")

	if strings.Contains(rendered, "\x1b[") {
		t.Fatalf("rendered output contains ANSI escapes when CLICOLOR=0: %q", rendered)
	}
}

func TestValidCustomPaletteOverridesTheme(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")

	// Apply default ember theme
	applyThemeWithPalette("ember", PaletteConfig{})
	defaultEmberAccentRender := styleAccent.Render("test")

	// Apply ember theme with custom green accent
	applyThemeWithPalette("ember", PaletteConfig{Accent: "#00FF00"})
	customEmberAccentRender := styleAccent.Render("test")

	if defaultEmberAccentRender == customEmberAccentRender {
		t.Fatalf("custom palette accent #00FF00 did not override default ember accent")
	}
}

func TestInvalidCustomPaletteFallback(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")

	applyThemeWithPalette("ember", PaletteConfig{})
	defaultEmberAccentRender := styleAccent.Render("test")

	// Invalid hex code in Accent should fall back to default theme color
	applyThemeWithPalette("ember", PaletteConfig{Accent: "INVALID_HEX_CODE", Panel: "#123456"})
	fallbackAccentRender := styleAccent.Render("test")

	if fallbackAccentRender != defaultEmberAccentRender {
		t.Fatalf("invalid hex accent did not fall back cleanly to default theme accent")
	}
}

func TestUnconfiguredPaletteKeysRetainDefaultTheme(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")

	// Apply default ocean theme
	applyThemeWithPalette("ocean", PaletteConfig{})
	defaultPanelRender := stylePanel.Render("panel")

	// Apply ocean theme with custom accent only
	applyThemeWithPalette("ocean", PaletteConfig{Accent: "#FF0000"})
	panelWithCustomAccentRender := stylePanel.Render("panel")

	if defaultPanelRender != panelWithCustomAccentRender {
		t.Fatalf("unconfigured panel palette key did not retain default ocean theme color")
	}
}

func TestCustomPalettePersistence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := defaultConfig()
	cfg.Palette = PaletteConfig{
		Accent: "#123456",
		Panel:  "#654321",
	}

	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}

	loaded := loadConfig()
	if loaded.Palette.Accent != "#123456" {
		t.Fatalf("loaded.Palette.Accent = %q, want #123456", loaded.Palette.Accent)
	}
	if loaded.Palette.Panel != "#654321" {
		t.Fatalf("loaded.Palette.Panel = %q, want #654321", loaded.Palette.Panel)
	}
}

func TestNOColorOverridesCustomPalette(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "1") // NO_COLOR overrides CLICOLOR_FORCE as well

	applyThemeWithPalette("ember", PaletteConfig{Accent: "#FF0000", Panel: "#00FF00"})
	renderedAccent := styleAccent.Render("accent")
	renderedPanel := stylePanel.Render("panel")

	if strings.Contains(renderedAccent, "\x1b[") || strings.Contains(renderedPanel, "\x1b[") {
		t.Fatalf("NO_COLOR=1 failed to override active custom hex palette")
	}
}

func TestCmdSetPaletteSetting(t *testing.T) {
	cfg := defaultConfig()

	// Valid hex
	reconnectNeeded, err := applyConfigSetting(&cfg, "palette.accent", "#00FF00")
	if err != nil || reconnectNeeded {
		t.Fatalf("applyConfigSetting palette.accent failed: reconnectNeeded=%v, err=%v", reconnectNeeded, err)
	}
	if cfg.Palette.Accent != "#00FF00" {
		t.Fatalf("cfg.Palette.Accent = %q, want #00FF00", cfg.Palette.Accent)
	}

	// Hex without leading #
	reconnectNeeded, err = applyConfigSetting(&cfg, "palette.panel", "123456")
	if err != nil || reconnectNeeded {
		t.Fatalf("applyConfigSetting palette.panel failed: reconnectNeeded=%v, err=%v", reconnectNeeded, err)
	}
	if cfg.Palette.Panel != "#123456" {
		t.Fatalf("cfg.Palette.Panel = %q, want #123456", cfg.Palette.Panel)
	}

	// Clear setting with "default"
	reconnectNeeded, err = applyConfigSetting(&cfg, "palette.accent", "default")
	if err != nil || reconnectNeeded {
		t.Fatalf("applyConfigSetting palette.accent default failed: reconnectNeeded=%v, err=%v", reconnectNeeded, err)
	}
	if cfg.Palette.Accent != "" {
		t.Fatalf("cfg.Palette.Accent = %q, want empty", cfg.Palette.Accent)
	}

	// Invalid hex setting
	_, err = applyConfigSetting(&cfg, "palette.accent", "invalid-color")
	if err == nil {
		t.Fatalf("expected error setting invalid hex color, got nil")
	}
}

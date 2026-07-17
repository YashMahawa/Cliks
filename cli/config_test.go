package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCaptureDefaultsToIsolatedAndRejectsUnknownModes(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Capture.Mode != "isolated" {
		t.Fatalf("default capture mode = %q, want isolated", cfg.Capture.Mode)
	}
	cfg.Capture.Mode = "trust-everything"
	normalizeConfig(&cfg)
	if cfg.Capture.Mode != "isolated" {
		t.Fatalf("normalized capture mode = %q, want isolated", cfg.Capture.Mode)
	}
}

func TestForgetTeamRemovesCurrentAndSelectsNext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-ONE111"
	cfg.Teams = []TeamConfig{
		{Code: "CLIK-ONE111", Name: "One"},
		{Code: "CLIK-TWO222", Name: "Two"},
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	next, err := forgetTeam("CLIK-ONE111")
	if err != nil {
		t.Fatalf("forget team: %v", err)
	}
	if next.CurrentTeamCode != "CLIK-TWO222" {
		t.Fatalf("current team = %q, want CLIK-TWO222", next.CurrentTeamCode)
	}
	if len(next.Teams) != 1 || next.Teams[0].Code != "CLIK-TWO222" {
		t.Fatalf("teams = %+v, want only CLIK-TWO222", next.Teams)
	}
}

func TestSanitizeNicknameCapsAtTenRunes(t *testing.T) {
	if got := sanitizeNickname("  Alice Long Name  "); got != "Alice Long" {
		t.Fatalf("nickname = %q, want Alice Long", got)
	}
}

func TestSanitizeNicknameStripsTerminalSequencesBeforeTruncating(t *testing.T) {
	input := "\x1b[31mAlice\x1b[0m\x1b]0;owned\x07 Long Name"
	if got := sanitizeNickname(input); got != "Alice Long" {
		t.Fatalf("nickname = %q, want Alice Long", got)
	}
}

func TestSanitizeNicknameRemovesControlAndFormatCharacters(t *testing.T) {
	if got := sanitizeNickname("Ali\x00ce\u202e Bob"); got != "Alice Bob" {
		t.Fatalf("nickname = %q, want Alice Bob", got)
	}
}

func TestTeamLabelIncludesNameAndCode(t *testing.T) {
	cfg := defaultConfig()
	cfg.Teams = []TeamConfig{{Code: "CLIK-ABC123", Name: "Design"}}
	if got := teamLabel(cfg, "clik-abc123"); got != "Design (CLIK-ABC123)" {
		t.Fatalf("teamLabel = %q, want name and code", got)
	}
}

func TestEnvironmentURLsOverrideSavedConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.APIURL = "https://saved.example"
	cfg.WSURL = "wss://saved.example/ws"
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	t.Setenv("CLIKS_API_URL", "https://env.example/")
	t.Setenv("CLIKS_WS_URL", "wss://env.example/socket")
	loaded := loadConfig()
	if loaded.APIURL != "https://env.example" || loaded.WSURL != "wss://env.example/socket" {
		t.Fatalf("loaded URLs = %q, %q; want environment overrides", loaded.APIURL, loaded.WSURL)
	}
}

func TestPublicBackendLocksBatchWindowToFiveHundredMilliseconds(t *testing.T) {
	cfg := defaultConfig()
	cfg.BatchWindowMs = 100
	normalizeConfig(&cfg)
	if cfg.BatchWindowMs != 500 {
		t.Fatalf("public batch window = %d, want 500", cfg.BatchWindowMs)
	}
}

func TestSelfHostedBackendKeepsCustomBatchWindow(t *testing.T) {
	cfg := defaultConfig()
	cfg.APIURL = "https://cliks.example.com"
	cfg.WSURL = toWSURL(cfg.APIURL)
	cfg.BatchWindowMs = 150
	normalizeConfig(&cfg)
	if cfg.BatchWindowMs != 150 || usesPublicBackend(cfg) {
		t.Fatalf("self-hosted config normalized incorrectly: %+v", cfg)
	}
}

func TestNormalizeBackendURLSupportsPublicAlias(t *testing.T) {
	if got, err := normalizeBackendURL("public"); err != nil || got != productionAPIURL {
		t.Fatalf("normalizeBackendURL(public) = %q, %v", got, err)
	}
	if _, err := normalizeBackendURL("not-a-url"); err == nil {
		t.Fatal("normalizeBackendURL accepted a URL without http(s)")
	}
}

func TestParseOnOffRejectsUnknownValues(t *testing.T) {
	if _, err := parseOnOff("sometimes"); err == nil {
		t.Fatal("parseOnOff accepted an ambiguous value")
	}
	if enabled, err := parseOnOff("enabled"); err != nil || !enabled {
		t.Fatalf("parseOnOff(enabled) = %v, %v", enabled, err)
	}
}

func TestInvalidConfigSurfacesWarningInsteadOfSilentDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "cliks", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = loadConfig()
	warning := lastConfigLoadWarning()
	if warning == "" {
		t.Fatal("expected config load warning for invalid JSON")
	}
	// Saving a clean config should clear the warning on the next load.
	if err := saveConfig(defaultConfig()); err != nil {
		t.Fatal(err)
	}
	_ = loadConfig()
	if lastConfigLoadWarning() != "" {
		t.Fatalf("warning still set after valid save: %q", lastConfigLoadWarning())
	}
}

func TestSaveConfigIsAtomicAndReadable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.Nickname = "Atomic"
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded := loadConfig()
	if loaded.Nickname != "Atomic" {
		t.Fatalf("nickname = %q", loaded.Nickname)
	}
	// No leftover temp files next to the real config.
	entries, err := os.ReadDir(filepath.Dir(configPath()))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, ".tmp-") || strings.Contains(name, ".tmp.") {
			t.Fatalf("leftover temp file: %s", name)
		}
	}
}

func TestConfigPathUsesAppDataOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows path layout")
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", `C:\Users\test\AppData\Roaming`)
	got := configPath()
	want := filepath.Join(`C:\Users\test\AppData\Roaming`, "cliks", "config.json")
	if got != want {
		t.Fatalf("configPath = %q, want %q", got, want)
	}
}

func TestSystemdQuoteHandlesSpaces(t *testing.T) {
	got := systemdQuote(`/home/user/My Apps/cliks`)
	if got != `"/home/user/My Apps/cliks"` {
		t.Fatalf("systemdQuote = %q", got)
	}
}

func TestScaleWavFileGainReducesAmplitude(t *testing.T) {
	samples, err := filepath.Glob("assets/sounds/keyboard/*.wav")
	if err != nil || len(samples) == 0 {
		t.Skip("bundled keyboard samples missing")
	}
	original, err := os.ReadFile(samples[0])
	if err != nil {
		t.Fatal(err)
	}
	out, cleanup, err := scaleWavFileGain(samples[0], 0.5)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	scaled, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(scaled) != len(original) {
		t.Fatalf("scaled length %d != original %d", len(scaled), len(original))
	}
	// Headers should match; payload should differ for a non-silent sample.
	if string(scaled[:44]) != string(original[:44]) {
		// Not all WAVs are exactly 44-byte headers; just ensure we wrote something.
		if len(scaled) < 44 {
			t.Fatal("scaled file too small")
		}
	}
	same := true
	for i := range scaled {
		if scaled[i] != original[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("scaled WAV is identical to original at gain 0.5")
	}
}

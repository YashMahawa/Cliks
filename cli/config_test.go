package main

import "testing"

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

func TestParseOnOffRejectsUnknownValues(t *testing.T) {
	if _, err := parseOnOff("sometimes"); err == nil {
		t.Fatal("parseOnOff accepted an ambiguous value")
	}
	if enabled, err := parseOnOff("enabled"); err != nil || !enabled {
		t.Fatalf("parseOnOff(enabled) = %v, %v", enabled, err)
	}
}

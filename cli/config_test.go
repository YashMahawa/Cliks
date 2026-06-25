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

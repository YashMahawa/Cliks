package main

import "testing"

func TestSetSupportsMultiplePairsAndSoloLevels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdSet([]string{
		"theme", "ocean",
		"solo.keyboardVolume", "0.35",
		"solo.mouseVolume", "0.45",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig()
	if cfg.Theme != "ocean" || cfg.Solo.KeyboardVolume != 0.35 || cfg.Solo.MouseVolume != 0.45 {
		t.Fatalf("batch settings were not saved: %+v", cfg)
	}
}

func TestSetKeepsUnquotedMultiwordNicknameCompatibility(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdSet([]string{"nickname", "Cosmic", "Otter"}); err != nil {
		t.Fatal(err)
	}
	if got := loadConfig().Nickname; got != "Cosmic Ott" {
		t.Fatalf("nickname = %q, want truncated multiword name", got)
	}
}

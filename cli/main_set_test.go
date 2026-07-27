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

func TestSetRequiresQuotedMultiwordValues(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdSet([]string{"nickname", "Cosmic", "Otter"}); err == nil {
		t.Fatal("expected odd argument list to fail")
	}
	if got := loadConfig().Nickname; got != "" {
		t.Fatalf("nickname changed after rejected command: %q", got)
	}
	if err := cmdSet([]string{"nickname", "Cosmic Otter"}); err != nil {
		t.Fatal(err)
	}
	if got := loadConfig().Nickname; got != "Cosmic Ott" {
		t.Fatalf("nickname = %q, want truncated multiword name", got)
	}
}

func TestSetRejectsInvalidBooleanWithoutPartialSave(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdSet([]string{"theme", "ocean", "notifications", "perhaps"}); err == nil {
		t.Fatal("expected invalid boolean to fail")
	}
	cfg := loadConfig()
	if cfg.Theme != "ember" {
		t.Fatalf("partial batch was saved: theme = %q", cfg.Theme)
	}
}

func TestSetPersistsZeroMasterVolume(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdSet([]string{"volume", "0"}); err != nil {
		t.Fatal(err)
	}
	if got := loadConfig().Listening.Volume; got != 0 {
		t.Fatalf("volume = %v, want 0", got)
	}
}

func TestSetEndpointQueuesRunningSessionReconnect(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	if err := cmdSet([]string{"api.url", "https://self-hosted.example"}); err != nil {
		t.Fatal(err)
	}
	var commands []localSessionCommand
	consumeSessionCommands(func(command localSessionCommand) { commands = append(commands, command) })
	if len(commands) != 1 || commands[0].Type != "reload_connection" {
		t.Fatalf("commands = %+v, want one reconnect", commands)
	}
}

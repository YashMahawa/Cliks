package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestUnifiedStatusTextStopped(t *testing.T) {
	cfg := CliksConfig{
		APIURL:          "https://139.59.29.207.sslip.io",
		CurrentTeamCode: "CLIK-LOCAL",
		Teams:           []TeamConfig{{Code: "CLIK-LOCAL", Name: "Local Room"}},
	}
	text := unifiedStatusText(cfg)

	expectedStrings := []string{
		"Cliks Status Summary",
		"Daemon:     stopped",
		"Team:       Local Room (CLIK-LOCAL)",
		"Connection: stopped (https://139.59.29.207.sslip.io)",
		"Autostart:",
		"Log:",
	}

	for _, want := range expectedStrings {
		if !strings.Contains(text, want) {
			t.Fatalf("unifiedStatusText missing %q:\n%s", want, text)
		}
	}
}

func TestUnifiedStatusTextNotJoined(t *testing.T) {
	cfg := CliksConfig{
		APIURL: "https://139.59.29.207.sslip.io",
	}
	text := unifiedStatusText(cfg)

	if !strings.Contains(text, "Team:       not joined") {
		t.Fatalf("expected 'Team:       not joined' in output:\n%s", text)
	}
}

func TestUnifiedStatusTextRunning(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	instance, err := acquireSessionInstance("CLIK-TEST1", runModeBackground)
	if err != nil {
		t.Fatalf("failed to acquire session instance: %v", err)
	}
	defer instance.release()

	now := time.Now().UTC().Format(time.RFC3339)
	sessionState := ActiveSessionState{
		PID:                 os.Getpid(),
		Version:             version,
		TeamCode:            "CLIK-TEST1",
		TeamName:            "Test Team",
		Mode:                "background",
		ConnectionStatus:    "connected",
		ActiveCount:         3,
		LocalCapturedEvents: 42,
		LocalSentEvents:     40,
		StartedAt:           now,
		UpdatedAt:           now,
	}

	if err := writeActiveSessionState(sessionState); err != nil {
		t.Fatalf("failed to write active session state: %v", err)
	}

	cfg := CliksConfig{
		APIURL:          "https://139.59.29.207.sslip.io",
		CurrentTeamCode: "CLIK-TEST1",
		Teams:           []TeamConfig{{Code: "CLIK-TEST1", Name: "Test Team"}},
	}

	text := unifiedStatusText(cfg)

	expectedStrings := []string{
		"Cliks Status Summary",
		"Daemon:     running",
		"Team:       Test Team (CLIK-TEST1)",
		"Connection: connected (https://139.59.29.207.sslip.io)",
		"Active:     3 users",
		"Activity:   42 captured, 40 sent",
	}

	for _, want := range expectedStrings {
		if !strings.Contains(text, want) {
			t.Fatalf("unifiedStatusText missing %q:\n%s", want, text)
		}
	}
}

func TestCmdStatusExecution(t *testing.T) {
	err := cmdStatus(nil)
	if err != nil {
		t.Fatalf("cmdStatus returned error: %v", err)
	}
}

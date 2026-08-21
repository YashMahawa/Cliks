package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDecoupledLockReadinessSucceedsImmediately(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	state := ActiveSessionState{
		PID:              os.Getpid(),
		TeamCode:         "CLIK-DECOUPLED",
		Mode:             runModeBackground,
		ConnectionStatus: "starting",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := waitForBackgroundReady(os.Getpid(), "CLIK-DECOUPLED", 2*time.Second); err != nil {
		t.Fatalf("expected lock readiness success, got: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("waitForBackgroundReady took %v, expected < 100ms", elapsed)
	}
}

func TestStatusQueryCleansUpDeadStartingStateAndReportsOffline(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	deadPID := 2147483000
	if processLooksAlive(deadPID) {
		t.Skip("unexpectedly live high pid")
	}

	state := ActiveSessionState{
		PID:              deadPID,
		TeamCode:         "CLIK-DEADSTART",
		Mode:             runModeBackground,
		ConnectionStatus: "starting",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644)
	_ = os.WriteFile(sessionStatePath(), append(data, '\n'), 0o644)
	_ = os.WriteFile(backgroundPIDPath(), []byte("2147483000\n"), 0o644)

	// Verify status command reports stopped / offline and cleans up state files
	statusMsg := backgroundStatusText()
	if !strings.Contains(statusMsg, "stopped") {
		t.Fatalf("backgroundStatusText = %q, want 'Cliks: stopped'", statusMsg)
	}

	if _, err := os.Stat(sessionLockPath()); !os.IsNotExist(err) {
		t.Fatalf("session.lock still exists after status query on dead PID")
	}
	if _, err := os.Stat(sessionStatePath()); !os.IsNotExist(err) {
		t.Fatalf("session.json still exists after status query on dead PID")
	}
	if _, err := os.Stat(backgroundPIDPath()); !os.IsNotExist(err) {
		t.Fatalf("background.pid still exists after status query on dead PID")
	}

	// Subsequent status query also reports accurate offline status
	statusMsg2 := backgroundStatusText()
	if !strings.Contains(statusMsg2, "stopped") {
		t.Fatalf("subsequent backgroundStatusText = %q, want 'Cliks: stopped'", statusMsg2)
	}
}

func TestStatusQueryCleansUpDeadStateWithoutLockFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	deadPID := 2147483001
	if processLooksAlive(deadPID) {
		t.Skip("unexpectedly live high pid")
	}

	state := ActiveSessionState{
		PID:              deadPID,
		TeamCode:         "CLIK-NOLOCK",
		Mode:             runModeBackground,
		ConnectionStatus: "starting",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(sessionStatePath(), append(data, '\n'), 0o644)

	statusMsg := backgroundStatusText()
	if !strings.Contains(statusMsg, "stopped") {
		t.Fatalf("backgroundStatusText = %q, want 'Cliks: stopped'", statusMsg)
	}

	if _, err := os.Stat(sessionStatePath()); !os.IsNotExist(err) {
		t.Fatalf("session.json still exists after status query on dead PID without lock file")
	}
}

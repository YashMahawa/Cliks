package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestFailedBackgroundStartupCleansUpFiles(t *testing.T) {
	// 1. A failed or timed-out background daemon startup results in the complete deletion
	// of the temporary session and process ID files.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Set sessionCleanupAllowed to true for active commands
	sessionCleanupAllowed = true
	defer func() { sessionCleanupAllowed = false }()

	// We simulate a failed startup by invoking startBackgroundForTeam on a team code
	// that will time out because no daemon is actually running or writing the lock.
	// But wait, startBackgroundForTeam executes the current executable with 'start'.
	// To prevent running a real subprocess that might hang or succeed, we can test
	// the deferred cleanup logic by setting up the files ourselves and asserting
	// they are deleted under a simulated failure condition, or we can let the actual
	// function fail and verify.
	// Let's verify that after startBackgroundForTeam fails, all files are gone.
	// To make startBackgroundForTeam run but fail quickly, we can use a small timeout?
	// The timeout in waitForBackgroundReady is hardcoded to 2 seconds, but we can still let it run.
	// Actually, if we just run it, it will execute currentExecutable(), which is `go test` during a test!
	// So `cmd.Start()` might succeed but it won't write a valid session lock for our team code,
	// so it will time out and fail after 2 seconds.
	// Let's verify that the files are indeed cleaned up!
	
	start := time.Now()
	_, err := startBackgroundForTeam("CLIK-FAIL99")
	if err == nil {
		t.Fatal("expected background startup to fail")
	}
	t.Logf("Failed as expected in %v: %v", time.Since(start), err)

	// Verify that state files are deleted
	if _, err := os.Stat(sessionLockPath()); !os.IsNotExist(err) {
		t.Errorf("expected session.lock to be deleted, but it exists")
	}
	if _, err := os.Stat(sessionStatePath()); !os.IsNotExist(err) {
		t.Errorf("expected session.json to be deleted, but it exists")
	}
	if _, err := os.Stat(backgroundPIDPath()); !os.IsNotExist(err) {
		t.Errorf("expected background.pid to be deleted, but it exists")
	}
}

func TestPassiveCommandsDoNotTriggerCleanup(t *testing.T) {
	// 2. Running passive commands such as status, live, or join does not trigger file deletion or write operations.
	// 3. Slow process responses during status checks do not trigger the cleanup of active session locks.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Setup a stale session lock file (PID is dead)
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	
	deadPID := 999999 // Assume this PID is dead/not running
	state := ActiveSessionState{
		PID:              deadPID,
		Version:          version,
		TeamCode:         "CLIK-PASSIVE",
		Mode:             runModeBackground,
		ConnectionStatus: "connected",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644)
	_ = os.WriteFile(sessionStatePath(), append(data, '\n'), 0o644)

	// Set sessionCleanupAllowed to false (default for passive commands)
	sessionCleanupAllowed = false

	// Query status / call activeSession(false)
	active, ok := activeSession(false)
	if ok {
		t.Errorf("expected activeSession to return false for dead PID, but got true")
	}
	_ = active

	// Verify that lock and state files are STILL ON DISK (not deleted by passive command)
	if _, err := os.Stat(sessionLockPath()); os.IsNotExist(err) {
		t.Errorf("expected session.lock to remain on disk during passive status check, but it was deleted")
	}
	if _, err := os.Stat(sessionStatePath()); os.IsNotExist(err) {
		t.Errorf("expected session.json to remain on disk during passive status check, but it was deleted")
	}
}

func TestOnlyExplicitCleanupParametersAllowsDeletion(t *testing.T) {
	// 4. Only commands with explicit cleanup parameters are permitted to delete stale session directories and metadata files.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	deadPID := 999999
	state := ActiveSessionState{
		PID:              deadPID,
		Version:          version,
		TeamCode:         "CLIK-EXPLICIT",
		Mode:             runModeBackground,
		ConnectionStatus: "connected",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644)
	_ = os.WriteFile(sessionStatePath(), append(data, '\n'), 0o644)

	// Calling activeSession(true) but with sessionCleanupAllowed = false
	sessionCleanupAllowed = false
	_, _ = activeSession(true)

	// Files must still exist since sessionCleanupAllowed is false
	if _, err := os.Stat(sessionLockPath()); os.IsNotExist(err) {
		t.Errorf("expected session.lock to remain since sessionCleanupAllowed is false")
	}

	// Now set sessionCleanupAllowed = true and query activeSession(true)
	sessionCleanupAllowed = true
	_, _ = activeSession(true)

	// Files should be deleted now
	if _, err := os.Stat(sessionLockPath()); !os.IsNotExist(err) {
		t.Errorf("expected session.lock to be deleted since sessionCleanupAllowed is true")
	}
}

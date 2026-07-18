package main

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestSessionInstancePreventsDuplicateLocalConnection(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	first, err := acquireSessionInstance("CLIK-LOCAL", runModeForeground)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer first.release()

	_, err = acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	var already alreadyRunningError
	if !errors.As(err, &already) {
		t.Fatalf("second acquire err = %v, want alreadyRunningError", err)
	}
	if already.state.PID == 0 || already.state.TeamCode != "CLIK-LOCAL" {
		t.Fatalf("already running state = %+v", already.state)
	}
}

func TestClassifySessionLockTreatsYoungEmptyLockAsWait(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	path := sessionLockPath()
	// Simulate O_EXCL create before metadata is written.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	action, _ := classifySessionLock(path)
	if action != lockWait {
		t.Fatalf("action = %v, want lockWait for young empty lock", action)
	}
}

func TestClassifySessionLockTreatsDeadPIDAsStale(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// PID 1 is usually init/systemd and looks alive; use an absurd high PID instead.
	deadPID := 2147483000
	if processLooksAlive(deadPID) {
		t.Skip("unexpectedly live high pid")
	}
	state := ActiveSessionState{PID: deadPID, TeamCode: "CLIK-DEAD", Mode: runModeForeground}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	action, _ := classifySessionLock(sessionLockPath())
	if action != lockStale {
		t.Fatalf("action = %v, want lockStale", action)
	}
}

func TestSessionInstanceReleaseAllowsNextConnection(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	first, err := acquireSessionInstance("CLIK-LOCAL", runModeForeground)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	first.release()

	second, err := acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	if err != nil {
		t.Fatalf("second acquire after release failed: %v", err)
	}
	second.release()
}

func TestSessionStateRecordsBinaryVersion(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	instance, err := acquireSessionInstance("CLIK-VERSN1", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()
	active, ok := activeSession()
	if !ok || active.Version != version {
		t.Fatalf("active version = %q, want %q", active.Version, version)
	}
	if sessionNeedsUpgrade(active) {
		t.Fatal("current session was marked stale")
	}
	active.Version = ""
	if !sessionNeedsUpgrade(active) {
		t.Fatal("legacy session without a version was not marked stale")
	}
}

func TestSessionInstanceIgnoresOwnPendingBackgroundPID(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := writeBackgroundPID(processIDForTest()); err != nil {
		t.Fatalf("write background pid: %v", err)
	}
	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	if err != nil {
		t.Fatalf("acquire with own pending pid failed: %v", err)
	}
	instance.release()
}

func TestActiveSessionReportsDuplicateLocalProcess(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	first, err := acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer first.release()

	restore := stubSiblingProcesses([]localStartProcess{{PID: 99999, Command: "cliks start"}})
	defer restore()

	active, ok := activeSession()
	if !ok {
		t.Fatal("activeSession returned false")
	}
	if len(active.DuplicateLocalPIDs) != 1 || active.DuplicateLocalPIDs[0] != 99999 {
		t.Fatalf("duplicates = %+v, want [99999]", active.DuplicateLocalPIDs)
	}
}

func TestActiveSessionFindsLegacyProcessWithoutLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	restore := stubSiblingProcesses([]localStartProcess{{PID: 54321, Command: "cliks start"}})
	defer restore()

	active, ok := activeSession()
	if !ok {
		t.Fatal("activeSession returned false")
	}
	if active.PID != 54321 || active.Mode != runModeExisting {
		t.Fatalf("active = %+v, want legacy pid 54321", active)
	}
}

func processIDForTest() int {
	return os.Getpid()
}

func stubSiblingProcesses(processes []localStartProcess) func() {
	previous := siblingProcessFinder
	siblingProcessFinder = func(excludePIDs ...int) []localStartProcess {
		return processes
	}
	return func() {
		siblingProcessFinder = previous
	}
}

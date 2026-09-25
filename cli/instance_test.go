package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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

func TestStartReportsHowToAttachToMatchingActiveSessionWhenNonInteractive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()
	err = cmdStart(nil)
	if err == nil || !strings.Contains(err.Error(), "cliks live") {
		t.Fatalf("cmdStart error = %v, want attach guidance", err)
	}
}

func TestSessionInstancePreventsConnectionToSecondTeam(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	first, err := acquireSessionInstance("CLIK-FIRST1", runModeForeground)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer first.release()

	_, err = acquireSessionInstance("CLIK-SECOND", runModeBackground)
	var already alreadyRunningError
	if !errors.As(err, &already) {
		t.Fatalf("second-team acquire err = %v, want alreadyRunningError", err)
	}
	if already.state.TeamCode != "CLIK-FIRST1" {
		t.Fatalf("active team = %q, want first team to retain ownership", already.state.TeamCode)
	}
}

func TestModeTransitionsKeepOnlyOneOwner(t *testing.T) {
	if transitionRequiresDisconnect("CLIK-FIRST1", "clik-first1") {
		t.Fatal("same-team background request should attach to the existing owner")
	}
	if !transitionRequiresDisconnect("CLIK-FIRST1", "CLIK-SECOND") {
		t.Fatal("switching teams must disconnect the old owner")
	}
	if !transitionRequiresDisconnect("CLIK-FIRST1", "") {
		t.Fatal("entering offline Solo must disconnect the team owner")
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

func TestBackgroundReadinessRequiresMatchingSessionLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	state := ActiveSessionState{PID: os.Getpid(), TeamCode: "CLIK-READY1", Mode: runModeBackground}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := waitForBackgroundReady(os.Getpid(), "CLIK-READY1", 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := waitForBackgroundReady(os.Getpid(), "CLIK-WRONG1", 100*time.Millisecond); err == nil {
		t.Fatal("expected mismatched team to fail readiness")
	}
}

func TestBackgroundReadinessTimesOutWithoutLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := waitForBackgroundReady(os.Getpid(), "CLIK-READY1", 25*time.Millisecond); err == nil {
		t.Fatal("expected missing lock to time out")
	}
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
	if active.Version != version {
		t.Fatalf("active.Version = %q, want %q", active.Version, version)
	}
	if sessionNeedsUpgrade(active) {
		t.Fatal("recovered legacy session was marked as needing upgrade")
	}
}

func TestActiveSessionPopulatesMissingVersionInMemoryDuringLockRecovery(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", temp)
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	lockState := ActiveSessionState{PID: os.Getpid(), TeamCode: "CLIK-NOVER", Mode: runModeForeground}
	data, _ := json.MarshalIndent(lockState, "", "  ")
	if err := os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	active, ok := activeSession()
	if !ok {
		t.Fatal("expected active session")
	}
	if active.Version != version {
		t.Fatalf("active.Version = %q, want %q", active.Version, version)
	}
	if sessionNeedsUpgrade(active) {
		t.Fatal("recovered session with missing version in lock file was marked as needing upgrade")
	}

	// Verify that lock file on disk was NOT modified (must operate strictly in memory)
	diskState, _ := readSessionFile(sessionLockPath())
	if diskState.Version != "" {
		t.Fatalf("disk lock version = %q, want empty string (must not repair disk file)", diskState.Version)
	}
}

func TestActiveSessionPopulatesMissingVersionInMemoryDuringBackgroundPIDRecovery(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", temp)

	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	if err := writeBackgroundPID(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}

	active, ok := activeSession()
	if !ok {
		t.Fatal("expected active session via background.pid fallback")
	}
	if active.PID != cmd.Process.Pid {
		t.Fatalf("active.PID = %d, want %d", active.PID, cmd.Process.Pid)
	}
	if active.Version != version {
		t.Fatalf("active.Version = %q, want %q", active.Version, version)
	}
	if sessionNeedsUpgrade(active) {
		t.Fatal("recovered background session was marked as needing upgrade")
	}

	// Verify session state file on disk was NOT written/modified
	stale, _ := readSessionFile(sessionStatePath())
	if stale.Version != "" {
		t.Fatalf("disk session state version = %q, want empty string", stale.Version)
	}
}

func TestActiveSessionPreservesMismatchedVersion(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", temp)
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	lockState := ActiveSessionState{PID: os.Getpid(), Version: "0.5.0", TeamCode: "CLIK-OLD", Mode: runModeBackground}
	data, _ := json.MarshalIndent(lockState, "", "  ")
	if err := os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	active, ok := activeSession()
	if !ok {
		t.Fatal("expected active session")
	}
	if active.Version != "0.5.0" {
		t.Fatalf("active.Version = %q, want %q", active.Version, "0.5.0")
	}
	if !sessionNeedsUpgrade(active) {
		t.Fatal("mismatched session version was not marked as needing upgrade")
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

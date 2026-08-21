package main

import (
	"encoding/json"
	"errors"
	"os"
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

func TestHandoffParentPIDExcludesParentFromProcessDiscovery(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("CLIKS_HANDOFF_PARENT_PID", "12345")
	defer setHandoffParentPIDForTest(0)

	pid := getHandoffParentPID()
	if pid != 12345 {
		t.Fatalf("getHandoffParentPID = %d, want 12345", pid)
	}
	if os.Getenv("CLIKS_HANDOFF_PARENT_PID") != "" {
		t.Fatal("CLIKS_HANDOFF_PARENT_PID was not cleared from environment")
	}

	restore := stubSiblingProcesses([]localStartProcess{
		{PID: 12345, Command: "cliks start"},
		{PID: 67890, Command: "cliks start"},
	})
	defer restore()

	siblings := findSiblingStartProcesses()
	if len(siblings) != 1 || siblings[0].PID != 67890 {
		t.Fatalf("findSiblingStartProcesses = %+v, want only PID 67890", siblings)
	}
}

func TestHandoffParentPIDAllowsLockAcquisitionWithExitingParent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	parentPID := 11111
	t.Setenv("CLIKS_HANDOFF_PARENT_PID", "11111")
	defer setHandoffParentPIDForTest(0)

	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	state := ActiveSessionState{PID: parentPID, TeamCode: "CLIK-LOCAL", Mode: runModeForeground}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(sessionLockPath(), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := stubSiblingProcesses([]localStartProcess{
		{PID: parentPID, Command: "cliks start"},
	})
	defer restore()

	if _, active := activeSession(); active {
		t.Fatal("activeSession returned true for exiting parent PID")
	}

	instance, err := acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed during handoff: %v", err)
	}
	defer instance.release()

	if instance.state.PID == parentPID {
		t.Fatalf("acquired instance PID = %d, expected current process PID %d", instance.state.PID, os.Getpid())
	}
}

func TestStandardStartupWithoutHandoffPIDDetectsDuplicate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setHandoffParentPIDForTest(0)

	restore := stubSiblingProcesses([]localStartProcess{
		{PID: 88888, Command: "cliks start"},
	})
	defer restore()

	active, ok := activeSession()
	if !ok {
		t.Fatal("activeSession returned false for duplicate sibling process")
	}
	if active.PID != 88888 {
		t.Fatalf("active PID = %d, want 88888", active.PID)
	}

	_, err := acquireSessionInstance("CLIK-LOCAL", runModeBackground)
	var already alreadyRunningError
	if !errors.As(err, &already) {
		t.Fatalf("acquireSessionInstance err = %v, want alreadyRunningError", err)
	}
}

func TestFilterHandoffEnv(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"CLIKS_HANDOFF_PARENT_PID=123",
		"CLIKS_PARENT_PID=456",
		"CLIKS_HANDOFF_PID=789",
		"CLIKS_RUN_MODE=background",
	}
	filtered := filterHandoffEnv(env)
	for _, item := range filtered {
		if strings.HasPrefix(item, "CLIKS_HANDOFF_PARENT_PID=") ||
			strings.HasPrefix(item, "CLIKS_PARENT_PID=") ||
			strings.HasPrefix(item, "CLIKS_HANDOFF_PID=") {
			t.Fatalf("filtered env contains handoff var: %s", item)
		}
	}
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}
}

func TestHandoffPIDEnvVarsAliases(t *testing.T) {
	t.Setenv("CLIKS_PARENT_PID", "54321")
	setHandoffParentPIDForTest(0)
	pid := getHandoffParentPID()
	if pid != 54321 {
		t.Fatalf("getHandoffParentPID() = %d, want 54321", pid)
	}
	setHandoffParentPIDForTest(0)
}


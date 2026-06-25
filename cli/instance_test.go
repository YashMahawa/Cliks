package main

import (
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

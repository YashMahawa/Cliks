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

func processIDForTest() int {
	return os.Getpid()
}

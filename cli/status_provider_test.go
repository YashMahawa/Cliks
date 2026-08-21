package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPassiveStatusProviderDetectsForegroundSession(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-FG1234"
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	pid := os.Getpid()
	instance, err := acquireSessionInstance("CLIK-FG1234", runModeForeground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	instance.update(SessionViewState{
		TeamCode:            "CLIK-FG1234",
		ConnectionStatus:    "connected",
		ActiveCount:         2,
		LocalCapturedEvents: 15,
		LocalSentEvents:     12,
	})

	status := getPassiveRuntimeStatus()
	if !status.IsRunning {
		t.Fatal("expected status.IsRunning to be true")
	}
	if status.PID != pid {
		t.Fatalf("status.PID = %d, want %d", status.PID, pid)
	}
	if status.ExecutionMode != "interactive foreground" {
		t.Fatalf("status.ExecutionMode = %q, want 'interactive foreground'", status.ExecutionMode)
	}
	if status.ConnectionStatus != "connected" {
		t.Fatalf("status.ConnectionStatus = %q, want 'connected'", status.ConnectionStatus)
	}
	if status.ActiveCount != 2 {
		t.Fatalf("status.ActiveCount = %d, want 2", status.ActiveCount)
	}
}

func TestPassiveStatusProviderDetectsBackgroundSession(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-BG1234"
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	pid := os.Getpid()
	instance, err := acquireSessionInstance("CLIK-BG1234", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	instance.update(SessionViewState{
		TeamCode:            "CLIK-BG1234",
		ConnectionStatus:    "connected",
		ActiveCount:         1,
		LocalCapturedEvents: 5,
		LocalSentEvents:     5,
	})

	status := getPassiveRuntimeStatus()
	if !status.IsRunning {
		t.Fatal("expected status.IsRunning to be true")
	}
	if status.PID != pid {
		t.Fatalf("status.PID = %d, want %d", status.PID, pid)
	}
	if status.ExecutionMode != "background" {
		t.Fatalf("status.ExecutionMode = %q, want 'background'", status.ExecutionMode)
	}
}

func TestPassiveStatusProviderDoesNotCleanupStaleLock(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	cliksDir := filepath.Join(stateDir, "cliks")
	if err := os.MkdirAll(cliksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	deadPID := 2147483000
	state := ActiveSessionState{PID: deadPID, TeamCode: "CLIK-DEAD", Mode: runModeForeground}
	data, _ := json.MarshalIndent(state, "", "  ")
	lockPath := sessionLockPath()
	if err := os.WriteFile(lockPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	status := getPassiveRuntimeStatus()
	if status.IsRunning {
		t.Fatal("expected status.IsRunning to be false for dead PID")
	}

	// Verify the lock file was NOT deleted by passive check.
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("passive status check deleted stale lock file")
	}
}

func TestServiceStatusDisplaysAutostartAndMode(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	autoPath := autostartPath()
	if autoPath != "" {
		_ = os.MkdirAll(filepath.Dir(autoPath), 0o755)
		_ = os.WriteFile(autoPath, []byte("fake"), 0o644)
		defer os.Remove(autoPath)
	}

	instance, err := acquireSessionInstance("CLIK-MODE1", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}

	text := backgroundStatusText()
	if !strings.Contains(text, "interactive foreground") {
		t.Fatalf("expected status output to contain 'interactive foreground', got:\n%s", text)
	}
	if !strings.Contains(text, "Autostart: enabled") {
		t.Fatalf("expected status output to contain 'Autostart: enabled', got:\n%s", text)
	}

	instance.release()

	stoppedText := backgroundStatusText()
	if !strings.Contains(stoppedText, "Cliks: stopped") {
		t.Fatalf("expected stopped text, got:\n%s", stoppedText)
	}
	if autoPath != "" && !strings.Contains(stoppedText, "Autostart: enabled") {
		t.Fatalf("expected stopped text to show autostart enabled, got:\n%s", stoppedText)
	}
}

func TestDoctorCommandIncludesProcessLivenessAndConnectionStatus(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-DOCTOR"
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-DOCTOR", runModeBackground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	instance.update(SessionViewState{
		TeamCode:         "CLIK-DOCTOR",
		ConnectionStatus: "connected",
	})

	report := buildDoctorReportOptions(cfg, false)
	foundLiveness := false
	foundConn := false
	for _, check := range report.checks {
		if check.label == "Process liveness" {
			foundLiveness = true
			if !strings.Contains(check.status, "running") || !strings.Contains(check.status, "background") {
				t.Fatalf("unexpected process liveness check: %s", check.status)
			}
		}
		if check.label == "Connection status" {
			foundConn = true
			if check.status != "connected" {
				t.Fatalf("unexpected connection status check: %s", check.status)
			}
		}
	}
	if !foundLiveness {
		t.Fatal("doctor checks missing 'Process liveness'")
	}
	if !foundConn {
		t.Fatal("doctor checks missing 'Connection status'")
	}
}

func TestConfigSummaryIncludesRuntimeDetails(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-CONFIG"
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-CONFIG", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	instance.update(SessionViewState{
		TeamCode:         "CLIK-CONFIG",
		ConnectionStatus: "connected",
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = printConfig()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printConfig failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("printConfig produced invalid JSON: %v\nOutput: %s", err, output)
	}

	if pid, ok := parsed["processId"].(float64); !ok || int(pid) != os.Getpid() {
		t.Fatalf("JSON processId = %v, want %d", parsed["processId"], os.Getpid())
	}
	if mode, ok := parsed["executionMode"].(string); !ok || mode != "interactive foreground" {
		t.Fatalf("JSON executionMode = %v, want 'interactive foreground'", parsed["executionMode"])
	}
	if conn, ok := parsed["connectionStatus"].(string); !ok || conn != "connected" {
		t.Fatalf("JSON connectionStatus = %v, want 'connected'", parsed["connectionStatus"])
	}
}

func TestStatusCommandsAreSequentialConsistent(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-SEQ"
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-SEQ", runModeBackground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	instance.update(SessionViewState{
		TeamCode:         "CLIK-SEQ",
		ConnectionStatus: "connected",
	})

	status := getPassiveRuntimeStatus()
	serviceText := backgroundStatusText()
	doctorReport := buildDoctorReportOptions(cfg, false)

	if !strings.Contains(serviceText, "background") || !strings.Contains(serviceText, "connected") {
		t.Fatalf("serviceText inconsistent: %s", serviceText)
	}

	var doctorLiveness, doctorConn string
	for _, check := range doctorReport.checks {
		if check.label == "Process liveness" {
			doctorLiveness = check.status
		}
		if check.label == "Connection status" {
			doctorConn = check.status
		}
	}

	if !strings.Contains(doctorLiveness, "background") || doctorConn != "connected" {
		t.Fatalf("doctor Report inconsistent: liveness=%q conn=%q", doctorLiveness, doctorConn)
	}

	if status.ExecutionMode != "background" || status.ConnectionStatus != "connected" {
		t.Fatalf("status struct inconsistent: %+v", status)
	}
}

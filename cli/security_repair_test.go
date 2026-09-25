package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSecurityRemediationRepairsWorldReadableFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission testing is for Unix systems")
	}

	tempStateDir := t.TempDir()
	tempConfigDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CONFIG_HOME", tempConfigDir)

	// Create legacy state directory with 0755 permissions
	cliksState := filepath.Join(tempStateDir, "cliks")
	if err := os.MkdirAll(cliksState, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(cliksState, 0o755)

	// Create legacy config directory with 0755 permissions
	cliksConfig := filepath.Join(tempConfigDir, "cliks")
	if err := os.MkdirAll(cliksConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(cliksConfig, 0o755)

	// Create legacy world-readable session state file
	sessionFile := filepath.Join(cliksState, "session.json")
	sessionData := []byte(`{"teamCode": "CLIK-SECRET1"}`)
	if err := os.WriteFile(sessionFile, sessionData, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(sessionFile, 0o644)

	// Create legacy world-readable lock file
	lockFile := filepath.Join(cliksState, "session.lock")
	if err := os.WriteFile(lockFile, sessionData, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(lockFile, 0o644)

	// Create legacy world-readable config file
	configFile := filepath.Join(cliksConfig, "config.json")
	if err := os.WriteFile(configFile, []byte(`{"currentTeamCode": "CLIK-SECRET1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(configFile, 0o644)

	// Verify files are initially world-readable
	for _, path := range []string{sessionFile, lockFile, configFile} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o044 == 0 {
			t.Fatalf("expected test setup file %s to be world-readable before repair, got %04o", path, info.Mode().Perm())
		}
	}

	// Trigger CLI startup / config load
	_ = loadConfig()

	// Verify all directories were repaired to owner-only (0700)
	for _, dirPath := range []string{cliksState, cliksConfig} {
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Fatalf("failed to stat repaired directory %s: %v", dirPath, err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("directory %s permissions = %04o, want 0700 (owner-only)", dirPath, perm)
		}
	}

	// Verify all state and config files were repaired to owner-only (0600)
	for _, filePath := range []string{sessionFile, lockFile, configFile} {
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat repaired file %s: %v", filePath, err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("file %s permissions = %04o, want 0600 (owner-only)", filePath, perm)
		}
	}
}

func TestNewlyCreatedStateAndConfigEnforceOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission testing is for Unix systems")
	}

	tempStateDir := t.TempDir()
	tempConfigDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CONFIG_HOME", tempConfigDir)

	// Save new config
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-NEW123"
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}

	configInfo, err := os.Stat(configPath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := configInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("config file mode = %04o, want owner-only 0600", perm)
	}

	configDirInfo, err := os.Stat(filepath.Dir(configPath()))
	if err != nil {
		t.Fatal(err)
	}
	if perm := configDirInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("config dir mode = %04o, want owner-only 0700", perm)
	}

	// Acquire session lock & state
	instance, err := acquireSessionInstance("CLIK-NEW123", runModeForeground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	lockInfo, err := os.Stat(sessionLockPath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := lockInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("session lock file mode = %04o, want owner-only 0600", perm)
	}

	stateInfo, err := os.Stat(sessionStatePath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := stateInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("session state file mode = %04o, want owner-only 0600", perm)
	}

	stateDirInfo, err := os.Stat(stateDir())
	if err != nil {
		t.Fatal(err)
	}
	if perm := stateDirInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("state dir mode = %04o, want owner-only 0700", perm)
	}
}

func TestAtomicWriteFileReturnsErrorOnPermissionRestrictionFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission validation test is for Unix systems")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.json")

	// Standard atomic write should succeed and enforce permissions
	if err := atomicWriteFile(path, []byte("valid"), 0o600); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("written file mode = %04o, want 0600", perm)
	}

	// Test validatePermissions error condition when permissions cannot be restricted
	fakePath := filepath.Join(dir, "fakefile.json")
	if err := os.WriteFile(fakePath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validatePermissions(fakePath, 0o600); err == nil {
		t.Fatal("expected validatePermissions to fail for 0644 file when target is 0600")
	}
}

func TestNoUnprivilegedRoomCodeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission testing is for Unix systems")
	}

	tempStateDir := t.TempDir()
	tempConfigDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tempStateDir)
	t.Setenv("XDG_CONFIG_HOME", tempConfigDir)

	roomCode := "CLIK-PRIVATE"
	state := ActiveSessionState{
		PID:      os.Getpid(),
		TeamCode: roomCode,
		Mode:     runModeForeground,
	}

	if err := writeActiveSessionState(state); err != nil {
		t.Fatalf("writeActiveSessionState failed: %v", err)
	}

	fileInfo, err := os.Stat(sessionStatePath())
	if err != nil {
		t.Fatal(err)
	}

	// Ensure group and world read bits (0044) are NOT set
	if fileInfo.Mode().Perm()&0o044 != 0 {
		t.Fatalf("active session file permissions %04o allow group/world reading!", fileInfo.Mode().Perm())
	}

	// Read and verify JSON content is intact for the owner
	data, err := os.ReadFile(sessionStatePath())
	if err != nil {
		t.Fatal(err)
	}
	var readState ActiveSessionState
	if err := json.Unmarshal(data, &readState); err != nil {
		t.Fatal(err)
	}
	if readState.TeamCode != roomCode {
		t.Fatalf("teamCode = %q, want %q", readState.TeamCode, roomCode)
	}
}

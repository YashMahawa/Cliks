package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestJoinEnv(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_STATE_HOME", tmpDir)

	origChecker := interactiveTerminalChecker
	origStdin := joinStdin

	cleanup := func() {
		interactiveTerminalChecker = origChecker
		joinStdin = origStdin
	}

	return tmpDir, cleanup
}

func mockAPIServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "CLIK-") || strings.Contains(r.URL.Path, "CLIK-LOCAL") {
			parts := strings.Split(r.URL.Path, "/")
			code := parts[len(parts)-1]
			_, _ = w.Write([]byte(`{"team":{"code":"` + code + `","name":"Test Room"}}`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	return server
}

func TestCmdJoinWhenNoActiveSession(t *testing.T) {
	_, cleanup := setupTestJoinEnv(t)
	defer cleanup()

	server := mockAPIServer(t)
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	_ = saveConfig(cfg)

	// In non-interactive mode without active session, join --no-start should succeed immediately without error or prompt.
	interactiveTerminalChecker = func() bool { return false }
	err := cmdJoin([]string{"--no-start", "CLIK-NEW123"})
	if err != nil {
		t.Fatalf("cmdJoin error = %v, want nil", err)
	}

	saved := loadConfig()
	if saved.CurrentTeamCode != "CLIK-NEW123" {
		t.Fatalf("saved team code = %q, want CLIK-NEW123", saved.CurrentTeamCode)
	}
}

func TestCmdJoinNonInteractiveWithActiveSessionFailsWithoutForce(t *testing.T) {
	_, cleanup := setupTestJoinEnv(t)
	defer cleanup()

	server := mockAPIServer(t)
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	_ = saveConfig(cfg)

	// Start an active session
	instance, err := acquireSessionInstance("CLIK-ACTIVE", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	// Non-interactive shell
	interactiveTerminalChecker = func() bool { return false }

	err = cmdJoin([]string{"CLIK-TARGET"})
	if err == nil {
		t.Fatal("cmdJoin in non-interactive shell with active session and no force flag should fail")
	}
	if !strings.Contains(err.Error(), "active room session") || (!strings.Contains(err.Error(), "--force") && !strings.Contains(err.Error(), "-f")) {
		t.Fatalf("err = %v, want message mentioning active session and force flag", err)
	}

	// Active session should remain untouched
	active, ok := activeSession()
	if !ok || active.TeamCode != "CLIK-ACTIVE" {
		t.Fatalf("active session = %+v, want CLIK-ACTIVE to remain connected", active)
	}
}

func TestCmdJoinNonInteractiveWithActiveSessionSucceedsWithForce(t *testing.T) {
	_, cleanup := setupTestJoinEnv(t)
	defer cleanup()

	server := mockAPIServer(t)
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-ACTIVE", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	interactiveTerminalChecker = func() bool { return false }

	// Using --force should bypass non-interactive fail-fast check
	err = cmdJoin([]string{"--force", "--no-start", "CLIK-TARGET"})
	if err != nil {
		t.Fatalf("cmdJoin with --force failed: %v", err)
	}

	saved := loadConfig()
	if saved.CurrentTeamCode != "CLIK-TARGET" {
		t.Fatalf("saved team = %q, want CLIK-TARGET", saved.CurrentTeamCode)
	}
}

func TestCmdJoinInteractiveDeclined(t *testing.T) {
	_, cleanup := setupTestJoinEnv(t)
	defer cleanup()

	server := mockAPIServer(t)
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-ACTIVE", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	interactiveTerminalChecker = func() bool { return true }
	joinStdin = strings.NewReader("n\n")

	err = cmdJoin([]string{"CLIK-TARGET"})
	if err != nil {
		t.Fatalf("cmdJoin error on decline = %v, want nil", err)
	}

	// Active session and saved team should be maintained
	saved := loadConfig()
	if saved.CurrentTeamCode == "CLIK-TARGET" {
		t.Fatalf("saved team code was changed to CLIK-TARGET despite decline")
	}

	active, ok := activeSession()
	if !ok || active.TeamCode != "CLIK-ACTIVE" {
		t.Fatalf("active session = %+v, want CLIK-ACTIVE maintained", active)
	}
}

func TestCmdJoinInteractiveAccepted(t *testing.T) {
	_, cleanup := setupTestJoinEnv(t)
	defer cleanup()

	server := mockAPIServer(t)
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-ACTIVE", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	interactiveTerminalChecker = func() bool { return true }
	joinStdin = strings.NewReader("y\n")

	err = cmdJoin([]string{"--no-start", "CLIK-TARGET"})
	if err != nil {
		t.Fatalf("cmdJoin error on accept = %v, want nil", err)
	}

	saved := loadConfig()
	if saved.CurrentTeamCode != "CLIK-TARGET" {
		t.Fatalf("saved team = %q, want CLIK-TARGET", saved.CurrentTeamCode)
	}
}

func TestCmdJoinInteractiveWithShortForceFlag(t *testing.T) {
	_, cleanup := setupTestJoinEnv(t)
	defer cleanup()

	server := mockAPIServer(t)
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	_ = saveConfig(cfg)

	instance, err := acquireSessionInstance("CLIK-ACTIVE", runModeBackground)
	if err != nil {
		t.Fatalf("acquireSessionInstance failed: %v", err)
	}
	defer instance.release()

	interactiveTerminalChecker = func() bool { return true }
	// Stdin says "n", but -f flag bypasses confirmation prompt
	joinStdin = strings.NewReader("n\n")

	err = cmdJoin([]string{"-f", "--no-start", "CLIK-TARGET"})
	if err != nil {
		t.Fatalf("cmdJoin error with -f = %v, want nil", err)
	}

	saved := loadConfig()
	if saved.CurrentTeamCode != "CLIK-TARGET" {
		t.Fatalf("saved team = %q, want CLIK-TARGET", saved.CurrentTeamCode)
	}
}

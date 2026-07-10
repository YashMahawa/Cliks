package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRateLimitHelpers(t *testing.T) {
	if !isRateLimitMessage("rate limit exceeded") {
		t.Fatal("expected rate limit message detection")
	}
	if isRateLimitMessage("team not found") {
		t.Fatal("did not expect rate limit for generic error")
	}
	if formatCountdown(65*time.Second) != "1:05" {
		t.Fatalf("formatCountdown = %q", formatCountdown(65*time.Second))
	}
	if got := rateLimitWait(&http.Response{Header: http.Header{"Retry-After": []string{"12"}}}); got != 12*time.Second {
		t.Fatalf("rateLimitWait = %v", got)
	}
	if got := rateLimitWait(nil); got != 5*time.Minute {
		t.Fatalf("default rateLimitWait = %v", got)
	}
}

func TestWriteSessionStateForcesLifecycleTransitions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	instance, err := acquireSessionInstance("CLIK-TEST1", runModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.release()

	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-TEST1"
	controller := newSessionController(cfg, StartOptions{}, instance)
	// Simulate a rapid starting -> connected transition within the throttle window.
	controller.lastStateWrite = time.Now()
	controller.set(func(state *SessionViewState) {
		state.ConnectionStatus = "connected"
	})
	active, ok := activeSession()
	if !ok {
		t.Fatal("expected active session")
	}
	if active.ConnectionStatus != "connected" {
		t.Fatalf("ConnectionStatus = %q, want connected after forced lifecycle write", active.ConnectionStatus)
	}
}

func TestServiceCommandHelpMentionsAliases(t *testing.T) {
	err := cmdService(nil)
	if err == nil || !strings.Contains(err.Error(), "start|stop|status|enable|disable") {
		t.Fatalf("cmdService help = %v", err)
	}
}

package main

import (
	"net/http"
	"os"
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

func TestAttachedControllerReadsOwnerViewWithoutNewSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	active := ActiveSessionState{
		PID:      os.Getpid(),
		TeamCode: "CLIK-ATTACH",
		Mode:     runModeBackground,
		View: SessionViewState{
			TeamCode:         "CLIK-ATTACH",
			TeamName:         "Night Shift",
			ConnectionStatus: "connected",
			ActiveCount:      4,
		},
	}
	if err := writeActiveSessionState(active); err != nil {
		t.Fatal(err)
	}
	controller := newAttachedSessionController(active)
	defer controller.stop()
	state := controller.viewState()
	if !controller.attached || state.TeamName != "Night Shift" || state.ActiveCount != 4 {
		t.Fatalf("attached state = %+v, attached=%v", state, controller.attached)
	}
}

func TestLocalSessionCommandBridgeQueuesRoomReaction(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := enqueueSessionCommand(localSessionCommand{Type: "reaction", Reaction: "wave"}); err != nil {
		t.Fatal(err)
	}
	var got []localSessionCommand
	consumeSessionCommands(func(command localSessionCommand) { got = append(got, command) })
	if len(got) != 1 || got[0].Type != "reaction" || got[0].Reaction != "wave" {
		t.Fatalf("commands = %#v", got)
	}
	consumeSessionCommands(func(command localSessionCommand) { got = append(got, command) })
	if len(got) != 1 {
		t.Fatalf("command was consumed more than once: %#v", got)
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

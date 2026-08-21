package main

import (
	"context"
	"testing"
	"time"
)

func TestPasscodeVerificationAndKick(t *testing.T) {
	store := NewMemoryTeamStore()
	hub := NewRoomHub(store)

	ctx := context.Background()

	// 1. Create a room with passcode
	teamProtected, err := store.CreateTeam(ctx, CreateTeamInput{
		Name:           "Secret Room",
		DeletePassword: "hostpassword123",
		Passcode:       "supersecret",
	})
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}
	if !teamProtected.HasPasscode {
		t.Fatalf("expected HasPasscode to be true for room created with passcode")
	}

	// Verify store GetTeamByCode
	loadedTeam, err := store.GetTeamByCode(ctx, teamProtected.Code)
	if err != nil || loadedTeam == nil {
		t.Fatalf("GetTeamByCode failed: %v", err)
	}
	if !loadedTeam.HasPasscode {
		t.Fatalf("expected loaded team HasPasscode to be true")
	}

	// Verify passcode directly
	valid, err := store.VerifyPasscode(ctx, teamProtected.Code, "supersecret")
	if err != nil || !valid {
		t.Fatalf("VerifyPasscode expected true for 'supersecret', got %v, %v", valid, err)
	}
	validWrong, err := store.VerifyPasscode(ctx, teamProtected.Code, "wrongpasscode")
	if err != nil || validWrong {
		t.Fatalf("VerifyPasscode expected false for 'wrongpasscode', got %v, %v", validWrong, err)
	}
	validEmpty, err := store.VerifyPasscode(ctx, teamProtected.Code, "")
	if err != nil || validEmpty {
		t.Fatalf("VerifyPasscode expected false for empty passcode, got %v, %v", validEmpty, err)
	}

	// 2. Create an unprotected room
	teamPublic, err := store.CreateTeam(ctx, CreateTeamInput{
		Name:           "Public Room",
		DeletePassword: "hostpassword123",
		Passcode:       "",
	})
	if err != nil {
		t.Fatalf("CreateTeam public failed: %v", err)
	}
	if teamPublic.HasPasscode {
		t.Fatalf("expected HasPasscode to be false for public room")
	}
	validPublic, err := store.VerifyPasscode(ctx, teamPublic.Code, "")
	if err != nil || !validPublic {
		t.Fatalf("VerifyPasscode for public room expected true for empty passcode, got %v, %v", validPublic, err)
	}

	// 3. Test Host Kick via Hub
	connHost := newClientConn("peer-host", nil, "test")
	connTarget := newClientConn("peer-target", nil, "test")
	hub.conns[connHost.id] = connHost
	hub.conns[connTarget.id] = connTarget

	hub.join(ctx, connHost, teamProtected.Code, "supersecret", "Host", "available", false)
	hub.join(ctx, connTarget, teamProtected.Code, "supersecret", "Target", "available", false)

	if got := hub.TotalPeers(); got != 2 {
		t.Fatalf("TotalPeers = %d, want 2", got)
	}

	// Kick with wrong delete password -> should fail
	kickedWrong, err := hub.KickPeer(ctx, teamProtected.Code, connTarget.id, "wrongpassword")
	if kickedWrong {
		t.Fatalf("KickPeer with wrong delete password should return false")
	}
	if got := hub.TotalPeers(); got != 2 {
		t.Fatalf("TotalPeers after failed kick = %d, want 2", got)
	}

	// Kick with correct delete password -> should succeed
	kickedOK, err := hub.KickPeer(ctx, teamProtected.Code, connTarget.id, "hostpassword123")
	if err != nil || !kickedOK {
		t.Fatalf("KickPeer with correct delete password failed: %v, %v", kickedOK, err)
	}
	if got := hub.TotalPeers(); got != 1 {
		t.Fatalf("TotalPeers after kick = %d, want 1", got)
	}

	// Verify target connection room code was cleared
	if connTarget.roomCode != "" {
		t.Fatalf("Target conn roomCode after kick = %q, want empty", connTarget.roomCode)
	}
}

func TestDummyPasscodeComparisonForMissingTeam(t *testing.T) {
	store := NewMemoryTeamStore()
	ctx := context.Background()

	start := time.Now()
	valid, err := store.VerifyPasscode(ctx, "CLIK-NONEXISTENT", "somepasscode")
	duration := time.Since(start)

	if err != nil || valid {
		t.Fatalf("VerifyPasscode for nonexistent team expected false, nil; got %v, %v", valid, err)
	}
	// Bcrypt comparison takes non-trivial time even for dummy comparison
	if duration < 1*time.Millisecond {
		t.Logf("Warning: dummy comparison was very fast: %v", duration)
	}
}

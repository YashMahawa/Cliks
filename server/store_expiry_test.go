package main

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreExpiresTeamsAfterFortyEightHoursWithoutConnection(t *testing.T) {
	store := NewMemoryTeamStore()
	team, err := store.CreateTeam(context.Background(), CreateTeamInput{Name: "Quiet room", DeletePassword: "secret1"})
	if err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	stored := store.teams[team.Code]
	stored.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	store.teams[team.Code] = stored
	store.mu.Unlock()

	expired, err := store.ExpireInactiveTeams(context.Background(), time.Now().Add(-teamIdleTTL))
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0] != team.Code {
		t.Fatalf("expired = %v, want %s", expired, team.Code)
	}
	got, err := store.GetTeamByCode(context.Background(), team.Code)
	if err != nil || got != nil {
		t.Fatalf("GetTeamByCode() = %#v, %v; want unavailable", got, err)
	}
}

func TestMemoryStoreConnectionRefreshesExpiry(t *testing.T) {
	store := NewMemoryTeamStore()
	team, err := store.CreateTeam(context.Background(), CreateTeamInput{Name: "Active room", DeletePassword: "secret1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.TouchTeam(context.Background(), team.Code); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetTeamByCode(context.Background(), team.Code)
	if err != nil || got == nil {
		t.Fatalf("GetTeamByCode() = %#v, %v", got, err)
	}
	expires, err := time.Parse(time.RFC3339Nano, got.ExpiresAt)
	if err != nil || time.Until(expires) < 47*time.Hour {
		t.Fatalf("expiry was not refreshed: %q (%v)", got.ExpiresAt, err)
	}
}

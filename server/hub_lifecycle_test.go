package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

type blockingTeamStore struct {
	team          Team
	getStarted    chan struct{}
	releaseGet    chan struct{}
	deleteStarted chan struct{}
	getOnce       sync.Once
	deleteOnce    sync.Once
}

func (s *blockingTeamStore) CreateTeam(context.Context, CreateTeamInput) (Team, error) {
	return Team{}, nil
}

func (s *blockingTeamStore) GetTeamByCode(context.Context, string) (*Team, error) {
	team := s.team
	s.getOnce.Do(func() { close(s.getStarted) })
	<-s.releaseGet
	return &team, nil
}

func (s *blockingTeamStore) DeleteTeam(context.Context, DeleteTeamInput) (bool, error) {
	s.deleteOnce.Do(func() { close(s.deleteStarted) })
	return true, nil
}

func TestDeleteWaitsForConcurrentJoinAndLeavesNoRoom(t *testing.T) {
	store := &blockingTeamStore{
		team:          Team{ID: "team-1", Code: "CLIK-RACE01", Name: "Race Room"},
		getStarted:    make(chan struct{}),
		releaseGet:    make(chan struct{}),
		deleteStarted: make(chan struct{}),
	}
	hub := NewRoomHub(store)
	conn := newClientConn("race-peer", nil, "test")
	hub.conns[conn.id] = conn

	joined := make(chan struct{})
	go func() {
		hub.join(context.Background(), conn, store.team.Code, "", "available", false)
		close(joined)
	}()
	<-store.getStarted

	deleted := make(chan struct{})
	deleteAttempted := make(chan struct{})
	go func() {
		close(deleteAttempted)
		ok, err := hub.DeleteTeam(context.Background(), DeleteTeamInput{Code: store.team.Code, DeletePassword: "secret"})
		if err != nil || !ok {
			t.Errorf("DeleteTeam() = %v, %v; want true, nil", ok, err)
		}
		close(deleted)
	}()
	<-deleteAttempted

	select {
	case <-store.deleteStarted:
		t.Fatal("delete reached the store before the concurrent join completed")
	case <-time.After(100 * time.Millisecond):
	}
	close(store.releaseGet)
	<-joined
	<-deleted

	if got := hub.TotalRooms(); got != 0 {
		t.Fatalf("rooms after concurrent join/delete = %d, want 0", got)
	}
	if got := hub.TotalPeers(); got != 0 {
		t.Fatalf("peers after concurrent join/delete = %d, want 0", got)
	}
	if conn.roomCode != "" {
		t.Fatalf("connection room after delete = %q, want empty", conn.roomCode)
	}
}

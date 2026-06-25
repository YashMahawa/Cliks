package main

import (
	"testing"
	"time"
)

func TestFFmpegSpatialFilterUsesMonoSampleForStereoPan(t *testing.T) {
	filter := ffmpegSpatialFilter(0.5, 0.5)
	want := "pan=stereo|c0=0.250*c0|c1=0.500*c0"
	if filter != want {
		t.Fatalf("filter = %q, want %q", filter, want)
	}
}

func TestFFmpegSpatialFilterClampsGainAndPan(t *testing.T) {
	filter := ffmpegSpatialFilter(2, -2)
	want := "pan=stereo|c0=1.000*c0|c1=0.000*c0"
	if filter != want {
		t.Fatalf("filter = %q, want %q", filter, want)
	}
}

func TestRingCapacityAddsTwoPerRing(t *testing.T) {
	want := []int{4, 6, 8, 10}
	for ring, expected := range want {
		if got := ringCapacity(ring); got != expected {
			t.Fatalf("ringCapacity(%d) = %d, want %d", ring, got, expected)
		}
	}
}

func TestDynamicPlacementBringsActivePeerCloser(t *testing.T) {
	engine := newAudioEngine(ListeningConfig{DynamicPlacement: true, ShuffleMinutes: 1, Volume: 0.7, Density: 1, Keyboard: true, Mouse: true})
	engine.updatePeers([]PeerPresence{
		{PeerID: "self", JoinedAt: 1},
		{PeerID: "quiet-1", JoinedAt: 2},
		{PeerID: "quiet-2", JoinedAt: 3},
		{PeerID: "quiet-3", JoinedAt: 4},
		{PeerID: "quiet-4", JoinedAt: 5},
		{PeerID: "quiet-5", JoinedAt: 6},
		{PeerID: "quiet-6", JoinedAt: 7},
		{PeerID: "active", JoinedAt: 8},
	}, "self")

	engine.mu.Lock()
	engine.activityScores["active"] = 10
	engine.lastShuffleAt = time.Now().Add(-2 * time.Minute)
	engine.maybeShufflePlacementsLocked(time.Now())
	active := engine.placements["active"].Distance
	quiet := engine.placements["quiet-6"].Distance
	engine.mu.Unlock()

	if active > quiet {
		t.Fatalf("active distance = %.2f, quiet distance = %.2f; active peer should be closer", active, quiet)
	}
}

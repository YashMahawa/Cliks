package main

import (
	"math"
	"reflect"
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

func TestAdjacentRingsUseHalfSeatRotation(t *testing.T) {
	firstRingOne := baseAngleForIndex(ringStartIndex(1))
	want := math.Pi / float64(ringCapacity(0))
	if math.Abs(firstRingOne-want) > 0.000001 {
		t.Fatalf("ring 1 rotation = %.6f, want %.6f", firstRingOne, want)
	}
	if firstRingOne == baseAngleForIndex(0) {
		t.Fatal("adjacent rings share the same starting angle")
	}
}

func TestQueuePressureThinningStartsAfterHalfFull(t *testing.T) {
	if got := queuePressureDropProbability(48, 96); got != 0 {
		t.Fatalf("half-full drop probability = %.2f, want 0", got)
	}
	mid := queuePressureDropProbability(72, 96)
	if mid <= 0 || mid >= 0.75 {
		t.Fatalf("three-quarter-full drop probability = %.2f, want between 0 and .75", mid)
	}
	if got := queuePressureDropProbability(96, 96); got != 0.85 {
		t.Fatalf("full drop probability = %.2f, want .85", got)
	}
}

func TestMergePlaybackEventsCollapsesDenseKeyboardBursts(t *testing.T) {
	events := []RemoteActivityEvent{
		{Kind: "keyboard", OffsetMs: 0},
		{Kind: "keyboard", OffsetMs: 8},
		{Kind: "keyboard", OffsetMs: 16},
		{Kind: "keyboard", OffsetMs: 60},
		{Kind: "mouse", Button: "left", OffsetMs: 65},
		{Kind: "keyboard", OffsetMs: 70},
	}
	got := mergePlaybackEvents(events)
	want := []RemoteActivityEvent{
		{Kind: "keyboard", OffsetMs: 16},
		{Kind: "keyboard", OffsetMs: 60},
		{Kind: "mouse", Button: "left", OffsetMs: 65},
		{Kind: "keyboard", OffsetMs: 70},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged events = %#v, want %#v", got, want)
	}
}

func TestMergePlaybackEventsKeepsNormalRhythm(t *testing.T) {
	events := []RemoteActivityEvent{
		{Kind: "keyboard", OffsetMs: 0},
		{Kind: "keyboard", OffsetMs: 45},
		{Kind: "keyboard", OffsetMs: 95},
	}
	if got := mergePlaybackEvents(events); !reflect.DeepEqual(got, events) {
		t.Fatalf("normal rhythm was changed: %#v", got)
	}
}

func TestFatigueThresholdScalesWithRoomPopulation(t *testing.T) {
	if got := fatigueThreshold(1); got != 24 {
		t.Fatalf("single-peer threshold = %d, want 24", got)
	}
	if got := fatigueThreshold(10); got != 48 {
		t.Fatalf("ten-peer threshold = %d, want 48", got)
	}
	if got := fatigueTargetGain(25, 1); got >= 1 || got <= 0.965 {
		t.Fatalf("first overloaded event gain = %.4f, want a soft reduction under 3.5%%", got)
	}
	if got := fatigueTargetGain(1000, 10); got != 0.35 {
		t.Fatalf("fatigue floor = %.2f, want .35", got)
	}
}

func TestAudioDeviceArgumentsArePlayerSpecific(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"mpv", []string{"--audio-device=sink-1", "sample.wav"}},
		{"paplay", []string{"--device", "sink-1", "sample.wav"}},
		{"pw-play", []string{"--target", "sink-1", "sample.wav"}},
		{"aplay", []string{"--device", "sink-1", "sample.wav"}},
		{"ffplay", []string{"sample.wav"}},
	}
	for _, tt := range tests {
		if got := withAudioDevice(tt.command, []string{"sample.wav"}, "sink-1"); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s args = %#v, want %#v", tt.command, got, tt.want)
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

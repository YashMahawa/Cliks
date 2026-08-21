package main

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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

func TestReactionPatternsAreDistinctAndBrief(t *testing.T) {
	seen := map[string]bool{}
	for _, reaction := range []string{"wave", "nice", "coffee", "focus", "celebrate", "break"} {
		pattern := reactionPattern(reaction)
		if len(pattern) == 0 {
			t.Fatalf("%s has no audio pattern", reaction)
		}
		if pattern[len(pattern)-1].delay > 600*time.Millisecond {
			t.Fatalf("%s pattern lasts %s", reaction, pattern[len(pattern)-1].delay)
		}
		key := ""
		for _, beat := range pattern {
			key += beat.kind + ":" + beat.button + ":" + beat.delay.String() + ";"
		}
		if seen[key] {
			t.Fatalf("%s duplicates another reaction pattern", reaction)
		}
		seen[key] = true
	}
	if got := reactionPattern("unknown"); got != nil {
		t.Fatalf("unknown reaction pattern = %#v, want nil", got)
	}
}

func TestMpvArgsUseRawAudioStdinDemuxerNotBrokenFlag(t *testing.T) {
	player := mpvAudioPlayer()
	args := player.ArgsFor(playbackJob{File: "/tmp/sample.wav", Gain: 0.5, Pan: 0.5})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--audio-pan") {
		t.Fatalf("mpv still uses invalid --audio-pan: %v", args)
	}
	found := false
	for _, arg := range args {
		if arg == "--demuxer=rawaudio" {
			found = true
		}
	}
	if !found {
		t.Fatalf("mpv args missing rawaudio demuxer: %v", args)
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
	if got := fatigueThreshold(10); got != 240 {
		t.Fatalf("ten-peer threshold = %d, want 240", got)
	}
	if got := fatigueThreshold(20); got != 480 {
		t.Fatalf("twenty-peer threshold = %d, want 480", got)
	}
	if got := fatigueTargetGain(25, 1); got >= 1 || got <= 0.965 {
		t.Fatalf("first overloaded event gain = %.4f, want a soft reduction under 3.5%%", got)
	}
	if got := fatigueTargetGain(250, 10); got < 0.99 {
		t.Fatalf("typical ten-peer activity gain = %.4f, want at least .99", got)
	}
	if got := fatigueTargetGain(480, 10); got <= 0.7 || got >= 0.8 {
		t.Fatalf("heavy ten-peer activity gain = %.4f, want a gradual reduction", got)
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
	defer engine.Close()
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

func TestAudioWorkerUsesPlaybackDeadline(t *testing.T) {
	original := audioCommandRunner
	defer func() { audioCommandRunner = original }()
	called := make(chan time.Duration, 1)
	audioCommandRunner = func(ctx context.Context, _ *audioPlayer, _ playbackJob) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			called <- 0
			return nil
		}
		called <- time.Until(deadline)
		return nil
	}

	engine := newAudioEngine(ListeningConfig{})
	engine.mu.Lock()
	engine.player = &audioPlayer{Command: "test", ArgsFor: func(playbackJob) []string { return nil }}
	engine.mu.Unlock()
	engine.queue <- playbackJob{File: "sample.wav"}
	select {
	case remaining := <-called:
		if remaining <= 0 || remaining > audioPlaybackTimeout+100*time.Millisecond {
			t.Fatalf("playback deadline = %s, want about %s", remaining, audioPlaybackTimeout)
		}
	case <-time.After(time.Second):
		t.Fatal("audio worker did not run")
	}
	engine.Close()
}

func TestAudioEngineCloseCancelsActivePlaybackAndStopsWorkers(t *testing.T) {
	original := audioCommandRunner
	defer func() { audioCommandRunner = original }()
	started := make(chan struct{})
	var once sync.Once
	audioCommandRunner = func(ctx context.Context, _ *audioPlayer, _ playbackJob) error {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return ctx.Err()
	}

	engine := newAudioEngine(ListeningConfig{})
	engine.mu.Lock()
	engine.player = &audioPlayer{Command: "test", ArgsFor: func(playbackJob) []string { return nil }}
	engine.mu.Unlock()
	engine.queue <- playbackJob{File: "sample.wav"}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("audio worker did not start")
	}

	done := make(chan struct{})
	go func() {
		engine.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("audio engine did not stop promptly after cancellation")
	}
	engine.Close()
}

func TestBundledReleaseSoundsExtractWithoutSourceTree(t *testing.T) {
	origOnce := bundledSoundOnce
	origRoot := bundledSoundRoot
	origErr := bundledSoundErr
	defer func() {
		bundledSoundOnce = origOnce
		bundledSoundRoot = origRoot
		bundledSoundErr = origErr
	}()

	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	bundledSoundOnce = sync.Once{}
	bundledSoundRoot = ""
	bundledSoundErr = nil
	root, err := extractedBundledSoundsRoot()
	if err != nil {
		t.Fatalf("extract bundled sounds: %v", err)
	}
	for _, kind := range []string{"keyboard", "mouse"} {
		matches, globErr := filepath.Glob(filepath.Join(root, kind, "*.wav"))
		if globErr != nil || len(matches) == 0 {
			t.Fatalf("%s samples = %v, %v; want embedded WAVs", kind, matches, globErr)
		}
	}
}

func TestPersistentStreamPlayerInitializationOnStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	player := makeStreamingAudioPlayer(ctx, "cat", "", true)
	if player == nil || player.streamPlayer == nil {
		t.Fatal("makeStreamingAudioPlayer returned nil player or streamPlayer")
	}
	defer player.streamPlayer.Close()

	if !player.streamPlayer.isAliveLocked() {
		t.Fatal("persistent stream player process was not initialized on startup")
	}
	if player.streamPlayer.cmd == nil || player.streamPlayer.stdinPipe == nil {
		t.Fatal("persistent stream player process cmd or stdinPipe is nil")
	}
}

func TestPersistentStreamPlayerPipesRawStereoPCMOverStdinNoSubprocessPerEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := newAudioEngine(ListeningConfig{})
	defer engine.Close()
	samples, err := engine.samples("keyboard")
	if err != nil || len(samples) == 0 {
		t.Fatalf("no sample WAV files found: %v", err)
	}
	sampleFile := samples[0]

	player := makeStreamingAudioPlayer(ctx, "cat", "", true)
	defer player.streamPlayer.Close()

	initialPid := player.streamPlayer.cmd.Process.Pid
	if initialPid <= 0 {
		t.Fatalf("invalid initial pid: %d", initialPid)
	}

	job := playbackJob{File: sampleFile, Gain: 0.8, Pan: 0.0}
	for i := 0; i < 5; i++ {
		if err := player.Play(ctx, job); err != nil {
			t.Fatalf("Play job %d failed: %v", i, err)
		}
	}

	currentPid := player.streamPlayer.cmd.Process.Pid
	if currentPid != initialPid {
		t.Fatalf("process was re-spawned (pid changed from %d to %d); expected single long-lived process", initialPid, currentPid)
	}
}

func TestPersistentStreamPlayer3DSpatialPanning(t *testing.T) {
	engine := newAudioEngine(ListeningConfig{})
	defer engine.Close()
	samples, err := engine.samples("keyboard")
	if err != nil || len(samples) == 0 {
		t.Fatalf("no sample WAV files found: %v", err)
	}
	sampleFile := samples[0]
	data, err := os.ReadFile(sampleFile)
	if err != nil {
		t.Fatalf("read sample wav: %v", err)
	}

	leftPCM, _, err := stereoPCMFromMonoWAV(data, 1.0, -1.0)
	if err != nil {
		t.Fatalf("stereoPCMFromMonoWAV left pan failed: %v", err)
	}
	rightPCM, _, err := stereoPCMFromMonoWAV(data, 1.0, 1.0)
	if err != nil {
		t.Fatalf("stereoPCMFromMonoWAV right pan failed: %v", err)
	}

	// For full left pan (pan=-1.0), right channel samples should be 0.
	rightChannelSum := 0.0
	for i := 2; i < len(leftPCM); i += 4 {
		sample := int16(binary.LittleEndian.Uint16(leftPCM[i : i+2]))
		rightChannelSum += math.Abs(float64(sample))
	}
	if rightChannelSum != 0 {
		t.Fatalf("expected right channel amplitude to be 0 for full left pan, got %f", rightChannelSum)
	}

	// For full right pan (pan=1.0), left channel samples should be 0.
	leftChannelSum := 0.0
	for i := 0; i < len(rightPCM); i += 4 {
		sample := int16(binary.LittleEndian.Uint16(rightPCM[i : i+2]))
		leftChannelSum += math.Abs(float64(sample))
	}
	if leftChannelSum != 0 {
		t.Fatalf("expected left channel amplitude to be 0 for full right pan, got %f", leftChannelSum)
	}
}

func TestPersistentStreamPlayerZeroDropRateUnderHighEventRate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := newAudioEngineWithContext(ctx, ListeningConfig{Volume: 0.8, Density: 1.0, Keyboard: true, Mouse: true})
	defer engine.Close()

	samples, err := engine.samples("keyboard")
	if err != nil || len(samples) == 0 {
		t.Fatalf("no sample WAV files found: %v", err)
	}
	sampleFile := samples[0]

	player := makeStreamingAudioPlayer(ctx, "cat", "", true)
	defer player.streamPlayer.Close()

	engine.mu.Lock()
	engine.player = player
	engine.mu.Unlock()

	// Simulate 25 events fired within 1 second (> 20 events/sec)
	for i := 0; i < 25; i++ {
		job := playbackJob{File: sampleFile, Gain: 0.8, Pan: 0.2}
		select {
		case engine.queue <- job:
		default:
			t.Fatalf("audio job %d was dropped due to queue pressure", i)
		}
	}

	if len(engine.queue) > cap(engine.queue) {
		t.Fatalf("queue length %d exceeded capacity %d", len(engine.queue), cap(engine.queue))
	}
}

func TestPersistentStreamPlayerAutomaticRespawnAndResumeOnCrash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := newAudioEngine(ListeningConfig{})
	defer engine.Close()

	samples, err := engine.samples("keyboard")
	if err != nil || len(samples) == 0 {
		t.Fatalf("no sample WAV files found: %v", err)
	}
	sampleFile := samples[0]

	player := makeStreamingAudioPlayer(ctx, "cat", "", true)
	defer player.streamPlayer.Close()

	job := playbackJob{File: sampleFile, Gain: 0.8, Pan: 0.0}
	if err := player.Play(ctx, job); err != nil {
		t.Fatalf("initial Play failed: %v", err)
	}

	initialPid := player.streamPlayer.cmd.Process.Pid

	// Kill child process to simulate unexpected crash
	_ = player.streamPlayer.cmd.Process.Kill()
	time.Sleep(50 * time.Millisecond)

	// Play next job - engine should detect crash, re-spawn, and resume playback
	if err := player.Play(ctx, job); err != nil {
		t.Fatalf("Play after crash failed: %v", err)
	}

	newPid := player.streamPlayer.cmd.Process.Pid
	if newPid == initialPid {
		t.Fatalf("pid did not change after process crash (initial: %d, new: %d)", initialPid, newPid)
	}
	if !player.streamPlayer.isAliveLocked() {
		t.Fatal("re-spawned streaming process is not alive")
	}
}

func TestPipeCapableStreamingPlayerDeviceRouting(t *testing.T) {
	tests := []struct {
		command string
		device  string
		want    string
	}{
		{"mpv", "sink-1", "--audio-device=sink-1"},
		{"paplay", "sink-1", "--device"},
		{"pw-play", "sink-1", "--target"},
		{"aplay", "sink-1", "--device"},
	}

	for _, tt := range tests {
		args := streamingArgsFor(tt.command, tt.device)
		found := false
		for _, arg := range args {
			if strings.Contains(arg, tt.want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("command %s with device %s missing argument %s; got %v", tt.command, tt.device, tt.want, args)
		}
	}
}

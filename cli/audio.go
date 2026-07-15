package main

import (
	"context"
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Release binaries carry the real WAV pack. External audio players require a
// path, so soundsRoot extracts these files to a versioned cache on first use.
//
//go:embed assets/sounds/*/*.wav
var bundledSoundFS embed.FS

var (
	bundledSoundOnce sync.Once
	bundledSoundRoot string
	bundledSoundErr  error
)

type RemoteActivityEvent struct {
	Kind     string `json:"kind"`
	OffsetMs int    `json:"offsetMs"`
	Button   string `json:"button,omitempty"`
}

type PeerPresence struct {
	PeerID   string `json:"peerId"`
	Nickname string `json:"nickname,omitempty"`
	JoinedAt int64  `json:"joinedAt,omitempty"`
	Status   string `json:"status,omitempty"`
}

type peerPlacement struct {
	Pan      float64
	Distance float64
	Warmth   float64
}

type audioPlayer struct {
	Command       string
	Spatial       bool
	DeviceRouting bool
	Play          func(context.Context, playbackJob) error
	// VolumeCapable is true when ArgsFor applies job.Gain natively.
	// When false, playWorker soft-scales the WAV before playback.
	VolumeCapable bool
	ArgsFor       func(playbackJob) []string
}

type playbackJob struct {
	File   string
	Gain   float64
	Pan    float64
	Device string
}

type AudioEngine struct {
	ctx             context.Context
	cancel          context.CancelFunc
	closeOnce       sync.Once
	workers         sync.WaitGroup
	listening       ListeningConfig
	player          *audioPlayer
	placements      map[string]peerPlacement
	peers           []PeerPresence
	ownPeerID       string
	activityScores  map[string]int
	lastShuffleAt   time.Time
	keyboardSamples []string
	mouseSamples    []string
	queue           chan playbackJob
	ambient         *ambientController
	warned          bool
	mu              sync.Mutex
	recent          []time.Time
	fatigueGain     float64
}

const (
	keyboardMergeWindowMs = 20
	audioPlaybackTimeout  = 2 * time.Second
)

func newAudioEngine(listening ListeningConfig) *AudioEngine {
	return newAudioEngineWithContext(context.Background(), listening)
}

func newAudioEngineWithContext(parent context.Context, listening ListeningConfig) *AudioEngine {
	ctx, cancel := context.WithCancel(parent)
	engine := &AudioEngine{
		ctx:            ctx,
		cancel:         cancel,
		listening:      listening,
		player:         detectAudioPlayerForDevice(listening.AudioDevice),
		placements:     map[string]peerPlacement{},
		activityScores: map[string]int{},
		queue:          make(chan playbackJob, 96),
		fatigueGain:    1,
	}
	engine.ambient = newAmbientController(ctx)
	engine.ambient.update(listening)
	for i := 0; i < 4; i++ {
		engine.workers.Add(1)
		go func() {
			defer engine.workers.Done()
			engine.playWorker()
		}()
	}
	return engine
}

func (a *AudioEngine) Close() {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		if a.ambient != nil {
			a.ambient.close()
		}
		a.cancel()
		a.workers.Wait()
	})
}

func (a *AudioEngine) updateListening(listening ListeningConfig) {
	a.mu.Lock()
	if strings.TrimSpace(listening.AudioDevice) != strings.TrimSpace(a.listening.AudioDevice) {
		a.player = detectAudioPlayerForDevice(listening.AudioDevice)
		a.warned = false
	}
	a.listening = listening
	a.recomputePlacementsLocked(false)
	a.mu.Unlock()
	if a.ambient != nil {
		a.ambient.update(listening)
	}
}

func (a *AudioEngine) updatePeers(peers []PeerPresence, ownPeerID string) {
	nextPeers := append([]PeerPresence(nil), peers...)
	a.mu.Lock()
	a.peers = nextPeers
	a.ownPeerID = ownPeerID
	a.recomputePlacementsLocked(false)
	a.mu.Unlock()
}

func (a *AudioEngine) recomputePlacementsLocked(useActivity bool) {
	peers := append([]PeerPresence(nil), a.peers...)
	sort.SliceStable(peers, func(i, j int) bool {
		if peers[i].JoinedAt == peers[j].JoinedAt {
			return peers[i].PeerID < peers[j].PeerID
		}
		return peers[i].JoinedAt < peers[j].JoinedAt
	})
	if useActivity {
		sort.SliceStable(peers, func(i, j int) bool {
			left := a.activityScores[peers[i].PeerID]
			right := a.activityScores[peers[j].PeerID]
			if left == right {
				if peers[i].JoinedAt == peers[j].JoinedAt {
					return peers[i].PeerID < peers[j].PeerID
				}
				return peers[i].JoinedAt < peers[j].JoinedAt
			}
			return left > right
		})
	}
	next := map[string]peerPlacement{}
	index := 0
	for _, peer := range peers {
		if peer.PeerID == a.ownPeerID {
			continue
		}
		next[peer.PeerID] = placementForIndex(index, peer.PeerID)
		index++
	}
	a.placements = next
}

func (a *AudioEngine) scheduleBatch(peerID string, events []RemoteActivityEvent) {
	a.mu.Lock()
	a.activityScores[peerID] += len(events)
	a.maybeShufflePlacementsLocked(time.Now())
	placement, ok := a.placements[peerID]
	if !ok {
		placement = placementForIndex(len(a.placements), peerID)
		a.placements[peerID] = placement
	}
	listening := a.listening
	a.mu.Unlock()

	for _, event := range mergePlaybackEvents(events) {
		event := event
		if listening.Muted {
			continue
		}
		if event.Kind == "keyboard" && !listening.Keyboard {
			continue
		}
		if event.Kind == "mouse" && !listening.Mouse {
			continue
		}
		if event.Kind == "mouse" && !isPlayableMouseButton(event.Button) {
			continue
		}
		if rand.Float64() > listening.Density {
			continue
		}
		delay := time.Duration(maxInt(0, event.OffsetMs)) * time.Millisecond
		time.AfterFunc(delay, func() {
			select {
			case <-a.ctx.Done():
				return
			default:
				a.enqueue(event, placement)
			}
		})
	}
}

func mergePlaybackEvents(events []RemoteActivityEvent) []RemoteActivityEvent {
	if len(events) <= 1 {
		return events
	}
	merged := make([]RemoteActivityEvent, 0, len(events))
	var pending RemoteActivityEvent
	hasPending := false
	flush := func() {
		if hasPending {
			merged = append(merged, pending)
			hasPending = false
		}
	}
	for _, event := range events {
		if event.Kind != "keyboard" {
			flush()
			merged = append(merged, event)
			continue
		}
		if delta := event.OffsetMs - pending.OffsetMs; hasPending && delta >= 0 && delta <= keyboardMergeWindowMs {
			pending = event
			continue
		}
		flush()
		pending = event
		hasPending = true
	}
	flush()
	return merged
}

func (a *AudioEngine) maybeShufflePlacementsLocked(now time.Time) {
	if !a.listening.DynamicPlacement {
		return
	}
	minutes := a.listening.ShuffleMinutes
	if minutes <= 0 {
		minutes = 10
	}
	interval := time.Duration(minutes) * time.Minute
	if a.lastShuffleAt.IsZero() {
		a.lastShuffleAt = now
		return
	}
	if now.Sub(a.lastShuffleAt) < interval {
		return
	}
	a.recomputePlacementsLocked(true)
	a.activityScores = map[string]int{}
	a.lastShuffleAt = now
}

func (a *AudioEngine) preview() error {
	select {
	case <-a.ctx.Done():
		return a.ctx.Err()
	default:
	}
	if a.player == nil {
		return errors.New(audioInstallMessage())
	}
	placement := peerPlacement{Pan: 0, Distance: 1, Warmth: 1}
	events := []RemoteActivityEvent{
		{Kind: "keyboard"},
		{Kind: "keyboard"},
		{Kind: "mouse", Button: "left"},
		{Kind: "mouse", Button: "left"},
	}
	for _, event := range events {
		a.enqueue(event, placement)
		time.Sleep(180 * time.Millisecond)
	}
	time.Sleep(350 * time.Millisecond)
	return nil
}

func (a *AudioEngine) enqueue(event RemoteActivityEvent, placement peerPlacement) {
	select {
	case <-a.ctx.Done():
		return
	default:
	}
	a.mu.Lock()
	playerAvailable := a.player != nil
	a.mu.Unlock()
	if !playerAvailable {
		a.warnUnavailableOnce()
		return
	}
	samples, err := a.samples(event.Kind)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if len(samples) == 0 {
		return
	}
	if event.Kind == "keyboard" && rand.Float64() < queuePressureDropProbability(len(a.queue), cap(a.queue)) {
		return
	}
	a.mu.Lock()
	listening := a.listening
	a.mu.Unlock()
	fatigueGain := 1.0
	if listening.FatigueProtection {
		fatigueGain = a.recordAndGetFatigueGain()
	}
	job := playbackJob{
		File:   samples[rand.Intn(len(samples))],
		Gain:   clamp(listening.Volume*(1/placement.Distance)*fatigueGain, 0, 1),
		Pan:    0,
		Device: listening.AudioDevice,
	}
	if listening.Spatial {
		job.Pan = placement.Pan
	}
	select {
	case <-a.ctx.Done():
		return
	case a.queue <- job:
	default:
		select {
		case <-a.queue:
		default:
		}
		select {
		case <-a.ctx.Done():
			return
		case a.queue <- job:
		default:
		}
	}
}

func queuePressureDropProbability(queueLength int, queueCapacity int) float64 {
	if queueCapacity <= 0 {
		return 0
	}
	fill := float64(queueLength) / float64(queueCapacity)
	if fill <= 0.5 {
		return 0
	}
	return clamp((fill-0.5)/0.4*0.75, 0, 0.85)
}

func (a *AudioEngine) playWorker() {
	for {
		var job playbackJob
		select {
		case <-a.ctx.Done():
			return
		case job = <-a.queue:
		}
		a.mu.Lock()
		player := a.player
		a.mu.Unlock()
		if player == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(a.ctx, audioPlaybackTimeout)
		err := runAudioCommand(ctx, player, job)
		playbackCanceled := ctx.Err() != nil
		cancel()
		if err != nil && !playbackCanceled && a.ctx.Err() == nil {
			a.warnUnavailableOnce()
		}
	}
}

var audioCommandRunner = func(ctx context.Context, player *audioPlayer, job playbackJob) error {
	if player != nil && player.Play != nil {
		return player.Play(ctx, job)
	}
	playJob := job
	cleanup := func() {}
	if player != nil && !player.VolumeCapable && job.Gain < 0.995 {
		if scaled, done, err := scaleWavFileGain(job.File, job.Gain); err == nil {
			playJob.File = scaled
			playJob.Gain = 1
			cleanup = done
		}
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, player.Command, player.ArgsFor(playJob)...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Run()
}

func runAudioCommand(ctx context.Context, player *audioPlayer, job playbackJob) error {
	return audioCommandRunner(ctx, player, job)
}

func (a *AudioEngine) samples(kind string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if kind == "keyboard" && len(a.keyboardSamples) > 0 {
		return a.keyboardSamples, nil
	}
	if kind == "mouse" && len(a.mouseSamples) > 0 {
		return a.mouseSamples, nil
	}
	root, err := soundsRoot()
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(root, kind, "*.wav"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no %s sound samples found in %s", kind, filepath.Join(root, kind))
	}
	if kind == "keyboard" {
		a.keyboardSamples = files
	} else {
		a.mouseSamples = files
	}
	return files, nil
}

func (a *AudioEngine) recordAndGetFatigueGain() float64 {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.recent[:0]
	for _, t := range a.recent {
		if now.Sub(t) <= 5*time.Second {
			next = append(next, t)
		}
	}
	wasQuiet := len(next) == 0
	next = append(next, now)
	a.recent = next
	if wasQuiet {
		a.fatigueGain = 1
	}
	target := fatigueTargetGain(len(a.recent), maxInt(1, len(a.placements)))
	alpha := 0.18
	if target > a.fatigueGain {
		alpha = 0.1
	}
	a.fatigueGain += (target - a.fatigueGain) * alpha
	return clamp(a.fatigueGain, 0.35, 1)
}

func fatigueThreshold(peerCount int) int {
	peerCount = clampInt(peerCount, 1, 20)
	return 24 * peerCount
}

func fatigueTargetGain(eventCount int, peerCount int) float64 {
	threshold := fatigueThreshold(peerCount)
	overload := maxInt(0, eventCount-threshold)
	if overload == 0 {
		return 1
	}
	pressure := float64(overload) / float64(2*threshold)
	return clamp(1-math.Pow(pressure, 1.35)*0.65, 0.35, 1)
}

func (a *AudioEngine) warnUnavailableOnce() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.warned {
		return
	}
	a.warned = true
	fmt.Fprintln(os.Stderr, "\nAudio disabled: "+audioInstallMessage())
}

func detectAudioPlayer() *audioPlayer {
	// macOS and Windows ship a built-in stereo PCM path, so ordinary installs
	// do not need mpv, PowerShell audio, or another player.
	if builtIn := newBuiltInAudioPlayer(); builtIn != nil {
		return builtIn
	}
	// Linux/Termux prefer players that can stereo-pan teammates.
	if _, err := exec.LookPath("mpv"); err == nil {
		return mpvAudioPlayer()
	}
	if _, err := exec.LookPath("ffplay"); err == nil {
		return &audioPlayer{
			Command:       "ffplay",
			Spatial:       true,
			VolumeCapable: true,
			ArgsFor: func(job playbackJob) []string {
				return withAudioDevice("ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet", "-af", ffmpegSpatialFilter(job.Gain, job.Pan), job.File}, job.Device)
			},
		}
	}
	if runtime.GOOS == "darwin" {
		// afplay is always present; distance via volume only (no stereo pan).
		return &audioPlayer{Command: "afplay", Spatial: false, VolumeCapable: true, ArgsFor: func(job playbackJob) []string {
			return withAudioDevice("afplay", []string{"-v", fmt.Sprintf("%.3f", clamp(job.Gain, 0, 1)), job.File}, job.Device)
		}}
	}
	if runtime.GOOS == "windows" {
		// Compatibility fallback for unusual Windows builds where native audio is unavailable.
		return windowsMediaPlayer()
	}
	if isTermuxRuntime() {
		if _, err := exec.LookPath("termux-media-player"); err == nil {
			return termuxAudioPlayer()
		}
	}
	if _, err := exec.LookPath("paplay"); err == nil {
		return paplayAudioPlayer()
	}
	if _, err := exec.LookPath("pw-play"); err == nil {
		return pipewireAudioPlayer()
	}
	if _, err := exec.LookPath("aplay"); err == nil {
		return alsaAudioPlayer()
	}
	return nil
}

func termuxAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "termux-media-player", Spatial: false, VolumeCapable: false, ArgsFor: func(job playbackJob) []string {
		return []string{"play", job.File}
	}}
}

func isTermuxRuntime() bool {
	return os.Getenv("TERMUX_VERSION") != "" || strings.Contains(os.Getenv("PREFIX"), "com.termux")
}

func windowsMediaPlayer() *audioPlayer {
	return &audioPlayer{
		Command:       "powershell.exe",
		Spatial:       false,
		VolumeCapable: true,
		ArgsFor: func(job playbackJob) []string {
			// System.Windows.Media.MediaPlayer respects Volume; SoundPlayer does not.
			path := strings.ReplaceAll(job.File, "'", "''")
			script := fmt.Sprintf(
				`Add-Type -AssemblyName PresentationCore; $p = New-Object System.Windows.Media.MediaPlayer; $p.Open([Uri]::new('%s')); $p.Volume = %0.3f; $p.Play(); while(-not $p.NaturalDuration.HasTimeSpan){ Start-Sleep -Milliseconds 20 }; $end = $p.NaturalDuration.TimeSpan; while($p.Position -lt $end){ Start-Sleep -Milliseconds 30 }; $p.Close()`,
				path, clamp(job.Gain, 0, 1),
			)
			return []string{"-NoProfile", "-STA", "-Command", script}
		},
	}
}

func detectAudioPlayerForDevice(device string) *audioPlayer {
	if strings.TrimSpace(device) != "" {
		if _, err := exec.LookPath("mpv"); err == nil {
			return mpvAudioPlayer()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("paplay"); err == nil {
				return paplayAudioPlayer()
			}
			if _, err := exec.LookPath("pw-play"); err == nil {
				return pipewireAudioPlayer()
			}
			if _, err := exec.LookPath("aplay"); err == nil {
				return alsaAudioPlayer()
			}
		}
	}
	return detectAudioPlayer()
}

func mpvAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "mpv", Spatial: true, DeviceRouting: true, VolumeCapable: true, ArgsFor: func(job playbackJob) []string {
		// mpv has no --audio-pan flag; use lavfi pan (same math as ffplay) for stereo placement.
		args := []string{
			"--no-video",
			"--really-quiet",
			"--no-terminal",
			"--keep-open=no",
			fmt.Sprintf("--volume=%d", int(clamp(job.Gain, 0, 1)*100)),
			"--af=lavfi=[" + ffmpegSpatialFilter(1, job.Pan) + "]",
			job.File,
		}
		return withAudioDevice("mpv", args, job.Device)
	}}
}

func paplayAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "paplay", DeviceRouting: true, VolumeCapable: true, ArgsFor: func(job playbackJob) []string {
		return withAudioDevice("paplay", []string{"--volume", fmt.Sprintf("%d", int(clamp(job.Gain, 0, 1)*65536)), job.File}, job.Device)
	}}
}

func pipewireAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "pw-play", DeviceRouting: true, VolumeCapable: true, ArgsFor: func(job playbackJob) []string {
		return withAudioDevice("pw-play", []string{"--volume", fmt.Sprintf("%.3f", clamp(job.Gain, 0, 1)), job.File}, job.Device)
	}}
}

func alsaAudioPlayer() *audioPlayer {
	// aplay has no volume flag — soft WAV gain is applied in playWorker.
	return &audioPlayer{Command: "aplay", DeviceRouting: true, VolumeCapable: false, ArgsFor: func(job playbackJob) []string {
		return withAudioDevice("aplay", []string{job.File}, job.Device)
	}}
}

func withAudioDevice(command string, args []string, device string) []string {
	device = strings.TrimSpace(device)
	if device == "" || strings.EqualFold(device, "default") {
		return args
	}
	switch command {
	case "mpv":
		return append([]string{"--audio-device=" + device}, args...)
	case "paplay":
		return append([]string{"--device", device}, args...)
	case "pw-play":
		return append([]string{"--target", device}, args...)
	case "aplay":
		return append([]string{"--device", device}, args...)
	default:
		return args
	}
}

func ffmpegSpatialFilter(gain float64, pan float64) string {
	pan = clamp(pan, -1, 1)
	gain = clamp(gain, 0, 1)
	left := gain
	right := gain
	if pan < 0 {
		right *= 1 + pan
	} else if pan > 0 {
		left *= 1 - pan
	}
	return fmt.Sprintf("pan=stereo|c0=%.3f*c0|c1=%.3f*c0", left, right)
}

// scaleWavFileGain writes a temporary 16-bit PCM WAV with sample amplitude * gain.
// Used for players (e.g. aplay) that cannot take a volume flag.
func scaleWavFileGain(path string, gain float64) (string, func(), error) {
	gain = clamp(gain, 0, 1)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return "", nil, fmt.Errorf("not a WAV file")
	}
	// Find "data" chunk.
	offset := 12
	dataStart := -1
	dataSize := 0
	bitsPerSample := 16
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		payload := offset + 8
		if chunkID == "fmt " && payload+16 <= len(data) {
			bitsPerSample = int(binary.LittleEndian.Uint16(data[payload+14 : payload+16]))
		}
		if chunkID == "data" {
			dataStart = payload
			dataSize = chunkSize
			break
		}
		offset = payload + chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	if dataStart < 0 || bitsPerSample != 16 || dataStart+dataSize > len(data) {
		return "", nil, fmt.Errorf("unsupported WAV layout")
	}
	out := append([]byte(nil), data...)
	for i := dataStart; i+1 < dataStart+dataSize; i += 2 {
		sample := int16(binary.LittleEndian.Uint16(out[i : i+2]))
		scaled := int(float64(sample) * gain)
		if scaled > 32767 {
			scaled = 32767
		}
		if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(out[i:i+2], uint16(int16(scaled)))
	}
	tmp, err := os.CreateTemp("", "cliks-vol-*.wav")
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", nil, err
	}
	_ = tmp.Close()
	cleanup := func() { _ = os.Remove(tmpPath) }
	return tmpPath, cleanup, nil
}

func getAudioPlayerStatus(device ...string) (player string, spatial bool, hint string, commands []string) {
	configuredDevice := ""
	if len(device) > 0 {
		configuredDevice = strings.TrimSpace(device[0])
	}
	detected := detectAudioPlayerForDevice(configuredDevice)
	if detected == nil {
		return "", false, audioInstallHint(), audioInstallCommands()
	}
	if configuredDevice != "" && !detected.DeviceRouting {
		return detected.Command, detected.Spatial, fmt.Sprintf("%s cannot select an output device; install mpv or clear audio.device", detected.Command), nil
	}
	return detected.Command, detected.Spatial, "", nil
}

func audioInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "Cliks includes stereo spatial audio on macOS. Install mpv only if you need advanced output-device routing."
	case "windows":
		return "Cliks includes stereo spatial audio on Windows. Install mpv only if you need advanced output-device routing."
	case "linux":
		if isTermuxRuntime() {
			return "install Termux:API for phone audio: pkg install termux-api (or install mpv)."
		}
		return "install mpv for stereo spatial sound, or a basic player (pulseaudio-utils / pipewire-utils)."
	default:
		return "no supported audio player found on this system."
	}
}

func audioInstallCommands() []string {
	switch runtime.GOOS {
	case "darwin":
		return nil
	case "windows":
		return nil
	case "linux":
		if isTermuxRuntime() {
			return []string{"pkg install termux-api", "pkg install mpv"}
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return []string{"sudo pacman -S --needed mpv"}
		}
		if _, err := exec.LookPath("apt"); err == nil || hasCommand("apt-get") {
			return []string{"sudo apt update", "sudo apt install -y mpv"}
		}
		if _, err := exec.LookPath("dnf"); err == nil {
			return []string{"sudo dnf install -y mpv"}
		}
		return []string{"sudo apt install -y mpv  # or: sudo dnf install mpv / sudo pacman -S mpv"}
	default:
		return nil
	}
}

func audioInstallMessage() string {
	commands := audioInstallCommands()
	if len(commands) == 0 {
		return audioInstallHint()
	}
	return audioInstallHint() + "\nRun:\n  " + stringsJoin(commands, "\n  ")
}

func soundsRoot() (string, error) {
	if root, err := extractedBundledSoundsRoot(); err == nil {
		return root, nil
	}
	exe, _ := os.Executable()
	candidates := []string{
		filepath.Join(filepath.Dir(exe), "..", "assets", "sounds"),
		filepath.Join(filepath.Dir(exe), "assets", "sounds"),
		filepath.Join(".", "assets", "sounds"),
		filepath.Join("cli", "assets", "sounds"),
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("could not locate bundled sound assets")
}

func extractedBundledSoundsRoot() (string, error) {
	bundledSoundOnce.Do(func() {
		cacheRoot, err := os.UserCacheDir()
		if err != nil || strings.TrimSpace(cacheRoot) == "" {
			cacheRoot = os.TempDir()
		}
		root := filepath.Join(cacheRoot, "cliks", "sounds", version)
		paths, err := fs.Glob(bundledSoundFS, "assets/sounds/*/*.wav")
		if err != nil || len(paths) == 0 {
			bundledSoundErr = errors.New("bundled sound pack is empty")
			return
		}
		for _, embeddedPath := range paths {
			data, readErr := bundledSoundFS.ReadFile(embeddedPath)
			if readErr != nil {
				bundledSoundErr = readErr
				return
			}
			relative := strings.TrimPrefix(embeddedPath, "assets/sounds/")
			destination := filepath.Join(root, filepath.FromSlash(relative))
			if stat, statErr := os.Stat(destination); statErr == nil && stat.Size() == int64(len(data)) {
				continue
			}
			if mkdirErr := os.MkdirAll(filepath.Dir(destination), 0o755); mkdirErr != nil {
				bundledSoundErr = mkdirErr
				return
			}
			if writeErr := atomicWriteFile(destination, data, 0o644); writeErr != nil {
				bundledSoundErr = writeErr
				return
			}
		}
		bundledSoundRoot = root
	})
	return bundledSoundRoot, bundledSoundErr
}

func placementForIndex(index int, peerID string) peerPlacement {
	seed := rand.New(rand.NewSource(int64(hashString(peerID))))
	ring := ringForIndex(index)
	capacity := ringCapacity(ring)
	baseAngle := baseAngleForIndex(index)
	jitter := (seed.Float64() - 0.5) * (math.Pi * 2 / float64(capacity))
	angle := baseAngle + jitter
	distance := 2 + float64(ring) + (seed.Float64()-0.5)*0.35
	return peerPlacement{
		Pan:      clamp(math.Sin(angle), -0.95, 0.95),
		Distance: distance,
		Warmth:   0.72 + seed.Float64()*0.5,
	}
}

func baseAngleForIndex(index int) float64 {
	ring := ringForIndex(index)
	positionInRing := index - ringStartIndex(ring)
	capacity := ringCapacity(ring)
	rotation := 0.0
	for current := 1; current <= ring; current++ {
		rotation += math.Pi / float64(ringCapacity(current-1))
	}
	return math.Mod(rotation+math.Pi*2*float64(positionInRing)/float64(capacity), math.Pi*2)
}

func ringForIndex(index int) int {
	ring := 0
	remaining := index
	for remaining >= ringCapacity(ring) {
		remaining -= ringCapacity(ring)
		ring++
	}
	return ring
}

func ringStartIndex(ring int) int {
	start := 0
	for current := 0; current < ring; current++ {
		start += ringCapacity(current)
	}
	return start
}

func ringCapacity(ring int) int {
	return 4 + ring*2
}

func hashString(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func isPlayableMouseButton(button string) bool {
	return button == "" || button == "left" || button == "right"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stringsJoin(values []string, separator string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += separator
		}
		out += value
	}
	return out
}

package main

import (
	"errors"
	"fmt"
	"hash/fnv"
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

type RemoteActivityEvent struct {
	Kind     string `json:"kind"`
	OffsetMs int    `json:"offsetMs"`
	Button   string `json:"button,omitempty"`
}

type PeerPresence struct {
	PeerID   string `json:"peerId"`
	Nickname string `json:"nickname,omitempty"`
	JoinedAt int64  `json:"joinedAt,omitempty"`
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
	ArgsFor       func(playbackJob) []string
}

type playbackJob struct {
	File   string
	Gain   float64
	Pan    float64
	Device string
}

type AudioEngine struct {
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
	warned          bool
	mu              sync.Mutex
	recent          []time.Time
	fatigueGain     float64
}

func newAudioEngine(listening ListeningConfig) *AudioEngine {
	engine := &AudioEngine{
		listening:      listening,
		player:         detectAudioPlayerForDevice(listening.AudioDevice),
		placements:     map[string]peerPlacement{},
		activityScores: map[string]int{},
		queue:          make(chan playbackJob, 96),
		fatigueGain:    1,
	}
	for i := 0; i < 4; i++ {
		go engine.playWorker()
	}
	return engine
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

	for _, event := range events {
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
			a.enqueue(event, placement)
		})
	}
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
	case a.queue <- job:
	default:
		select {
		case <-a.queue:
		default:
		}
		select {
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
	for job := range a.queue {
		a.mu.Lock()
		player := a.player
		a.mu.Unlock()
		if player == nil {
			continue
		}
		cmd := exec.Command(player.Command, player.ArgsFor(job)...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Stdin = nil
		if err := cmd.Run(); err != nil {
			a.warnUnavailableOnce()
		}
	}
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
	peerCount = clampInt(peerCount, 1, 10)
	return 24 + int(math.Round(24*float64(peerCount-1)/9))
}

func fatigueTargetGain(eventCount int, peerCount int) float64 {
	threshold := fatigueThreshold(peerCount)
	overload := maxInt(0, eventCount-threshold)
	if overload == 0 {
		return 1
	}
	pressure := float64(overload) / float64(threshold)
	return clamp(1-math.Pow(pressure, 1.35)*0.9, 0.35, 1)
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
	if _, err := exec.LookPath("ffplay"); err == nil {
		return &audioPlayer{
			Command: "ffplay",
			Spatial: true,
			ArgsFor: func(job playbackJob) []string {
				return withAudioDevice("ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet", "-af", ffmpegSpatialFilter(job.Gain, job.Pan), job.File}, job.Device)
			},
		}
	}
	if _, err := exec.LookPath("mpv"); err == nil {
		return mpvAudioPlayer()
	}
	if runtime.GOOS == "darwin" {
		return &audioPlayer{Command: "afplay", Spatial: false, ArgsFor: func(job playbackJob) []string {
			return withAudioDevice("afplay", []string{"-v", fmt.Sprintf("%.3f", clamp(job.Gain, 0, 1)), job.File}, job.Device)
		}}
	}
	if runtime.GOOS == "windows" {
		return &audioPlayer{Command: "powershell.exe", Spatial: false, ArgsFor: func(job playbackJob) []string {
			return withAudioDevice("powershell.exe", []string{"-NoProfile", "-Command", "(New-Object Media.SoundPlayer $args[0]).PlaySync();", job.File}, job.Device)
		}}
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
	return &audioPlayer{Command: "mpv", Spatial: true, DeviceRouting: true, ArgsFor: func(job playbackJob) []string {
		return withAudioDevice("mpv", []string{"--no-video", "--really-quiet", "--no-terminal", fmt.Sprintf("--volume=%d", int(clamp(job.Gain, 0, 1)*100)), fmt.Sprintf("--audio-pan=%.3f", clamp(job.Pan, -1, 1)), job.File}, job.Device)
	}}
}

func paplayAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "paplay", DeviceRouting: true, ArgsFor: func(job playbackJob) []string {
		return withAudioDevice("paplay", []string{"--volume", fmt.Sprintf("%d", int(clamp(job.Gain, 0, 1)*65536)), job.File}, job.Device)
	}}
}

func pipewireAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "pw-play", DeviceRouting: true, ArgsFor: func(job playbackJob) []string {
		return withAudioDevice("pw-play", []string{"--volume", fmt.Sprintf("%.3f", clamp(job.Gain, 0, 1)), job.File}, job.Device)
	}}
}

func alsaAudioPlayer() *audioPlayer {
	return &audioPlayer{Command: "aplay", DeviceRouting: true, ArgsFor: func(job playbackJob) []string {
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
	if runtime.GOOS == "linux" {
		return "no audio player found. Install PulseAudio/PipeWire playback tools such as pulseaudio-utils, pipewire-utils, or alsa-utils."
	}
	return "no supported audio player found on this system."
}

func audioInstallCommands() []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return []string{"sudo pacman -S --needed libpulse"}
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return []string{"sudo apt update", "sudo apt install -y pulseaudio-utils"}
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return []string{"sudo dnf install pulseaudio-utils"}
	}
	return nil
}

func audioInstallMessage() string {
	commands := audioInstallCommands()
	if len(commands) == 0 {
		return audioInstallHint()
	}
	return audioInstallHint() + "\nRun:\n  " + stringsJoin(commands, "\n  ")
}

func soundsRoot() (string, error) {
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

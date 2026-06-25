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
	Command string
	Spatial bool
	ArgsFor func(playbackJob) []string
}

type playbackJob struct {
	File string
	Gain float64
	Pan  float64
}

type AudioEngine struct {
	listening       ListeningConfig
	player          *audioPlayer
	placements      map[string]peerPlacement
	keyboardSamples []string
	mouseSamples    []string
	queue           chan playbackJob
	warned          bool
	mu              sync.Mutex
	recent          []time.Time
}

func newAudioEngine(listening ListeningConfig) *AudioEngine {
	engine := &AudioEngine{
		listening:  listening,
		player:     detectAudioPlayer(),
		placements: map[string]peerPlacement{},
		queue:      make(chan playbackJob, 96),
	}
	for i := 0; i < 4; i++ {
		go engine.playWorker()
	}
	return engine
}

func (a *AudioEngine) updateListening(listening ListeningConfig) {
	a.mu.Lock()
	a.listening = listening
	a.mu.Unlock()
}

func (a *AudioEngine) updatePeers(peers []PeerPresence, ownPeerID string) {
	sort.SliceStable(peers, func(i, j int) bool {
		if peers[i].JoinedAt == peers[j].JoinedAt {
			return peers[i].PeerID < peers[j].PeerID
		}
		return peers[i].JoinedAt < peers[j].JoinedAt
	})
	next := map[string]peerPlacement{}
	index := 0
	for _, peer := range peers {
		if peer.PeerID == ownPeerID {
			continue
		}
		next[peer.PeerID] = placementForIndex(index, peer.PeerID)
		index++
	}
	a.mu.Lock()
	a.placements = next
	a.mu.Unlock()
}

func (a *AudioEngine) scheduleBatch(peerID string, events []RemoteActivityEvent) {
	a.mu.Lock()
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
	if a.player == nil {
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
	a.mu.Lock()
	listening := a.listening
	a.mu.Unlock()
	fatigueGain := 1.0
	if listening.FatigueProtection {
		fatigueGain = a.recordAndGetFatigueGain()
	}
	job := playbackJob{
		File: samples[rand.Intn(len(samples))],
		Gain: clamp(listening.Volume*(1/placement.Distance)*fatigueGain, 0, 1),
		Pan:  0,
	}
	if listening.Spatial {
		job.Pan = placement.Pan
	}
	select {
	case a.queue <- job:
	default:
		<-a.queue
		a.queue <- job
	}
}

func (a *AudioEngine) playWorker() {
	for job := range a.queue {
		player := a.player
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
	next = append(next, now)
	a.recent = next
	overload := len(a.recent) - 24
	if overload < 0 {
		overload = 0
	}
	return clamp(1-float64(overload)*0.035, 0.35, 1)
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
				return []string{"-nodisp", "-autoexit", "-loglevel", "quiet", "-af", ffmpegSpatialFilter(job.Gain, job.Pan), job.File}
			},
		}
	}
	if _, err := exec.LookPath("mpv"); err == nil {
		return &audioPlayer{
			Command: "mpv",
			Spatial: true,
			ArgsFor: func(job playbackJob) []string {
				return []string{"--no-video", "--really-quiet", "--no-terminal", fmt.Sprintf("--volume=%d", int(clamp(job.Gain, 0, 1)*100)), fmt.Sprintf("--audio-pan=%.3f", clamp(job.Pan, -1, 1)), job.File}
			},
		}
	}
	if runtime.GOOS == "darwin" {
		return &audioPlayer{Command: "afplay", Spatial: false, ArgsFor: func(job playbackJob) []string {
			return []string{"-v", fmt.Sprintf("%.3f", clamp(job.Gain, 0, 1)), job.File}
		}}
	}
	if runtime.GOOS == "windows" {
		return &audioPlayer{Command: "powershell.exe", Spatial: false, ArgsFor: func(job playbackJob) []string {
			return []string{"-NoProfile", "-Command", "(New-Object Media.SoundPlayer $args[0]).PlaySync();", job.File}
		}}
	}
	if _, err := exec.LookPath("paplay"); err == nil {
		return &audioPlayer{Command: "paplay", Spatial: false, ArgsFor: func(job playbackJob) []string {
			return []string{"--volume", fmt.Sprintf("%d", int(clamp(job.Gain, 0, 1)*65536)), job.File}
		}}
	}
	if _, err := exec.LookPath("pw-play"); err == nil {
		return &audioPlayer{Command: "pw-play", Spatial: false, ArgsFor: func(job playbackJob) []string {
			return []string{"--volume", fmt.Sprintf("%.3f", clamp(job.Gain, 0, 1)), job.File}
		}}
	}
	if _, err := exec.LookPath("aplay"); err == nil {
		return &audioPlayer{Command: "aplay", Spatial: false, ArgsFor: func(job playbackJob) []string { return []string{job.File} }}
	}
	return nil
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
	return fmt.Sprintf("pan=stereo|c0=%.3f*c0|c1=%.3f*c1,volume=%.3f", left, right, gain)
}

func getAudioPlayerStatus() (player string, spatial bool, hint string, commands []string) {
	detected := detectAudioPlayer()
	if detected == nil {
		return "", false, audioInstallHint(), audioInstallCommands()
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
	ringStart := ringStartIndex(ring)
	positionInRing := index - ringStart
	capacity := ringCapacity(ring)
	baseAngle := math.Pi * 2 * float64(positionInRing) / float64(capacity)
	jitter := (seed.Float64() - 0.5) * (math.Pi / float64(capacity)) * 0.7
	angle := baseAngle + jitter
	distance := 2 + float64(ring) + (seed.Float64()-0.5)*0.35
	return peerPlacement{
		Pan:      clamp(math.Sin(angle), -0.95, 0.95),
		Distance: distance,
		Warmth:   0.72 + seed.Float64()*0.5,
	}
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
	return 4 + ring*4
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type StartOptions struct {
	CaptureMode string
	SelfMonitor bool
}

type SessionViewState struct {
	TeamName            string
	TeamCode            string
	ActiveCount         int
	ConnectionStatus    string
	CaptureMode         string
	PermissionHint      string
	LocalCapturedEvents int
	LocalSentEvents     int
	Listening           ListeningConfig
	HearingSelf         bool
	Notice              string
	Peers               []PeerPresence
}

type sessionController struct {
	cfg       CliksConfig
	opts      StartOptions
	ctx       context.Context
	cancel    context.CancelFunc
	audio     *AudioEngine
	updates   chan SessionViewState
	local     chan LocalActivityEvent
	mu        sync.Mutex
	wsMu      sync.Mutex
	ws        *websocket.Conn
	state     SessionViewState
	ownPeerID string
}

func startSession(cfg CliksConfig, opts StartOptions) error {
	controller := newSessionController(cfg, opts)
	if err := controller.start(); err != nil {
		return err
	}
	defer controller.stop()
	if term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		program := tea.NewProgram(newSessionModel(controller), tea.WithAltScreen(), tea.WithMouseCellMotion())
		_, err := program.Run()
		return err
	}
	fmt.Println("Cliks started. Press Ctrl+C to stop.")
	for state := range controller.updates {
		fmt.Printf("%s | %s | captured=%d sent=%d\n", state.TeamName, state.ConnectionStatus, state.LocalCapturedEvents, state.LocalSentEvents)
	}
	return nil
}

func newSessionController(cfg CliksConfig, opts StartOptions) *sessionController {
	ctx, cancel := context.WithCancel(context.Background())
	listening := cfg.Listening
	listening.Self = opts.SelfMonitor
	controller := &sessionController{
		cfg:     cfg,
		opts:    opts,
		ctx:     ctx,
		cancel:  cancel,
		audio:   newAudioEngine(listening),
		updates: make(chan SessionViewState, 32),
		local:   make(chan LocalActivityEvent, 256),
		state: SessionViewState{
			TeamName:         cfg.CurrentTeamCode,
			TeamCode:         cfg.CurrentTeamCode,
			ActiveCount:      1,
			ConnectionStatus: "starting",
			CaptureMode:      "starting",
			Listening:        listening,
			HearingSelf:      listening.Self,
		},
	}
	return controller
}

func (s *sessionController) start() error {
	captureState := CaptureState{Mode: "terminal"}
	if !(s.opts.CaptureMode == "terminal" && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))) {
		capture := newActivityCapture()
		captureState = capture.start(s.ctx, s.cfg.Sharing, s.opts.CaptureMode)
		go func() {
			for {
				select {
				case <-s.ctx.Done():
					return
				case event := <-capture.Events:
					s.recordLocalActivity(event)
				}
			}
		}()
		go func() {
			<-s.ctx.Done()
			capture.stop()
		}()
	}
	s.set(func(state *SessionViewState) {
		state.CaptureMode = captureState.Mode
		state.PermissionHint = captureState.PermissionHint
	})
	go s.batchLoop(s.local)
	go s.connectLoop()
	return nil
}

func (s *sessionController) stop() {
	s.cancel()
	s.wsMu.Lock()
	if s.ws != nil {
		_ = s.ws.Close()
	}
	s.wsMu.Unlock()
}

func (s *sessionController) viewState() SessionViewState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *sessionController) set(mutator func(*SessionViewState)) {
	s.mu.Lock()
	mutator(&s.state)
	state := s.state
	s.mu.Unlock()
	select {
	case s.updates <- state:
	default:
	}
}

func (s *sessionController) adjustVolume(delta float64) {
	s.set(func(state *SessionViewState) {
		state.Listening.Volume = clamp(state.Listening.Volume+delta, 0, 1)
		if state.Listening.Volume > 0 {
			state.Listening.Muted = false
		}
		s.cfg.Listening = state.Listening
	})
	s.audio.updateListening(s.viewState().Listening)
	_ = saveConfig(s.cfg)
}

func (s *sessionController) adjustDensity(delta float64) {
	s.set(func(state *SessionViewState) {
		state.Listening.Density = clamp(state.Listening.Density+delta, 0.15, 1)
		s.cfg.Listening = state.Listening
	})
	s.audio.updateListening(s.viewState().Listening)
	_ = saveConfig(s.cfg)
}

func (s *sessionController) toggle(key string) {
	s.set(func(state *SessionViewState) {
		switch key {
		case "muted":
			state.Listening.Muted = !state.Listening.Muted
		case "spatial":
			state.Listening.Spatial = !state.Listening.Spatial
		case "fade":
			state.Listening.FatigueProtection = !state.Listening.FatigueProtection
		}
		s.cfg.Listening = state.Listening
	})
	s.audio.updateListening(s.viewState().Listening)
	_ = saveConfig(s.cfg)
}

func (s *sessionController) recordLocalActivity(event LocalActivityEvent) {
	select {
	case s.local <- event:
	default:
	}
}

func (s *sessionController) batchLoop(events <-chan LocalActivityEvent) {
	window := time.Duration(s.cfg.BatchWindowMs) * time.Millisecond
	if window <= 0 {
		window = 500 * time.Millisecond
	}
	var batch []LocalActivityEvent
	var timer *time.Timer
	flush := func() {
		if len(batch) == 0 {
			return
		}
		startedAt := batch[0].At
		remote := make([]RemoteActivityEvent, 0, len(batch))
		for _, event := range batch {
			remote = append(remote, RemoteActivityEvent{
				Kind:     event.Kind,
				OffsetMs: int(event.At.Sub(startedAt).Milliseconds()),
				Button:   event.Button,
			})
		}
		batch = nil
		s.sendBatch(startedAt, remote)
	}
	for {
		select {
		case <-s.ctx.Done():
			flush()
			return
		case event := <-events:
			s.set(func(state *SessionViewState) {
				state.LocalCapturedEvents++
			})
			batch = append(batch, event)
			if timer == nil {
				timer = time.NewTimer(window)
			}
		case <-timerC(timer):
			flush()
			timer = nil
		}
	}
}

func timerC(timer *time.Timer) <-chan time.Time {
	if timer == nil {
		return nil
	}
	return timer.C
}

func (s *sessionController) sendBatch(startedAt time.Time, events []RemoteActivityEvent) {
	payload := map[string]any{
		"type":           "activity_batch",
		"teamCode":       s.cfg.CurrentTeamCode,
		"batchStartedAt": startedAt.UnixMilli(),
		"events":         events,
	}
	data, _ := json.Marshal(payload)
	s.wsMu.Lock()
	conn := s.ws
	if conn != nil {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
	s.wsMu.Unlock()
	if conn != nil {
		s.set(func(state *SessionViewState) {
			state.LocalSentEvents += len(events)
		})
	}
	if s.viewState().HearingSelf {
		s.audio.scheduleBatch("self", events)
	}
}

func (s *sessionController) connectLoop() {
	attempt := 0
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		status := "connecting"
		if attempt > 0 {
			status = fmt.Sprintf("reconnecting (%d)", attempt)
		}
		s.set(func(state *SessionViewState) { state.ConnectionStatus = status })
		conn, _, err := websocket.DefaultDialer.Dial(s.cfg.WSURL, nil)
		if err != nil {
			attempt++
			delay := reconnectDelay(attempt)
			s.set(func(state *SessionViewState) {
				state.ConnectionStatus = fmt.Sprintf("offline; retrying in %ds", int(delay.Seconds()))
			})
			sleepContext(s.ctx, delay)
			continue
		}
		attempt = 0
		s.wsMu.Lock()
		s.ws = conn
		s.wsMu.Unlock()
		_ = conn.WriteJSON(map[string]any{
			"type":     "join",
			"teamCode": s.cfg.CurrentTeamCode,
			"nickname": s.cfg.Nickname,
			"client":   map[string]string{"name": "cliks", "version": version},
		})
		s.set(func(state *SessionViewState) { state.ConnectionStatus = "connected" })
		closed := make(chan struct{})
		go s.pingLoop(conn, closed)
		s.readLoop(conn)
		close(closed)
		s.wsMu.Lock()
		if s.ws == conn {
			s.ws = nil
		}
		s.wsMu.Unlock()
		_ = conn.Close()
		attempt++
		delay := reconnectDelay(attempt)
		s.set(func(state *SessionViewState) {
			state.ConnectionStatus = fmt.Sprintf("disconnected; retrying in %ds", int(delay.Seconds()))
		})
		sleepContext(s.ctx, delay)
	}
}

func (s *sessionController) pingLoop(conn *websocket.Conn, closed <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-closed:
			return
		case <-ticker.C:
			_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
		}
	}
}

func (s *sessionController) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var envelope struct {
			Type           string                `json:"type"`
			Message        string                `json:"message,omitempty"`
			PeerID         string                `json:"peerId,omitempty"`
			TeamCode       string                `json:"teamCode,omitempty"`
			ActiveCount    int                   `json:"activeCount,omitempty"`
			Nickname       string                `json:"nickname,omitempty"`
			BatchStartedAt int64                 `json:"batchStartedAt,omitempty"`
			Events         []RemoteActivityEvent `json:"events,omitempty"`
			Peers          []PeerPresence        `json:"peers,omitempty"`
			Team           struct {
				Code string `json:"code"`
				Name string `json:"name"`
			} `json:"team,omitempty"`
		}
		if json.Unmarshal(data, &envelope) != nil {
			continue
		}
		switch envelope.Type {
		case "welcome":
			s.ownPeerID = envelope.PeerID
			teamName := envelope.Team.Name
			if teamName == "" {
				teamName = envelope.Team.Code
			}
			if teamName == "" {
				teamName = s.cfg.CurrentTeamCode
			}
			s.set(func(state *SessionViewState) {
				state.TeamName = teamName
				state.ActiveCount = envelope.ActiveCount
			})
		case "presence":
			s.audio.updatePeers(envelope.Peers, s.ownPeerID)
			s.set(func(state *SessionViewState) {
				state.ActiveCount = envelope.ActiveCount
				state.Peers = envelope.Peers
			})
		case "peer_activity_batch":
			s.audio.scheduleBatch(envelope.PeerID, envelope.Events)
		case "error":
			s.set(func(state *SessionViewState) {
				state.Notice = "Server: " + envelope.Message
			})
		}
	}
}

func reconnectDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second << minInt(attempt-1, 5)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

func sleepContext(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func validateWSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("WebSocket URL must start with ws:// or wss://")
	}
	return nil
}

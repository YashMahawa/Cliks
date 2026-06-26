package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
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

type sessionExitAction string

const (
	sessionExitStop sessionExitAction = "stop"
	sessionExitBack sessionExitAction = "back"
)

type SessionViewState struct {
	TeamName            string
	TeamCode            string
	OwnPeerID           string
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
	RecentPeerActivity  []PeerActivityStatus
}

type PeerActivityStatus struct {
	PeerID         string
	Nickname       string
	LastActivityAt time.Time
}

type sessionController struct {
	cfg            CliksConfig
	opts           StartOptions
	ctx            context.Context
	cancel         context.CancelFunc
	audio          *AudioEngine
	updates        chan SessionViewState
	local          chan LocalActivityEvent
	instance       *sessionInstance
	lastStateWrite time.Time
	mu             sync.Mutex
	wsMu           sync.Mutex
	ws             *websocket.Conn
	state          SessionViewState
	ownPeerID      string
}

func startSession(cfg CliksConfig, opts StartOptions) error {
	exit, err := runSession(cfg, opts)
	if err != nil {
		return err
	}
	if exit == sessionExitBack && isInteractiveTerminal() {
		return runHomeTUI(loadConfig())
	}
	return nil
}

func runSession(cfg CliksConfig, opts StartOptions) (sessionExitAction, error) {
	instance, err := acquireSessionInstance(cfg.CurrentTeamCode, runModeFromEnv())
	if err != nil {
		return sessionExitStop, err
	}
	controller := newSessionController(cfg, opts, instance)
	if err := controller.start(); err != nil {
		instance.release()
		return sessionExitStop, err
	}
	if term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		stopSignals := installSessionSignalHandler(controller)
		defer stopSignals()
		program := tea.NewProgram(newSessionModel(controller), tea.WithAltScreen(), tea.WithMouseAllMotion())
		finalModel, err := program.Run()
		if err != nil {
			controller.stop()
			return sessionExitStop, err
		}
		result, _ := finalModel.(sessionModel)
		if result.exit == "" {
			result.exit = sessionExitStop
		}
		keepRunning := result.exit != sessionExitStop && controller.cfg.KeepRunning && runModeFromEnv() == runModeForeground
		if message, err := finishSessionForExit(controller, keepRunning); err == nil && message != "" {
			fmt.Fprintln(os.Stderr, message)
		}
		return result.exit, nil
	}
	defer controller.stop()
	fmt.Println("Cliks started. Press Ctrl+C to stop.")
	for {
		select {
		case state := <-controller.updates:
			fmt.Printf("%s | %s | captured=%d sent=%d\n", state.TeamName, state.ConnectionStatus, state.LocalCapturedEvents, state.LocalSentEvents)
		case <-controller.ctx.Done():
			return sessionExitStop, nil
		}
	}
}

func installSessionSignalHandler(controller *sessionController) func() {
	exitSignals := tuiExitSignals()
	if len(exitSignals) == 0 {
		return func() {}
	}
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, exitSignals...)
	go func() {
		select {
		case <-signals:
			_, _ = finishSessionForExit(controller, controller.cfg.KeepRunning && runModeFromEnv() == runModeForeground)
			os.Exit(0)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

func finishSessionForExit(controller *sessionController, keepRunning bool) (string, error) {
	code := controller.cfg.CurrentTeamCode
	controller.stop()
	if !keepRunning {
		return "", nil
	}
	return startBackgroundForTeam(code)
}

func newSessionController(cfg CliksConfig, opts StartOptions, instance *sessionInstance) *sessionController {
	ctx, cancel := context.WithCancel(context.Background())
	listening := cfg.Listening
	listening.Self = opts.SelfMonitor
	controller := &sessionController{
		cfg:      cfg,
		opts:     opts,
		ctx:      ctx,
		cancel:   cancel,
		audio:    newAudioEngine(listening),
		updates:  make(chan SessionViewState, 32),
		local:    make(chan LocalActivityEvent, 256),
		instance: instance,
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
	s.writeSessionState(true)
	go s.batchLoop(s.local)
	go s.configLoop()
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
	s.writeSessionState(true)
	s.instance.release()
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
	s.writeSessionState(false)
	select {
	case s.updates <- state:
	default:
	}
}

func (s *sessionController) writeSessionState(force bool) {
	if s.instance == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(s.lastStateWrite) < time.Second {
		return
	}
	s.lastStateWrite = now
	s.instance.update(s.viewState())
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

func (s *sessionController) configLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			cfg := loadConfig()
			nickname := sanitizeNickname(cfg.Nickname)
			if nickname != sanitizeNickname(s.cfg.Nickname) {
				s.cfg.Nickname = nickname
				s.sendProfile(nickname)
			}
			if cfg.Listening != s.cfg.Listening {
				s.cfg.Listening = cfg.Listening
				s.audio.updateListening(cfg.Listening)
				s.set(func(state *SessionViewState) {
					state.Listening = cfg.Listening
					state.HearingSelf = cfg.Listening.Self
				})
			}
		}
	}
}

func (s *sessionController) sendProfile(nickname string) {
	s.wsMu.Lock()
	conn := s.ws
	if conn != nil {
		_ = conn.WriteJSON(map[string]any{
			"type":     "profile",
			"nickname": nickname,
		})
	}
	s.wsMu.Unlock()
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
			"nickname": sanitizeNickname(s.cfg.Nickname),
			"client": map[string]any{
				"name":     "cliks",
				"version":  version,
				"features": []string{"compact-v1"},
			},
		})
		s.set(func(state *SessionViewState) { state.ConnectionStatus = "connected" })
		closed := make(chan struct{})
		go s.pingLoop(conn, closed)
		fatal := s.readLoop(conn)
		close(closed)
		s.wsMu.Lock()
		if s.ws == conn {
			s.ws = nil
		}
		s.wsMu.Unlock()
		_ = conn.Close()
		if fatal {
			s.cancel()
			return
		}
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

func (s *sessionController) readLoop(conn *websocket.Conn) bool {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return false
		}
		var envelope struct {
			Type           string                `json:"type"`
			Message        string                `json:"message,omitempty"`
			Reason         string                `json:"reason,omitempty"`
			PeerID         string                `json:"peerId,omitempty"`
			TeamCode       string                `json:"teamCode,omitempty"`
			ActiveCount    int                   `json:"activeCount,omitempty"`
			Nickname       string                `json:"nickname,omitempty"`
			BatchStartedAt int64                 `json:"batchStartedAt,omitempty"`
			Events         []RemoteActivityEvent `json:"events,omitempty"`
			Peers          []PeerPresence        `json:"peers,omitempty"`
			CompactPeerID  string                `json:"p,omitempty"`
			CompactName    string                `json:"n,omitempty"`
			CompactAt      int64                 `json:"t,omitempty"`
			CompactEvents  []json.RawMessage     `json:"e,omitempty"`
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
			teamCode := strings.ToUpper(strings.TrimSpace(envelope.Team.Code))
			if teamCode == "" {
				teamCode = s.cfg.CurrentTeamCode
			}
			if teamName == "" {
				teamName = teamCode
			}
			if teamName == "" {
				teamName = s.cfg.CurrentTeamCode
			}
			s.set(func(state *SessionViewState) {
				state.TeamName = teamName
				state.TeamCode = teamCode
				state.OwnPeerID = envelope.PeerID
				state.ActiveCount = envelope.ActiveCount
			})
		case "presence":
			s.audio.updatePeers(envelope.Peers, s.ownPeerID)
			s.set(func(state *SessionViewState) {
				state.ActiveCount = envelope.ActiveCount
				state.Peers = envelope.Peers
				state.OwnPeerID = s.ownPeerID
			})
		case "peer_activity_batch":
			s.set(func(state *SessionViewState) {
				state.RecentPeerActivity = markPeerActive(state.RecentPeerActivity, envelope.PeerID, envelope.Nickname, time.Now())
			})
			s.audio.scheduleBatch(envelope.PeerID, envelope.Events)
		case "a":
			events := parseCompactEvents(envelope.CompactEvents)
			if envelope.CompactPeerID != "" && len(events) > 0 {
				s.set(func(state *SessionViewState) {
					state.RecentPeerActivity = markPeerActive(state.RecentPeerActivity, envelope.CompactPeerID, envelope.CompactName, time.Now())
				})
				s.audio.scheduleBatch(envelope.CompactPeerID, events)
			}
		case "team_deleted", "team_unavailable":
			s.handleTeamUnavailable(envelope.TeamCode, envelope.Message)
			return true
		case "error":
			s.set(func(state *SessionViewState) {
				state.Notice = "Server: " + envelope.Message
			})
		}
	}
}

func (s *sessionController) handleTeamUnavailable(teamCode string, message string) {
	teamCode = strings.ToUpper(strings.TrimSpace(teamCode))
	if teamCode == "" {
		teamCode = s.cfg.CurrentTeamCode
	}
	cfg, err := forgetTeam(teamCode)
	if err == nil {
		s.cfg = cfg
	}
	_, _ = autostartAction([]string{"disable"})
	if message == "" {
		message = "Team code was not found or was deleted."
	}
	s.set(func(state *SessionViewState) {
		state.ConnectionStatus = "stopped: team unavailable"
		state.Notice = message + " Removed it from this device."
		state.ActiveCount = 0
	})
}

func parseCompactEvents(raw []json.RawMessage) []RemoteActivityEvent {
	events := make([]RemoteActivityEvent, 0, len(raw))
	for _, item := range raw {
		var tuple []any
		if json.Unmarshal(item, &tuple) != nil || len(tuple) < 2 {
			continue
		}
		kind, _ := tuple[0].(string)
		offset, ok := numericJSON(tuple[1])
		if !ok {
			continue
		}
		switch kind {
		case "k":
			events = append(events, RemoteActivityEvent{Kind: "keyboard", OffsetMs: offset})
		case "m":
			button := "unknown"
			if len(tuple) > 2 {
				if compactButton, ok := tuple[2].(string); ok {
					switch compactButton {
					case "l":
						button = "left"
					case "r":
						button = "right"
					}
				}
			}
			events = append(events, RemoteActivityEvent{Kind: "mouse", OffsetMs: offset, Button: button})
		}
	}
	return events
}

func numericJSON(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func markPeerActive(current []PeerActivityStatus, peerID string, nickname string, at time.Time) []PeerActivityStatus {
	if peerID == "" {
		return current
	}
	nickname = sanitizeNickname(nickname)
	next := make([]PeerActivityStatus, 0, len(current)+1)
	updated := false
	cutoff := at.Add(-5 * time.Second)
	for _, item := range current {
		if item.LastActivityAt.Before(cutoff) {
			continue
		}
		if item.PeerID == peerID {
			item.Nickname = nickname
			item.LastActivityAt = at
			updated = true
		}
		next = append(next, item)
	}
	if !updated {
		next = append(next, PeerActivityStatus{PeerID: peerID, Nickname: nickname, LastActivityAt: at})
	}
	return next
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

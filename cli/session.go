package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

const clientWebSocketReadTimeout = 75 * time.Second

type sessionExitAction string

const (
	sessionExitStop   sessionExitAction = "stop"
	sessionExitBack   sessionExitAction = "back"
	sessionExitSwitch sessionExitAction = "switch"
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
	LastLocalActivityAt time.Time
	LastPeerActivityAt  time.Time
	LocalBurstCount     int
	RecentReactions     []PeerReactionStatus
}

type PeerActivityStatus struct {
	PeerID         string
	Nickname       string
	LastActivityAt time.Time
}

type PeerReactionStatus struct {
	PeerID       string
	Nickname     string
	Reaction     string
	TargetPeerID string
	At           time.Time
}

type sessionController struct {
	cfg              CliksConfig
	opts             StartOptions
	ctx              context.Context
	cancel           context.CancelFunc
	audio            *AudioEngine
	updates          chan SessionViewState
	local            chan LocalActivityEvent
	instance         *sessionInstance
	lastStateWrite   time.Time
	stateWriteTimer  *time.Timer
	stateWriteMu     sync.Mutex
	stopOnce         sync.Once
	mu               sync.Mutex
	wsMu             sync.Mutex
	ws               *websocket.Conn
	state            SessionViewState
	ownPeerID        string
	capture          *ActivityCapture
	rateLimitedUntil time.Time
	attached         bool
	attachedPID      int
}

func startSession(cfg CliksConfig, opts StartOptions) error {
	applyTheme(cfg.Theme)
	exit, err := runSession(cfg, opts)
	if err != nil {
		return err
	}
	if exit == sessionExitSwitch && isInteractiveTerminal() {
		return startSession(loadConfig(), opts)
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
		tuiCtx, stopSignals := tuiSignalContext(context.Background())
		defer stopSignals()
		model := newSessionModel(controller)
		program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithContext(tuiCtx))
		finalModel, err := program.Run()
		if err != nil && !errors.Is(err, context.Canceled) {
			controller.stop()
			return sessionExitStop, err
		}
		result, ok := finalModel.(sessionModel)
		if !ok {
			result = model
		}
		signalExit := tuiCtx.Err() != nil
		if result.exit == "" {
			result.exit = sessionExitStop
		}
		keepRunning := (result.exit != sessionExitStop || signalExit) && controller.cfg.KeepRunning && runModeFromEnv() == runModeForeground
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

func runAttachedSession(active ActiveSessionState) error {
	controller := newAttachedSessionController(active)
	defer controller.stop()
	tuiCtx, stopSignals := tuiSignalContext(context.Background())
	defer stopSignals()
	model := newSessionModel(controller)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithContext(tuiCtx))
	finalModel, err := program.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	result, ok := finalModel.(sessionModel)
	if !ok {
		result = model
	}
	switch result.exit {
	case sessionExitStop:
		_, err := stopActiveSession()
		return err
	case sessionExitSwitch:
		if _, err := stopActiveSession(); err != nil {
			return err
		}
		cfg := loadConfig()
		return startSession(cfg, StartOptions{CaptureMode: cfg.Capture.Mode, SelfMonitor: cfg.Listening.Self})
	default:
		return nil
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
	// Use the lightweight doctor path so session start stays snappy (no hook self-tests).
	setupNotice := quickDoctorWarning(cfg)
	if platformNotice := platformStartupCaptureNotice(); platformNotice != "" {
		if setupNotice != "" {
			setupNotice = setupNotice + " " + platformNotice
		} else {
			setupNotice = platformNotice
		}
	}
	controller := &sessionController{
		cfg:      cfg,
		opts:     opts,
		ctx:      ctx,
		cancel:   cancel,
		audio:    newAudioEngineWithContext(ctx, listening),
		updates:  make(chan SessionViewState, 32),
		local:    make(chan LocalActivityEvent, 1024),
		instance: instance,
		state: SessionViewState{
			TeamName:         cfg.CurrentTeamCode,
			TeamCode:         cfg.CurrentTeamCode,
			ActiveCount:      1,
			ConnectionStatus: "starting",
			CaptureMode:      "starting",
			Listening:        listening,
			HearingSelf:      listening.Self,
			Notice:           setupNotice,
		},
	}
	return controller
}

func newAttachedSessionController(active ActiveSessionState) *sessionController {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := loadConfig()
	state := active.View
	if state.TeamCode == "" {
		state = SessionViewState{
			TeamName:            valuePlain(active.TeamName, active.TeamCode),
			TeamCode:            active.TeamCode,
			ActiveCount:         maxInt(1, active.ActiveCount),
			ConnectionStatus:    valuePlain(active.ConnectionStatus, "running"),
			CaptureMode:         active.CaptureMode,
			LocalCapturedEvents: active.LocalCapturedEvents,
			LocalSentEvents:     active.LocalSentEvents,
			Listening:           cfg.Listening,
		}
	}
	return &sessionController{
		cfg:         cfg,
		ctx:         ctx,
		cancel:      cancel,
		updates:     make(chan SessionViewState),
		local:       make(chan LocalActivityEvent),
		state:       state,
		attached:    true,
		attachedPID: active.PID,
	}
}

func (s *sessionController) start() error {
	// Capture init can block on OS hooks; keep the TUI responsive by warming it up in the background.
	s.set(func(state *SessionViewState) {
		state.CaptureMode = "starting"
	})
	s.writeSessionState(true)
	go s.startCaptureAsync()
	go s.batchLoop(s.local)
	go s.configLoop()
	go s.commandLoop()
	go s.connectLoop()
	return nil
}

func (s *sessionController) startCaptureAsync() {
	captureState := CaptureState{Mode: "terminal"}
	// In a pure terminal session with --terminal, let the live TUI own keyboard/mouse capture.
	if s.opts.CaptureMode == "terminal" && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		s.set(func(state *SessionViewState) {
			state.CaptureMode = "terminal"
			state.PermissionHint = ""
		})
		return
	}
	capture := newActivityCapture()
	captureState = capture.start(s.ctx, s.cfg.Sharing, s.opts.CaptureMode)
	// If global capture failed, fall back to terminal mode when interactive so users are not stuck silent.
	if captureState.Mode == "off" && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) && s.opts.CaptureMode != "evdev" {
		if err := capture.startTerminal(s.ctx, s.cfg.Sharing); err == nil {
			hint := captureState.PermissionHint
			if hint != "" {
				hint += " "
			}
			hint += "Fell back to terminal capture for this session."
			captureState = CaptureState{Mode: "terminal", PermissionHint: hint}
		}
	}
	s.mu.Lock()
	s.capture = capture
	s.mu.Unlock()
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case event, ok := <-capture.Events:
				if !ok {
					return
				}
				s.recordLocalActivity(event)
			}
		}
	}()
	go func() {
		<-s.ctx.Done()
		capture.stop()
	}()
	s.set(func(state *SessionViewState) {
		state.CaptureMode = captureState.Mode
		state.PermissionHint = captureState.PermissionHint
	})
}

func (s *sessionController) stop() {
	if s.attached {
		s.cancel()
		return
	}
	s.stopOnce.Do(func() {
		s.cancel()
		s.mu.Lock()
		capture := s.capture
		s.mu.Unlock()
		if capture != nil {
			capture.stop()
		}
		s.wsMu.Lock()
		if s.ws != nil {
			_ = s.ws.Close()
		}
		s.wsMu.Unlock()
		s.audio.Close()
		s.flushSessionState()
		s.instance.release()
	})
}

func (s *sessionController) viewState() SessionViewState {
	if s.attached {
		if active, ok := activeSession(); ok && active.PID == s.attachedPID {
			if active.View.TeamCode != "" {
				s.mu.Lock()
				s.state = active.View
				s.mu.Unlock()
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *sessionController) set(mutator func(*SessionViewState)) {
	s.mu.Lock()
	prevStatus := s.state.ConnectionStatus
	prevCapture := s.state.CaptureMode
	prevNotice := s.state.Notice
	mutator(&s.state)
	// Lifecycle transitions must not be swallowed by the write throttle.
	force := s.state.ConnectionStatus != prevStatus ||
		s.state.CaptureMode != prevCapture ||
		s.state.Notice != prevNotice
	state := s.state
	s.mu.Unlock()
	s.writeSessionState(force)
	select {
	case s.updates <- state:
	default:
	}
}

const sessionViewWriteInterval = 250 * time.Millisecond

func (s *sessionController) writeSessionState(force bool) {
	if s.instance == nil || s.attached {
		return
	}
	now := time.Now()
	if force || now.Sub(s.lastStateWrite) >= sessionViewWriteInterval {
		s.flushSessionState()
		return
	}
	// Trailing-edge debounce: always persist the final state after a burst of updates.
	s.stateWriteMu.Lock()
	defer s.stateWriteMu.Unlock()
	if s.stateWriteTimer != nil {
		s.stateWriteTimer.Stop()
	}
	delay := sessionViewWriteInterval - now.Sub(s.lastStateWrite)
	if delay < 50*time.Millisecond {
		delay = 50 * time.Millisecond
	}
	s.stateWriteTimer = time.AfterFunc(delay, func() {
		s.flushSessionState()
	})
}

func (s *sessionController) flushSessionState() {
	if s.instance == nil {
		return
	}
	s.stateWriteMu.Lock()
	if s.stateWriteTimer != nil {
		s.stateWriteTimer.Stop()
		s.stateWriteTimer = nil
	}
	s.stateWriteMu.Unlock()
	s.lastStateWrite = time.Now()
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
	if s.audio != nil {
		s.audio.updateListening(s.viewState().Listening)
	}
	_ = saveConfig(s.cfg)
}

func (s *sessionController) adjustDensity(delta float64) {
	s.set(func(state *SessionViewState) {
		state.Listening.Density = clamp(state.Listening.Density+delta, 0.15, 1)
		s.cfg.Listening = state.Listening
	})
	if s.audio != nil {
		s.audio.updateListening(s.viewState().Listening)
	}
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
	if s.audio != nil {
		s.audio.updateListening(s.viewState().Listening)
	}
	_ = saveConfig(s.cfg)
}

func (s *sessionController) recordLocalActivity(event LocalActivityEvent) {
	if s.attached {
		return
	}
	s.set(func(state *SessionViewState) {
		if !state.LastLocalActivityAt.IsZero() && event.At.Sub(state.LastLocalActivityAt) <= 5*time.Second {
			state.LocalBurstCount++
		} else {
			state.LocalBurstCount = 1
		}
		state.LastLocalActivityAt = event.At
	})
	select {
	case s.local <- event:
	case <-s.ctx.Done():
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
	ticker := time.NewTicker(500 * time.Millisecond)
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
				s.sendProfile(nickname, cfg.PresenceStatus)
			}
			if cfg.PresenceStatus != s.cfg.PresenceStatus {
				s.cfg.PresenceStatus = cfg.PresenceStatus
				s.sendProfile(nickname, cfg.PresenceStatus)
			}
			s.cfg.Notifications = cfg.Notifications
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

func (s *sessionController) commandLoop() {
	if s.attached {
		return
	}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			consumeSessionCommands(func(command localSessionCommand) {
				if command.Type == "reaction" {
					s.sendReaction(command.Reaction)
				}
			})
		}
	}
}

func (s *sessionController) sendProfile(nickname string, status string) {
	s.wsMu.Lock()
	conn := s.ws
	if conn != nil {
		_ = conn.WriteJSON(map[string]any{
			"type":     "profile",
			"nickname": nickname,
			"status":   status,
		})
	}
	s.wsMu.Unlock()
}

func (s *sessionController) sendReaction(reaction string) {
	if s.attached {
		_ = enqueueSessionCommand(localSessionCommand{Type: "reaction", Reaction: reaction})
		return
	}
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.ws == nil {
		return
	}
	_ = s.ws.WriteJSON(map[string]any{"type": "reaction", "reaction": reaction})
}

func (s *sessionController) cyclePresence() {
	s.cfg.PresenceStatus = nextPresence(s.cfg.PresenceStatus, 1)
	_ = saveConfig(s.cfg)
	if !s.attached {
		s.sendProfile(sanitizeNickname(s.cfg.Nickname), s.cfg.PresenceStatus)
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
		if remaining := time.Until(s.rateLimitedUntil); remaining > 0 {
			s.set(func(state *SessionViewState) {
				state.ConnectionStatus = fmt.Sprintf("rate limited; resume in %s", formatCountdown(remaining))
				state.Notice = "Rate limited by the server. Waiting before reconnecting."
			})
			sleepContext(s.ctx, remaining)
			if s.ctx.Err() != nil {
				return
			}
			s.rateLimitedUntil = time.Time{}
		}
		status := "connecting"
		if attempt > 0 {
			status = fmt.Sprintf("reconnecting (%d)", attempt)
		}
		// Clear stale error notices so reconnecting is not shown under an old failure banner.
		s.set(func(state *SessionViewState) {
			state.ConnectionStatus = status
			if strings.Contains(strings.ToLower(state.Notice), "rate limit") ||
				strings.Contains(strings.ToLower(state.Notice), "server:") ||
				strings.Contains(strings.ToLower(state.Notice), "offline") {
				state.Notice = ""
			}
		})
		conn, resp, err := websocket.DefaultDialer.Dial(s.cfg.WSURL, nil)
		if err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if isRateLimitResponse(resp, err) {
				s.enterRateLimit(attempt, resp)
				attempt++
				continue
			}
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
		_ = conn.SetReadDeadline(time.Now().Add(clientWebSocketReadTimeout))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(clientWebSocketReadTimeout))
		})
		err = conn.WriteJSON(map[string]any{
			"type":     "join",
			"teamCode": s.cfg.CurrentTeamCode,
			"nickname": sanitizeNickname(s.cfg.Nickname),
			"status":   s.cfg.PresenceStatus,
			"client": map[string]any{
				"name":     "cliks",
				"version":  version,
				"features": []string{"compact-v1"},
			},
		})
		s.wsMu.Unlock()
		if err != nil {
			_ = conn.Close()
			s.wsMu.Lock()
			if s.ws == conn {
				s.ws = nil
			}
			s.wsMu.Unlock()
			attempt++
			delay := reconnectDelay(attempt)
			s.set(func(state *SessionViewState) {
				state.ConnectionStatus = fmt.Sprintf("offline; retrying in %ds", int(delay.Seconds()))
			})
			sleepContext(s.ctx, delay)
			continue
		}
		s.set(func(state *SessionViewState) {
			state.ConnectionStatus = "connected"
			state.Notice = ""
		})
		closed := make(chan struct{})
		go s.pingLoop(conn, closed)
		go closeWebSocketOnContext(s.ctx, conn, closed)
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

func (s *sessionController) enterRateLimit(attempt int, resp *http.Response) {
	wait := rateLimitWait(resp)
	s.rateLimitedUntil = time.Now().Add(wait)
	s.set(func(state *SessionViewState) {
		state.ConnectionStatus = fmt.Sprintf("rate limited; resume in %s", formatCountdown(wait))
		state.Notice = "Rate limited (HTTP 429). Pausing automatic reconnects until the window ends."
	})
	sleepContext(s.ctx, wait)
	s.rateLimitedUntil = time.Time{}
	_ = attempt
}

func isRateLimitResponse(resp *http.Response, err error) bool {
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests")
}

func rateLimitWait(resp *http.Response) time.Duration {
	if resp != nil {
		if retry := strings.TrimSpace(resp.Header.Get("Retry-After")); retry != "" {
			if seconds, err := strconv.Atoi(retry); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	// Server blocks for about five minutes; wait that window instead of spinning reconnects.
	return 5 * time.Minute
}

func formatCountdown(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second).Seconds())
	minutes := total / 60
	seconds := total % 60
	if minutes > 0 {
		return fmt.Sprintf("%d:%02d", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
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
			// Gorilla WebSocket requires a single concurrent writer; heartbeats must
			// take the same mutex as activity batches and profile updates.
			s.wsMu.Lock()
			active := s.ws
			if active == conn {
				_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
			s.wsMu.Unlock()
		}
	}
}

func (s *sessionController) readLoop(conn *websocket.Conn) bool {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return false
		}
		_ = conn.SetReadDeadline(time.Now().Add(clientWebSocketReadTimeout))
		var envelope struct {
			Type           string                `json:"type"`
			Message        string                `json:"message,omitempty"`
			Reason         string                `json:"reason,omitempty"`
			PeerID         string                `json:"peerId,omitempty"`
			TeamCode       string                `json:"teamCode,omitempty"`
			ActiveCount    int                   `json:"activeCount,omitempty"`
			Nickname       string                `json:"nickname,omitempty"`
			Reaction       string                `json:"reaction,omitempty"`
			TargetPeerID   string                `json:"targetPeerId,omitempty"`
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
				state.LastPeerActivityAt = time.Now()
				state.RecentPeerActivity = markPeerActive(state.RecentPeerActivity, envelope.PeerID, envelope.Nickname, time.Now())
			})
			s.audio.scheduleBatch(envelope.PeerID, envelope.Events)
		case "peer_reaction":
			now := time.Now()
			cfg := loadConfig()
			isOwn := envelope.PeerID == s.ownPeerID
			if isOwn || !cfg.Listening.Muted {
				s.set(func(state *SessionViewState) {
					state.RecentReactions = append(state.RecentReactions, PeerReactionStatus{PeerID: envelope.PeerID, Nickname: envelope.Nickname, Reaction: envelope.Reaction, At: now})
					if len(state.RecentReactions) > 8 {
						state.RecentReactions = state.RecentReactions[len(state.RecentReactions)-8:]
					}
				})
			}
			if !isOwn && !cfg.Listening.Muted {
				go func(cfg CliksConfig, nickname string, reaction string) {
					if err := notifyReaction(cfg, nickname, reaction); err != nil {
						s.set(func(state *SessionViewState) {
							state.Notice = "Notification failed: " + err.Error() + ". Run cliks notification-test."
						})
					}
				}(cfg, envelope.Nickname, envelope.Reaction)
			}
		case "a":
			events := parseCompactEvents(envelope.CompactEvents)
			if envelope.CompactPeerID != "" && len(events) > 0 {
				s.set(func(state *SessionViewState) {
					state.LastPeerActivityAt = time.Now()
					state.RecentPeerActivity = markPeerActive(state.RecentPeerActivity, envelope.CompactPeerID, envelope.CompactName, time.Now())
				})
				s.audio.scheduleBatch(envelope.CompactPeerID, events)
			}
		case "team_deleted", "team_unavailable":
			s.handleTeamUnavailable(envelope.TeamCode, envelope.Message)
			return true
		case "error":
			msg := strings.TrimSpace(envelope.Message)
			if isRateLimitMessage(msg) {
				s.rateLimitedUntil = time.Now().Add(5 * time.Minute)
				s.set(func(state *SessionViewState) {
					state.ConnectionStatus = "rate limited; resume in 5:00"
					state.Notice = "Rate limited: " + msg
				})
				return false
			}
			s.set(func(state *SessionViewState) {
				state.Notice = "Server: " + msg
			})
		}
	}
}

func closeWebSocketOnContext(ctx context.Context, conn *websocket.Conn, closed <-chan struct{}) {
	select {
	case <-ctx.Done():
		_ = conn.Close()
	case <-closed:
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
	return randomizedRetryDelay(attempt)
}

func isRateLimitMessage(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many") ||
		strings.Contains(msg, "429")
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

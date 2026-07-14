package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	timingBucketMs              = 50
	maxPeersPerRoom             = 20
	compactFeatureV1            = "compact-v1"
	maxWebSocketMessageBytes    = 8 * 1024
	maxWebSocketMessagesPerTick = 30
	webSocketRateWindow         = time.Second
	webSocketReadTimeout        = 75 * time.Second
	webSocketWriteTimeout       = 5 * time.Second
	webSocketOutboundBuffer     = 32
	maxReactionsPerWindow       = 6
	reactionRateWindow          = 10 * time.Second
	waveCooldown                = 30 * time.Second
)

type ActivityEvent struct {
	Kind     string `json:"kind"`
	OffsetMs int    `json:"offsetMs"`
	Button   string `json:"button,omitempty"`
}

type PeerPresence struct {
	PeerID   string `json:"peerId"`
	Nickname string `json:"nickname,omitempty"`
	JoinedAt int64  `json:"joinedAt"`
	Status   string `json:"status,omitempty"`
}

type clientConn struct {
	id              string
	socket          *websocket.Conn
	outbound        chan outboundFrame
	done            chan struct{}
	closeOnce       sync.Once
	alive           bool
	roomCode        string
	rateLimitKey    string
	readWindowStart time.Time
	readWindowCount int
}

type outboundFrame struct {
	messageType int
	data        []byte
	closeAfter  bool
}

type peer struct {
	id                  string
	nickname            string
	conn                *clientConn
	team                Team
	joinedAt            int64
	lastSeen            int64
	compactV1           bool
	status              string
	reactionWindowStart time.Time
	reactionCount       int
	lastWaves           map[string]time.Time
}

type room struct {
	team  Team
	peers map[string]*peer
}

type teamGate struct {
	mu   sync.Mutex
	refs int
}

type RoomHub struct {
	store     TeamStore
	upgrader  websocket.Upgrader
	mu        sync.Mutex
	conns     map[string]*clientConn
	rooms     map[string]*room
	joinRate  *rateLimiter
	gateMu    sync.Mutex
	teamGates map[string]*teamGate
	maxPeers  int
}

func NewRoomHub(store TeamStore, joinLimiters ...*rateLimiter) *RoomHub {
	hub := &RoomHub{
		store: store,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		conns:     map[string]*clientConn{},
		rooms:     map[string]*room{},
		teamGates: map[string]*teamGate{},
		maxPeers:  configuredMaxPeers(),
	}
	if len(joinLimiters) > 0 {
		hub.joinRate = joinLimiters[0]
	}
	if hub.joinRate == nil {
		hub.joinRate = newRateLimiter(5*time.Minute, 20)
	}
	return hub
}

func configuredMaxPeers() int {
	value, err := strconv.Atoi(os.Getenv("CLIKS_MAX_PEERS_PER_ROOM"))
	if err != nil || value < 2 {
		return maxPeersPerRoom
	}
	if value > 200 {
		return 200
	}
	return value
}

func (h *RoomHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	socket, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	socket.SetReadLimit(maxWebSocketMessageBytes)
	conn := newClientConn(newPeerID(), socket, rateLimitKey(r))
	defer recoverAndLog("WebSocket connection")
	conn.refreshReadDeadline()
	socket.SetPongHandler(func(string) error {
		conn.refreshReadDeadline()
		h.mu.Lock()
		conn.alive = true
		h.mu.Unlock()
		return nil
	})
	h.mu.Lock()
	h.conns[conn.id] = conn
	h.mu.Unlock()
	go conn.writeLoop()
	defer func() {
		h.leave(conn.id)
		h.mu.Lock()
		delete(h.conns, conn.id)
		h.mu.Unlock()
		_ = conn.close()
	}()

	for {
		_, data, err := socket.ReadMessage()
		if err != nil {
			return
		}
		conn.refreshReadDeadline()
		if !conn.allowRead(time.Now()) {
			conn.sendJSONAndCloseWait(protocolError("message_rate_limited", "Too many WebSocket messages. Slow down and reconnect."))
			return
		}
		h.handleMessage(r.Context(), conn, data)
	}
}

func (h *RoomHub) handleMessage(ctx context.Context, conn *clientConn, data []byte) {
	var header struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(data, &header) != nil {
		conn.sendJSON(serverError("Invalid message."))
		return
	}

	switch header.Type {
	case "join":
		var message struct {
			TeamCode string `json:"teamCode"`
			Nickname string `json:"nickname"`
			Status   string `json:"status"`
			Client   struct {
				Name     string   `json:"name"`
				Version  string   `json:"version"`
				Features []string `json:"features"`
			} `json:"client"`
		}
		if json.Unmarshal(data, &message) != nil {
			conn.sendJSON(serverError("Invalid join message."))
			return
		}
		h.join(ctx, conn, normalizeTeamCode(message.TeamCode), normalizeNickname(message.Nickname), normalizePresenceStatus(message.Status), boolFeature(message.Client.Features, compactFeatureV1))
	case "profile":
		var message struct {
			Nickname string `json:"nickname"`
			Status   string `json:"status"`
		}
		if json.Unmarshal(data, &message) != nil {
			conn.sendJSON(serverError("Invalid profile message."))
			return
		}
		h.updatePeerProfile(conn.id, normalizeNickname(message.Nickname), normalizePresenceStatus(message.Status))
	case "reaction":
		var message struct {
			Reaction     string `json:"reaction"`
			TargetPeerID string `json:"targetPeerId,omitempty"`
		}
		if json.Unmarshal(data, &message) != nil {
			conn.sendJSON(serverError("Invalid reaction message."))
			return
		}
		h.forwardReaction(conn.id, message.Reaction, message.TargetPeerID)
	case "activity_batch":
		var message struct {
			TeamCode       string          `json:"teamCode"`
			BatchStartedAt int64           `json:"batchStartedAt"`
			Events         []ActivityEvent `json:"events"`
		}
		if json.Unmarshal(data, &message) != nil {
			conn.sendJSON(serverError("Invalid activity batch."))
			return
		}
		h.forwardActivity(conn.id, normalizeTeamCode(message.TeamCode), message.BatchStartedAt, sanitizeEvents(message.Events))
	default:
		conn.sendJSON(serverError("Join a team before sending activity."))
	}
}

func (h *RoomHub) join(ctx context.Context, conn *clientConn, teamCode string, nickname string, status string, compactV1 bool) {
	if h.joinRate.Blocked(conn.rateLimitKey) {
		conn.sendJSONAndClose(joinRateLimitedPayload())
		return
	}
	if teamCode == "" || len(teamCode) > 16 {
		h.rejectUnavailableJoin(conn, teamCode)
		return
	}
	unlockTeam := h.lockTeam(teamCode)
	defer unlockTeam()
	team, err := h.store.GetTeamByCode(ctx, teamCode)
	if err != nil {
		conn.sendJSONAndClose(serverError("Could not load team."))
		return
	}
	if team == nil {
		h.rejectUnavailableJoin(conn, teamCode)
		return
	}
	if activityStore, ok := h.store.(TeamActivityStore); ok {
		if err := activityStore.TouchTeam(ctx, team.Code); err != nil {
			if errors.Is(err, errTeamUnavailable) {
				h.rejectUnavailableJoin(conn, teamCode)
				return
			}
			conn.sendJSONAndClose(serverError("Could not refresh team. Please retry."))
			return
		}
		team.ExpiresAt = time.Now().Add(teamIdleTTL).UTC().Format(time.RFC3339Nano)
	}

	joinedAt := time.Now().UnixMilli()
	p := &peer{
		id:        conn.id,
		nickname:  nickname,
		conn:      conn,
		team:      *team,
		joinedAt:  joinedAt,
		lastSeen:  joinedAt,
		compactV1: compactV1,
		status:    status,
		lastWaves: map[string]time.Time{},
	}

	var previousPayload any
	var previousPeers []*peer
	h.mu.Lock()
	currentRoom := h.rooms[team.Code]
	if currentRoom == nil {
		currentRoom = &room{team: *team, peers: map[string]*peer{}}
		h.rooms[team.Code] = currentRoom
	}
	if len(currentRoom.peers) >= h.maxPeers && currentRoom.peers[p.id] == nil {
		h.mu.Unlock()
		conn.sendJSONAndClose(roomFullPayload(h.maxPeers))
		return
	}
	if previousCode := conn.roomCode; previousCode != "" && previousCode != team.Code {
		if previousRoom := h.rooms[previousCode]; previousRoom != nil {
			delete(previousRoom.peers, p.id)
			if len(previousRoom.peers) == 0 {
				delete(h.rooms, previousCode)
			} else {
				previousPayload, previousPeers = presenceLocked(previousRoom)
			}
		}
	}
	currentRoom.peers[p.id] = p
	conn.roomCode = team.Code
	activeCount := len(currentRoom.peers)
	presencePayload, presencePeers := presenceLocked(currentRoom)
	h.mu.Unlock()

	sendToPeers(previousPeers, previousPayload)
	conn.sendJSON(map[string]any{
		"type":        "welcome",
		"peerId":      p.id,
		"team":        team,
		"activeCount": activeCount,
	})
	sendToPeers(presencePeers, presencePayload)
}

func (h *RoomHub) rejectUnavailableJoin(conn *clientConn, teamCode string) {
	if !h.joinRate.Allow(conn.rateLimitKey) {
		conn.sendJSONAndClose(joinRateLimitedPayload())
	} else {
		conn.sendJSONAndClose(teamUnavailablePayload(teamCode))
	}
}

func (h *RoomHub) updatePeerProfile(peerID string, nickname string, status string) {
	var payload any
	var peers []*peer
	h.mu.Lock()
	conn := h.conns[peerID]
	if conn != nil {
		if currentRoom := h.rooms[conn.roomCode]; currentRoom != nil {
			if p := currentRoom.peers[peerID]; p != nil {
				p.nickname = nickname
				p.status = status
				p.lastSeen = time.Now().UnixMilli()
				payload, peers = presenceLocked(currentRoom)
			}
		}
	}
	h.mu.Unlock()
	sendToPeers(peers, payload)
}

func (h *RoomHub) forwardReaction(peerID string, reaction string, targetPeerID string) {
	reaction = normalizeReaction(reaction)
	if reaction == "" {
		return
	}
	now := time.Now()
	var recipients []*peer
	var sender *peer
	var senderID string
	var senderNickname string
	h.mu.Lock()
	conn := h.conns[peerID]
	if conn != nil {
		if currentRoom := h.rooms[conn.roomCode]; currentRoom != nil {
			sender = currentRoom.peers[peerID]
			if sender != nil {
				if sender.reactionWindowStart.IsZero() || now.Sub(sender.reactionWindowStart) >= reactionRateWindow {
					sender.reactionWindowStart = now
					sender.reactionCount = 0
				}
				if sender.reactionCount < maxReactionsPerWindow {
					sender.reactionCount++
					if reaction == "wave" {
						if targetPeerID == "" || targetPeerID == peerID || currentRoom.peers[targetPeerID] == nil || now.Sub(sender.lastWaves[targetPeerID]) < waveCooldown {
							sender = nil
						} else {
							sender.lastWaves[targetPeerID] = now
							recipients = []*peer{sender, currentRoom.peers[targetPeerID]}
						}
					} else {
						targetPeerID = ""
						for _, p := range currentRoom.peers {
							recipients = append(recipients, p)
						}
					}
					if sender != nil {
						senderID = sender.id
						senderNickname = sender.nickname
					}
				} else {
					sender = nil
				}
			}
		}
	}
	h.mu.Unlock()
	if senderID == "" {
		return
	}
	payload := map[string]any{
		"type": "peer_reaction", "peerId": senderID, "nickname": senderNickname,
		"reaction": reaction, "targetPeerId": targetPeerID, "sentAt": now.UnixMilli(),
	}
	sendToPeers(recipients, payload)
}

func (h *RoomHub) forwardActivity(peerID string, teamCode string, batchStartedAt int64, events []ActivityEvent) {
	if len(events) == 0 {
		return
	}
	var recipients []*peer
	var sender *peer
	var resolvedTeam string

	h.mu.Lock()
	conn := h.conns[peerID]
	if conn != nil {
		currentRoom := h.rooms[conn.roomCode]
		if currentRoom != nil && (teamCode == "" || currentRoom.team.Code == teamCode) {
			sender = currentRoom.peers[peerID]
			if sender != nil {
				sender.lastSeen = time.Now().UnixMilli()
				resolvedTeam = currentRoom.team.Code
				for _, candidate := range currentRoom.peers {
					if candidate.id != peerID {
						recipients = append(recipients, candidate)
					}
				}
			}
		}
	}
	h.mu.Unlock()

	if sender == nil {
		return
	}
	for _, recipient := range recipients {
		if recipient.compactV1 {
			recipient.conn.sendJSON(compactActivityPayload(sender.id, sender.nickname, batchStartedAt, events))
		} else {
			recipient.conn.sendJSON(verboseActivityPayload(resolvedTeam, sender.id, sender.nickname, batchStartedAt, events))
		}
	}
}

func (h *RoomHub) DeleteTeam(ctx context.Context, input DeleteTeamInput) (bool, error) {
	input.Code = normalizeTeamCode(input.Code)
	unlockTeam := h.lockTeam(input.Code)
	defer unlockTeam()
	deleted, err := h.store.DeleteTeam(ctx, input)
	if err != nil || !deleted {
		return deleted, err
	}
	h.closeRoom(input.Code, "This team was deleted.")
	return true, nil
}

func (h *RoomHub) closeRoom(teamCode string, message string) {
	var peers []*peer
	h.mu.Lock()
	if room := h.rooms[teamCode]; room != nil {
		for _, p := range room.peers {
			p.conn.roomCode = ""
			peers = append(peers, p)
		}
		delete(h.rooms, teamCode)
	}
	h.mu.Unlock()
	payload := deletedPayload(teamCode, message)
	for _, p := range peers {
		p.conn.sendJSONAndClose(payload)
	}
}

func (h *RoomHub) ActiveTeamCodes() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	codes := make([]string, 0, len(h.rooms))
	for code := range h.rooms {
		codes = append(codes, code)
	}
	return codes
}

func (h *RoomHub) lockTeam(teamCode string) func() {
	h.gateMu.Lock()
	gate := h.teamGates[teamCode]
	if gate == nil {
		gate = &teamGate{}
		h.teamGates[teamCode] = gate
	}
	gate.refs++
	h.gateMu.Unlock()

	gate.mu.Lock()
	return func() {
		gate.mu.Unlock()
		h.gateMu.Lock()
		gate.refs--
		if gate.refs == 0 && h.teamGates[teamCode] == gate {
			delete(h.teamGates, teamCode)
		}
		h.gateMu.Unlock()
	}
}

func (h *RoomHub) leave(peerID string) {
	var payload any
	var peers []*peer
	h.mu.Lock()
	conn := h.conns[peerID]
	if conn != nil && conn.roomCode != "" {
		code := conn.roomCode
		if currentRoom := h.rooms[code]; currentRoom != nil {
			delete(currentRoom.peers, peerID)
			if len(currentRoom.peers) == 0 {
				delete(h.rooms, code)
			} else {
				payload, peers = presenceLocked(currentRoom)
			}
		}
		conn.roomCode = ""
	}
	h.mu.Unlock()
	sendToPeers(peers, payload)
}

func (h *RoomHub) heartbeatLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		runSafely("heartbeat tick", h.heartbeatTick)
	}
}

func (h *RoomHub) heartbeatTick() {
	var stale []*clientConn
	var live []*clientConn
	h.mu.Lock()
	for _, conn := range h.conns {
		if !conn.alive {
			stale = append(stale, conn)
			continue
		}
		conn.alive = false
		live = append(live, conn)
	}
	h.mu.Unlock()
	for _, conn := range stale {
		h.leave(conn.id)
		h.mu.Lock()
		if h.conns[conn.id] == conn {
			delete(h.conns, conn.id)
		}
		h.mu.Unlock()
		_ = conn.close()
	}
	for _, conn := range live {
		conn.sendPing()
	}
}

func (h *RoomHub) TotalRooms() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

func (h *RoomHub) TotalPeers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	total := 0
	for _, room := range h.rooms {
		total += len(room.peers)
	}
	return total
}

func newClientConn(id string, socket *websocket.Conn, rateLimitKey string) *clientConn {
	return &clientConn{
		id:           id,
		socket:       socket,
		outbound:     make(chan outboundFrame, webSocketOutboundBuffer),
		done:         make(chan struct{}),
		alive:        true,
		rateLimitKey: rateLimitKey,
	}
}

func (c *clientConn) sendJSON(value any) bool {
	return c.enqueueJSON(value, false)
}

func (c *clientConn) sendJSONAndClose(value any) bool {
	return c.enqueueJSON(value, true)
}

func (c *clientConn) sendJSONAndCloseWait(value any) bool {
	if !c.sendJSONAndClose(value) {
		return false
	}
	timer := time.NewTimer(webSocketWriteTimeout + time.Second)
	defer timer.Stop()
	select {
	case <-c.done:
		return true
	case <-timer.C:
		_ = c.close()
		return false
	}
}

func (c *clientConn) enqueueJSON(value any, closeAfter bool) bool {
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return c.enqueue(outboundFrame{messageType: websocket.TextMessage, data: data, closeAfter: closeAfter})
}

func (c *clientConn) sendPing() bool {
	return c.enqueue(outboundFrame{messageType: websocket.PingMessage})
}

func (c *clientConn) enqueue(frame outboundFrame) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case <-c.done:
		return false
	case c.outbound <- frame:
		return true
	default:
		log.Printf("closing slow WebSocket peer %s: outbound buffer full", c.id)
		_ = c.close()
		return false
	}
}

func (c *clientConn) writeLoop() {
	defer recoverAndLog("WebSocket writer")
	defer c.close()
	for {
		select {
		case <-c.done:
			return
		case frame := <-c.outbound:
			if c.socket == nil {
				_ = c.close()
				return
			}
			deadline := time.Now().Add(webSocketWriteTimeout)
			var err error
			if frame.messageType == websocket.PingMessage {
				err = c.socket.WriteControl(websocket.PingMessage, frame.data, deadline)
			} else {
				_ = c.socket.SetWriteDeadline(deadline)
				err = c.socket.WriteMessage(frame.messageType, frame.data)
			}
			if err != nil || frame.closeAfter {
				_ = c.close()
				return
			}
		}
	}
}

func (c *clientConn) refreshReadDeadline() {
	if c.socket != nil {
		_ = c.socket.SetReadDeadline(time.Now().Add(webSocketReadTimeout))
	}
}

func (c *clientConn) close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		if c.socket != nil {
			err = c.socket.Close()
		}
	})
	return err
}

func (c *clientConn) allowRead(now time.Time) bool {
	if c.readWindowStart.IsZero() || now.Sub(c.readWindowStart) >= webSocketRateWindow {
		c.readWindowStart = now
		c.readWindowCount = 0
	}
	c.readWindowCount++
	return c.readWindowCount <= maxWebSocketMessagesPerTick
}

func sendToPeers(peers []*peer, payload any) {
	if payload == nil {
		return
	}
	for _, p := range peers {
		p.conn.sendJSON(payload)
	}
}

func presenceLocked(room *room) (any, []*peer) {
	peers := make([]*peer, 0, len(room.peers))
	presence := make([]PeerPresence, 0, len(room.peers))
	for _, p := range room.peers {
		peers = append(peers, p)
		presence = append(presence, PeerPresence{
			PeerID:   p.id,
			Nickname: p.nickname,
			JoinedAt: p.joinedAt,
			Status:   p.status,
		})
	}
	return map[string]any{
		"type":        "presence",
		"teamCode":    room.team.Code,
		"activeCount": len(room.peers),
		"peers":       presence,
	}, peers
}

func normalizePresenceStatus(value string) string {
	switch value {
	case "available", "focus", "break", "dnd":
		return value
	default:
		return "available"
	}
}

func normalizeReaction(value string) string {
	switch value {
	case "wave", "nice", "coffee", "focus", "celebrate":
		return value
	default:
		return ""
	}
}

func sanitizeEvents(input []ActivityEvent) []ActivityEvent {
	if len(input) > 128 {
		input = input[:128]
	}
	events := make([]ActivityEvent, 0, len(input))
	for _, event := range input {
		switch event.Kind {
		case "keyboard":
			events = append(events, ActivityEvent{Kind: "keyboard", OffsetMs: quantizeOffset(event.OffsetMs)})
		case "mouse":
			button := event.Button
			if button != "left" && button != "right" {
				button = "unknown"
			}
			events = append(events, ActivityEvent{Kind: "mouse", OffsetMs: quantizeOffset(event.OffsetMs), Button: button})
		}
	}
	return events
}

func newPeerID() string {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "peer_fallback"
	}
	return "peer_" + hex.EncodeToString(bytes[:])
}

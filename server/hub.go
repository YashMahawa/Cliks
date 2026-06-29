package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
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
}

type clientConn struct {
	id              string
	socket          *websocket.Conn
	writeMu         sync.Mutex
	alive           bool
	roomCode        string
	rateLimitKey    string
	readWindowStart time.Time
	readWindowCount int
}

type peer struct {
	id        string
	nickname  string
	conn      *clientConn
	team      Team
	joinedAt  int64
	lastSeen  int64
	compactV1 bool
}

type room struct {
	team  Team
	peers map[string]*peer
}

type RoomHub struct {
	store    TeamStore
	upgrader websocket.Upgrader
	mu       sync.Mutex
	conns    map[string]*clientConn
	rooms    map[string]*room
	joinRate *rateLimiter
}

func NewRoomHub(store TeamStore, joinLimiters ...*rateLimiter) *RoomHub {
	hub := &RoomHub{
		store: store,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		conns: map[string]*clientConn{},
		rooms: map[string]*room{},
	}
	if len(joinLimiters) > 0 {
		hub.joinRate = joinLimiters[0]
	}
	if hub.joinRate == nil {
		hub.joinRate = newRateLimiter(5*time.Minute, 20)
	}
	return hub
}

func (h *RoomHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	socket, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	socket.SetReadLimit(maxWebSocketMessageBytes)
	conn := &clientConn{id: newPeerID(), socket: socket, alive: true, rateLimitKey: rateLimitKey(r)}
	defer recoverAndLog("WebSocket connection")
	socket.SetPongHandler(func(string) error {
		h.mu.Lock()
		conn.alive = true
		h.mu.Unlock()
		return nil
	})
	h.mu.Lock()
	h.conns[conn.id] = conn
	h.mu.Unlock()
	defer func() {
		h.leave(conn.id)
		h.mu.Lock()
		delete(h.conns, conn.id)
		h.mu.Unlock()
		_ = socket.Close()
	}()

	for {
		_, data, err := socket.ReadMessage()
		if err != nil {
			return
		}
		if !conn.allowRead(time.Now()) {
			conn.sendJSON(protocolError("message_rate_limited", "Too many WebSocket messages. Slow down and reconnect."))
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
		h.join(ctx, conn, normalizeTeamCode(message.TeamCode), normalizeNickname(message.Nickname), boolFeature(message.Client.Features, compactFeatureV1))
	case "profile":
		var message struct {
			Nickname string `json:"nickname"`
		}
		if json.Unmarshal(data, &message) != nil {
			conn.sendJSON(serverError("Invalid profile message."))
			return
		}
		h.updatePeerProfile(conn.id, normalizeNickname(message.Nickname))
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

func (h *RoomHub) join(ctx context.Context, conn *clientConn, teamCode string, nickname string, compactV1 bool) {
	if h.joinRate.Blocked(conn.rateLimitKey) {
		conn.sendJSON(joinRateLimitedPayload())
		_ = conn.close()
		return
	}
	if teamCode == "" || len(teamCode) > 16 {
		h.rejectUnavailableJoin(conn, teamCode)
		return
	}
	team, err := h.store.GetTeamByCode(ctx, teamCode)
	if err != nil {
		conn.sendJSON(serverError("Could not load team."))
		_ = conn.close()
		return
	}
	if team == nil {
		h.rejectUnavailableJoin(conn, teamCode)
		return
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
	}

	var previousPayload any
	var previousPeers []*peer
	h.mu.Lock()
	currentRoom := h.rooms[team.Code]
	if currentRoom == nil {
		currentRoom = &room{team: *team, peers: map[string]*peer{}}
		h.rooms[team.Code] = currentRoom
	}
	if len(currentRoom.peers) >= maxPeersPerRoom && currentRoom.peers[p.id] == nil {
		h.mu.Unlock()
		conn.sendJSON(roomFullPayload())
		_ = conn.close()
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
		conn.sendJSON(joinRateLimitedPayload())
	} else {
		conn.sendJSON(teamUnavailablePayload(teamCode))
	}
	_ = conn.close()
}

func (h *RoomHub) updatePeerProfile(peerID string, nickname string) {
	var payload any
	var peers []*peer
	h.mu.Lock()
	conn := h.conns[peerID]
	if conn != nil {
		if currentRoom := h.rooms[conn.roomCode]; currentRoom != nil {
			if p := currentRoom.peers[peerID]; p != nil {
				p.nickname = nickname
				p.lastSeen = time.Now().UnixMilli()
				payload, peers = presenceLocked(currentRoom)
			}
		}
	}
	h.mu.Unlock()
	sendToPeers(peers, payload)
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

func (h *RoomHub) CloseRoom(teamCode string, message string) {
	teamCode = normalizeTeamCode(teamCode)
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
		p.conn.sendJSON(payload)
		_ = p.conn.close()
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
		conn.writeMu.Lock()
		_ = conn.socket.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
		conn.writeMu.Unlock()
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

func (c *clientConn) sendJSON(value any) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.socket.WriteJSON(value)
}

func (c *clientConn) close() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.socket.Close()
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
		})
	}
	return map[string]any{
		"type":        "presence",
		"teamCode":    room.team.Code,
		"activeCount": len(room.peers),
		"peers":       presence,
	}, peers
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

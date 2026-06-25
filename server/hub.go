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
	timingBucketMs   = 50
	maxPeersPerRoom  = 20
	compactFeatureV1 = "compact-v1"
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
	id      string
	socket  *websocket.Conn
	writeMu sync.Mutex
	alive   bool
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
}

func NewRoomHub(store TeamStore) *RoomHub {
	return &RoomHub{
		store: store,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		conns: map[string]*clientConn{},
		rooms: map[string]*room{},
	}
}

func (h *RoomHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	socket, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn := &clientConn{id: newPeerID(), socket: socket, alive: true}
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
	if teamCode == "" || len(teamCode) > 16 {
		conn.sendJSON(teamUnavailablePayload(teamCode))
		_ = conn.close()
		return
	}
	team, err := h.store.GetTeamByCode(ctx, teamCode)
	if err != nil {
		conn.sendJSON(serverError("Could not load team."))
		_ = conn.close()
		return
	}
	if team == nil {
		conn.sendJSON(teamUnavailablePayload(teamCode))
		_ = conn.close()
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

	h.mu.Lock()
	currentRoom := h.rooms[team.Code]
	if currentRoom == nil {
		currentRoom = &room{team: *team, peers: map[string]*peer{}}
		h.rooms[team.Code] = currentRoom
	}
	if len(currentRoom.peers) >= maxPeersPerRoom {
		h.mu.Unlock()
		conn.sendJSON(roomFullPayload())
		_ = conn.close()
		return
	}
	currentRoom.peers[p.id] = p
	activeCount := len(currentRoom.peers)
	presencePayload, presencePeers := presenceLocked(currentRoom)
	h.mu.Unlock()

	conn.sendJSON(map[string]any{
		"type":        "welcome",
		"peerId":      p.id,
		"team":        team,
		"activeCount": activeCount,
	})
	sendToPeers(presencePeers, presencePayload)
}

func (h *RoomHub) updatePeerProfile(peerID string, nickname string) {
	var payload any
	var peers []*peer
	h.mu.Lock()
	for _, room := range h.rooms {
		if p := room.peers[peerID]; p != nil {
			p.nickname = nickname
			p.lastSeen = time.Now().UnixMilli()
			payload, peers = presenceLocked(room)
			break
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
	for code, room := range h.rooms {
		p := room.peers[peerID]
		if p == nil {
			continue
		}
		if teamCode != "" && code != teamCode {
			h.mu.Unlock()
			return
		}
		sender = p
		sender.lastSeen = time.Now().UnixMilli()
		resolvedTeam = room.team.Code
		for _, candidate := range room.peers {
			if candidate.id != peerID {
				recipients = append(recipients, candidate)
			}
		}
		break
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
	for code, room := range h.rooms {
		if room.peers[peerID] == nil {
			continue
		}
		delete(room.peers, peerID)
		if len(room.peers) == 0 {
			delete(h.rooms, code)
		} else {
			payload, peers = presenceLocked(room)
		}
		break
	}
	h.mu.Unlock()
	sendToPeers(peers, payload)
}

func (h *RoomHub) heartbeatLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		var stale []*clientConn
		var live []*clientConn
		h.mu.Lock()
		for _, conn := range h.conns {
			if !conn.alive {
				stale = append(stale, conn)
				delete(h.conns, conn.id)
				continue
			}
			conn.alive = false
			live = append(live, conn)
		}
		h.mu.Unlock()
		for _, conn := range stale {
			h.leave(conn.id)
			_ = conn.close()
		}
		for _, conn := range live {
			conn.writeMu.Lock()
			_ = conn.socket.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			conn.writeMu.Unlock()
		}
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

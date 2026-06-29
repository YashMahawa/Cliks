package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketAllowsMaxLegitimateActivityBatch(t *testing.T) {
	url, closeServer := testWebSocketURL(t)
	defer closeServer()

	sender := dialTestWebSocket(t, url)
	defer sender.Close()
	recipient := dialTestWebSocket(t, url)
	defer recipient.Close()

	writeTestJSON(t, sender, joinMessage("CLIK-LOCAL"))
	readUntilType(t, sender, "welcome")
	writeTestJSON(t, recipient, joinMessage("CLIK-LOCAL"))
	readUntilType(t, recipient, "welcome")

	events := make([]ActivityEvent, 128)
	for i := range events {
		events[i] = ActivityEvent{Kind: "keyboard", OffsetMs: i * 5}
	}
	writeTestJSON(t, sender, map[string]any{
		"type":           "activity_batch",
		"teamCode":       "CLIK-LOCAL",
		"batchStartedAt": time.Now().UnixMilli(),
		"events":         events,
	})

	message := readUntilType(t, recipient, "peer_activity_batch")
	if got := len(message["events"].([]any)); got != 128 {
		t.Fatalf("forwarded events = %d, want 128", got)
	}
}

func TestWebSocketRejectsOversizedPayload(t *testing.T) {
	url, closeServer := testWebSocketURL(t)
	defer closeServer()

	conn := dialTestWebSocket(t, url)
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", maxWebSocketMessageBytes+1))); err != nil {
		t.Fatalf("write oversized payload: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("oversized payload left the connection open")
	}
}

func TestWebSocketRejectsMessageFlood(t *testing.T) {
	url, closeServer := testWebSocketURL(t)
	defer closeServer()

	conn := dialTestWebSocket(t, url)
	defer conn.Close()

	writeTestJSON(t, conn, joinMessage("CLIK-LOCAL"))
	readUntilType(t, conn, "welcome")
	for i := 0; i < maxWebSocketMessagesPerTick; i++ {
		writeTestJSON(t, conn, map[string]any{"type": "profile", "nickname": "Flood"})
	}

	message := readUntilType(t, conn, "error")
	if message["code"] != "message_rate_limited" {
		t.Fatalf("error code = %v, want message_rate_limited", message["code"])
	}
}

func testWebSocketURL(t *testing.T) (string, func()) {
	t.Helper()
	hub := NewRoomHub(NewMemoryTeamStore())
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	return "ws" + strings.TrimPrefix(server.URL, "http"), server.Close
}

func dialTestWebSocket(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func joinMessage(code string) map[string]any {
	return map[string]any{
		"type":     "join",
		"teamCode": code,
		"client": map[string]any{
			"name":     "cliks-test",
			"version":  "test",
			"features": []string{},
		},
	}
}

func writeTestJSON(t *testing.T, conn *websocket.Conn, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal websocket message: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}
}

func readUntilType(t *testing.T, conn *websocket.Conn, target string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	conn.SetReadDeadline(deadline)
	defer conn.SetReadDeadline(time.Time{})
	for time.Now().Before(deadline) {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket message: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(data, &message); err != nil {
			t.Fatalf("decode websocket message %s: %v", data, err)
		}
		if message["type"] == target {
			return message
		}
	}
	t.Fatalf("timed out waiting for websocket message type %q", target)
	return nil
}

package main

import (
	"testing"
	"time"
)

func TestPresenceStatusAndRoomWideWave(t *testing.T) {
	url, closeServer := testWebSocketURL(t)
	defer closeServer()

	sender := dialTestWebSocket(t, url)
	defer sender.Close()
	recipient := dialTestWebSocket(t, url)
	defer recipient.Close()
	secondRecipient := dialTestWebSocket(t, url)
	defer secondRecipient.Close()

	join := joinMessage("CLIK-LOCAL")
	join["nickname"] = "Mira"
	join["status"] = "focus"
	writeTestJSON(t, sender, join)
	senderWelcome := readUntilType(t, sender, "welcome")
	senderID := senderWelcome["peerId"].(string)

	writeTestJSON(t, recipient, joinMessage("CLIK-LOCAL"))
	readUntilType(t, recipient, "welcome")
	writeTestJSON(t, secondRecipient, joinMessage("CLIK-LOCAL"))
	readUntilType(t, secondRecipient, "welcome")

	presence := readUntilType(t, sender, "presence")
	foundStatus := false
	for _, raw := range presence["peers"].([]any) {
		peer := raw.(map[string]any)
		if peer["peerId"] == senderID && peer["status"] == "focus" {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Fatalf("presence did not preserve the sender's focus status: %#v", presence)
	}

	writeTestJSON(t, sender, map[string]any{"type": "reaction", "reaction": "wave", "targetPeerId": "ignored-old-client-target"})
	reaction := readUntilType(t, recipient, "peer_reaction")
	if reaction["reaction"] != "wave" || reaction["peerId"] != senderID {
		t.Fatalf("unexpected room-wide wave: %#v", reaction)
	}
	if _, ok := reaction["targetPeerId"]; ok {
		t.Fatalf("room-wide reaction leaked a target: %#v", reaction)
	}
	if got := readUntilType(t, secondRecipient, "peer_reaction")["reaction"]; got != "wave" {
		t.Fatalf("second teammate did not receive room-wide wave: %v", got)
	}
}

func TestRoomReactionBroadcastAndAllowlist(t *testing.T) {
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

	writeTestJSON(t, sender, map[string]any{"type": "reaction", "reaction": "celebrate"})
	if got := readUntilType(t, recipient, "peer_reaction")["reaction"]; got != "celebrate" {
		t.Fatalf("reaction = %v, want celebrate", got)
	}

	writeTestJSON(t, sender, map[string]any{"type": "reaction", "reaction": "break"})
	if got := readUntilType(t, recipient, "peer_reaction")["reaction"]; got != "break" {
		t.Fatalf("reaction = %v, want break", got)
	}

	writeTestJSON(t, sender, map[string]any{"type": "reaction", "reaction": "arbitrary-untrusted-value"})
	_ = recipient.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, _, err := recipient.ReadMessage(); err == nil {
		t.Fatal("unrecognized reaction was relayed")
	}
}

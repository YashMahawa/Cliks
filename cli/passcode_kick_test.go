package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCLIKickPeerViaAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/teams/CLIK-TEST01/kick" && r.Method == "POST" {
			var input struct {
				TargetPeerID   string `json:"targetPeerId"`
				DeletePassword string `json:"deletePassword"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input.TargetPeerID == "peer_123" && input.DeletePassword == "secret" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
				return
			}
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Could not kick participant."})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := defaultConfig()
	cfg.APIURL = ts.URL

	// Valid kick
	if err := kickPeerViaAPI(cfg, "CLIK-TEST01", "peer_123", "secret"); err != nil {
		t.Fatalf("kickPeerViaAPI failed: %v", err)
	}

	// Invalid kick
	if err := kickPeerViaAPI(cfg, "CLIK-TEST01", "peer_123", "wrongpassword"); err == nil {
		t.Fatalf("kickPeerViaAPI expected error with wrong password, got nil")
	}
}

func TestCLICreateAndJoinPasscode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/teams" && r.Method == "POST" {
			var input struct {
				Name           string `json:"name"`
				DeletePassword string `json:"deletePassword"`
				Passcode       string `json:"passcode"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			hasPasscode := input.Passcode != ""
			_ = json.NewEncoder(w).Encode(map[string]any{
				"team": map[string]any{
					"code":        "CLIK-PASS01",
					"name":        input.Name,
					"hasPasscode": hasPasscode,
				},
			})
			return
		}
		if r.URL.Path == "/api/teams/CLIK-PASS01" && r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"team": map[string]any{
					"code":        "CLIK-PASS01",
					"name":        "Passcode Room",
					"hasPasscode": true,
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := defaultConfig()
	cfg.APIURL = ts.URL

	team, err := createTeamViaAPI(cfg, "Passcode Room", "secret123", "mypasscode")
	if err != nil {
		t.Fatalf("createTeamViaAPI failed: %v", err)
	}
	if !team.HasPasscode {
		t.Fatalf("expected HasPasscode to be true")
	}

	fetched, err := getTeamViaAPI(cfg, "CLIK-PASS01")
	if err != nil {
		t.Fatalf("getTeamViaAPI failed: %v", err)
	}
	if !fetched.HasPasscode {
		t.Fatalf("expected fetched team HasPasscode to be true")
	}
}

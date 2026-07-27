package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIRejectsInvalidTeamCodeFromServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"team":{"code":"CLIK-OK1234\"\r\nMsgBox 1","name":"bad"}}`))
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	if _, err := getTeamViaAPI(cfg, "CLIK-ABC123"); err == nil || !strings.Contains(err.Error(), "invalid team code") {
		t.Fatalf("error = %v, want invalid team code", err)
	}
}

func TestAPIValidatesTeamCodeBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.APIURL = server.URL
	if _, err := getTeamViaAPI(cfg, "../admin"); err == nil {
		t.Fatal("expected invalid team code")
	}
	if requests != 0 {
		t.Fatalf("server received %d request(s), want 0", requests)
	}
}

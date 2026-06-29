package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

const (
	defaultPort        = "8787"
	heartbeatInterval  = 30 * time.Second
	maxRequestBodySize = 1 << 20
)

type apiServer struct {
	store             TeamStore
	hub               *RoomHub
	corsOrigins       map[string]bool
	allowAnyOrigin    bool
	createTeamLimiter *rateLimiter
	deleteTeamLimiter *rateLimiter
	lookupTeamLimiter *rateLimiter
}

func main() {
	store, err := createTeamStoreFromEnv()
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	joinTeamLimiter := newRateLimiter(5*time.Minute, 20)
	srv := &apiServer{
		store:             store,
		hub:               NewRoomHub(store, joinTeamLimiter),
		corsOrigins:       parseCORSOrigins(os.Getenv("CORS_ORIGIN")),
		allowAnyOrigin:    os.Getenv("CORS_ORIGIN") == "",
		createTeamLimiter: newRateLimiter(5*time.Minute, 20),
		deleteTeamLimiter: newRateLimiter(5*time.Minute, 30),
		lookupTeamLimiter: newRateLimiter(5*time.Minute, 40),
	}
	go runSafely("heartbeat loop", func() { srv.hub.heartbeatLoop(heartbeatInterval) })

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.withCORS(srv.handleHealth))
	mux.HandleFunc("/api/teams", srv.withCORS(srv.handleTeams))
	mux.HandleFunc("/api/teams/", srv.withCORS(srv.handleTeamByCode))
	mux.HandleFunc("/ws", srv.handleWebSocket)

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("DO_APP_PORT")
	}
	if port == "" {
		port = defaultPort
	}

	httpServer := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("cliks server listening on :%s", port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"totalRooms": s.hub.TotalRooms(),
		"totalPeers": s.hub.TotalPeers(),
	})
}

func (s *apiServer) handleTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.createTeamLimiter.Allow(rateLimitKey(r)) {
		writeError(w, http.StatusTooManyRequests, "Too many team creation requests. Please wait a moment and try again.")
		return
	}

	var input struct {
		Name           string `json:"name"`
		DeletePassword string `json:"deletePassword"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Please provide a team name and a delete password.")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if len([]rune(input.Name)) < 2 || len([]rune(input.Name)) > 80 || len(input.DeletePassword) < 6 || len(input.DeletePassword) > 128 {
		writeError(w, http.StatusBadRequest, "Please provide a team name and a delete password.")
		return
	}

	team, err := s.store.CreateTeam(r.Context(), CreateTeamInput{Name: input.Name, DeletePassword: input.DeletePassword})
	if err != nil {
		log.Printf("create team: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not create team.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]Team{"team": team})
}

func (s *apiServer) handleTeamByCode(w http.ResponseWriter, r *http.Request) {
	code := normalizeTeamCode(strings.TrimPrefix(r.URL.Path, "/api/teams/"))
	if code == "" || len(code) > 16 {
		writeError(w, http.StatusNotFound, "Team not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if s.lookupTeamLimiter != nil && !s.lookupTeamLimiter.Allow(rateLimitKey(r)) {
			writeError(w, http.StatusTooManyRequests, "Too many team lookup requests. Please wait a moment and try again.")
			return
		}
		team, err := s.store.GetTeamByCode(r.Context(), code)
		if err != nil {
			log.Printf("get team: %v", err)
			writeError(w, http.StatusInternalServerError, "Could not load team.")
			return
		}
		if team == nil {
			writeError(w, http.StatusNotFound, "Team not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]Team{"team": *team})
	case http.MethodDelete:
		if !s.deleteTeamLimiter.Allow(rateLimitKey(r)) {
			writeError(w, http.StatusTooManyRequests, "Too many delete attempts. Please wait a moment and try again.")
			return
		}
		var input struct {
			DeletePassword string `json:"deletePassword"`
		}
		if err := readJSON(r, &input); err != nil || input.DeletePassword == "" || len(input.DeletePassword) > 128 {
			writeError(w, http.StatusBadRequest, "Invalid delete request.")
			return
		}
		deleted, err := s.store.DeleteTeam(r.Context(), DeleteTeamInput{Code: code, DeletePassword: input.DeletePassword})
		if err != nil {
			log.Printf("delete team: %v", err)
			writeError(w, http.StatusInternalServerError, "Could not delete that team.")
			return
		}
		if !deleted {
			writeError(w, http.StatusForbidden, "Could not delete that team.")
			return
		}
		s.hub.CloseRoom(code, "This team was deleted.")
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *apiServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.applyCORS(w, r)
	s.hub.HandleWebSocket(w, r)
}

func (s *apiServer) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.applyCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *apiServer) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if s.allowAnyOrigin || s.corsOrigins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}
}

func parseCORSOrigins(value string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = true
		}
	}
	return result
}

func readJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func rateLimitKey(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func normalizeTeamCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeNickname(value string) string {
	value = ansi.Strip(value)
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	runes := []rune(value)
	if len(runes) > 10 {
		value = string(runes[:10])
	}
	return value
}

func boolFeature(features []string, target string) bool {
	for _, feature := range features {
		if feature == target {
			return true
		}
	}
	return false
}

func boundedInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func quantizeOffset(offsetMs int) int {
	return boundedInt(((offsetMs+timingBucketMs/2)/timingBucketMs)*timingBucketMs, 0, 2000)
}

func serverError(message string) map[string]string {
	return map[string]string{"type": "error", "message": message}
}

func protocolError(code string, message string) map[string]string {
	return map[string]string{"type": "error", "code": code, "message": message}
}

func teamUnavailablePayload(code string) map[string]string {
	return map[string]string{
		"type":     "team_unavailable",
		"teamCode": code,
		"reason":   "not_found",
		"message":  "Team code was not found or was deleted.",
	}
}

func roomFullPayload() map[string]string {
	return protocolError("room_full", "This room is full. Cliks rooms are capped at 20 people.")
}

func joinRateLimitedPayload() map[string]string {
	return protocolError("join_rate_limited", "Too many team join attempts. Wait a few minutes and try again.")
}

func deletedPayload(code string, message string) map[string]string {
	return map[string]string{"type": "team_deleted", "teamCode": code, "message": message}
}

func compactActivityPayload(peerID string, nickname string, batchStartedAt int64, events []ActivityEvent) map[string]any {
	compactEvents := make([][]any, 0, len(events))
	for _, event := range events {
		if event.Kind == "mouse" {
			button := "u"
			switch event.Button {
			case "left":
				button = "l"
			case "right":
				button = "r"
			}
			compactEvents = append(compactEvents, []any{"m", event.OffsetMs, button})
		} else {
			compactEvents = append(compactEvents, []any{"k", event.OffsetMs})
		}
	}
	payload := map[string]any{
		"type": "a",
		"p":    peerID,
		"t":    batchStartedAt,
		"e":    compactEvents,
	}
	if nickname != "" {
		payload["n"] = nickname
	}
	return payload
}

func verboseActivityPayload(teamCode string, peerID string, nickname string, batchStartedAt int64, events []ActivityEvent) map[string]any {
	payload := map[string]any{
		"type":           "peer_activity_batch",
		"teamCode":       teamCode,
		"peerId":         peerID,
		"batchStartedAt": batchStartedAt,
		"events":         events,
	}
	if nickname != "" {
		payload["nickname"] = nickname
	}
	return payload
}

func formatHTTPError(status int, body []byte) error {
	return fmt.Errorf("server returned %d: %s", status, strings.TrimSpace(string(body)))
}

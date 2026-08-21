package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeNicknameStripsTerminalSequencesBeforeTruncating(t *testing.T) {
	input := "\x1b[31mAlice\x1b[0m\x1b]0;owned\x07 Long Name"
	if got := normalizeNickname(input); got != "Alice Long" {
		t.Fatalf("nickname = %q, want Alice Long", got)
	}
}

func TestNormalizeNicknameRemovesControlAndFormatCharacters(t *testing.T) {
	if got := normalizeNickname("Ali\x00ce\u202e Bob"); got != "Alice Bob" {
		t.Fatalf("nickname = %q, want Alice Bob", got)
	}
}

func TestNormalizeStatusTextStripsTerminalSequencesAndTruncates(t *testing.T) {
	input := "\x1b[31mReviewing\x1b[0m\x1b]0;hack\x07 PR #104 with \x00control \u202efiltered text and a very long note that exceeds sixty characters in total length"
	got := normalizeStatusText(input)
	want := "Reviewing PR #104 with control filtered text and a very long"
	if got != want {
		t.Fatalf("statusText = %q (len %d), want %q (len %d)", got, len([]rune(got)), want, len([]rune(want)))
	}
}

func TestTeamLookupRateLimit(t *testing.T) {
	srv := &apiServer{
		store:             NewMemoryTeamStore(),
		lookupTeamLimiter: newRateLimiter(time.Minute, 2),
	}
	for i, want := range []int{http.StatusNotFound, http.StatusNotFound, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodGet, "/api/teams/CLIK-MISSING", nil)
		req.RemoteAddr = "198.51.100.1:1234"
		res := httptest.NewRecorder()
		srv.handleTeamByCode(res, req)
		if res.Code != want {
			t.Fatalf("lookup %d status = %d, want %d", i+1, res.Code, want)
		}
	}
}

//go:build linux

package main

import (
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func mockCaptureSocket(t *testing.T, handshake string) string {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "capture.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				_, _ = io.WriteString(c, handshake)
				// keep connection open briefly if needed
				time.Sleep(100 * time.Millisecond)
			}(conn)
		}
	}()
	return socketPath
}

func TestProbeLinuxCaptureHelper(t *testing.T) {
	tests := []struct {
		handshake string
		wantReady bool
		wantSeat  string
	}{
		{"ready\n", true, "active"},
		{"ready seat:active\n", true, "active"},
		{"ready seat:fallback\n", true, "fallback"},
		{"ready seat:muted\n", true, "muted"},
		{"invalid response\n", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.handshake, func(t *testing.T) {
			sock := mockCaptureSocket(t, tc.handshake)
			t.Setenv("CLIKS_CAPTURE_SOCKET", sock)

			hs := probeLinuxCaptureHelper()
			if hs.ready != tc.wantReady {
				t.Fatalf("probe.ready = %v, want %v", hs.ready, tc.wantReady)
			}
			if tc.wantReady && hs.seat != tc.wantSeat {
				t.Fatalf("probe.seat = %q, want %q", hs.seat, tc.wantSeat)
			}
		})
	}
}

func TestAppendPlatformCaptureChecksMutedStatus(t *testing.T) {
	sock := mockCaptureSocket(t, "ready seat:muted\n")
	t.Setenv("CLIKS_CAPTURE_SOCKET", sock)

	report := doctorReport{}
	appendPlatformCaptureChecks(&report, true)

	foundMutedCheck := false
	for _, check := range report.checks {
		if check.label == "Isolated capture helper" && check.status == "muted (seat ownership check failed)" {
			foundMutedCheck = true
			break
		}
	}
	if !foundMutedCheck {
		t.Fatalf("appendPlatformCaptureChecks did not report muted helper status, checks = %+v", report.checks)
	}

	foundMutedIssue := false
	for _, issue := range report.issues {
		if issue.title == "Activate active user session or check seat status" {
			foundMutedIssue = true
			break
		}
	}
	if !foundMutedIssue {
		t.Fatalf("appendPlatformCaptureChecks did not report muted capture issue, issues = %+v", report.issues)
	}
}

func TestAppendPlatformCaptureChecksFallbackStatus(t *testing.T) {
	sock := mockCaptureSocket(t, "ready seat:fallback\n")
	t.Setenv("CLIKS_CAPTURE_SOCKET", sock)

	report := doctorReport{}
	appendPlatformCaptureChecks(&report, true)

	foundFallbackCheck := false
	for _, check := range report.checks {
		if check.label == "Isolated capture helper" && check.status == "yes (fallback owner verification)" {
			foundFallbackCheck = true
			break
		}
	}
	if !foundFallbackCheck {
		t.Fatalf("appendPlatformCaptureChecks did not report fallback helper status, checks = %+v", report.checks)
	}
}

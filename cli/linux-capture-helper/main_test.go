//go:build linux

package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func testUnixConnection(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()
	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := <-accepted
	if server == nil {
		t.Fatal("listener did not accept the test connection")
	}
	return client, server
}

func configureCurrentProcess(t *testing.T) {
	t.Helper()
	executable, err := os.Readlink("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLIKS_CAPTURE_UID", strconv.Itoa(os.Getuid()))
	t.Setenv("CLIKS_CAPTURE_CLIENT_EXE", executable)
}

func TestVerifyClientPinsConnectingProcessIdentity(t *testing.T) {
	configureCurrentProcess(t)
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()
	verified, ok := verifyClient(serverConn)
	if !ok || verified == nil {
		t.Fatal("current executable was not verified")
	}
	defer verified.close()
	if !pidfdAlive(verified.pidfd) {
		t.Fatal("verified client pidfd is not live")
	}
}

func TestVerifyClientRejectsUnexpectedExecutable(t *testing.T) {
	t.Setenv("CLIKS_CAPTURE_UID", strconv.Itoa(os.Getuid()))
	t.Setenv("CLIKS_CAPTURE_CLIENT_EXE", "/not/the/cliks/executable")
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()
	defer serverConn.Close()
	if verified, ok := verifyClient(serverConn); ok || verified != nil {
		t.Fatal("unexpected executable was accepted")
	}
}

func TestVerifiedClientIsRemovedWhenSocketCloses(t *testing.T) {
	configureCurrentProcess(t)
	clientConn, serverConn := testUnixConnection(t)
	verified, ok := verifyClient(serverConn)
	if !ok {
		t.Fatal("current executable was not verified")
	}
	peers := &clients{items: map[*verifiedClient]struct{}{}}
	peers.add(verified)
	_ = clientConn.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		peers.Lock()
		remaining := len(peers.items)
		peers.Unlock()
		if remaining == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("closed client socket remained authorized")
}

func TestCheckSeatStatusFallbackWhenLoginctlMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	st := checkSeatStatus(1000)
	if st != seatStatusFallback {
		t.Fatalf("checkSeatStatus = %q, want %q", st, seatStatusFallback)
	}
	gate := &activeSeatGate{targetUID: 1000}
	if !gate.allowed() {
		t.Fatal("gate.allowed() = false when seat status is fallback")
	}
}

func TestClientsSendDeliversEventsWhenSeatGateFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	configureCurrentProcess(t)
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()

	verified, ok := verifyClient(serverConn)
	if !ok {
		t.Fatal("current executable was not verified")
	}
	gate := &activeSeatGate{targetUID: os.Getuid()}
	peers := &clients{items: map[*verifiedClient]struct{}{}, seatGate: gate}
	peers.add(verified)

	peers.send("k")

	buf := make([]byte, 16)
	_ = clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read streamed token: %v", err)
	}
	if string(buf[:n]) != "k\n" {
		t.Fatalf("streamed token = %q, want %q", string(buf[:n]), "k\n")
	}
}

func TestActiveSeatGateStatusAllowed(t *testing.T) {
	gateActive := &activeSeatGate{status: seatStatusActive, checkedAt: time.Now()}
	if !gateActive.allowed() {
		t.Fatal("active status should be allowed")
	}
	gateFallback := &activeSeatGate{status: seatStatusFallback, checkedAt: time.Now()}
	if !gateFallback.allowed() {
		t.Fatal("fallback status should be allowed")
	}
	gateMuted := &activeSeatGate{status: seatStatusMuted, checkedAt: time.Now()}
	if gateMuted.allowed() {
		t.Fatal("muted status should not be allowed")
	}
}

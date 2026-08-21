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

func TestVerifyClientRejectsMismatchedUID(t *testing.T) {
	executable, err := os.Readlink("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLIKS_CAPTURE_UID", strconv.Itoa(os.Getuid()+999))
	t.Setenv("CLIKS_CAPTURE_CLIENT_EXE", executable)
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()
	defer serverConn.Close()
	if verified, ok := verifyClient(serverConn); ok || verified != nil {
		t.Fatal("mismatched UID was accepted")
	}
}

func TestActiveSeatGateBlocksTokensWhenInactive(t *testing.T) {
	configureCurrentProcess(t)
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()
	verified, ok := verifyClient(serverConn)
	if !ok {
		t.Fatal("current executable was not verified")
	}
	defer verified.close()

	inactiveSeat := &activeSeatGate{targetUID: os.Getuid(), active: false, checkedAt: time.Now()}
	peers := &clients{items: map[*verifiedClient]struct{}{}, seatGate: inactiveSeat}
	peers.add(verified)

	peers.send("k")

	_ = clientConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 32)
	n, _ := clientConn.Read(buf)
	if n > 0 {
		t.Fatalf("expected no token transmission when active seat gate is false, got %q", string(buf[:n]))
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

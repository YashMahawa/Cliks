//go:build linux

package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	t.Setenv("CLIKS_CAPTURE_UID", strconv.Itoa(os.Getuid()))
}

func TestVerifyClientPinsConnectingProcessIdentity(t *testing.T) {
	configureCurrentProcess(t)
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()
	verified, ok := verifyClient(serverConn)
	if !ok || verified == nil {
		t.Fatal("current process was not verified")
	}
	defer verified.close()
	if !pidfdAlive(verified.pidfd) {
		t.Fatal("verified client pidfd is not live")
	}
}

func TestVerifyClientIgnoresExecutableEnvVar(t *testing.T) {
	configureCurrentProcess(t)
	t.Setenv("CLIKS_CAPTURE_CLIENT_EXE", "/not/a/real/executable/path")
	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()
	verified, ok := verifyClient(serverConn)
	if !ok || verified == nil {
		t.Fatal("client verification failed when CLIKS_CAPTURE_CLIENT_EXE environment variable was set")
	}
	defer verified.close()
}

func TestEphemeralTokenGenerationAndStorage(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	peers := &clients{items: map[*verifiedClient]struct{}{}}

	tm, err := newTokenManager(tokenPath, os.Getuid(), peers)
	if err != nil {
		t.Fatalf("newTokenManager failed: %v", err)
	}

	if tm.token == "" {
		t.Fatal("generated token is empty")
	}

	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("stat token file failed: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected token file permissions 0600, got %o", perm)
	}

	content, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read token file failed: %v", err)
	}

	if strings.TrimSpace(string(content)) != tm.token {
		t.Fatalf("token file content %q does not match active token %q", string(content), tm.token)
	}
}

func TestTokenHandshakeSuccessAndRejection(t *testing.T) {
	configureCurrentProcess(t)
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	peers := &clients{items: map[*verifiedClient]struct{}{}}

	tm, err := newTokenManager(tokenPath, os.Getuid(), peers)
	if err != nil {
		t.Fatalf("newTokenManager failed: %v", err)
	}

	// 1. Success case: present valid token
	{
		clientConn, serverConn := testUnixConnection(t)
		defer clientConn.Close()

		verified, ok := verifyClient(serverConn)
		if !ok {
			t.Fatal("verifyClient failed")
		}

		go func() {
			_ = serverConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			presentedToken, err := readTokenLine(serverConn)
			if err != nil || !tm.ValidToken(presentedToken) {
				_ = serverConn.Close()
				return
			}
			_, _ = io.WriteString(serverConn, "ready\n")
			peers.add(verified)
		}()

		_, _ = io.WriteString(clientConn, tm.token+"\n")
		reader := bufio.NewReader(clientConn)
		resp, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(resp) != "ready" {
			t.Fatalf("expected ready response, got %q, err: %v", resp, err)
		}
	}

	// 2. Failure case: present invalid token
	{
		clientConn, serverConn := testUnixConnection(t)
		defer clientConn.Close()

		verified, ok := verifyClient(serverConn)
		if !ok {
			t.Fatal("verifyClient failed")
		}

		go func() {
			_ = serverConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			presentedToken, err := readTokenLine(serverConn)
			if err != nil || !tm.ValidToken(presentedToken) {
				_ = serverConn.Close()
				if verified.pidfd >= 0 {
					verified.close()
				}
				return
			}
			_, _ = io.WriteString(serverConn, "ready\n")
		}()

		_, _ = io.WriteString(clientConn, "invalid-token-secret\n")
		reader := bufio.NewReader(clientConn)
		_, err := reader.ReadString('\n')
		if err == nil {
			t.Fatal("expected connection rejection for invalid token, but got response")
		}
	}
}

func TestTokenRegenerationInvalidatesConnectedClients(t *testing.T) {
	configureCurrentProcess(t)
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	peers := &clients{items: map[*verifiedClient]struct{}{}}

	tm, err := newTokenManager(tokenPath, os.Getuid(), peers)
	if err != nil {
		t.Fatalf("newTokenManager failed: %v", err)
	}

	oldToken := tm.token

	clientConn, serverConn := testUnixConnection(t)
	defer clientConn.Close()

	verified, ok := verifyClient(serverConn)
	if !ok {
		t.Fatal("verifyClient failed")
	}
	peers.add(verified)

	if err := tm.Regenerate(); err != nil {
		t.Fatalf("Regenerate failed: %v", err)
	}

	if tm.token == oldToken {
		t.Fatal("token did not change after regeneration")
	}

	if tm.ValidToken(oldToken) {
		t.Fatal("old token is still considered valid after regeneration")
	}

	// Check that connected peer was closed upon token regeneration
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		peers.Lock()
		count := len(peers.items)
		peers.Unlock()
		if count == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("connected client was not evicted after token regeneration")
}

func TestVerifiedClientIsRemovedWhenSocketCloses(t *testing.T) {
	configureCurrentProcess(t)
	clientConn, serverConn := testUnixConnection(t)
	verified, ok := verifyClient(serverConn)
	if !ok {
		t.Fatal("current process was not verified")
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

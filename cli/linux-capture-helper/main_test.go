//go:build linux

package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
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

func TestGetDeviceSeatDefaultsToSeat0(t *testing.T) {
	// Test untagged path defaults to seat0
	seat := getDeviceSeat("/non/existent/dev/path")
	if seat != "seat0" {
		t.Fatalf("expected default seat0, got %q", seat)
	}
}

func TestDeviceScannerFiltersDevicesByActiveSeat(t *testing.T) {
	tmpDir := t.TempDir()
	dev0 := filepath.Join(tmpDir, "event0")
	dev1 := filepath.Join(tmpDir, "event1")
	dev2 := filepath.Join(tmpDir, "event2")

	for _, p := range []string{dev0, dev1, dev2} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	oldGetSeat := getDeviceSeatOverride
	oldGetActiveSeats := getActiveSeatsOverride
	defer func() {
		getDeviceSeatOverride = oldGetSeat
		getActiveSeatsOverride = oldGetActiveSeats
	}()

	getActiveSeatsOverride = func(targetUID int) map[string]bool {
		return map[string]bool{"seat0": true}
	}

	getDeviceSeatOverride = func(path string) string {
		switch path {
		case dev0:
			return "seat0"
		case dev1:
			return "seat1"
		case dev2:
			return "" // untagged -> defaults to seat0 evaluation
		default:
			return "seat0"
		}
	}

	openedPaths := make(map[string]bool)
	var openedMu sync.Mutex

	scanner := &deviceScanner{
		opened: make(map[string]*os.File),
		opener: func(path string) (*os.File, error) {
			openedMu.Lock()
			openedPaths[path] = true
			openedMu.Unlock()
			return os.Open(path)
		},
	}

	peers := &clients{items: map[*verifiedClient]struct{}{}}
	scanner.scan(1000, peers, filepath.Join(tmpDir, "event*"))

	openedMu.Lock()
	defer openedMu.Unlock()

	if !openedPaths[dev0] {
		t.Errorf("expected dev0 (seat0) to be opened")
	}
	if openedPaths[dev1] {
		t.Errorf("expected dev1 (seat1) to be excluded before opening handle")
	}
	if !openedPaths[dev2] {
		t.Errorf("expected dev2 (untagged -> seat0) to be opened")
	}
}

func TestDeviceScannerReevaluatesSeatsMidSession(t *testing.T) {
	tmpDir := t.TempDir()
	dev0 := filepath.Join(tmpDir, "event0")
	dev1 := filepath.Join(tmpDir, "event1")

	for _, p := range []string{dev0, dev1} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	oldGetSeat := getDeviceSeatOverride
	oldGetActiveSeats := getActiveSeatsOverride
	defer func() {
		getDeviceSeatOverride = oldGetSeat
		getActiveSeatsOverride = oldGetActiveSeats
	}()

	activeSeats := map[string]bool{"seat0": true}
	getActiveSeatsOverride = func(targetUID int) map[string]bool {
		return activeSeats
	}

	getDeviceSeatOverride = func(path string) string {
		if path == dev1 {
			return "seat1"
		}
		return "seat0"
	}

	scanner := newDeviceScanner()
	peers := &clients{items: map[*verifiedClient]struct{}{}}

	// Pass 1: only seat0 is active
	scanner.scan(1000, peers, filepath.Join(tmpDir, "event*"))

	scanner.mu.Lock()
	if _, dev0Open := scanner.opened[dev0]; !dev0Open {
		t.Errorf("Pass 1: expected dev0 to be open")
	}
	if _, dev1Open := scanner.opened[dev1]; dev1Open {
		t.Errorf("Pass 1: expected dev1 (seat1) to be excluded")
	}
	scanner.mu.Unlock()

	// Pass 2: user attaches/switches to seat1 as well (both seat0 and seat1 active)
	activeSeats = map[string]bool{"seat0": true, "seat1": true}
	scanner.scan(1000, peers, filepath.Join(tmpDir, "event*"))

	scanner.mu.Lock()
	if _, dev0Open := scanner.opened[dev0]; !dev0Open {
		t.Errorf("Pass 2: expected dev0 to remain open")
	}
	if _, dev1Open := scanner.opened[dev1]; !dev1Open {
		t.Errorf("Pass 2: expected dev1 to be opened upon seat activation")
	}
	scanner.mu.Unlock()

	// Pass 3: user leaves seat1 (only seat0 active again)
	activeSeats = map[string]bool{"seat0": true}
	scanner.scan(1000, peers, filepath.Join(tmpDir, "event*"))

	scanner.mu.Lock()
	if _, dev0Open := scanner.opened[dev0]; !dev0Open {
		t.Errorf("Pass 3: expected dev0 to remain open")
	}
	if _, dev1Open := scanner.opened[dev1]; dev1Open {
		t.Errorf("Pass 3: expected dev1 to be closed when seat1 is no longer active")
	}
	scanner.mu.Unlock()
}

func TestActiveSeatGateAndZeroActivityOnInactiveSeats(t *testing.T) {
	oldGetActiveSeats := getActiveSeatsOverride
	defer func() { getActiveSeatsOverride = oldGetActiveSeats }()

	var targetActive bool
	getActiveSeatsOverride = func(targetUID int) map[string]bool {
		if targetActive {
			return map[string]bool{"seat0": true}
		}
		return map[string]bool{}
	}

	gate := &activeSeatGate{targetUID: 1000}
	peers := &clients{items: map[*verifiedClient]struct{}{}, seatGate: gate}
	_ = peers

	targetActive = false
	if gate.allowed() {
		t.Fatalf("expected gate to deny when target UID has no active seat")
	}

	targetActive = true
	gate.checkedAt = time.Time{} // reset cache
	if !gate.allowed() {
		t.Fatalf("expected gate to allow when target UID has active seat")
	}
}


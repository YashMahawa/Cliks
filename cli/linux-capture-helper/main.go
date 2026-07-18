//go:build linux

package main

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	evKey    = 1
	btnLeft  = 0x110
	btnRight = 0x111
)

type clients struct {
	sync.Mutex
	items    map[net.Conn]struct{}
	seatGate *activeSeatGate
}

func (c *clients) send(token string) {
	if c.seatGate != nil && !c.seatGate.allowed() {
		return
	}
	c.Lock()
	defer c.Unlock()
	for conn := range c.items {
		_ = conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if _, err := io.WriteString(conn, token+"\n"); err != nil {
			_ = conn.Close()
			delete(c.items, conn)
		}
	}
}

func main() {
	socket := "/run/cliks/capture.sock"
	if value := os.Getenv("CLIKS_CAPTURE_SOCKET"); value != "" {
		socket = value
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		log.Fatal(err)
	}
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	targetUID, err := configuredUID()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chown(socket, targetUID, -1); err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		log.Fatal(err)
	}
	peers := &clients{items: map[net.Conn]struct{}{}, seatGate: &activeSeatGate{targetUID: targetUID}}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if !allowedClient(conn) {
				_ = conn.Close()
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			if _, err := io.WriteString(conn, "ready\n"); err != nil {
				_ = conn.Close()
				continue
			}
			peers.Lock()
			peers.items[conn] = struct{}{}
			peers.Unlock()
		}
	}()
	opened := map[string]bool{}
	var openedMu sync.Mutex
	for {
		devices, _ := filepath.Glob("/dev/input/event*")
		for _, path := range devices {
			openedMu.Lock()
			alreadyOpen := opened[path]
			openedMu.Unlock()
			if alreadyOpen {
				continue
			}
			file, err := os.Open(path)
			if err != nil {
				continue
			}
			openedMu.Lock()
			opened[path] = true
			openedMu.Unlock()
			go readDevice(file, peers, func() { openedMu.Lock(); delete(opened, path); openedMu.Unlock() })
		}
		time.Sleep(3 * time.Second)
	}
}

func allowedClient(conn net.Conn) bool {
	want, err := configuredUID()
	if err != nil {
		return false
	}
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return false
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return false
	}
	var credential *unix.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		credential, socketErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil || socketErr != nil || credential == nil {
		return false
	}
	if int(credential.Uid) != want {
		return false
	}
	wantExecutable := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_CLIENT_EXE"))
	if wantExecutable == "" || credential.Pid <= 0 {
		return false
	}
	gotExecutable, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(int(credential.Pid)), "exe"))
	if err != nil {
		return false
	}
	// The installer stores a canonical path. Do not resolve it again here:
	// ProtectHome=true intentionally prevents this root service from traversing
	// the user's home, while /proc/PID/exe still exposes the canonical target.
	return filepath.Clean(gotExecutable) == filepath.Clean(wantExecutable)
}

func configuredUID() (int, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_UID")), 10, 32)
	return int(value), err
}

type activeSeatGate struct {
	sync.Mutex
	targetUID int
	checkedAt time.Time
	active    bool
}

func (g *activeSeatGate) allowed() bool {
	g.Lock()
	defer g.Unlock()
	if time.Since(g.checkedAt) < 1500*time.Millisecond {
		return g.active
	}
	g.checkedAt = time.Now()
	g.active = targetOwnsActiveSeat(g.targetUID)
	return g.active
}

func targetOwnsActiveSeat(targetUID int) bool {
	output, err := exec.Command("loginctl", "list-seats", "--no-legend", "--no-pager").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		session, err := exec.Command("loginctl", "show-seat", fields[0], "-p", "ActiveSession", "--value").Output()
		if err != nil || strings.TrimSpace(string(session)) == "" {
			continue
		}
		uid, err := exec.Command("loginctl", "show-session", strings.TrimSpace(string(session)), "-p", "User", "--value").Output()
		if err == nil && strings.TrimSpace(string(uid)) == strconv.Itoa(targetUID) {
			return true
		}
	}
	return false
}

func readDevice(file *os.File, peers *clients, done func()) {
	defer file.Close()
	defer done()
	buf := make([]byte, 24*32)
	for {
		n, err := file.Read(buf)
		if err != nil {
			return
		}
		eventSize := 24
		if n%24 != 0 && n%16 == 0 {
			eventSize = 16
		}
		for offset := 0; offset+eventSize <= n; offset += eventSize {
			typeValue := binary.LittleEndian.Uint16(buf[offset+eventSize-8:])
			code := binary.LittleEndian.Uint16(buf[offset+eventSize-6:])
			value := int32(binary.LittleEndian.Uint32(buf[offset+eventSize-4:]))
			if typeValue != evKey || value != 1 {
				continue
			}
			switch code {
			case btnLeft:
				peers.send("l")
			case btnRight:
				peers.send("r")
			default:
				// Mouse/touch buttons outside the explicit allowlist are ignored.
				if code < 0x100 {
					peers.send("k")
				}
			}
		}
	}
}

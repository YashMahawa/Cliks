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
	items    map[*verifiedClient]struct{}
	seatGate *activeSeatGate
}

type verifiedClient struct {
	conn      net.Conn
	pidfd     int
	done      chan struct{}
	closeOnce sync.Once
}

func (c *verifiedClient) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
		if c.pidfd >= 0 {
			_ = unix.Close(c.pidfd)
		}
	})
}

func (c *clients) remove(client *verifiedClient) {
	c.Lock()
	if _, ok := c.items[client]; ok {
		delete(c.items, client)
		client.close()
	}
	c.Unlock()
}

func (c *clients) add(client *verifiedClient) {
	c.Lock()
	c.items[client] = struct{}{}
	c.Unlock()
	go func() {
		for {
			poll := []unix.PollFd{{Fd: int32(client.pidfd), Events: unix.POLLIN}}
			count, _ := unix.Poll(poll, 1000)
			select {
			case <-client.done:
				return
			default:
			}
			if count > 0 {
				c.remove(client)
				return
			}
		}
	}()
	go func() {
		var unexpected [1]byte
		_, _ = client.conn.Read(unexpected[:])
		c.remove(client)
	}()
}

func (c *clients) send(token string) {
	if c.seatGate != nil && !c.seatGate.allowed() {
		return
	}
	c.Lock()
	defer c.Unlock()
	for client := range c.items {
		_ = client.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if _, err := io.WriteString(client.conn, token+"\n"); err != nil {
			delete(c.items, client)
			client.close()
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
	peers := &clients{items: map[*verifiedClient]struct{}{}, seatGate: &activeSeatGate{targetUID: targetUID}}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			client, ok := verifyClient(conn)
			if !ok {
				_ = conn.Close()
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			if _, err := io.WriteString(conn, "ready\n"); err != nil {
				_ = conn.Close()
				continue
			}
			peers.add(client)
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

func verifyClient(conn net.Conn) (*verifiedClient, bool) {
	want, err := configuredUID()
	if err != nil {
		return nil, false
	}
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, false
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return nil, false
	}
	var credential *unix.Ucred
	pidfd := -1
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		credential, socketErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if socketErr == nil {
			// Linux 6.5+ returns a race-free reference to the exact process that
			// connected. Older kernels fall back to pidfd_open below.
			pidfd, _ = unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_PEERPIDFD)
		}
	}); err != nil || socketErr != nil || credential == nil {
		return nil, false
	}
	if int(credential.Uid) != want {
		if pidfd >= 0 {
			_ = unix.Close(pidfd)
		}
		return nil, false
	}
	wantExecutable := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_CLIENT_EXE"))
	if wantExecutable == "" || credential.Pid <= 0 {
		if pidfd >= 0 {
			_ = unix.Close(pidfd)
		}
		return nil, false
	}
	if pidfd < 0 {
		pidfd, err = unix.PidfdOpen(int(credential.Pid), 0)
		if err != nil {
			return nil, false
		}
	}
	unix.CloseOnExec(pidfd)
	if !pidfdAlive(pidfd) {
		_ = unix.Close(pidfd)
		return nil, false
	}
	gotExecutable, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(int(credential.Pid)), "exe"))
	if err != nil {
		_ = unix.Close(pidfd)
		return nil, false
	}
	// The installer stores a canonical path. Do not resolve it again here:
	// ProtectHome=true intentionally prevents this root service from traversing
	// the user's home, while /proc/PID/exe still exposes the canonical target.
	if filepath.Clean(gotExecutable) != filepath.Clean(wantExecutable) || !pidfdAlive(pidfd) {
		_ = unix.Close(pidfd)
		return nil, false
	}
	return &verifiedClient{conn: conn, pidfd: pidfd, done: make(chan struct{})}, true
}

func pidfdAlive(pidfd int) bool {
	return pidfd >= 0 && unix.PidfdSendSignal(pidfd, 0, nil, 0) == nil
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
		return true
	}
	lines := strings.Split(string(output), "\n")
	hasSeats := false
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		hasSeats = true
		session, err := exec.Command("loginctl", "show-seat", fields[0], "-p", "ActiveSession", "--value").Output()
		if err != nil || strings.TrimSpace(string(session)) == "" {
			continue
		}
		uid, err := exec.Command("loginctl", "show-session", strings.TrimSpace(string(session)), "-p", "User", "--value").Output()
		if err == nil && strings.TrimSpace(string(uid)) == strconv.Itoa(targetUID) {
			return true
		}
	}
	if !hasSeats {
		return true
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

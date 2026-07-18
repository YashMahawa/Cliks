//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func prepareAmbientCommand(cmd *exec.Cmd) {
	// If Cliks is killed before its context can cancel playback, Linux also
	// terminates the room-tone player when its direct parent disappears.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

func cleanupOrphanAmbientPlayers() int {
	root, err := os.UserCacheDir()
	if err != nil {
		return 0
	}
	marker := filepath.Clean(filepath.Join(root, "cliks", "ambient-v2")) + string(os.PathSeparator)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	stopped := 0
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || pid == os.Getpid() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		args := splitNullArgs(data)
		if !isCliksAmbientPlayer(args, marker) {
			continue
		}
		if syscall.Kill(pid, syscall.SIGTERM) == nil {
			stopped++
		}
	}
	return stopped
}

func isCliksAmbientPlayer(args []string, marker string) bool {
	player := false
	for i, arg := range args {
		if i < 2 {
			switch strings.ToLower(filepath.Base(arg)) {
			case "mpv", "ffplay", "paplay", "pw-play", "aplay", "termux-media-player":
				player = true
			}
		}
	}
	if !player {
		return false
	}
	for _, arg := range args {
		path := filepath.Clean(arg)
		if strings.HasPrefix(path, marker) && strings.EqualFold(filepath.Ext(path), ".wav") {
			return true
		}
	}
	return false
}

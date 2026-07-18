//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCliksAmbientPlayerRecognitionIsNarrow(t *testing.T) {
	marker := filepath.Clean(filepath.Join(t.TempDir(), "cliks", "ambient-v2")) + string(os.PathSeparator)
	track := filepath.Join(marker, "cafe.wav")
	for _, test := range []struct {
		name string
		args []string
		want bool
	}{
		{"cliks mpv track", []string{"/usr/bin/mpv", "--loop-file=inf", track}, true},
		{"shell launched player", []string{"/bin/sh", "/tmp/mpv", track}, true},
		{"unrelated mpv", []string{"/usr/bin/mpv", "/home/user/music.wav"}, false},
		{"unrelated process", []string{"/usr/bin/tail", "-f", track}, false},
		{"wrong extension", []string{"/usr/bin/mpv", filepath.Join(marker, "notes.txt")}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isCliksAmbientPlayer(test.args, marker); got != test.want {
				t.Fatalf("isCliksAmbientPlayer(%q) = %v, want %v", test.args, got, test.want)
			}
		})
	}
}

func TestCleanupOrphanAmbientPlayersStopsOnlyCliksTrack(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	trackDir := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "cliks", "ambient-v2")
	if err := os.MkdirAll(trackDir, 0o755); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(trackDir, "cafe.wav")
	if err := os.WriteFile(track, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	player := filepath.Join(t.TempDir(), "mpv")
	if err := os.WriteFile(player, []byte("#!/bin/sh\nwhile :; do sleep 1; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(player, track)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	defer func() {
		_ = cmd.Process.Kill()
		<-done
	}()
	time.Sleep(50 * time.Millisecond)
	if stopped := cleanupOrphanAmbientPlayers(); stopped != 1 {
		t.Fatalf("stopped = %d, want 1", stopped)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("ambient test player %d survived cleanup", cmd.Process.Pid)
	}
}

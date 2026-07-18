//go:build !darwin && !windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func playAmbient(ctx context.Context, mode string, volume float64) error {
	path, err := ambientWAVPath(mode)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	if player, err := exec.LookPath("mpv"); err == nil {
		cmd = exec.CommandContext(ctx, player, "--no-video", "--really-quiet", "--no-terminal", "--loop-file=inf", fmt.Sprintf("--volume=%d", int(clamp(volume, 0, 1)*100)), path)
	} else if player, err := exec.LookPath("ffplay"); err == nil {
		cmd = exec.CommandContext(ctx, player, "-nodisp", "-loglevel", "quiet", "-stream_loop", "-1", "-volume", fmt.Sprintf("%d", int(clamp(volume, 0, 1)*100)), path)
	} else if player, err := exec.LookPath("paplay"); err == nil {
		return loopAmbientCommand(ctx, 0, func() *exec.Cmd {
			return exec.CommandContext(ctx, player, fmt.Sprintf("--volume=%d", int(clamp(volume, 0, 1)*65536)), path)
		})
	} else if player, err := exec.LookPath("pw-play"); err == nil {
		return loopAmbientCommand(ctx, 0, func() *exec.Cmd {
			return exec.CommandContext(ctx, player, fmt.Sprintf("--volume=%.2f", clamp(volume, 0, 1)), path)
		})
	} else if player, err := exec.LookPath("aplay"); err == nil {
		return loopAmbientCommand(ctx, 0, func() *exec.Cmd { return exec.CommandContext(ctx, player, "-q", path) })
	} else if player, err := exec.LookPath("termux-media-player"); err == nil {
		return loopAmbientCommand(ctx, 20*time.Second, func() *exec.Cmd { return exec.CommandContext(ctx, player, "play", path) })
	} else {
		return fmt.Errorf("ambient room tones need mpv, ffplay, PulseAudio, PipeWire, ALSA, or Termux:API")
	}
	return cmd.Run()
}

func loopAmbientCommand(ctx context.Context, minimumCycle time.Duration, command func() *exec.Cmd) error {
	for ctx.Err() == nil {
		started := time.Now()
		if err := command().Run(); err != nil && ctx.Err() == nil {
			return err
		}
		remaining := minimumCycle - time.Since(started)
		if remaining > 0 {
			timer := time.NewTimer(remaining)
			select {
			case <-ctx.Done():
				timer.Stop()
				if player, err := exec.LookPath("termux-media-player"); err == nil {
					_ = exec.Command(player, "stop").Run()
				}
				return nil
			case <-timer.C:
			}
		}
	}
	return nil
}

func ambientWAVPath(mode string) (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		root = os.TempDir()
	}
	dir := filepath.Join(root, "cliks", "ambient-v2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, mode+".wav")
	if info, err := os.Stat(path); err == nil && info.Size() > 44 {
		return path, nil
	}
	pcm, err := ambientStereoPCM(mode)
	if err != nil {
		return "", err
	}
	if err := atomicWriteFile(path, pcmWAV(pcm), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

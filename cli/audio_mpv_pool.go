package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	mpvPoolSize = 4
)

type mpvWorker struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	mu    sync.Mutex
	dead  bool
}

type mpvPlayerPool struct {
	workers []*mpvWorker
	index   uint32
	mu      sync.Mutex
	device  string
	script  string
	closed  bool
}

func newMPVPlayerPool(device string) (*mpvPlayerPool, error) {
	scriptPath, err := ensureMPVStdinScript()
	if err != nil {
		return nil, err
	}

	pool := &mpvPlayerPool{
		workers: make([]*mpvWorker, mpvPoolSize),
		device:  device,
		script:  scriptPath,
	}

	for i := 0; i < mpvPoolSize; i++ {
		w, err := newMPVWorker(device, scriptPath)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to spawn mpv pool worker %d: %w", i, err)
		}
		pool.workers[i] = w
	}

	return pool, nil
}

func newMPVWorker(device string, script string) (*mpvWorker, error) {
	args := []string{
		"--idle=yes",
		"--no-config",
		"--no-video",
		"--really-quiet",
		"--script=" + script,
	}
	if device != "" && !strings.EqualFold(device, "default") {
		args = append(args, "--audio-device="+device)
	}

	cmd := exec.Command("mpv", args...)
	prepareAmbientCommand(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}

	w := &mpvWorker{
		cmd:   cmd,
		stdin: stdin,
	}

	go func() {
		_ = cmd.Wait()
		w.mu.Lock()
		w.dead = true
		w.mu.Unlock()
	}()

	return w, nil
}

func (w *mpvWorker) isDead() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dead
}

type mpvJSONCommand struct {
	Command []interface{} `json:"command"`
}

func (w *mpvWorker) play(file string, gain float64, pan float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dead {
		return fmt.Errorf("mpv worker is dead")
	}

	vol := int(clamp(gain, 0, 1) * 100)
	panFilter := ffmpegSpatialFilter(1, pan)

	c1, _ := json.Marshal(mpvJSONCommand{Command: []interface{}{"set", "volume", vol}})
	c2, _ := json.Marshal(mpvJSONCommand{Command: []interface{}{"set", "af", "lavfi=[" + panFilter + "]"}})
	c3, _ := json.Marshal(mpvJSONCommand{Command: []interface{}{"loadfile", file, "replace"}})

	payload := string(c1) + "\n" + string(c2) + "\n" + string(c3) + "\n"

	_, err := io.WriteString(w.stdin, payload)
	if err != nil {
		w.dead = true
		_ = w.stdin.Close()
		if w.cmd != nil && w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
		return err
	}

	return nil
}

func (w *mpvWorker) close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.dead {
		w.dead = true
		quitCmd, _ := json.Marshal(mpvJSONCommand{Command: []interface{}{"quit"}})
		_, _ = io.WriteString(w.stdin, string(quitCmd)+"\n")
		_ = w.stdin.Close()

		done := make(chan struct{})
		go func() {
			if w.cmd != nil {
				_ = w.cmd.Wait()
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			if w.cmd != nil && w.cmd.Process != nil {
				_ = w.cmd.Process.Kill()
			}
		}
	}
}

func (p *mpvPlayerPool) Play(ctx context.Context, job playbackJob) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("mpv player pool is closed")
	}
	workerCount := len(p.workers)
	if workerCount == 0 {
		p.mu.Unlock()
		return fmt.Errorf("no mpv workers in pool")
	}

	idx := atomic.AddUint32(&p.index, 1) % uint32(workerCount)
	worker := p.workers[idx]
	p.mu.Unlock()

	if worker == nil || worker.isDead() {
		p.replaceWorker(idx, worker)
		p.mu.Lock()
		worker = p.workers[idx]
		p.mu.Unlock()
	}

	if worker == nil {
		return fmt.Errorf("mpv worker replacement failed")
	}

	err := worker.play(job.File, job.Gain, job.Pan)
	if err != nil {
		p.replaceWorker(idx, worker)
		p.mu.Lock()
		worker = p.workers[idx]
		p.mu.Unlock()
		if worker != nil {
			err = worker.play(job.File, job.Gain, job.Pan)
		}
	}

	return err
}

func (p *mpvPlayerPool) replaceWorker(idx uint32, oldWorker *mpvWorker) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || idx >= uint32(len(p.workers)) {
		return
	}

	if p.workers[idx] != oldWorker && p.workers[idx] != nil && !p.workers[idx].isDead() {
		return
	}

	if oldWorker != nil {
		oldWorker.close()
	}

	newW, err := newMPVWorker(p.device, p.script)
	if err == nil {
		p.workers[idx] = newW
	}
}

func (p *mpvPlayerPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	workers := p.workers
	p.workers = nil
	p.mu.Unlock()

	for _, w := range workers {
		if w != nil {
			w.close()
		}
	}
}

var (
	mpvStdinScriptOnce sync.Once
	mpvStdinScriptPath string
	mpvStdinScriptErr  error
)

func ensureMPVStdinScript() (string, error) {
	mpvStdinScriptOnce.Do(func() {
		cacheRoot, err := os.UserCacheDir()
		if err != nil || strings.TrimSpace(cacheRoot) == "" {
			cacheRoot = os.TempDir()
		}
		dir := filepath.Join(cacheRoot, "cliks", "scripts")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			mpvStdinScriptErr = err
			return
		}
		scriptPath := filepath.Join(dir, "cliks-mpv-stdin-v1.lua")
		luaContent := `
local io = require('io')
local utils = require('mp.utils')

mp.add_periodic_timer(0.002, function()
    while true do
        local line = io.read('*l')
        if not line or line == '' then break end
        local obj, err = utils.parse_json(line)
        if obj and obj.command then
            mp.command_native(obj.command)
        else
            mp.command(line)
        end
    end
end)
`
		if stat, err := os.Stat(scriptPath); err == nil && stat.Size() == int64(len(luaContent)) {
			mpvStdinScriptPath = scriptPath
			return
		}
		if err := atomicWriteFile(scriptPath, []byte(luaContent), 0o644); err != nil {
			mpvStdinScriptErr = err
			return
		}
		mpvStdinScriptPath = scriptPath
	})
	return mpvStdinScriptPath, mpvStdinScriptErr
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	runModeForeground = "foreground"
	runModeBackground = "background"
	runModeBoot       = "boot"
)

type ActiveSessionState struct {
	PID                 int    `json:"pid"`
	TeamCode            string `json:"teamCode"`
	TeamName            string `json:"teamName,omitempty"`
	Mode                string `json:"mode"`
	ConnectionStatus    string `json:"connectionStatus"`
	CaptureMode         string `json:"captureMode,omitempty"`
	ActiveCount         int    `json:"activeCount,omitempty"`
	LocalCapturedEvents int    `json:"localCapturedEvents,omitempty"`
	LocalSentEvents     int    `json:"localSentEvents,omitempty"`
	StartedAt           string `json:"startedAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type sessionInstance struct {
	path     string
	state    ActiveSessionState
	released bool
}

type alreadyRunningError struct {
	state ActiveSessionState
}

func (e alreadyRunningError) Error() string {
	return fmt.Sprintf("Cliks is already running for %s (%s, pid %d). Use `cliks background status` or `cliks background stop`.", e.state.TeamCode, modeLabel(e.state.Mode), e.state.PID)
}

func acquireSessionInstance(teamCode string, mode string) (*sessionInstance, error) {
	if mode == "" {
		mode = runModeForeground
	}
	if active, ok := activeSession(); ok {
		return nil, alreadyRunningError{state: active}
	}
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return nil, err
	}
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	state := ActiveSessionState{
		PID:              os.Getpid(),
		TeamCode:         strings.ToUpper(teamCode),
		Mode:             mode,
		ConnectionStatus: "starting",
		StartedAt:        startedAt,
		UpdatedAt:        startedAt,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	path := sessionLockPath()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			cleanupStaleSession()
			file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		}
		if err != nil {
			if active, ok := activeSession(); ok {
				return nil, alreadyRunningError{state: active}
			}
			return nil, err
		}
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	instance := &sessionInstance{path: path, state: state}
	_ = writeActiveSessionState(state)
	return instance, nil
}

func (s *sessionInstance) update(view SessionViewState) {
	if s == nil || s.released {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.state.TeamCode = view.TeamCode
	if s.state.TeamCode == "" {
		s.state.TeamCode = view.TeamName
	}
	s.state.TeamName = view.TeamName
	s.state.ConnectionStatus = view.ConnectionStatus
	s.state.CaptureMode = view.CaptureMode
	s.state.ActiveCount = view.ActiveCount
	s.state.LocalCapturedEvents = view.LocalCapturedEvents
	s.state.LocalSentEvents = view.LocalSentEvents
	s.state.UpdatedAt = now
	_ = writeActiveSessionState(s.state)
}

func (s *sessionInstance) release() {
	if s == nil || s.released {
		return
	}
	s.released = true
	stopped := s.state
	stopped.ConnectionStatus = "stopped"
	stopped.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = writeActiveSessionState(stopped)
	if lock, ok := readSessionFile(sessionLockPath()); ok && lock.PID == os.Getpid() {
		_ = os.Remove(sessionLockPath())
	}
	if pid, ok := readBackgroundPID(); ok && pid == os.Getpid() {
		_ = os.Remove(backgroundPIDPath())
	}
}

func runModeFromEnv() string {
	switch strings.ToLower(os.Getenv("CLIKS_RUN_MODE")) {
	case runModeBackground:
		return runModeBackground
	case runModeBoot:
		return runModeBoot
	default:
		return runModeForeground
	}
}

func activeSession() (ActiveSessionState, bool) {
	if lock, ok := readSessionFile(sessionLockPath()); ok {
		if processLooksAlive(lock.PID) {
			state, _ := readSessionFile(sessionStatePath())
			if state.PID == 0 {
				state = lock
			}
			state.PID = lock.PID
			if state.Mode == "" {
				state.Mode = lock.Mode
			}
			if state.TeamCode == "" {
				state.TeamCode = lock.TeamCode
			}
			return state, true
		}
		cleanupStaleSession()
	}
	if pid, ok := readBackgroundPID(); ok && pid != os.Getpid() && processLooksAlive(pid) {
		state, _ := readSessionFile(sessionStatePath())
		state.PID = pid
		if state.Mode == "" {
			state.Mode = runModeBackground
		}
		if state.ConnectionStatus == "" {
			state.ConnectionStatus = "starting"
		}
		return state, true
	}
	return ActiveSessionState{}, false
}

func stopActiveSession() (string, error) {
	active, ok := activeSession()
	if !ok {
		return "Cliks is not running.", nil
	}
	if active.PID == os.Getpid() {
		return "", fmt.Errorf("this terminal owns the active Cliks session")
	}
	process, err := os.FindProcess(active.PID)
	if err == nil {
		_ = process.Kill()
	}
	_ = os.Remove(sessionLockPath())
	_ = os.Remove(backgroundPIDPath())
	stopped := active
	stopped.ConnectionStatus = "stopped"
	stopped.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = writeActiveSessionState(stopped)
	return fmt.Sprintf("Stopped Cliks for %s.", valuePlain(active.TeamCode, "the current team")), nil
}

func cleanupStaleSession() {
	_ = os.Remove(sessionLockPath())
	if pid, ok := readBackgroundPID(); ok && !processLooksAlive(pid) {
		_ = os.Remove(backgroundPIDPath())
	}
}

func sessionLockPath() string {
	return filepath.Join(stateDir(), "session.lock")
}

func sessionStatePath() string {
	return filepath.Join(stateDir(), "session.json")
}

func writeActiveSessionState(state ActiveSessionState) error {
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionStatePath(), append(data, '\n'), 0o644)
}

func readSessionFile(path string) (ActiveSessionState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ActiveSessionState{}, false
	}
	var state ActiveSessionState
	if json.Unmarshal(data, &state) != nil || state.PID <= 0 {
		return ActiveSessionState{}, false
	}
	return state, true
}

func modeLabel(mode string) string {
	switch mode {
	case runModeBackground:
		return "background"
	case runModeBoot:
		return "launch at login"
	case runModeForeground:
		return "live terminal"
	default:
		return valuePlain(mode, "running")
	}
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func cmdBackground(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cliks background start|stop|status [team-code]")
	}
	switch args[0] {
	case "start":
		cfg := loadConfig()
		code := cfg.CurrentTeamCode
		if len(args) > 1 {
			code = strings.ToUpper(args[1])
		}
		if code == "" {
			return fmt.Errorf("no team selected. Run: cliks join CLIK-XXXXXX")
		}
		message, err := startBackgroundForTeam(code)
		if message != "" {
			fmt.Println(message)
		}
		return err
	case "stop":
		message, err := stopBackground()
		if message != "" {
			fmt.Println(message)
		}
		return err
	case "status":
		fmt.Print(backgroundStatusText())
		return nil
	default:
		return fmt.Errorf("usage: cliks background start|stop|status [team-code]")
	}
}

func startBackgroundForTeam(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	active, switched, err := disconnectActiveSessionForTransition(code)
	if err != nil {
		return "", fmt.Errorf("switch active team: %w", err)
	}
	if current, ok := activeSession(); ok {
		return fmt.Sprintf("Cliks is already running for %s (%s, pid %d).", valuePlain(current.TeamCode, code), modeLabel(current.Mode), current.PID), nil
	}
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, "background.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(currentExecutable(), "start")
	cmd.Env = append(os.Environ(), "CLIKS_AUTOSTART_TEAM="+code, "CLIKS_RUN_MODE="+runModeBackground)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	prepareBackgroundCommand(cmd)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return "", err
	}
	_ = writeBackgroundPID(cmd.Process.Pid)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_ = writeActiveSessionState(ActiveSessionState{
		PID:              cmd.Process.Pid,
		Version:          version,
		TeamCode:         code,
		TeamName:         teamNameForCode(loadConfig(), code),
		Mode:             runModeBackground,
		ConnectionStatus: "starting",
		StartedAt:        now,
		UpdatedAt:        now,
	})
	if err := waitForBackgroundReady(cmd.Process.Pid, code, 2*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = logFile.Close()
		detail := tailBackgroundLog(logPath, 4096)
		if detail != "" {
			return "", fmt.Errorf("%w\nLast log output:\n%s\nLog: %s", err, detail, logPath)
		}
		return "", fmt.Errorf("%w\nLog: %s", err, logPath)
	}
	_ = cmd.Process.Release()
	_ = logFile.Close()
	prefix := ""
	if switched {
		prefix = fmt.Sprintf("Switched from %s.\n", valuePlain(active.TeamCode, "the previous team"))
	}
	return prefix + fmt.Sprintf("Cliks is running in the background for %s.\nStatus: cliks service status\nStop:   cliks service stop\n(Aliases: cliks background status|stop)", code), nil
}

func tailBackgroundLog(path string, limit int) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || limit <= 0 {
		return ""
	}
	if len(data) > limit {
		data = data[len(data)-limit:]
		if newline := strings.IndexByte(string(data), '\n'); newline >= 0 {
			data = data[newline+1:]
		}
	}
	return strings.TrimSpace(string(data))
}

func waitForBackgroundReady(pid int, code string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lockSeenAt time.Time
	for time.Now().Before(deadline) {
		if !processLooksAlive(pid) {
			return fmt.Errorf("Cliks background session exited during startup")
		}
		if lock, ok := readSessionFile(sessionLockPath()); ok && lock.PID == pid {
			if lock.TeamCode == "" || strings.EqualFold(lock.TeamCode, code) {
				if lockSeenAt.IsZero() {
					lockSeenAt = time.Now()
				}
				if time.Since(lockSeenAt) >= 150*time.Millisecond {
					return nil
				}
				time.Sleep(40 * time.Millisecond)
				continue
			}
			return fmt.Errorf("background session started for unexpected team %s", lock.TeamCode)
		}
		time.Sleep(40 * time.Millisecond)
	}
	return fmt.Errorf("Cliks background session did not become ready within %s", timeout.Round(time.Millisecond))
}

func stopBackground() (string, error) {
	return stopActiveSession()
}

func backgroundStatusText() string {
	if active, ok := activeSession(); ok {
		return fmt.Sprintf("Cliks: running for %s (%s, pid %d)\nConnection: %s\nActive users: %d\nCaptured: %d\nSent: %d\nLog: %s\n",
			valuePlain(active.TeamCode, "current team"),
			modeLabel(active.Mode),
			active.PID,
			valuePlain(active.ConnectionStatus, "starting"),
			active.ActiveCount,
			active.LocalCapturedEvents,
			active.LocalSentEvents,
			filepath.Join(stateDir(), "background.log"),
		)
	}
	return "Cliks: stopped\n"
}

func stateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "cliks")
	}
	if runtime.GOOS == "windows" {
		if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
			return filepath.Join(local, "cliks")
		}
		if appdata := strings.TrimSpace(os.Getenv("APPDATA")); appdata != "" {
			return filepath.Join(appdata, "cliks", "state")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cliks", "state")
	}
	return filepath.Join(home, ".local", "state", "cliks")
}

func backgroundPIDPath() string {
	return filepath.Join(stateDir(), "background.pid")
}

func readBackgroundPID() (int, bool) {
	data, err := os.ReadFile(backgroundPIDPath())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func writeBackgroundPID(pid int) error {
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(backgroundPIDPath(), []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

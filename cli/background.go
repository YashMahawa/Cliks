package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	if active, ok := activeSession(); ok {
		return fmt.Sprintf("Cliks is already running for %s (%s, pid %d).", valuePlain(active.TeamCode, code), modeLabel(active.Mode), active.PID), nil
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
	_ = cmd.Process.Release()
	_ = logFile.Close()
	return fmt.Sprintf("Cliks is running in the background for %s.\nStatus: cliks background status\nStop:   cliks background stop", code), nil
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
	return os.WriteFile(backgroundPIDPath(), []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

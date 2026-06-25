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
		return startBackground(code)
	case "stop":
		return stopBackground()
	case "status":
		return backgroundStatus()
	default:
		return fmt.Errorf("usage: cliks background start|stop|status [team-code]")
	}
}

func startBackground(code string) error {
	if pid, ok := readBackgroundPID(); ok && processLooksAlive(pid) {
		fmt.Printf("Cliks background is already running (pid %d).\n", pid)
		return nil
	}
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "background.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(currentExecutable(), "start")
	cmd.Env = append(os.Environ(), "CLIKS_AUTOSTART_TEAM="+code)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	prepareBackgroundCommand(cmd)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = writeBackgroundPID(cmd.Process.Pid)
	_ = cmd.Process.Release()
	_ = logFile.Close()
	fmt.Printf("Cliks is running in the background for %s.\n", code)
	fmt.Printf("Status: cliks background status\n")
	fmt.Printf("Stop:   cliks background stop\n")
	return nil
}

func stopBackground() error {
	pid, ok := readBackgroundPID()
	if !ok {
		fmt.Println("Cliks background is not running.")
		return nil
	}
	process, err := os.FindProcess(pid)
	if err == nil {
		_ = process.Kill()
	}
	_ = os.Remove(backgroundPIDPath())
	fmt.Println("Cliks background stopped.")
	return nil
}

func backgroundStatus() error {
	pid, ok := readBackgroundPID()
	if !ok {
		fmt.Println("Cliks background: stopped")
		return nil
	}
	if processLooksAlive(pid) {
		fmt.Printf("Cliks background: running (pid %d)\n", pid)
	} else {
		fmt.Printf("Cliks background: stale pid %d\n", pid)
	}
	fmt.Printf("Log: %s\n", filepath.Join(stateDir(), "background.log"))
	return nil
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

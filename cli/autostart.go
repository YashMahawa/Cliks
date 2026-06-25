package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const serviceName = "cliks"
const launchAgentID = "io.cliks.app"

func cmdAutostart(args []string) error {
	message, err := autostartAction(args)
	if message != "" {
		fmt.Println(message)
	}
	return err
}

func autostartAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: cliks autostart enable|disable|status [team-code]")
	}
	cfg := loadConfig()
	action := args[0]
	code := cfg.CurrentTeamCode
	if len(args) > 1 {
		code = strings.ToUpper(args[1])
	}
	if action == "enable" && code == "" {
		return "", fmt.Errorf("no team selected. Run: cliks join CLIK-XXXXXX")
	}
	switch runtime.GOOS {
	case "linux":
		return linuxAutostart(action, code)
	case "darwin":
		return macAutostart(action, code)
	case "windows":
		return windowsAutostart(action, code)
	default:
		return "", fmt.Errorf("autostart is supported on Linux, macOS, and Windows")
	}
}

func currentExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return "cliks"
	}
	return exe
}

func linuxAutostart(action, code string) (string, error) {
	home, _ := os.UserHomeDir()
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "systemd", "user")
	path := filepath.Join(dir, serviceName+".service")
	switch action {
	case "status":
		return autostartStatusText(path, "systemd user service"), nil
	case "disable":
		_ = exec.Command("systemctl", "--user", "disable", "--now", serviceName+".service").Run()
		_ = os.Remove(path)
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		return "Cliks autostart disabled.", nil
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		body := fmt.Sprintf(`[Unit]
Description=Cliks ambient coworking
After=network-online.target

[Service]
Type=simple
ExecStart=%s start
Restart=on-failure
RestartSec=10
Environment=CLIKS_AUTOSTART_TEAM=%s
Environment=CLIKS_RUN_MODE=%s

[Install]
WantedBy=default.target
`, currentExecutable(), code, runModeBoot)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
		reload := exec.Command("systemctl", "--user", "daemon-reload").Run()
		enable := exec.Command("systemctl", "--user", "enable", "--now", serviceName+".service").Run()
		if reload != nil || enable != nil {
			return fmt.Sprintf("Service file written for %s. Run: systemctl --user enable --now %s.service", code, serviceName), nil
		}
		return fmt.Sprintf("Cliks autostart enabled for %s.", code), nil
	default:
		return "", fmt.Errorf("unknown autostart action: %s", action)
	}
}

func macAutostart(action, code string) (string, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	path := filepath.Join(dir, launchAgentID+".plist")
	switch action {
	case "status":
		return autostartStatusText(path, "LaunchAgent"), nil
	case "disable":
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path).Run()
		_ = os.Remove(path)
		return "Cliks autostart disabled.", nil
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>start</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>CLIKS_AUTOSTART_TEAM</key>
    <string>%s</string>
    <key>CLIKS_RUN_MODE</key>
    <string>%s</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, launchAgentID, currentExecutable(), code, runModeBoot, filepath.Join(home, "Library", "Logs", "cliks.log"), filepath.Join(home, "Library", "Logs", "cliks.err.log"))
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
		err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path).Run()
		if err != nil {
			return fmt.Sprintf("LaunchAgent written for %s. Run: launchctl bootstrap gui/$(id -u) %q", code, path), nil
		}
		return fmt.Sprintf("Cliks autostart enabled for %s.", code), nil
	default:
		return "", fmt.Errorf("unknown autostart action: %s", action)
	}
}

func windowsAutostart(action, code string) (string, error) {
	startup := os.Getenv("APPDATA")
	if startup == "" {
		return "", fmt.Errorf("could not locate Windows Startup folder")
	}
	dir := filepath.Join(startup, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	path := filepath.Join(dir, "Cliks.cmd")
	switch action {
	case "status":
		return autostartStatusText(path, "Startup script"), nil
	case "disable":
		_ = os.Remove(path)
		return "Cliks autostart disabled.", nil
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		body := fmt.Sprintf("@echo off\r\nset CLIKS_AUTOSTART_TEAM=%s\r\nset CLIKS_RUN_MODE=%s\r\nstart \"Cliks\" /min \"%s\" start\r\n", code, runModeBoot, currentExecutable())
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("Cliks autostart enabled for %s. It will start after your next sign-in.", code), nil
	default:
		return "", fmt.Errorf("unknown autostart action: %s", action)
	}
}

func autostartStatusText(path, label string) string {
	_, err := os.Stat(path)
	if err == nil {
		return fmt.Sprintf("Cliks autostart: enabled\n%s: %s", label, path)
	}
	return fmt.Sprintf("Cliks autostart: disabled\n%s: %s", label, path)
}

func autostartEnabled() bool {
	cfg := loadConfig()
	message, err := autostartAction([]string{"status", cfg.CurrentTeamCode})
	return err == nil && strings.Contains(message, "enabled")
}

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
	if len(args) == 0 {
		return fmt.Errorf("usage: cliks autostart enable|disable|status [team-code]")
	}
	cfg := loadConfig()
	action := args[0]
	code := cfg.CurrentTeamCode
	if len(args) > 1 {
		code = strings.ToUpper(args[1])
	}
	if action == "enable" && code == "" {
		return fmt.Errorf("no team selected. Run: cliks join CLIK-XXXXXX")
	}
	switch runtime.GOOS {
	case "linux":
		return linuxAutostart(action, code)
	case "darwin":
		return macAutostart(action, code)
	case "windows":
		return windowsAutostart(action, code)
	default:
		return fmt.Errorf("autostart is supported on Linux, macOS, and Windows")
	}
}

func currentExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return "cliks"
	}
	return exe
}

func linuxAutostart(action, code string) error {
	home, _ := os.UserHomeDir()
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "systemd", "user")
	path := filepath.Join(dir, serviceName+".service")
	switch action {
	case "status":
		printAutostartStatus(path, "systemd user service")
	case "disable":
		_ = exec.Command("systemctl", "--user", "disable", "--now", serviceName+".service").Run()
		_ = os.Remove(path)
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		fmt.Println("Cliks autostart disabled.")
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		body := fmt.Sprintf(`[Unit]
Description=Cliks ambient coworking
After=network-online.target

[Service]
Type=simple
ExecStart=%s start
Restart=always
RestartSec=10
Environment=CLIKS_AUTOSTART_TEAM=%s

[Install]
WantedBy=default.target
`, currentExecutable(), code)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		reload := exec.Command("systemctl", "--user", "daemon-reload").Run()
		enable := exec.Command("systemctl", "--user", "enable", "--now", serviceName+".service").Run()
		fmt.Printf("Cliks autostart enabled for %s.\n", code)
		if reload != nil || enable != nil {
			fmt.Printf("Service file written. Run: systemctl --user enable --now %s.service\n", serviceName)
		}
	default:
		return fmt.Errorf("unknown autostart action: %s", action)
	}
	return nil
}

func macAutostart(action, code string) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	path := filepath.Join(dir, launchAgentID+".plist")
	switch action {
	case "status":
		printAutostartStatus(path, "LaunchAgent")
	case "disable":
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path).Run()
		_ = os.Remove(path)
		fmt.Println("Cliks autostart disabled.")
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
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
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, launchAgentID, currentExecutable(), code, filepath.Join(home, "Library", "Logs", "cliks.log"), filepath.Join(home, "Library", "Logs", "cliks.err.log"))
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path).Run()
		fmt.Printf("Cliks autostart enabled for %s.\n", code)
		if err != nil {
			fmt.Printf("LaunchAgent written. Run: launchctl bootstrap gui/$(id -u) %q\n", path)
		}
	default:
		return fmt.Errorf("unknown autostart action: %s", action)
	}
	return nil
}

func windowsAutostart(action, code string) error {
	startup := os.Getenv("APPDATA")
	if startup == "" {
		return fmt.Errorf("could not locate Windows Startup folder")
	}
	dir := filepath.Join(startup, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	path := filepath.Join(dir, "Cliks.cmd")
	switch action {
	case "status":
		printAutostartStatus(path, "Startup script")
	case "disable":
		_ = os.Remove(path)
		fmt.Println("Cliks autostart disabled.")
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		body := fmt.Sprintf("@echo off\r\nset CLIKS_AUTOSTART_TEAM=%s\r\nstart \"Cliks\" /min \"%s\" start\r\n", code, currentExecutable())
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Printf("Cliks autostart enabled for %s.\n", code)
		fmt.Println("It will start after your next sign-in.")
	default:
		return fmt.Errorf("unknown autostart action: %s", action)
	}
	return nil
}

func printAutostartStatus(path, label string) {
	_, err := os.Stat(path)
	if err == nil {
		fmt.Println("Cliks autostart: enabled")
	} else {
		fmt.Println("Cliks autostart: disabled")
	}
	fmt.Printf("%s: %s\n", label, path)
}

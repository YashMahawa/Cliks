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
	if err == nil && len(args) > 0 && (args[0] == "enable" || args[0] == "disable") {
		cfg := loadConfig()
		cfg.AutostartWanted = args[0] == "enable"
		_ = saveConfig(cfg)
	}
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
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		return resolved
	}
	return exe
}

// stableServiceExecutable registers the executable that handled the user's
// enable command. Release installers update that stable path in place, while a
// local build must not accidentally register an older global binary from PATH.
func stableServiceExecutable() string {
	return currentExecutable()
}

func systemdQuote(path string) string {
	// systemd unit values: wrap in double quotes and escape \ and ".
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
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
		exe := stableServiceExecutable()
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
`, systemdQuote(exe), code, runModeBoot)
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
		exe := stableServiceExecutable()
		// Reloading by label first makes upgrades idempotent when an older
		// LaunchAgent is already registered with launchd.
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		_ = exec.Command("launchctl", "bootout", domain+"/"+launchAgentID).Run()
		_ = exec.Command("launchctl", "bootout", domain, path).Run()
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
	<key>ProcessType</key>
	<string>Background</string>
  <key>KeepAlive</key>
  <false/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, launchAgentID, exe, code, runModeBoot, filepath.Join(home, "Library", "Logs", "cliks.log"), filepath.Join(home, "Library", "Logs", "cliks.err.log"))
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
		err := exec.Command("launchctl", "bootstrap", domain, path).Run()
		if err != nil {
			return fmt.Sprintf("LaunchAgent written for %s. Run: launchctl bootstrap gui/$(id -u) %q", code, path), nil
		}
		return fmt.Sprintf("Cliks autostart enabled for %s.", code), nil
	default:
		return "", fmt.Errorf("unknown autostart action: %s", action)
	}
}

func enableWantedAutostart(cfg CliksConfig) string {
	if !cfg.AutostartWanted || cfg.CurrentTeamCode == "" {
		return ""
	}
	message, err := autostartAction([]string{"enable", cfg.CurrentTeamCode})
	if err != nil {
		return "Launch at login still needs attention: " + err.Error()
	}
	return message
}

func windowsAutostart(action, code string) (string, error) {
	startup := os.Getenv("APPDATA")
	if startup == "" {
		return "", fmt.Errorf("could not locate Windows Startup folder")
	}
	dir := filepath.Join(startup, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	// Prefer silent VBScript; remove legacy .cmd that flashed a console.
	vbsPath := filepath.Join(dir, "Cliks.vbs")
	cmdPath := filepath.Join(dir, "Cliks.cmd")
	switch action {
	case "status":
		if _, err := os.Stat(vbsPath); err == nil {
			return autostartStatusText(vbsPath, "Startup script (silent)"), nil
		}
		return autostartStatusText(cmdPath, "Startup script"), nil
	case "disable":
		_ = os.Remove(vbsPath)
		_ = os.Remove(cmdPath)
		return "Cliks autostart disabled.", nil
	case "enable":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		exe := stableServiceExecutable()
		// WindowStyle 0 = hidden. No console flash at login.
		body := fmt.Sprintf(
			"Set sh = CreateObject(\"Wscript.Shell\")\r\n"+
				"sh.Environment(\"Process\")(\"CLIKS_AUTOSTART_TEAM\") = \"%s\"\r\n"+
				"sh.Environment(\"Process\")(\"CLIKS_RUN_MODE\") = \"%s\"\r\n"+
				"sh.Run \"\"\"%s\"\" start\", 0, False\r\n",
			code, runModeBoot, strings.ReplaceAll(exe, `"`, `""`),
		)
		if err := os.WriteFile(vbsPath, []byte(body), 0o644); err != nil {
			return "", err
		}
		_ = os.Remove(cmdPath)
		return fmt.Sprintf("Cliks autostart enabled for %s. It will start silently after your next sign-in.", code), nil
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

// repairAutostartIfEnabled rewrites login launchers with the current binary path.
// Safe no-op when autostart is disabled or no team is selected.
func repairAutostartIfEnabled() string {
	if !autostartEnabled() {
		return ""
	}
	cfg := loadConfig()
	if cfg.CurrentTeamCode == "" {
		return ""
	}
	message, err := autostartAction([]string{"enable", cfg.CurrentTeamCode})
	if err != nil {
		return ""
	}
	if message == "" {
		return "Refreshed launch-at-login path."
	}
	return "Refreshed launch-at-login: " + message
}

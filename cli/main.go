package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const version = "0.3.0"

func main() {
	// Terminal panic shield: always restore cooked mode / mouse reporting after a crash.
	defer func() {
		if recovered := recover(); recovered != nil {
			repairTerminal()
			fmt.Fprintf(os.Stderr, "cliks crashed: %v\nRun: cliks fix-terminal\n", recovered)
			os.Exit(1)
		}
	}()
	if err := run(os.Args); err != nil {
		repairTerminal()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	commandName := filepath.Base(args[0])
	rest := args[1:]
	if len(rest) == 0 {
		return runHomeTUI(loadConfig())
	}

	switch rest[0] {
	case "help", "--help", "-h":
		printHelp(commandName)
	case "version", "--version", "-v":
		fmt.Println(version)
	case "join":
		return cmdJoin(rest[1:])
	case "create":
		return cmdCreate(rest[1:])
	case "delete":
		return cmdDelete(rest[1:])
	case "nickname", "name":
		return cmdNickname(rest[1:])
	case "start":
		return cmdStart(rest[1:])
	case "settings", "ui":
		return runHomeTUI(loadConfig())
	case "setup":
		return cmdSetup(rest[1:])
	case "doctor":
		return runDoctor()
	case "sound-test":
		return runSoundTest()
	case "notification-test":
		return runNotificationTest()
	case "capture-test":
		return cmdCaptureTest(rest[1:])
	case "fix-terminal":
		repairTerminal()
		fmt.Println("Terminal input restored. If it still looks wrong, close and reopen this terminal tab.")
	case "teams":
		printTeams(loadConfig())
	case "switch":
		return cmdSwitch(rest[1:])
	case "config":
		return printConfig()
	case "preset":
		return cmdPreset(rest[1:])
	case "service":
		return cmdService(rest[1:])
	case "autostart":
		// Legacy alias: prefer `cliks service enable|disable|status`.
		return cmdAutostart(rest[1:])
	case "background":
		// Legacy alias: prefer `cliks service start|stop|status`.
		return cmdBackground(rest[1:])
	case "set":
		return cmdSet(rest[1:])
	default:
		if strings.HasPrefix(rest[0], "-") {
			return cmdStart(rest)
		}
		return fmt.Errorf("unknown command: %s\nrun %s help", rest[0], commandName)
	}
	return nil
}

// cmdService is the canonical interface for long-running / persistence behavior.
// start/stop/status manage the background session; enable/disable manage login autostart.
func cmdService(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cliks service start|stop|status|enable|disable [team-code]\n\n" +
			"  start|stop|status   run Cliks in the background (same as cliks background ...)\n" +
			"  enable|disable      launch at login (same as cliks autostart ...)\n" +
			"  keep.running        set with: cliks set keep.running on|off")
	}
	switch args[0] {
	case "start", "stop", "status":
		return cmdBackground(args)
	case "enable", "disable":
		return cmdAutostart(args)
	case "login-status":
		return cmdAutostart([]string{"status"})
	default:
		return fmt.Errorf("unknown service action %q\nusage: cliks service start|stop|status|enable|disable", args[0])
	}
}

func cmdJoin(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cliks join [--no-start] CLIK-XXXXXX")
	}
	nickname := ""
	code := ""
	autoStart := true
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--nickname":
			if i+1 >= len(args) {
				return errors.New("--nickname needs a value")
			}
			nickname = args[i+1]
			i++
		case "--no-start":
			autoStart = false
		case "--start":
			autoStart = true
		default:
			if code == "" {
				code = args[i]
			}
		}
	}
	if code == "" {
		return errors.New("usage: cliks join [--no-start] CLIK-XXXXXX")
	}
	cfg := loadConfig()
	team, err := getTeamViaAPI(cfg, code)
	if err != nil {
		return err
	}
	cfg, err = rememberTeam(team.Code, team.Name)
	if err != nil {
		return err
	}
	if nickname != "" {
		cfg.Nickname = sanitizeNickname(nickname)
		if err := saveConfig(cfg); err != nil {
			return err
		}
	}
	if !autoStart {
		fmt.Printf("Joined %s. Run \"cliks start\" to begin.\n", formatTeamLabel(team.Name, team.Code))
		if warning := quickDoctorWarning(cfg); warning != "" {
			fmt.Println(warning)
		}
		return nil
	}
	message, err := startBackgroundForTeam(team.Code)
	if err != nil {
		return fmt.Errorf("joined %s, but could not start Cliks in the background: %w", formatTeamLabel(team.Name, team.Code), err)
	}
	fmt.Printf("Joined %s.\n%s\n", formatTeamLabel(team.Name, team.Code), message)
	if warning := quickDoctorWarning(cfg); warning != "" {
		fmt.Println(warning)
	}
	return nil
}

func cmdCreate(args []string) error {
	reader := bufio.NewReader(os.Stdin)
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		line, err := readPrompt(reader, "Team name: ")
		if err != nil {
			return err
		}
		name = line
	}
	if name == "" {
		name = "Cliks Room"
	}
	password, err := readSecret("Delete password: ")
	if err != nil {
		return err
	}
	if len(password) < 6 {
		return errors.New("delete password must be at least 6 characters")
	}
	cfg := loadConfig()
	team, err := createTeamViaAPI(cfg, name, password)
	if err != nil {
		return err
	}
	cfg, err = rememberTeam(team.Code, team.Name)
	if err != nil {
		return err
	}
	_ = saveConfig(cfg)
	fmt.Printf("Created %s.\n", formatTeamLabel(team.Name, team.Code))
	fmt.Println(clipboardStatus(team.Code))
	fmt.Printf("Start now: cliks start\n")
	return nil
}

func cmdDelete(args []string) error {
	reader := bufio.NewReader(os.Stdin)
	cfg := loadConfig()
	code := cfg.CurrentTeamCode
	if len(args) > 0 {
		code = strings.ToUpper(args[0])
	}
	if code == "" {
		line, err := readPrompt(reader, "Team code: ")
		if err != nil {
			return err
		}
		code = strings.ToUpper(strings.TrimSpace(line))
	}
	if code == "" {
		return errors.New("team code is required")
	}
	password, err := readSecret("Delete password: ")
	if err != nil {
		return err
	}
	if password == "" {
		return errors.New("delete password is required")
	}
	if err := deleteTeamViaAPI(cfg, code, password); err != nil {
		return err
	}
	cfg, err = forgetTeam(code)
	if err != nil {
		return err
	}
	stopDeletedTeamSession(code)
	fmt.Printf("Deleted %s.\n", code)
	return nil
}

func cmdNickname(args []string) error {
	cfg := loadConfig()
	name := sanitizeNickname(strings.Join(args, " "))
	if name == "" {
		reader := bufio.NewReader(os.Stdin)
		line, err := readPrompt(reader, "Display name: ")
		if err != nil {
			return err
		}
		name = sanitizeNickname(line)
	}
	cfg.Nickname = name
	if err := saveConfig(cfg); err != nil {
		return err
	}
	if name == "" {
		fmt.Println("Nickname cleared.")
		return nil
	}
	fmt.Printf("Nickname set to %s.\n", name)
	return nil
}

func cmdStart(args []string) error {
	cfg := loadConfig()
	opts := StartOptions{CaptureMode: "auto", SelfMonitor: cfg.Listening.Self}
	codeArg := ""
	for _, arg := range args {
		switch arg {
		case "--evdev":
			opts.CaptureMode = "evdev"
		case "--terminal":
			opts.CaptureMode = "terminal"
		case "--self":
			opts.SelfMonitor = true
		default:
			if strings.HasPrefix(strings.ToUpper(arg), "CLIK-") {
				codeArg = strings.ToUpper(arg)
			}
		}
	}
	if team := os.Getenv("CLIKS_AUTOSTART_TEAM"); team != "" {
		cfg.CurrentTeamCode = strings.ToUpper(team)
	} else if codeArg != "" {
		team, err := getTeamViaAPI(cfg, codeArg)
		if err != nil {
			return err
		}
		cfg, err = rememberTeam(team.Code, team.Name)
		if err != nil {
			return err
		}
	}
	if cfg.CurrentTeamCode == "" {
		printFirstRunHelp()
		return nil
	}
	return startSession(cfg, opts)
}

func cmdCaptureTest(args []string) error {
	mode := "auto"
	seconds := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--evdev":
			mode = "evdev"
		case "--terminal":
			mode = "terminal"
		case "--seconds":
			if i+1 >= len(args) {
				return errors.New("--seconds needs a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil {
				return err
			}
			seconds = parsed
			i++
		}
	}
	return runCaptureTest(loadConfig(), mode, seconds)
}

func cmdSwitch(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cliks switch CLIK-XXXXXX")
	}
	cfg := loadConfig()
	code := strings.ToUpper(args[0])
	for _, team := range cfg.Teams {
		if team.Code == code {
			cfg.CurrentTeamCode = code
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Current team is now %s.\n", code)
			return nil
		}
	}
	return fmt.Errorf("team %s is not saved. Run: cliks join %s", code, code)
}

func printConfig() error {
	type configSummary struct {
		CliksConfig
		AutostartEnabled bool `json:"autostartEnabled"`
	}
	cfg := loadConfig()
	if warning := lastConfigLoadWarning(); warning != "" {
		// Always surface on `cliks config` even if the once-flag already fired.
		fmt.Fprintln(os.Stderr, "Warning:", warning)
	}
	data, err := json.MarshalIndent(configSummary{CliksConfig: cfg, AutostartEnabled: autostartEnabled()}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func cmdPreset(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cliks preset deep|balanced|social|quiet")
	}
	cfg := loadConfig()
	switch strings.ToLower(args[0]) {
	case "deep":
		cfg.Listening.Volume = 0.45
		cfg.Listening.Density = 0.45
		cfg.Listening.FatigueProtection = true
		cfg.Listening.Spatial = true
		cfg.Listening.Muted = false
	case "balanced":
		cfg.Listening.Volume = 0.65
		cfg.Listening.Density = 0.75
		cfg.Listening.FatigueProtection = true
		cfg.Listening.Spatial = true
		cfg.Listening.Muted = false
	case "social":
		cfg.Listening.Volume = 0.8
		cfg.Listening.Density = 1
		cfg.Listening.FatigueProtection = false
		cfg.Listening.Spatial = true
		cfg.Listening.Muted = false
	case "quiet":
		cfg.Listening.Volume = 0.3
		cfg.Listening.Density = 0.3
		cfg.Listening.FatigueProtection = true
		cfg.Listening.Spatial = true
		cfg.Listening.Muted = false
	default:
		return errors.New("unknown preset. Use: deep, balanced, social, or quiet")
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Applied %s preset.\n", strings.ToLower(args[0]))
	return nil
}

func cmdSet(args []string) error {
	if len(args) == 1 && (args[0] == "--list" || args[0] == "-l") {
		printSettingCatalog()
		return nil
	}
	if len(args) < 2 {
		return errors.New("usage: cliks set <key> <value>\n       cliks set --list")
	}
	cfg := loadConfig()
	key, value := args[0], args[1]
	boolValue := parseBool(value)
	switch key {
	case "autostart":
		enabled, err := parseOnOff(value)
		if err != nil {
			return err
		}
		if enabled {
			return cmdAutostart([]string{"enable", cfg.CurrentTeamCode})
		}
		return cmdAutostart([]string{"disable"})
	case "share.keyboard":
		cfg.Sharing.Keyboard = boolValue
	case "share.mouse":
		cfg.Sharing.Mouse = boolValue
	case "hear.keyboard":
		cfg.Listening.Keyboard = boolValue
	case "hear.mouse":
		cfg.Listening.Mouse = boolValue
	case "hear.self":
		cfg.Listening.Self = boolValue
	case "hear.muted":
		cfg.Listening.Muted = boolValue
	case "hear.spatial":
		cfg.Listening.Spatial = boolValue
	case "hear.fade":
		cfg.Listening.FatigueProtection = boolValue
	case "notifications":
		cfg.Notifications.Enabled = boolValue
		cfg.Notifications.Configured = true
	case "notifications.sound":
		cfg.Notifications.Sound = boolValue
		cfg.Notifications.Configured = true
	case "presence":
		status := strings.ToLower(strings.TrimSpace(value))
		switch status {
		case "available", "focus", "break", "dnd":
			cfg.PresenceStatus = status
		default:
			return fmt.Errorf("presence must be available, focus, break, or dnd")
		}
	case "spatial.dynamic":
		cfg.Listening.DynamicPlacement = boolValue
	case "keep.running":
		cfg.KeepRunning = boolValue
	case "volume":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		cfg.Listening.Volume = clamp(parsed, 0, 1)
	case "density":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		cfg.Listening.Density = clamp(parsed, 0.15, 1)
	case "batch.ms":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		if parsed < 100 {
			parsed = 100
		}
		if parsed > 2000 {
			parsed = 2000
		}
		cfg.BatchWindowMs = parsed
	case "spatial.shuffleMinutes":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Listening.ShuffleMinutes = parsed
	case "audio.device":
		device := strings.TrimSpace(strings.Join(args[1:], " "))
		if strings.EqualFold(device, "default") {
			device = ""
		}
		cfg.Listening.AudioDevice = device
	case "api.url":
		cfg.APIURL = strings.TrimRight(value, "/")
		cfg.WSURL = toWSURL(cfg.APIURL)
	case "ws.url":
		cfg.WSURL = value
	case "nickname", "name":
		cfg.Nickname = sanitizeNickname(strings.Join(args[1:], " "))
	default:
		return fmt.Errorf("unknown setting: %s", key)
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Println("Saved.")
	return nil
}

func printTeams(cfg CliksConfig) {
	if len(cfg.Teams) == 0 {
		fmt.Println("No teams saved yet. Run: cliks join CLIK-XXXXXX")
		return
	}
	for _, team := range cfg.Teams {
		marker := " "
		if team.Code == cfg.CurrentTeamCode {
			marker = "*"
		}
		name := ""
		if team.Name != "" {
			name = "  " + team.Name
		}
		fmt.Printf("%s %s%s\n", marker, team.Code, name)
	}
}

func filterTeams(teams []TeamConfig, code string) []TeamConfig {
	code = strings.ToUpper(strings.TrimSpace(code))
	var next []TeamConfig
	for _, team := range teams {
		if !strings.EqualFold(team.Code, code) {
			next = append(next, team)
		}
	}
	return next
}

func stopDeletedTeamSession(code string) {
	active, ok := activeSession()
	if !ok || !strings.EqualFold(active.TeamCode, code) {
		return
	}
	_, _ = stopActiveSession()
}

func readPrompt(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		value, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(value)), nil
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func parseBool(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on", "y":
		return true
	default:
		return false
	}
}

func parseOnOff(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enable", "enabled":
		return true, nil
	case "0", "false", "no", "off", "disable", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("expected on or off, got %q", value)
	}
}

func printFirstRunHelp() {
	fmt.Println("Cliks is installed.")
	fmt.Println("")
	fmt.Println("First time on this computer? Run once:")
	fmt.Println("   cliks setup")
	fmt.Println("")
	fmt.Println("Then:")
	fmt.Println("1. Create or get a team code from the Cliks website.")
	fmt.Println("2. Join it here:")
	fmt.Println("   cliks join CLIK-XXXXXX")
	fmt.Println("   This starts one background Cliks session automatically.")
	fmt.Println("")
	fmt.Println("Or open the friendly interface:")
	fmt.Println("   cliks")
	fmt.Println("")
	fmt.Println("Useful checks:")
	fmt.Println("   cliks setup")
	fmt.Println("   cliks sound-test")
	fmt.Println("   cliks doctor")
}

func printHelp(commandName string) {
	fmt.Printf(`Cliks ambient coworking CLI

Usage:
  %[1]s                  Open the control interface
  %[1]s create           Create a team
  %[1]s delete [CODE]    Delete a team
  %[1]s join CODE        Save, select, and start a background session
  %[1]s join --no-start CODE
  %[1]s nickname [NAME]  Set your 10-character display name
  %[1]s start            Start coworking ambience
  %[1]s start CODE       Join/select a code and start immediately
  %[1]s settings         Open the control screen
  %[1]s setup            One-time easy setup (sound + capture)
  %[1]s doctor           Print the full setup and permission report
  %[1]s sound-test       Play local sample sounds
  %[1]s notification-test
                         Send one native notification using your sound preference
  %[1]s capture-test     Verify local activity capture
  %[1]s service ...      Canonical service control (background + login)
  %[1]s service start|stop|status [CODE]
  %[1]s service enable|disable [CODE]
  %[1]s background ...   Alias for service start|stop|status
  %[1]s autostart ...    Alias for service enable|disable (+ status)
  %[1]s config           Show settings and launch-at-login state
  %[1]s set --list       List every setting key and TUI label
  %[1]s set autostart on|off
  %[1]s set audio.device DEVICE|default
  %[1]s set keep.running on|off
                         Keep a live session running after the terminal closes
                         (hands off to a background service when enabled)

`, commandName)
}

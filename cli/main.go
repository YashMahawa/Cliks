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

const version = "0.2.0"

func main() {
	if err := run(os.Args); err != nil {
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
	case "start":
		return cmdStart(rest[1:])
	case "settings", "ui":
		return runHomeTUI(loadConfig())
	case "doctor":
		return runDoctor()
	case "sound-test":
		return runSoundTest()
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
	case "autostart":
		return cmdAutostart(rest[1:])
	case "background":
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

func cmdJoin(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cliks join CLIK-XXXXXX")
	}
	nickname := ""
	code := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--nickname":
			if i+1 >= len(args) {
				return errors.New("--nickname needs a value")
			}
			nickname = args[i+1]
			i++
		default:
			if code == "" {
				code = args[i]
			}
		}
	}
	cfg, err := rememberTeam(code, "")
	if err != nil {
		return err
	}
	if nickname != "" {
		cfg.Nickname = nickname
		if err := saveConfig(cfg); err != nil {
			return err
		}
	}
	fmt.Printf("Joined %s. Run \"cliks start\" to begin.\n", strings.ToUpper(code))
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
	fmt.Printf("Created %s (%s).\n", team.Code, team.Name)
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
	cfg.Teams = filterTeams(cfg.Teams, code)
	if cfg.CurrentTeamCode == code {
		cfg.CurrentTeamCode = ""
		if len(cfg.Teams) > 0 {
			cfg.CurrentTeamCode = cfg.Teams[0].Code
		}
	}
	_ = saveConfig(cfg)
	fmt.Printf("Deleted %s.\n", code)
	return nil
}

func cmdStart(args []string) error {
	cfg := loadConfig()
	if team := os.Getenv("CLIKS_AUTOSTART_TEAM"); team != "" {
		cfg.CurrentTeamCode = strings.ToUpper(team)
	}
	if cfg.CurrentTeamCode == "" {
		printFirstRunHelp()
		return nil
	}
	opts := StartOptions{CaptureMode: "auto", SelfMonitor: cfg.Listening.Self}
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
				cfg.CurrentTeamCode = strings.ToUpper(arg)
			}
		}
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
	data, err := json.MarshalIndent(loadConfig(), "", "  ")
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
	if len(args) < 2 {
		return errors.New("usage: cliks set <key> <value>")
	}
	cfg := loadConfig()
	key, value := args[0], args[1]
	boolValue := parseBool(value)
	switch key {
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
	case "api.url":
		cfg.APIURL = strings.TrimRight(value, "/")
		cfg.WSURL = toWSURL(cfg.APIURL)
	case "ws.url":
		cfg.WSURL = value
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
	var next []TeamConfig
	for _, team := range teams {
		if team.Code != code {
			next = append(next, team)
		}
	}
	return next
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

func printFirstRunHelp() {
	fmt.Println("Cliks is installed.")
	fmt.Println("")
	fmt.Println("1. Create or get a team code from the Cliks website.")
	fmt.Println("2. Join it here:")
	fmt.Println("   cliks join CLIK-XXXXXX")
	fmt.Println("3. Start the room:")
	fmt.Println("   cliks start")
	fmt.Println("")
	fmt.Println("Or open the interface:")
	fmt.Println("   cliks")
	fmt.Println("")
	fmt.Println("Useful checks:")
	fmt.Println("   cliks doctor")
	fmt.Println("   cliks sound-test")
	fmt.Println("   cliks capture-test")
}

func printHelp(commandName string) {
	fmt.Printf(`Cliks ambient coworking CLI

Usage:
  %[1]s                  Open the Bubble Tea interface
  %[1]s create           Create a team
  %[1]s delete [CODE]    Delete a team
  %[1]s join CODE        Save and select a team
  %[1]s start            Start coworking ambience
  %[1]s settings         Open settings UI
  %[1]s doctor           Check setup and permissions
  %[1]s sound-test       Play local sample sounds
  %[1]s capture-test     Verify local activity capture
  %[1]s autostart ...    Manage background autoconnect
  %[1]s background ...   Start, stop, or inspect background Cliks

`, commandName)
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// configLoadWarning is set when the on-disk config exists but cannot be parsed.
// The CLI keeps running on defaults so non-technical users are not hard-blocked.
var (
	configLoadMu      sync.RWMutex
	configLoadWarning string
)

func lastConfigLoadWarning() string {
	configLoadMu.RLock()
	defer configLoadMu.RUnlock()
	return configLoadWarning
}

func setConfigLoadWarning(message string) {
	configLoadMu.Lock()
	configLoadWarning = message
	configLoadMu.Unlock()
}

// printConfigLoadWarningOnce prints the invalid-config notice once per process
// so join/start/service/TUI are not silent when JSON is broken.
var configLoadWarnOnce sync.Once

func printConfigLoadWarningOnce() {
	warning := lastConfigLoadWarning()
	if warning == "" {
		return
	}
	configLoadWarnOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "Warning:", warning)
	})
}

const productionAPIURL = "https://139.59.29.207.sslip.io"

func usesPublicBackend(cfg CliksConfig) bool {
	return strings.EqualFold(strings.TrimRight(cfg.APIURL, "/"), productionAPIURL) &&
		strings.EqualFold(strings.TrimRight(cfg.WSURL, "/"), strings.TrimRight(toWSURL(productionAPIURL), "/"))
}

func normalizeBackendURL(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if strings.EqualFold(value, "default") || strings.EqualFold(value, "public") {
		return productionAPIURL, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("server URL must start with http:// or https://")
	}
	return value, nil
}

type TeamConfig struct {
	Code         string `json:"code"`
	Name         string `json:"name,omitempty"`
	LastJoinedAt string `json:"lastJoinedAt"`
}

type SharingConfig struct {
	Keyboard bool `json:"keyboard"`
	Mouse    bool `json:"mouse"`
}

type ListeningConfig struct {
	Keyboard          bool    `json:"keyboard"`
	Mouse             bool    `json:"mouse"`
	Self              bool    `json:"self"`
	Volume            float64 `json:"volume"`
	Muted             bool    `json:"muted"`
	Spatial           bool    `json:"spatial"`
	FatigueProtection bool    `json:"fatigueProtection"`
	Density           float64 `json:"density"`
	DynamicPlacement  bool    `json:"dynamicPlacement"`
	ShuffleMinutes    int     `json:"shuffleMinutes"`
	AudioDevice       string  `json:"audioDevice,omitempty"`
	Ambient           string  `json:"ambient,omitempty"`
	AmbientVolume     float64 `json:"ambientVolume,omitempty"`
}

type SoloConfig struct {
	People         int     `json:"people"`
	Keyboard       bool    `json:"keyboard"`
	Mouse          bool    `json:"mouse"`
	KeyboardVolume float64 `json:"keyboardVolume,omitempty"`
	MouseVolume    float64 `json:"mouseVolume,omitempty"`
}

type NotificationConfig struct {
	Enabled    bool `json:"enabled"`
	Sound      bool `json:"sound"`
	Configured bool `json:"configured,omitempty"`
}

// CaptureConfig controls the local trust boundary. "isolated" is the safe
// default: a tiny OS-specific companion receives broad input permission and
// emits only allowlisted activity kinds to the network client. "direct" is a
// clearly opt-in compatibility fallback, and "terminal" never observes other
// applications.
type CaptureConfig struct {
	Mode string `json:"mode,omitempty"`
}

type CliksConfig struct {
	APIURL          string             `json:"apiUrl"`
	WSURL           string             `json:"wsUrl"`
	CurrentTeamCode string             `json:"currentTeamCode,omitempty"`
	Nickname        string             `json:"nickname,omitempty"`
	PresenceStatus  string             `json:"presenceStatus,omitempty"`
	WelcomeSeen     bool               `json:"welcomeSeen,omitempty"`
	LaunchSeen      bool               `json:"launchSeen,omitempty"`
	OnboardingSeen  bool               `json:"onboardingSeen,omitempty"`
	Theme           string             `json:"theme,omitempty"`
	KeepRunning     bool               `json:"keepRunning"`
	AutostartWanted bool               `json:"autostartWanted,omitempty"`
	Teams           []TeamConfig       `json:"teams"`
	Sharing         SharingConfig      `json:"sharing"`
	Listening       ListeningConfig    `json:"listening"`
	Notifications   NotificationConfig `json:"notifications"`
	Capture         CaptureConfig      `json:"capture"`
	Solo            SoloConfig         `json:"solo"`
	BatchWindowMs   int                `json:"batchWindowMs"`
}

func defaultConfig() CliksConfig {
	apiURL := getenvDefault("CLIKS_API_URL", productionAPIURL)
	wsURL := getenvDefault("CLIKS_WS_URL", toWSURL(apiURL))
	return CliksConfig{
		APIURL:         apiURL,
		WSURL:          wsURL,
		PresenceStatus: "available",
		Theme:          "ember",
		Notifications:  NotificationConfig{Sound: true, Configured: true},
		Capture:        CaptureConfig{Mode: "isolated"},
		Teams:          []TeamConfig{},
		Sharing: SharingConfig{
			Keyboard: true,
			Mouse:    true,
		},
		Listening: ListeningConfig{
			Keyboard:          true,
			Mouse:             true,
			Self:              false,
			Volume:            0.7,
			Muted:             false,
			Spatial:           true,
			FatigueProtection: true,
			Density:           0.8,
			DynamicPlacement:  true,
			ShuffleMinutes:    10,
			Ambient:           "off",
			AmbientVolume:     0.22,
		},
		Solo:          SoloConfig{People: 4, Keyboard: true, Mouse: true, KeyboardVolume: 0.7, MouseVolume: 0.8},
		BatchWindowMs: 500,
	}
}

func configPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "cliks", "config.json")
	}
	if runtime.GOOS == "windows" {
		if appdata := strings.TrimSpace(os.Getenv("APPDATA")); appdata != "" {
			return filepath.Join(appdata, "cliks", "config.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cliks", "config.json")
	}
	return filepath.Join(home, ".config", "cliks", "config.json")
}

func configBackupPath() string { return configPath() + ".bak" }

// legacyConfigPath is the pre-migration location (Unix-style path used on Windows too).
func legacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "cliks", "config.json")
}

func loadConfig() CliksConfig {
	cfg := defaultConfig()
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		// Migrate from the old Unix-style path on Windows (and any host that moved).
		if legacy := legacyConfigPath(); legacy != "" && legacy != path {
			if legacyData, legacyErr := os.ReadFile(legacy); legacyErr == nil {
				data = legacyData
				err = nil
				// Best-effort migrate so the next load uses the native path.
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				if writeErr := atomicWriteFile(path, legacyData, 0o644); writeErr == nil {
					// Keep a tiny marker note in doctor only if migration happened.
					setConfigLoadWarning("")
				}
			}
		}
	}
	if err != nil {
		setConfigLoadWarning("")
		return applyEnvURLOverrides(cfg)
	}
	if unmarshalErr := json.Unmarshal(data, &cfg); unmarshalErr != nil {
		backup, backupErr := os.ReadFile(configBackupPath())
		if backupErr == nil && json.Unmarshal(backup, &cfg) == nil {
			setConfigLoadWarning(fmt.Sprintf("Recovered settings and team history from the last-known-good backup (%s).", configBackupPath()))
			_ = atomicWriteFile(path, backup, 0o644)
		} else {
			setConfigLoadWarning(fmt.Sprintf("Config file has invalid JSON (%s), and no valid backup was available. Using safe defaults until you save settings again.", path))
			cfg = defaultConfig()
		}
		// Surface once on stderr so everyday commands are not silent about bad config.
		printConfigLoadWarningOnce()
	} else {
		setConfigLoadWarning("")
	}
	normalizeConfig(&cfg)
	return applyEnvURLOverrides(cfg)
}

func applyEnvURLOverrides(cfg CliksConfig) CliksConfig {
	if apiURL := strings.TrimSpace(os.Getenv("CLIKS_API_URL")); apiURL != "" {
		cfg.APIURL = strings.TrimRight(apiURL, "/")
		if strings.TrimSpace(os.Getenv("CLIKS_WS_URL")) == "" {
			cfg.WSURL = toWSURL(cfg.APIURL)
		}
	}
	if wsURL := strings.TrimSpace(os.Getenv("CLIKS_WS_URL")); wsURL != "" {
		cfg.WSURL = wsURL
	}
	if usesPublicBackend(cfg) {
		cfg.BatchWindowMs = 500
	}
	return cfg
}

func saveConfig(cfg CliksConfig) error {
	normalizeConfig(&cfg)
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := atomicWriteFile(path, data, 0o644); err != nil {
		return err
	}
	if err := atomicWriteFile(configBackupPath(), data, 0o600); err != nil {
		return fmt.Errorf("save config backup: %w", err)
	}
	setConfigLoadWarning("")
	return nil
}

func rememberTeam(code string, name string) (CliksConfig, error) {
	cfg := loadConfig()
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return cfg, errors.New("team code cannot be empty")
	}
	if name == "" {
		name = teamNameForCode(cfg, code)
	}
	next := []TeamConfig{{Code: code, Name: name, LastJoinedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	for _, team := range cfg.Teams {
		if strings.EqualFold(team.Code, code) {
			continue
		}
		next = append(next, team)
	}
	if len(next) > 12 {
		next = next[:12]
	}
	cfg.Teams = next
	cfg.CurrentTeamCode = code
	return cfg, saveConfig(cfg)
}

func forgetTeam(code string) (CliksConfig, error) {
	cfg := loadConfig()
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return cfg, nil
	}
	cfg.Teams = filterTeams(cfg.Teams, code)
	if strings.EqualFold(cfg.CurrentTeamCode, code) {
		cfg.CurrentTeamCode = ""
		if len(cfg.Teams) > 0 {
			cfg.CurrentTeamCode = cfg.Teams[0].Code
		}
	}
	return cfg, saveConfig(cfg)
}

func teamNameForCode(cfg CliksConfig, code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, team := range cfg.Teams {
		if strings.EqualFold(team.Code, code) {
			return team.Name
		}
	}
	return ""
}

func teamLabel(cfg CliksConfig, code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return ""
	}
	return formatTeamLabel(teamNameForCode(cfg, code), code)
}

func formatTeamLabel(name string, code string) string {
	name = strings.TrimSpace(name)
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return name
	}
	if name == "" || strings.EqualFold(name, code) {
		return code
	}
	return fmt.Sprintf("%s (%s)", name, code)
}

func normalizeConfig(cfg *CliksConfig) {
	def := defaultConfig()
	switch cfg.Capture.Mode {
	case "isolated", "direct", "terminal":
	default:
		cfg.Capture = def.Capture
	}
	if !cfg.Notifications.Configured {
		cfg.Notifications = def.Notifications
	}
	switch cfg.PresenceStatus {
	case "available", "focus", "break", "dnd":
	default:
		cfg.PresenceStatus = def.PresenceStatus
	}
	switch cfg.Theme {
	case "ember", "ocean", "mono":
	default:
		cfg.Theme = def.Theme
	}
	if cfg.APIURL == "" {
		cfg.APIURL = def.APIURL
	}
	if cfg.WSURL == "" {
		cfg.WSURL = toWSURL(cfg.APIURL)
	}
	if cfg.BatchWindowMs == 0 {
		cfg.BatchWindowMs = def.BatchWindowMs
	}
	if usesPublicBackend(*cfg) {
		cfg.BatchWindowMs = 500
	}
	if cfg.Teams == nil {
		cfg.Teams = []TeamConfig{}
	}
	cfg.CurrentTeamCode = strings.ToUpper(strings.TrimSpace(cfg.CurrentTeamCode))
	for index := range cfg.Teams {
		cfg.Teams[index].Code = strings.ToUpper(strings.TrimSpace(cfg.Teams[index].Code))
	}
	cfg.Sharing.Keyboard = cfg.Sharing.Keyboard || (!cfg.Sharing.Keyboard && !cfg.Sharing.Mouse && cfg.BatchWindowMs == def.BatchWindowMs)
	if !cfg.Sharing.Keyboard && !cfg.Sharing.Mouse {
		cfg.Sharing = def.Sharing
	}
	if cfg.Listening.Volume == 0 {
		cfg.Listening.Volume = def.Listening.Volume
	}
	if cfg.Listening.Density == 0 {
		cfg.Listening.Density = def.Listening.Density
	}
	if !cfg.Listening.Keyboard && !cfg.Listening.Mouse {
		cfg.Listening.Keyboard = true
		cfg.Listening.Mouse = true
	}
	if cfg.Listening.Volume < 0 {
		cfg.Listening.Volume = 0
	}
	if cfg.Listening.Volume > 1 {
		cfg.Listening.Volume = 1
	}
	if cfg.Listening.Density < 0.15 {
		cfg.Listening.Density = 0.15
	}
	if cfg.Listening.Density > 1 {
		cfg.Listening.Density = 1
	}
	if cfg.Listening.ShuffleMinutes == 0 {
		cfg.Listening.ShuffleMinutes = def.Listening.ShuffleMinutes
	}
	if cfg.Listening.ShuffleMinutes < 1 {
		cfg.Listening.ShuffleMinutes = 1
	}
	if cfg.Listening.ShuffleMinutes > 60 {
		cfg.Listening.ShuffleMinutes = 60
	}
	switch cfg.Listening.Ambient {
	case "off", "rain", "fire", "cafe", "cloud", "contemplation", "downtempo":
	default:
		cfg.Listening.Ambient = def.Listening.Ambient
	}
	if cfg.Listening.AmbientVolume == 0 {
		cfg.Listening.AmbientVolume = def.Listening.AmbientVolume
	}
	cfg.Listening.AmbientVolume = clamp(cfg.Listening.AmbientVolume, 0.05, 0.6)
	if cfg.Solo.People == 0 {
		cfg.Solo = def.Solo
	}
	cfg.Solo.People = clampInt(cfg.Solo.People, 1, 12)
	if cfg.Solo.KeyboardVolume == 0 {
		cfg.Solo.KeyboardVolume = def.Solo.KeyboardVolume
	}
	if cfg.Solo.MouseVolume == 0 {
		cfg.Solo.MouseVolume = def.Solo.MouseVolume
	}
	cfg.Solo.KeyboardVolume = clamp(cfg.Solo.KeyboardVolume, 0.05, 1)
	cfg.Solo.MouseVolume = clamp(cfg.Solo.MouseVolume, 0.05, 1)
	cfg.Nickname = sanitizeNickname(cfg.Nickname)
	cfg.Listening.AudioDevice = strings.TrimSpace(cfg.Listening.AudioDevice)
}

func sanitizeNickname(value string) string {
	value = ansi.Strip(value)
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len([]rune(value)) <= 10 {
		return value
	}
	runes := []rune(value)
	return string(runes[:10])
}

func toWSURL(apiURL string) string {
	trimmed := strings.TrimRight(apiURL, "/")
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		if parsed.Scheme == "https" {
			parsed.Scheme = "wss"
		} else if parsed.Scheme == "http" {
			parsed.Scheme = "ws"
		}
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/ws"
		return parsed.String()
	}
	return strings.Replace(trimmed, "http", "ws", 1) + "/ws"
}

func getenvDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

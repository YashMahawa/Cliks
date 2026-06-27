package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

const productionAPIURL = "https://139.59.29.207.sslip.io"

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
}

type CliksConfig struct {
	APIURL          string          `json:"apiUrl"`
	WSURL           string          `json:"wsUrl"`
	CurrentTeamCode string          `json:"currentTeamCode,omitempty"`
	Nickname        string          `json:"nickname,omitempty"`
	KeepRunning     bool            `json:"keepRunning"`
	Teams           []TeamConfig    `json:"teams"`
	Sharing         SharingConfig   `json:"sharing"`
	Listening       ListeningConfig `json:"listening"`
	BatchWindowMs   int             `json:"batchWindowMs"`
}

func defaultConfig() CliksConfig {
	apiURL := getenvDefault("CLIKS_API_URL", productionAPIURL)
	wsURL := getenvDefault("CLIKS_WS_URL", toWSURL(apiURL))
	return CliksConfig{
		APIURL: apiURL,
		WSURL:  wsURL,
		Teams:  []TeamConfig{},
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
			DynamicPlacement:  false,
			ShuffleMinutes:    10,
		},
		BatchWindowMs: 500,
	}
}

func configPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "cliks", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cliks", "config.json")
	}
	return filepath.Join(home, ".config", "cliks", "config.json")
}

func loadConfig() CliksConfig {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	normalizeConfig(&cfg)
	if apiURL := strings.TrimSpace(os.Getenv("CLIKS_API_URL")); apiURL != "" {
		cfg.APIURL = strings.TrimRight(apiURL, "/")
		if strings.TrimSpace(os.Getenv("CLIKS_WS_URL")) == "" {
			cfg.WSURL = toWSURL(cfg.APIURL)
		}
	}
	if wsURL := strings.TrimSpace(os.Getenv("CLIKS_WS_URL")); wsURL != "" {
		cfg.WSURL = wsURL
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
	return os.WriteFile(path, data, 0o644)
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
	if cfg.APIURL == "" {
		cfg.APIURL = def.APIURL
	}
	if cfg.WSURL == "" {
		cfg.WSURL = toWSURL(cfg.APIURL)
	}
	if cfg.BatchWindowMs == 0 {
		cfg.BatchWindowMs = def.BatchWindowMs
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

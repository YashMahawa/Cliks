package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
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
}

type CliksConfig struct {
	APIURL          string          `json:"apiUrl"`
	WSURL           string          `json:"wsUrl"`
	CurrentTeamCode string          `json:"currentTeamCode,omitempty"`
	Nickname        string          `json:"nickname,omitempty"`
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
	cfg.Nickname = sanitizeNickname(cfg.Nickname)
}

func sanitizeNickname(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len([]rune(value)) <= 32 {
		return value
	}
	runes := []rune(value)
	return string(runes[:32])
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

package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

type configSettingMetadata struct {
	Key         string
	Category    string
	Label       string
	Description string
	ControlType string   // "toggle", "range", "select", "text"
	Options     []string // For "select"
	Min         float64  // For "range"
	Max         float64  // For "range"
	Step        float64  // For "range"
}

type SettingCategory struct {
	ID   string
	Name string
}

var settingCategories = []SettingCategory{
	{ID: "audio", Name: "Audio & Sound"},
	{ID: "privacy", Name: "Sharing & Privacy"},
	{ID: "appearance", Name: "Appearance & Spatial"},
	{ID: "solo", Name: "Solo Mode"},
	{ID: "system", Name: "System & Network"},
}

var configSettingCatalog = []configSettingMetadata{
	{Key: "autostart", Category: "system", Label: "Launch at login", Description: "service enable: connect the selected team after sign-in (on/off). CLI: cliks service enable|disable", ControlType: "toggle"},
	{Key: "keep.running", Category: "system", Label: "Keep Running", Description: "hand a live session off to the background service when the terminal closes (on/off). Related: cliks service start|stop", ControlType: "toggle"},
	{Key: "nickname", Category: "privacy", Label: "Nickname", Description: "display name shared with teammates (10 characters max)", ControlType: "text"},
	{Key: "name", Category: "privacy", Label: "Nickname", Description: "alias for nickname", ControlType: "text"},
	{Key: "volume", Category: "audio", Label: "Volume", Description: "local playback volume from 0 to 1", ControlType: "range", Min: 0, Max: 1, Step: 0.05},
	{Key: "density", Category: "audio", Label: "Density", Description: "fraction of activity sounds played, from 0.15 to 1", ControlType: "range", Min: 0.15, Max: 1, Step: 0.05},
	{Key: "hear.muted", Category: "audio", Label: "Muted", Description: "silence local playback (on/off)", ControlType: "toggle"},
	{Key: "hear.spatial", Category: "audio", Label: "Spatial audio", Description: "pan and attenuate teammates locally (on/off)", ControlType: "toggle"},
	{Key: "hear.fade", Category: "audio", Label: "Fatigue fade", Description: "soften sustained dense activity (on/off)", ControlType: "toggle"},
	{Key: "hear.keyboard", Category: "audio", Label: "Hear keyboard", Description: "play teammate keyboard activity (on/off)", ControlType: "toggle"},
	{Key: "hear.mouse", Category: "audio", Label: "Hear mouse", Description: "play teammate left/right clicks (on/off)", ControlType: "toggle"},
	{Key: "hear.self", Category: "audio", Label: "Self monitor", Description: "play your own captured activity locally (on/off)", ControlType: "toggle"},
	{Key: "ambient", Category: "audio", Label: "Room tone", Description: "private embedded soundscape with six CC0 choices", ControlType: "select", Options: []string{"off", "rain", "fire", "cafe", "cloud", "contemplation", "downtempo"}},
	{Key: "ambient.volume", Category: "audio", Label: "Room tone volume", Description: "personal room-tone volume from 0.05 to 0.6", ControlType: "range", Min: 0.05, Max: 0.60, Step: 0.05},
	{Key: "solo.people", Category: "solo", Label: "Solo coworkers", Description: "number of locally simulated coworkers from 1 to 12", ControlType: "range", Min: 1, Max: 12, Step: 1},
	{Key: "solo.keyboard", Category: "solo", Label: "Solo keyboard", Description: "simulate keyboard ambience in Solo Desk (on/off)", ControlType: "toggle"},
	{Key: "solo.mouse", Category: "solo", Label: "Solo mouse", Description: "simulate click ambience in Solo Desk (on/off)", ControlType: "toggle"},
	{Key: "solo.keyboardVolume", Category: "solo", Label: "Solo keyboard level", Description: "offline simulated keyboard level from 0.05 to 1", ControlType: "range", Min: 0.05, Max: 1, Step: 0.05},
	{Key: "solo.mouseVolume", Category: "solo", Label: "Solo click level", Description: "offline simulated mouse-click level from 0.05 to 1", ControlType: "range", Min: 0.05, Max: 1, Step: 0.05},
	{Key: "notifications", Category: "system", Label: "Notifications", Description: "show native alerts for quick signals (on/off)", ControlType: "toggle"},
	{Key: "notifications.sound", Category: "system", Label: "Signal sound", Description: "play subtle spatial cues for teammate signals (on/off)", ControlType: "toggle"},
	{Key: "presence", Category: "appearance", Label: "Presence", Description: "available, focus, break, or dnd", ControlType: "select", Options: []string{"available", "focus", "break", "dnd"}},
	{Key: "theme", Category: "appearance", Label: "Theme", Description: "ember, ocean, or mono terminal palette", ControlType: "select", Options: []string{"ember", "ocean", "forest", "sunset", "aurora", "mono"}},
	{Key: "capture.mode", Category: "privacy", Label: "Capture safety", Description: "isolated (recommended), terminal-only, or direct compatibility fallback", ControlType: "select", Options: []string{"isolated", "direct", "terminal"}},
	{Key: "share.keyboard", Category: "privacy", Label: "Share keyboard", Description: "send keyboard activity kind only (on/off)", ControlType: "toggle"},
	{Key: "share.mouse", Category: "privacy", Label: "Share mouse", Description: "send left/right click activity only (on/off)", ControlType: "toggle"},
	{Key: "spatial.dynamic", Category: "appearance", Label: "Dynamic circle", Description: "move active teammates closer locally (on/off)", ControlType: "toggle"},
	{Key: "spatial.shuffleMinutes", Category: "appearance", Label: "Shuffle minutes", Description: "dynamic placement refresh interval from 1 to 60", ControlType: "range", Min: 1, Max: 60, Step: 1},
	{Key: "batch.ms", Category: "system", Label: "Batch window", Description: "public relay: fixed 500 ms; self-hosted: 100 to 2000 ms", ControlType: "range", Min: 100, Max: 2000, Step: 50},
	{Key: "audio.device", Category: "audio", Label: "Audio device", Description: "player output device name, or default", ControlType: "text"},
	{Key: "api.url", Category: "system", Label: "Server", Description: "public/default or a self-hosted http(s) backend URL", ControlType: "text"},
	{Key: "ws.url", Category: "system", Label: "WebSocket URL", Description: "advanced relay WebSocket override", ControlType: "text"},
}

func settingsForCategory(categoryID string) []configSettingMetadata {
	var list []configSettingMetadata
	for _, item := range configSettingCatalog {
		if item.Category == categoryID {
			list = append(list, item)
		}
	}
	return list
}

func printSettingCatalog() {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tTUI LABEL\tDESCRIPTION")
	for _, setting := range configSettingCatalog {
		fmt.Fprintf(w, "%s\t%s\t%s\n", setting.Key, setting.Label, setting.Description)
	}
	_ = w.Flush()
}

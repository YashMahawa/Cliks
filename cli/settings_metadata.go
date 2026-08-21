package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

type configSettingMetadata struct {
	Key         string
	Label       string
	Description string
}

var configSettingCatalog = []configSettingMetadata{
	{Key: "autostart", Label: "Launch at login", Description: "service enable: connect the selected team after sign-in (on/off). CLI: cliks service enable|disable"},
	{Key: "keep.running", Label: "Keep Running", Description: "hand a live session off to the background service when the terminal closes (on/off). Related: cliks service start|stop"},
	{Key: "nickname", Label: "Nickname", Description: "display name shared with teammates (10 characters max)"},
	{Key: "name", Label: "Nickname", Description: "alias for nickname"},
	{Key: "volume", Label: "Volume", Description: "local playback volume from 0 to 1"},
	{Key: "density", Label: "Density", Description: "fraction of activity sounds played, from 0.15 to 1"},
	{Key: "hear.muted", Label: "Muted", Description: "silence local playback (on/off)"},
	{Key: "hear.spatial", Label: "Spatial audio", Description: "pan and attenuate teammates locally (on/off)"},
	{Key: "hear.fade", Label: "Fatigue fade", Description: "soften sustained dense activity (on/off)"},
	{Key: "hear.keyboard", Label: "Hear keyboard", Description: "play teammate keyboard activity (on/off)"},
	{Key: "hear.mouse", Label: "Hear mouse", Description: "play teammate left/right clicks (on/off)"},
	{Key: "hear.self", Label: "Self monitor", Description: "play your own captured activity locally (on/off)"},
	{Key: "ambient", Label: "Room tone", Description: "private embedded soundscape with six CC0 choices"},
	{Key: "ambient.volume", Label: "Room tone volume", Description: "personal room-tone volume from 0.05 to 0.6"},
	{Key: "solo.people", Label: "Solo coworkers", Description: "number of locally simulated coworkers from 1 to 12"},
	{Key: "solo.keyboard", Label: "Solo keyboard", Description: "simulate keyboard ambience in Solo Desk (on/off)"},
	{Key: "solo.mouse", Label: "Solo mouse", Description: "simulate click ambience in Solo Desk (on/off)"},
	{Key: "solo.keyboardVolume", Label: "Solo keyboard level", Description: "offline simulated keyboard level from 0.05 to 1"},
	{Key: "solo.mouseVolume", Label: "Solo click level", Description: "offline simulated mouse-click level from 0.05 to 1"},
	{Key: "notifications", Label: "Notifications", Description: "show native alerts for quick signals (on/off)"},
	{Key: "notifications.sound", Label: "Signal sound", Description: "play subtle spatial cues for teammate signals (on/off)"},
	{Key: "presence", Label: "Presence", Description: "available, focus, break, or dnd"},
	{Key: "theme", Label: "Theme", Description: "ember, ocean, or mono terminal palette"},
	{Key: "palette.accent", Label: "Custom Accent Color", Description: "inline hex color override (e.g. #F2A65A)"},
	{Key: "palette.dim", Label: "Custom Dim Color", Description: "inline hex color override (e.g. #A9A39A)"},
	{Key: "palette.warn", Label: "Custom Warn Color", Description: "inline hex color override (e.g. #FFB454)"},
	{Key: "palette.ok", Label: "Custom OK Color", Description: "inline hex color override (e.g. #55D98B)"},
	{Key: "palette.panel", Label: "Custom Panel Color", Description: "inline hex color override (e.g. #D97746)"},
	{Key: "palette.select", Label: "Custom Select Color", Description: "inline hex color override (e.g. #D97746)"},
	{Key: "palette.onPick", Label: "Custom OnPick Color", Description: "inline hex color override (e.g. #071013)"},
	{Key: "palette.second", Label: "Custom Second Color", Description: "inline hex color override (e.g. #FF7A7A)"},
	{Key: "palette.third", Label: "Custom Third Color", Description: "inline hex color override (e.g. #FFD166)"},
	{Key: "capture.mode", Label: "Capture safety", Description: "isolated (recommended), terminal-only, or direct compatibility fallback"},
	{Key: "share.keyboard", Label: "Share keyboard", Description: "send keyboard activity kind only (on/off)"},
	{Key: "share.mouse", Label: "Share mouse", Description: "send left/right click activity only (on/off)"},
	{Key: "spatial.dynamic", Label: "Dynamic circle", Description: "move active teammates closer locally (on/off)"},
	{Key: "spatial.shuffleMinutes", Label: "Shuffle minutes", Description: "dynamic placement refresh interval from 1 to 60"},
	{Key: "batch.ms", Label: "Batch window", Description: "public relay: fixed 500 ms; self-hosted: 100 to 2000 ms"},
	{Key: "audio.device", Label: "Audio device", Description: "player output device name, or default"},
	{Key: "api.url", Label: "Server", Description: "public/default or a self-hosted http(s) backend URL"},
	{Key: "ws.url", Label: "WebSocket URL", Description: "advanced relay WebSocket override"},
}

func printSettingCatalog() {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tTUI LABEL\tDESCRIPTION")
	for _, setting := range configSettingCatalog {
		fmt.Fprintf(w, "%s\t%s\t%s\n", setting.Key, setting.Label, setting.Description)
	}
	_ = w.Flush()
}

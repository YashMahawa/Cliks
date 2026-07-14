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
	{Key: "notifications", Label: "Notifications", Description: "show native notifications for direct waves (on/off)"},
	{Key: "notifications.sound", Label: "Notification sound", Description: "play sound with native wave notifications (on/off)"},
	{Key: "presence", Label: "Presence", Description: "available, focus, break, or dnd"},
	{Key: "theme", Label: "Theme", Description: "ember, ocean, or mono terminal palette"},
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

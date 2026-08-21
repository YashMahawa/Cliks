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
	{Key: "capture.mode", Label: "Capture safety", Description: "isolated (recommended), terminal-only, or direct compatibility fallback"},
	{Key: "share.keyboard", Label: "Share keyboard", Description: "send keyboard activity kind only (on/off)"},
	{Key: "share.mouse", Label: "Share mouse", Description: "send left/right click activity only (on/off)"},
	{Key: "spatial.dynamic", Label: "Dynamic circle", Description: "move active teammates closer locally (on/off)"},
	{Key: "spatial.shuffleMinutes", Label: "Shuffle minutes", Description: "dynamic placement refresh interval from 1 to 60"},
	{Key: "batch.ms", Label: "Batch window", Description: "public relay: fixed 500 ms; self-hosted: 100 to 2000 ms"},
	{Key: "audio.device", Label: "Audio device", Description: "player output device name, or default"},
	{Key: "api.url", Label: "Server", Description: "public/default or a self-hosted http(s) backend URL"},
	{Key: "ws.url", Label: "WebSocket URL", Description: "advanced relay WebSocket override"},
	{Key: "keymap.up", Label: "Keymap Up", Description: "shortcut binding for up navigation (e.g. up, k or w)"},
	{Key: "keymap.down", Label: "Keymap Down", Description: "shortcut binding for down navigation (e.g. down, j or s)"},
	{Key: "keymap.left", Label: "Keymap Left", Description: "shortcut binding for left navigation (e.g. left, h or a)"},
	{Key: "keymap.right", Label: "Keymap Right", Description: "shortcut binding for right navigation (e.g. right, l or d)"},
	{Key: "keymap.select", Label: "Keymap Select", Description: "shortcut binding for select/submit (e.g. enter, space)"},
	{Key: "keymap.back", Label: "Keymap Back", Description: "shortcut binding for back/close (e.g. esc, q)"},
	{Key: "keymap.help", Label: "Keymap Help", Description: "shortcut binding to toggle help overlay (e.g. ?)"},
	{Key: "keymap.signal1", Label: "Keymap Signal 1", Description: "shortcut binding for wave signal (e.g. 1)"},
	{Key: "keymap.signal2", Label: "Keymap Signal 2", Description: "shortcut binding for nice signal (e.g. 2)"},
	{Key: "keymap.signal3", Label: "Keymap Signal 3", Description: "shortcut binding for coffee signal (e.g. 3)"},
	{Key: "keymap.signal4", Label: "Keymap Signal 4", Description: "shortcut binding for celebrate signal (e.g. 4)"},
	{Key: "keymap.signal5", Label: "Keymap Signal 5", Description: "shortcut binding for break signal (e.g. 5)"},
	{Key: "keymap.presence", Label: "Keymap Presence", Description: "shortcut binding to cycle presence status (e.g. p)"},
	{Key: "keymap.volumeUp", Label: "Keymap Volume Up", Description: "shortcut binding to increase volume (e.g. up, +)"},
	{Key: "keymap.volumeDown", Label: "Keymap Volume Down", Description: "shortcut binding to decrease volume (e.g. down, -)"},
	{Key: "keymap.densityUp", Label: "Keymap Density Up", Description: "shortcut binding to increase density (e.g. right, ])"},
	{Key: "keymap.densityDown", Label: "Keymap Density Down", Description: "shortcut binding to decrease density (e.g. left, [)"},
	{Key: "keymap.toggleMute", Label: "Keymap Toggle Mute", Description: "shortcut binding to toggle mute (e.g. m)"},
	{Key: "keymap.toggleSpatial", Label: "Keymap Toggle Spatial", Description: "shortcut binding to toggle spatial audio (e.g. s)"},
	{Key: "keymap.toggleFatigue", Label: "Keymap Toggle Fatigue", Description: "shortcut binding to toggle fatigue fade (e.g. f)"},
	{Key: "keymap.livePreferences", Label: "Keymap Live Preferences", Description: "shortcut binding for live preferences (e.g. tab, shift+s)"},
	{Key: "keymap.stop", Label: "Keymap Stop", Description: "shortcut binding to stop session (e.g. x, ctrl+c)"},
	{Key: "keymap.formNext", Label: "Keymap Form Next", Description: "shortcut binding for next field (e.g. tab, down)"},
	{Key: "keymap.formPrev", Label: "Keymap Form Prev", Description: "shortcut binding for previous field (e.g. shift+tab, up)"},
	{Key: "keymap.formSubmit", Label: "Keymap Form Submit", Description: "shortcut binding to submit form (e.g. enter)"},
	{Key: "keymap.formCancel", Label: "Keymap Form Cancel", Description: "shortcut binding to cancel form (e.g. esc)"},
}

func printSettingCatalog() {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tTUI LABEL\tDESCRIPTION")
	for _, setting := range configSettingCatalog {
		fmt.Fprintf(w, "%s\t%s\t%s\n", setting.Key, setting.Label, setting.Description)
	}
	_ = w.Flush()
}

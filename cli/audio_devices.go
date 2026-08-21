package main

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type AudioDevice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

var (
	discoveredDevicesCache     []AudioDevice
	discoveredDevicesCacheTime time.Time
	discoveredDevicesMu         sync.Mutex
)

// DiscoverAudioDevices discovers available audio output devices across OS backends and players.
// It completes well within 500ms to maintain responsive UI rendering.
func DiscoverAudioDevices() []AudioDevice {
	return DiscoverAudioDevicesWithTimeout(400 * time.Millisecond)
}

func DiscoverAudioDevicesWithTimeout(timeout time.Duration) []AudioDevice {
	discoveredDevicesMu.Lock()
	if time.Since(discoveredDevicesCacheTime) < 2*time.Second && len(discoveredDevicesCache) > 0 {
		devices := append([]AudioDevice(nil), discoveredDevicesCache...)
		discoveredDevicesMu.Unlock()
		return devices
	}
	discoveredDevicesMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	devices := []AudioDevice{
		{ID: "default", Name: "Default (System Output)", IsDefault: true},
	}
	seen := map[string]bool{"default": true, "": true}

	addDevice := func(id, name string) {
		id = strings.TrimSpace(id)
		name = strings.TrimSpace(name)
		if id == "" || strings.EqualFold(id, "default") || strings.EqualFold(id, "auto") {
			return
		}
		if name == "" {
			name = id
		}
		key := strings.ToLower(id)
		if seen[key] {
			return
		}
		seen[key] = true
		devices = append(devices, AudioDevice{
			ID:        id,
			Name:      name,
			IsDefault: false,
		})
	}

	// 1. mpv device discovery if mpv is installed
	if _, err := exec.LookPath("mpv"); err == nil {
		discoverMPVDevices(ctx, addDevice)
	}

	// 2. OS specific discovery
	switch runtime.GOOS {
	case "linux":
		discoverLinuxDevices(ctx, addDevice)
	case "darwin":
		discoverDarwinDevices(ctx, addDevice)
	case "windows":
		discoverWindowsDevices(ctx, addDevice)
	}

	discoveredDevicesMu.Lock()
	discoveredDevicesCache = append([]AudioDevice(nil), devices...)
	discoveredDevicesCacheTime = time.Now()
	discoveredDevicesMu.Unlock()

	return devices
}

func IsAudioDeviceAvailable(device string) bool {
	device = strings.TrimSpace(device)
	if device == "" || strings.EqualFold(device, "default") {
		return true
	}
	devices := DiscoverAudioDevices()
	target := strings.ToLower(device)
	for _, d := range devices {
		if strings.EqualFold(d.ID, device) || strings.EqualFold(d.Name, device) {
			return true
		}
		if strings.Contains(strings.ToLower(d.ID), target) || strings.Contains(strings.ToLower(d.Name), target) {
			return true
		}
		if strings.Contains(target, strings.ToLower(d.ID)) || strings.Contains(target, strings.ToLower(d.Name)) {
			return true
		}
	}
	return false
}

func discoverMPVDevices(ctx context.Context, add func(id, name string)) {
	cmd := exec.CommandContext(ctx, "mpv", "--audio-device=help")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		return
	}
	parseMPVDeviceHelp(out.String(), add)
}

func parseMPVDeviceHelp(output string, add func(id, name string)) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "'") {
			continue
		}
		firstQuote := strings.Index(line, "'")
		if firstQuote == -1 {
			continue
		}
		secondQuote := strings.Index(line[firstQuote+1:], "'")
		if secondQuote == -1 {
			continue
		}
		secondQuote += firstQuote + 1
		id := line[firstQuote+1 : secondQuote]

		name := id
		firstParen := strings.Index(line[secondQuote:], "(")
		lastParen := strings.LastIndex(line[secondQuote:], ")")
		if firstParen != -1 && lastParen > firstParen {
			name = line[secondQuote+firstParen+1 : secondQuote+lastParen]
		}
		add(id, name)
	}
}

func discoverLinuxDevices(ctx context.Context, add func(id, name string)) {
	if _, err := exec.LookPath("pactl"); err == nil {
		cmd := exec.CommandContext(ctx, "pactl", "list", "sinks")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			parsePactlSinks(out.String(), add)
		} else {
			cmdShort := exec.CommandContext(ctx, "pactl", "list", "sinks", "short")
			out.Reset()
			cmdShort.Stdout = &out
			if err := cmdShort.Run(); err == nil {
				parsePactlSinksShort(out.String(), add)
			}
		}
	}

	if _, err := exec.LookPath("aplay"); err == nil {
		cmd := exec.CommandContext(ctx, "aplay", "-L")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			parseALSAOutput(out.String(), add)
		}
	}
}

func parsePactlSinks(output string, add func(id, name string)) {
	lines := strings.Split(output, "\n")
	var currentName, currentDesc string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Name:") {
			currentName = strings.TrimSpace(strings.TrimPrefix(trimmed, "Name:"))
		} else if strings.HasPrefix(trimmed, "Description:") {
			currentDesc = strings.TrimSpace(strings.TrimPrefix(trimmed, "Description:"))
		} else if strings.HasPrefix(trimmed, "Sink #") {
			if currentName != "" {
				add(currentName, currentDesc)
				currentName = ""
				currentDesc = ""
			}
		}
	}
	if currentName != "" {
		add(currentName, currentDesc)
	}
}

func parsePactlSinksShort(output string, add func(id, name string)) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			add(fields[1], fields[1])
		}
	}
}

func parseALSAOutput(output string, add func(id, name string)) {
	lines := strings.Split(output, "\n")
	var currentID string
	var currentDesc string
	for _, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if currentID != "" && currentID != "null" {
				add(currentID, currentDesc)
			}
			currentID = strings.TrimSpace(line)
			currentDesc = ""
		} else if currentID != "" && currentDesc == "" {
			currentDesc = strings.TrimSpace(line)
		}
	}
	if currentID != "" && currentID != "null" {
		add(currentID, currentDesc)
	}
}

func discoverDarwinDevices(ctx context.Context, add func(id, name string)) {
	if _, err := exec.LookPath("switchaudio-cli"); err == nil {
		cmd := exec.CommandContext(ctx, "switchaudio-cli", "-a", "-t", "output")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			for _, line := range strings.Split(out.String(), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					add("coreaudio/"+line, line)
					add(line, line)
				}
			}
		}
	}

	cmd := exec.CommandContext(ctx, "system_profiler", "SPAudioDataType")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		parseSystemProfilerAudio(out.String(), add)
	}
}

func parseSystemProfilerAudio(output string, add func(id, name string)) {
	lines := strings.Split(output, "\n")
	var currentDevice string
	isOutput := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, "Manufacturer:") && !strings.Contains(trimmed, "Default Output Device:") {
			if currentDevice != "" && isOutput {
				add("coreaudio/"+currentDevice, currentDevice)
				add(currentDevice, currentDevice)
			}
			currentDevice = strings.TrimSuffix(trimmed, ":")
			isOutput = false
		}
		if strings.Contains(trimmed, "Default Output Device: Yes") || strings.Contains(trimmed, "Output Channels:") {
			isOutput = true
		}
	}
	if currentDevice != "" && isOutput {
		add("coreaudio/"+currentDevice, currentDevice)
		add(currentDevice, currentDevice)
	}
}

func discoverWindowsDevices(ctx context.Context, add func(id, name string)) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", "Get-CimInstance Win32_SoundDevice | Select-Object -ExpandProperty Name")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		for _, line := range strings.Split(out.String(), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				add("wasapi/"+line, line)
				add(line, line)
			}
		}
	}
}

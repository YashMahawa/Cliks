package main

import (
	"testing"
	"time"
)

func TestDiscoverAudioDevices(t *testing.T) {
	start := time.Now()
	devices := DiscoverAudioDevices()
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("DiscoverAudioDevices took %v, expected < 500ms", elapsed)
	}

	if len(devices) == 0 {
		t.Fatalf("DiscoverAudioDevices returned empty slice, expected at least default device")
	}

	if devices[0].ID != "default" || !devices[0].IsDefault {
		t.Errorf("First device should be default, got %+v", devices[0])
	}
}

func TestIsAudioDeviceAvailable(t *testing.T) {
	if !IsAudioDeviceAvailable("default") {
		t.Errorf("IsAudioDeviceAvailable('default') should be true")
	}
	if !IsAudioDeviceAvailable("") {
		t.Errorf("IsAudioDeviceAvailable('') should be true")
	}
	if IsAudioDeviceAvailable("nonexistent_device_xyz_999") {
		t.Errorf("IsAudioDeviceAvailable('nonexistent_device_xyz_999') should be false")
	}
}

func TestParseMPVDeviceHelp(t *testing.T) {
	sampleOutput := `
List of enabled audio devices:
  'auto' (Autoselect device)
  'alsa/default' (Default (alsa))
  'alsa/hdmi:CARD=PCH,DEV=0' (HDA Intel PCH, HDMI 0 / HDMI Audio Output)
  'pulse/alsa_output.pci-0000_00_1f.3.analog-stereo' (Built-in Audio Analog Stereo)
`
	found := make(map[string]string)
	parseMPVDeviceHelp(sampleOutput, func(id, name string) {
		found[id] = name
	})

	if _, ok := found["alsa/hdmi:CARD=PCH,DEV=0"]; !ok {
		t.Errorf("Expected alsa/hdmi device in parsed mpv output, got map: %v", found)
	}
	if found["alsa/hdmi:CARD=PCH,DEV=0"] != "HDA Intel PCH, HDMI 0 / HDMI Audio Output" {
		t.Errorf("Unexpected name: %s", found["alsa/hdmi:CARD=PCH,DEV=0"])
	}
	if _, ok := found["pulse/alsa_output.pci-0000_00_1f.3.analog-stereo"]; !ok {
		t.Errorf("Expected pulse alsa_output device in parsed mpv output")
	}
}

func TestParsePactlSinks(t *testing.T) {
	sampleOutput := `
Sink #0
	State: SUSPENDED
	Name: alsa_output.pci-0000_00_1f.3.analog-stereo
	Description: Built-in Audio Analog Stereo
	Driver: module-alsa-card.c
`
	found := make(map[string]string)
	parsePactlSinks(sampleOutput, func(id, name string) {
		found[id] = name
	})

	if name, ok := found["alsa_output.pci-0000_00_1f.3.analog-stereo"]; !ok || name != "Built-in Audio Analog Stereo" {
		t.Errorf("Failed to parse pactl sinks output, got %v", found)
	}
}

func TestParseALSAOutput(t *testing.T) {
	sampleOutput := `
null
    Discard all samples (playback) or generate zero samples (capture)
default
    Default Audio Device
hdmi:CARD=PCH,DEV=0
    HDA Intel PCH, HDMI 0
    HDMI Audio Output
`
	found := make(map[string]string)
	parseALSAOutput(sampleOutput, func(id, name string) {
		found[id] = name
	})

	if name, ok := found["hdmi:CARD=PCH,DEV=0"]; !ok || name != "HDA Intel PCH, HDMI 0" {
		t.Errorf("Failed to parse ALSA output, got %v", found)
	}
}

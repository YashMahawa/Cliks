package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBootSettingCLIUpdatesConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	err := cmdSet([]string{
		"boot.delay", "12",
		"boot.capture.mode", "direct",
		"boot.audio.device", "hdmi-output",
		"boot.volume", "0.45",
	})
	if err != nil {
		t.Fatalf("cmdSet returned unexpected error: %v", err)
	}

	cfg := loadConfig()
	if cfg.Boot.DelaySec != 12 {
		t.Errorf("expected Boot.DelaySec 12, got %d", cfg.Boot.DelaySec)
	}
	if cfg.Boot.CaptureMode != "direct" {
		t.Errorf("expected Boot.CaptureMode 'direct', got %q", cfg.Boot.CaptureMode)
	}
	if cfg.Boot.AudioDevice != "hdmi-output" {
		t.Errorf("expected Boot.AudioDevice 'hdmi-output', got %q", cfg.Boot.AudioDevice)
	}
	if cfg.Boot.Volume != 0.45 || !cfg.Boot.VolumeConfigured {
		t.Errorf("expected Boot.Volume 0.45 (configured), got %f (configured=%v)", cfg.Boot.Volume, cfg.Boot.VolumeConfigured)
	}
}

func TestLauncherTemplatesIncludeBootEnvVars(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("APPDATA", tempDir)

	cfg := loadConfig()
	cfg.CurrentTeamCode = "CLIK-TEST01"
	cfg.Boot.DelaySec = 15
	cfg.Boot.CaptureMode = "direct"
	cfg.Boot.AudioDevice = "usb-headset"
	cfg.Boot.Volume = 0.35
	cfg.Boot.VolumeConfigured = true
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}

	// 1. Linux systemd
	msgLinux, _ := linuxAutostart("enable", "CLIK-TEST01")
	if msgLinux == "" {
		t.Errorf("linuxAutostart enable returned empty message")
	}
	servicePath := filepath.Join(tempDir, "systemd", "user", serviceName+".service")
	serviceData, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("failed to read systemd service file: %v", err)
	}
	serviceContent := string(serviceData)
	if !strings.Contains(serviceContent, "Environment=CLIKS_BOOT_DELAY=15") {
		t.Errorf("systemd service missing CLIKS_BOOT_DELAY=15:\n%s", serviceContent)
	}
	if !strings.Contains(serviceContent, "Environment=CLIKS_BOOT_CAPTURE_MODE=direct") {
		t.Errorf("systemd service missing CLIKS_BOOT_CAPTURE_MODE=direct:\n%s", serviceContent)
	}
	if !strings.Contains(serviceContent, "Environment=CLIKS_BOOT_AUDIO_DEVICE=usb-headset") {
		t.Errorf("systemd service missing CLIKS_BOOT_AUDIO_DEVICE=usb-headset:\n%s", serviceContent)
	}
	if !strings.Contains(serviceContent, "Environment=CLIKS_BOOT_VOLUME=0.35") {
		t.Errorf("systemd service missing CLIKS_BOOT_VOLUME=0.35:\n%s", serviceContent)
	}

	// 2. macOS LaunchAgent plist
	_, _ = macAutostart("enable", "CLIK-TEST01")
	plistPath := filepath.Join(tempDir, "Library", "LaunchAgents", launchAgentID+".plist")
	plistData, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("failed to read LaunchAgent plist file: %v", err)
	}
	plistContent := string(plistData)
	if !strings.Contains(plistContent, "<key>CLIKS_BOOT_DELAY</key>\n    <string>15</string>") {
		t.Errorf("plist missing CLIKS_BOOT_DELAY 15:\n%s", plistContent)
	}
	if !strings.Contains(plistContent, "<key>CLIKS_BOOT_CAPTURE_MODE</key>\n    <string>direct</string>") {
		t.Errorf("plist missing CLIKS_BOOT_CAPTURE_MODE direct:\n%s", plistContent)
	}
	if !strings.Contains(plistContent, "<key>CLIKS_BOOT_AUDIO_DEVICE</key>\n    <string>usb-headset</string>") {
		t.Errorf("plist missing CLIKS_BOOT_AUDIO_DEVICE usb-headset:\n%s", plistContent)
	}
	if !strings.Contains(plistContent, "<key>CLIKS_BOOT_VOLUME</key>\n    <string>0.35</string>") {
		t.Errorf("plist missing CLIKS_BOOT_VOLUME 0.35:\n%s", plistContent)
	}

	// 3. Windows Startup VBScript
	_, _ = windowsAutostart("enable", "CLIK-TEST01")
	vbsPath := filepath.Join(tempDir, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Cliks.vbs")
	vbsData, err := os.ReadFile(vbsPath)
	if err != nil {
		t.Fatalf("failed to read VBScript file: %v", err)
	}
	vbsContent := string(vbsData)
	if !strings.Contains(vbsContent, `sh.Environment("Process")("CLIKS_BOOT_DELAY") = "15"`) {
		t.Errorf("VBScript missing CLIKS_BOOT_DELAY 15:\n%s", vbsContent)
	}
	if !strings.Contains(vbsContent, `sh.Environment("Process")("CLIKS_BOOT_CAPTURE_MODE") = "direct"`) {
		t.Errorf("VBScript missing CLIKS_BOOT_CAPTURE_MODE direct:\n%s", vbsContent)
	}
	if !strings.Contains(vbsContent, `sh.Environment("Process")("CLIKS_BOOT_AUDIO_DEVICE") = "usb-headset"`) {
		t.Errorf("VBScript missing CLIKS_BOOT_AUDIO_DEVICE usb-headset:\n%s", vbsContent)
	}
	if !strings.Contains(vbsContent, `sh.Environment("Process")("CLIKS_BOOT_VOLUME") = "0.35"`) {
		t.Errorf("VBScript missing CLIKS_BOOT_VOLUME 0.35:\n%s", vbsContent)
	}
}

func TestUpdatingBootSettingRegeneratesLauncherWhenAutostartEnabled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("APPDATA", tempDir)

	cfg := loadConfig()
	cfg.CurrentTeamCode = "CLIK-AUTO01"
	cfg.Boot.DelaySec = 5
	_ = saveConfig(cfg)

	var launcherPath string
	var expectedInit, expectedUpdated string

	switch runtime.GOOS {
	case "linux":
		_, _ = linuxAutostart("enable", "CLIK-AUTO01")
		launcherPath = filepath.Join(tempDir, "systemd", "user", serviceName+".service")
		expectedInit = "CLIKS_BOOT_DELAY=5"
		expectedUpdated = "CLIKS_BOOT_DELAY=25"
	case "darwin":
		_, _ = macAutostart("enable", "CLIK-AUTO01")
		launcherPath = filepath.Join(tempDir, "Library", "LaunchAgents", launchAgentID+".plist")
		expectedInit = "<string>5</string>"
		expectedUpdated = "<string>25</string>"
	case "windows":
		_, _ = windowsAutostart("enable", "CLIK-AUTO01")
		launcherPath = filepath.Join(tempDir, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Cliks.vbs")
		expectedInit = `CLIKS_BOOT_DELAY") = "5"`
		expectedUpdated = `CLIKS_BOOT_DELAY") = "25"`
	default:
		t.Skip("Unsupported OS for autostart test")
	}

	data1, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatalf("failed to read initial service/launcher file: %v", err)
	}
	if !strings.Contains(string(data1), expectedInit) {
		t.Fatalf("initial launcher file expected %q:\n%s", expectedInit, string(data1))
	}

	// Now update boot setting via CLI set command
	err = cmdSet([]string{"boot.delay", "25"})
	if err != nil {
		t.Fatalf("cmdSet boot.delay 25 failed: %v", err)
	}

	// Verify launcher file was automatically regenerated with delay=25
	data2, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatalf("failed to read updated launcher file: %v", err)
	}
	if !strings.Contains(string(data2), expectedUpdated) {
		t.Errorf("regenerated launcher file missing updated %q:\n%s", expectedUpdated, string(data2))
	}
}

func TestBootModeOverridesAndDelay(t *testing.T) {
	t.Setenv("CLIKS_RUN_MODE", "boot")
	t.Setenv("CLIKS_BOOT_DELAY", "3")
	t.Setenv("CLIKS_BOOT_CAPTURE_MODE", "direct")
	t.Setenv("CLIKS_BOOT_AUDIO_DEVICE", "custom-dac")
	t.Setenv("CLIKS_BOOT_VOLUME", "0.6")

	cfg := defaultConfig()
	opts := StartOptions{}

	delay := applyBootModeOverrides(&cfg, &opts)

	if delay != 3 {
		t.Errorf("expected delay 3, got %d", delay)
	}
	if cfg.Capture.Mode != "direct" || opts.CaptureMode != "direct" {
		t.Errorf("expected capture mode 'direct', got cfg=%q opts=%q", cfg.Capture.Mode, opts.CaptureMode)
	}
	if cfg.Listening.AudioDevice != "custom-dac" {
		t.Errorf("expected audio device 'custom-dac', got %q", cfg.Listening.AudioDevice)
	}
	if cfg.Listening.Volume != 0.6 {
		t.Errorf("expected volume 0.6, got %f", cfg.Listening.Volume)
	}
}

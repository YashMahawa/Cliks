package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type setupStep struct {
	title   string
	status  string // ok | fixed | tip | action
	detail  string
	command string
}

// cmdSetup makes a fresh machine ready for capture + spatial sound with
// plain-language guidance. It auto-applies every safe fix and only asks the
// user for OS permission dialogs that apps cannot skip (macOS Input Monitoring).
func cmdSetup(args []string) error {
	quiet := false
	for _, arg := range args {
		if arg == "--quiet" || arg == "-q" {
			quiet = true
		}
	}
	steps := runSetupChecks(!quiet)
	if quiet {
		return nil
	}
	printSetupReport(steps)
	if setupIsReady(steps) {
		fmt.Println()
		fmt.Println("You're set. Next:")
		fmt.Println("  1. Create a team on the Cliks website (or: cliks create)")
		fmt.Println("  2. cliks join CLIK-XXXXXX")
		fmt.Println("  3. Type and click anywhere — teammates hear soft ambience, never your keys.")
		return nil
	}
	fmt.Println()
	fmt.Println("Almost there. Finish any \"action\" steps above, then run: cliks setup")
	return nil
}

func runSetupChecks(verifySound bool) []setupStep {
	cfg := loadConfig()
	steps := []setupStep{}
	if warning := lastConfigLoadWarning(); warning != "" {
		steps = append(steps, setupStep{
			title:  "Saved settings",
			status: "tip",
			detail: warning + " Open Preferences and save once to write a clean file.",
		})
	}
	steps = append(steps, ensureAudioReady()...)
	steps = append(steps, ensureCaptureReady()...)
	if ready, detail := nativeNotificationStatus(); ready {
		steps = append(steps, setupStep{title: "Native notifications", status: "ok", detail: "Quick signals can use " + detail + ". You control banners and their sound separately in Preferences."})
	} else {
		steps = append(steps, setupStep{title: "Native notifications", status: "tip", detail: "Optional and currently off: " + detail + "."})
	}
	if msg := repairAutostartIfEnabled(); msg != "" {
		steps = append(steps, setupStep{
			title:  "Launch at login",
			status: "fixed",
			detail: msg,
		})
	}
	if verifySound {
		steps = append(steps, verifySoundSamples(cfg))
	}
	return steps
}

func setupReportLines(steps []setupStep) []string {
	lines := []string{"Cliks setup", ""}
	for _, step := range steps {
		badge := "[ ]"
		switch step.status {
		case "ok":
			badge = "[ok]"
		case "fixed":
			badge = "[fixed]"
		case "tip":
			badge = "[tip]"
		case "action":
			badge = "[do]"
		}
		lines = append(lines, fmt.Sprintf("%s %s", badge, step.title))
		if step.detail != "" {
			lines = append(lines, "    "+step.detail)
		}
		if step.command != "" {
			lines = append(lines, "    "+step.command)
		}
	}
	return lines
}

func setupSummaryMessage(steps []setupStep) string {
	if setupIsReady(steps) {
		return "Setup complete. Sound and capture look ready."
	}
	actions := 0
	for _, step := range steps {
		if step.status == "action" {
			actions++
		}
	}
	if actions == 1 {
		return "1 easy step left — see the report."
	}
	return fmt.Sprintf("%d easy steps left — see the report.", actions)
}

func ensureAudioReady() []setupStep {
	player, spatial, _, commands := getAudioPlayerStatus()
	if player != "" && spatial {
		return []setupStep{{
			title:  "Spatial sound",
			status: "ok",
			detail: fmt.Sprintf("Ready with %s (stereo pan + distance).", player),
		}}
	}
	if player != "" {
		step := setupStep{
			title:  "Spatial sound",
			status: "tip",
			detail: fmt.Sprintf("Sound works with %s (distance only). Install mpv for left/right teammate placement.", player),
		}
		if len(commands) > 0 {
			step.command = commands[0]
		}
		if fixed, msg := tryInstallMpv(); fixed {
			player2, spatial2, _, _ := getAudioPlayerStatus()
			if spatial2 {
				return []setupStep{{
					title:  "Spatial sound",
					status: "fixed",
					detail: fmt.Sprintf("Installed mpv. Stereo spatial sound is ready (%s).", player2),
				}}
			}
			return []setupStep{{
				title:  "Spatial sound",
				status: "tip",
				detail: msg + " Sound still works with the built-in player.",
			}}
		}
		return []setupStep{step}
	}
	if fixed, msg := tryInstallMpv(); fixed {
		player2, spatial2, _, _ := getAudioPlayerStatus()
		if player2 != "" {
			detail := fmt.Sprintf("Installed audio support (%s).", player2)
			if spatial2 {
				detail = fmt.Sprintf("Installed mpv. Stereo spatial sound is ready (%s).", player2)
			}
			return []setupStep{{title: "Spatial sound", status: "fixed", detail: detail}}
		}
		return []setupStep{{title: "Spatial sound", status: "tip", detail: msg}}
	}
	step := setupStep{
		title:  "Spatial sound",
		status: "action",
		detail: audioInstallHint(),
	}
	if len(commands) > 0 {
		step.command = strings.Join(commands, " && ")
	}
	return []setupStep{step}
}

func tryInstallMpv() (bool, string) {
	if _, err := exec.LookPath("mpv"); err == nil {
		return false, "mpv already available"
	}
	switch runtime.GOOS {
	case "darwin":
		return false, "Cliks already includes stereo audio on macOS; mpv is optional only for advanced output-device routing."
	case "linux":
		return tryLinuxInstallMpv()
	case "windows":
		return false, "Cliks already includes stereo audio on Windows; mpv is optional only for advanced output-device routing."
	default:
		return false, "Automatic audio install is not available on this platform."
	}
}

func tryLinuxInstallMpv() (bool, string) {
	managers := [][]string{
		{"pacman", "-S", "--needed", "--noconfirm", "mpv"},
		{"apt-get", "install", "-y", "mpv"},
		{"apt", "install", "-y", "mpv"},
		{"dnf", "install", "-y", "mpv"},
		{"zypper", "install", "-y", "mpv"},
		{"apk", "add", "mpv"},
	}
	run := func(name string, args ...string) bool {
		cmd := exec.Command(name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run() == nil
	}
	for _, args := range managers {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		if run(args[0], args[1:]...) || run("sudo", append([]string{"-n"}, args...)...) {
			if _, err := exec.LookPath("mpv"); err == nil {
				return true, "Installed mpv."
			}
		}
	}
	return false, "Could not install mpv automatically (sudo may be required). Run: sudo apt install mpv"
}

func ensureCaptureReady() []setupStep {
	return platformCaptureSetup()
}

func verifySoundSamples(cfg CliksConfig) setupStep {
	player, _, _, _ := getAudioPlayerStatus(cfg.Listening.AudioDevice)
	if player == "" {
		return setupStep{title: "Sound check", status: "tip", detail: "Skipped until an audio player is available. Later: cliks sound-test"}
	}
	return setupStep{
		title:  "Sound check",
		status: "ok",
		detail: fmt.Sprintf("Player %s is ready. Optional: cliks sound-test", player),
	}
}

func printSetupReport(steps []setupStep) {
	for _, line := range setupReportLines(steps) {
		fmt.Println(line)
	}
}

func setupIsReady(steps []setupStep) bool {
	for _, step := range steps {
		if step.status == "action" {
			return false
		}
	}
	return true
}

// openURLBestEffort opens a URL or system settings pane without failing setup.
func openURLBestEffort(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", target)
		}
	}
	if cmd == nil {
		return
	}
	_ = cmd.Start()
	time.AfterFunc(50*time.Millisecond, func() {})
}

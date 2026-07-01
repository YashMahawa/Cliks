package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type doctorIssue struct {
	title    string
	detail   string
	commands []string
}

type doctorCheck struct {
	label  string
	status string
}

type doctorReport struct {
	privacy        []string
	checks         []doctorCheck
	recommendation []string
	issues         []doctorIssue
}

func buildDoctorReport(cfg CliksConfig) doctorReport {
	report := doctorReport{privacy: []string{
		"Cliks sends only event kind: keyboard or mouse.",
		"Cliks relays coarse timing offsets inside each 500ms batch.",
		"Cliks does not send key values, key codes, words, coordinates, windows, or app names.",
	}}
	report.checks = append(report.checks, doctorCheck{"Runtime", "Go " + runtime.Version()})

	player, spatial, hint, commands := getAudioPlayerStatus(cfg.Listening.AudioDevice)
	if player != "" {
		report.checks = append(report.checks, doctorCheck{"Audio player", "ok (" + player + ")"})
		if cfg.Listening.AudioDevice != "" {
			if hint == "" {
				report.checks = append(report.checks, doctorCheck{"Audio output", cfg.Listening.AudioDevice})
			} else {
				report.issues = append(report.issues, doctorIssue{"Choose a supported audio output", hint, []string{"cliks set audio.device default"}})
			}
		}
		if spatial {
			report.checks = append(report.checks, doctorCheck{"Spatial audio", "stereo pan + distance"})
		} else {
			report.checks = append(report.checks, doctorCheck{"Spatial audio", "distance volume only; install ffplay or mpv for stereo panning"})
		}
	} else {
		report.checks = append(report.checks,
			doctorCheck{"Audio player", "missing"},
			doctorCheck{"Spatial audio", "unavailable until an audio player is installed"},
		)
		report.issues = append(report.issues, doctorIssue{"Install an audio playback tool", hint, append(commands, "cliks sound-test")})
	}

	report.checks = append(report.checks,
		doctorCheck{"Platform", runtime.GOOS},
		doctorCheck{"Current team", valuePlain(cfg.CurrentTeamCode, "not joined")},
		doctorCheck{"Sharing keyboard", yesNo(cfg.Sharing.Keyboard)},
		doctorCheck{"Sharing mouse", yesNo(cfg.Sharing.Mouse)},
	)
	if cfg.CurrentTeamCode == "" {
		report.issues = append(report.issues, doctorIssue{"Join a team", "Cliks does not have a selected team code.", []string{"cliks join CLIK-XXXXXX"}})
	}
	if !cfg.Sharing.Keyboard {
		report.issues = append(report.issues, doctorIssue{"Turn keyboard sharing on", "Your keyboard activity will not reach teammates while share.keyboard is off.", []string{"cliks set share.keyboard on"}})
	}
	if !cfg.Sharing.Mouse {
		report.issues = append(report.issues, doctorIssue{"Turn mouse sharing on", "Your mouse activity will not reach teammates while share.mouse is off.", []string{"cliks set share.mouse on"}})
	}

	if runtime.GOOS == "linux" {
		input := linuxInputStatus()
		report.checks = append(report.checks, doctorCheck{"Linux input devices", yesNo(input.hasInputDir)})
		if input.hasInputDir {
			report.checks = append(report.checks,
				doctorCheck{"Readable event devices", fmt.Sprintf("%d/%d", input.readableCount, input.eventCount)},
				doctorCheck{"Active input group", yesNo(input.inputGroupActive)},
			)
		}
		switch {
		case !input.hasInputDir:
			report.issues = append(report.issues, doctorIssue{"Global capture is unavailable here", "Cliks cannot see /dev/input. This is normal in containers, SSH sessions, and locked-down environments.", []string{"Use a normal desktop terminal", "cliks start --terminal --self"}})
		case input.eventCount == 0:
			report.issues = append(report.issues, doctorIssue{"No input event devices found", "Cliks found /dev/input, but no /dev/input/event* devices.", []string{"ls -l /dev/input", "Try again from the real desktop session"}})
		case input.readableCount == 0:
			report.issues = append(report.issues, doctorIssue{"Allow Cliks to read input events", "Linux global capture needs permission to read /dev/input/event*. Cliks still sends only event type and timing, never key values.", []string{"sudo usermod -aG input " + input.username, "Log out and back in, or reboot", "cliks doctor"}})
		}
		if input.readableCount > 0 {
			report.recommendation = []string{"Recommended run command:", "cliks start --evdev"}
		} else {
			report.recommendation = []string{"Recommended local test:", "cliks start --terminal --self"}
		}
	} else if runtime.GOOS == "darwin" {
		report.issues = append(report.issues, doctorIssue{"Allow Accessibility permission", "macOS global input capture needs Accessibility permission for the terminal app. Native capture is still being polished in the Go CLI.", []string{"Open System Settings > Privacy & Security > Accessibility", "Allow your terminal app", "cliks capture-test"}})
	} else if runtime.GOOS == "windows" {
		report.recommendation = []string{"Recommended run command:", "cliks start"}
	}
	return report
}

func doctorReportLines(report doctorReport) []string {
	lines := []string{"Cliks doctor", "", "Privacy:"}
	for _, item := range report.privacy {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "System:")
	for _, check := range report.checks {
		lines = append(lines, fmt.Sprintf("- %s: %s", check.label, check.status))
	}
	if len(report.recommendation) > 0 {
		lines = append(lines, "", report.recommendation[0])
		for _, command := range report.recommendation[1:] {
			lines = append(lines, "  "+command)
		}
	}
	lines = append(lines, "")
	if len(report.issues) == 0 {
		return append(lines, "No blocking issues detected.", "Test playback: cliks sound-test")
	}
	lines = append(lines, "Fixes:")
	for _, issue := range report.issues {
		lines = append(lines, "", issue.title+":", "  "+issue.detail)
		for _, command := range issue.commands {
			lines = append(lines, "  "+command)
		}
	}
	return lines
}

func doctorSummary(report doctorReport) string {
	if len(report.issues) == 0 {
		return "Doctor OK. No blocking issues detected."
	}
	if len(report.issues) == 1 {
		return "Found 1 setup item. Review the steps below."
	}
	return fmt.Sprintf("Found %d setup items. Review the steps below.", len(report.issues))
}

func passiveDoctorWarning(report doctorReport) string {
	var titles []string
	for _, issue := range report.issues {
		if issue.title == "Join a team" || issue.title == "Turn keyboard sharing on" || issue.title == "Turn mouse sharing on" {
			continue
		}
		titles = append(titles, issue.title)
		if len(titles) == 2 {
			break
		}
	}
	if len(titles) == 0 {
		return ""
	}
	return "Setup note: " + strings.Join(titles, "; ") + ". Open Diagnostics in Cliks or run cliks doctor."
}

func runDoctor() error {
	fmt.Println(strings.Join(doctorReportLines(buildDoctorReport(loadConfig())), "\n"))
	return nil
}

type inputStatus struct {
	hasInputDir      bool
	eventCount       int
	readableCount    int
	inputGroupActive bool
	username         string
}

func linuxInputStatus() inputStatus {
	username := getenvDefault("USER", getenvDefault("LOGNAME", "$USER"))
	status := inputStatus{username: username}
	entries, err := os.ReadDir("/dev/input")
	if err != nil {
		return status
	}
	status.hasInputDir = true
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "event") {
			continue
		}
		status.eventCount++
		if file, err := os.Open(filepath.Join("/dev/input", entry.Name())); err == nil {
			status.readableCount++
			_ = file.Close()
		}
	}
	groupBytes, err := os.ReadFile("/etc/group")
	if err == nil {
		for _, line := range strings.Split(string(groupBytes), "\n") {
			if strings.HasPrefix(line, "input:") {
				parts := strings.Split(line, ":")
				if len(parts) > 2 {
					gid, _ := strconv.Atoi(parts[2])
					groups, _ := os.Getgroups()
					for _, group := range groups {
						if group == gid {
							status.inputGroupActive = true
						}
					}
				}
			}
		}
	}
	return status
}

func runSoundTest() error {
	audio := newAudioEngine(loadConfig().Listening)
	defer audio.Close()
	if err := audio.preview(); err != nil {
		return err
	}
	fmt.Println("Played keyboard, keyboard, mouse, mouse test clicks.")
	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func valuePlain(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func hasCommand(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

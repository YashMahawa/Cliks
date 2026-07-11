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
	return buildDoctorReportOptions(cfg, true)
}

// buildDoctorReportOptions builds diagnostics. When thorough is false, skip probes that
// start global hooks so passive startup notices stay instant.
func buildDoctorReportOptions(cfg CliksConfig, thorough bool) doctorReport {
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
			report.checks = append(report.checks, doctorCheck{"Spatial audio", "distance only — run cliks setup (installs mpv for stereo pan)"})
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

	appendPlatformCaptureChecks(&report, thorough)
	if warning := lastConfigLoadWarning(); warning != "" {
		report.issues = append([]doctorIssue{{
			title:    "Fix or reset saved settings",
			detail:   warning,
			commands: []string{"Open Preferences in Cliks and press s to save", "or delete the broken config file and run cliks setup"},
		}}, report.issues...)
		report.checks = append(report.checks, doctorCheck{"Config file", "invalid JSON — using defaults"})
	} else {
		report.checks = append(report.checks, doctorCheck{"Config file", configPath()})
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
		return append(lines, "All good. Optional: cliks sound-test", "If something ever feels off: cliks setup")
	}
	lines = append(lines, "Easy fixes (or run: cliks setup):")
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
		return "All good. Sound and capture look ready."
	}
	if len(report.issues) == 1 {
		return "1 easy setup step left. Prefer: cliks setup"
	}
	return fmt.Sprintf("%d easy setup steps left. Prefer: cliks setup", len(report.issues))
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

func quickDoctorWarning(cfg CliksConfig) string {
	return passiveDoctorWarning(buildDoctorReportOptions(cfg, false))
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

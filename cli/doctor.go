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

func runDoctor() error {
	cfg := loadConfig()
	var issues []doctorIssue
	fmt.Println("Cliks doctor")
	fmt.Println("")
	fmt.Println("Privacy:")
	fmt.Println("- Cliks sends only event kind: keyboard or mouse.")
	fmt.Println("- Cliks relays coarse timing offsets inside each 500ms batch.")
	fmt.Println("- Cliks does not send key values, key codes, words, coordinates, windows, or app names.")
	fmt.Println("")
	fmt.Println("System:")
	fmt.Printf("- Runtime: Go %s\n", runtime.Version())
	player, spatial, hint, commands := getAudioPlayerStatus()
	if player != "" {
		fmt.Printf("- Audio player: ok (%s)\n", player)
		if spatial {
			fmt.Println("- Spatial audio: stereo pan + distance")
		} else {
			fmt.Println("- Spatial audio: distance volume only")
			fmt.Println("  For stereo panning, install ffplay or mpv.")
		}
	} else {
		fmt.Println("- Audio player: missing")
		issues = append(issues, doctorIssue{"Install an audio playback tool", hint, append(commands, "cliks sound-test")})
	}
	fmt.Printf("- Platform: %s\n", runtime.GOOS)
	fmt.Printf("- Current team: %s\n", valuePlain(cfg.CurrentTeamCode, "not joined"))
	fmt.Printf("- Sharing keyboard: %s\n", yesNo(cfg.Sharing.Keyboard))
	fmt.Printf("- Sharing mouse: %s\n", yesNo(cfg.Sharing.Mouse))
	if cfg.CurrentTeamCode == "" {
		issues = append(issues, doctorIssue{"Join a team", "Cliks does not have a selected team code.", []string{"cliks join CLIK-XXXXXX"}})
	}
	if !cfg.Sharing.Keyboard {
		issues = append(issues, doctorIssue{"Turn keyboard sharing on", "Your keyboard activity will not reach teammates while share.keyboard is off.", []string{"cliks set share.keyboard on"}})
	}
	if !cfg.Sharing.Mouse {
		issues = append(issues, doctorIssue{"Turn mouse sharing on", "Your mouse activity will not reach teammates while share.mouse is off.", []string{"cliks set share.mouse on"}})
	}
	if runtime.GOOS == "linux" {
		input := linuxInputStatus()
		fmt.Printf("- Linux input devices: %s\n", yesNo(input.hasInputDir))
		if input.hasInputDir {
			fmt.Printf("- Readable event devices: %d/%d\n", input.readableCount, input.eventCount)
			fmt.Printf("- Active input group: %s\n", yesNo(input.inputGroupActive))
		}
		if !input.hasInputDir {
			issues = append(issues, doctorIssue{"Global capture is unavailable here", "Cliks cannot see /dev/input. This is normal in containers, SSH sessions, and locked-down environments.", []string{"Use a normal desktop terminal", "cliks start --terminal --self"}})
		} else if input.eventCount == 0 {
			issues = append(issues, doctorIssue{"No input event devices found", "Cliks found /dev/input, but no /dev/input/event* devices.", []string{"ls -l /dev/input", "Try again from the real desktop session"}})
		} else if input.readableCount == 0 {
			issues = append(issues, doctorIssue{"Allow Cliks to read input events", "Linux global capture needs permission to read /dev/input/event*. Cliks still sends only event type and timing, never key values.", []string{"sudo usermod -aG input " + input.username, "Log out and back in, or reboot", "cliks doctor"}})
		}
		fmt.Println("")
		if input.readableCount > 0 {
			fmt.Println("Recommended run command:")
			fmt.Println("  cliks start --evdev")
		} else {
			fmt.Println("Recommended local test:")
			fmt.Println("  cliks start --terminal --self")
		}
	} else if runtime.GOOS == "darwin" {
		issues = append(issues, doctorIssue{"Allow Accessibility permission", "macOS global input capture needs Accessibility permission for the terminal app. Native capture is still being polished in the Go CLI.", []string{"Open System Settings > Privacy & Security > Accessibility", "Allow your terminal app", "cliks capture-test"}})
	} else if runtime.GOOS == "windows" {
		fmt.Println("")
		fmt.Println("Recommended run command:")
		fmt.Println("  cliks start")
	}
	fmt.Println("")
	if len(issues) == 0 {
		fmt.Println("No blocking issues detected.")
		fmt.Println("Test playback: cliks sound-test")
		return nil
	}
	fmt.Println("Fixes:")
	for _, issue := range issues {
		fmt.Println("")
		fmt.Println(issue.title + ":")
		fmt.Println("  " + issue.detail)
		for _, command := range issue.commands {
			fmt.Println("  " + command)
		}
	}
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

//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func platformCaptureSetup() []setupStep {
	input := linuxInputStatus()
	steps := []setupStep{}

	if !input.hasInputDir {
		return []setupStep{{
			title:  "Background capture",
			status: "tip",
			detail: "This environment has no /dev/input (container/SSH/remote). Use a normal desktop session, or run: cliks start --terminal --self for a local test.",
		}}
	}

	if input.readableCount > 0 {
		steps = append(steps, setupStep{
			title:  "Background capture",
			status: "ok",
			detail: fmt.Sprintf("Can read %d input device(s). Global keyboard/mouse ambience is ready.", input.readableCount),
		})
		return steps
	}

	// Try session ACL grant first — works immediately without logout.
	if granted, detail := tryLinuxInputACL(); granted {
		// Re-check.
		input = linuxInputStatus()
		if input.readableCount > 0 {
			steps = append(steps, setupStep{
				title:  "Background capture",
				status: "fixed",
				detail: detail,
			})
			// Also ensure permanent group membership when possible.
			if permanent, pdetail := tryLinuxInputGroup(); permanent {
				steps = append(steps, setupStep{
					title:  "Capture after reboot",
					status: "fixed",
					detail: pdetail,
				})
			} else {
				steps = append(steps, setupStep{
					title:  "Capture after reboot",
					status: "tip",
					detail: "Session access is ready. For permanent access after reboot: sudo usermod -aG input $USER then log out/in.",
					command: "sudo usermod -aG input " + input.username,
				})
			}
			return steps
		}
	}

	if permanent, detail := tryLinuxInputGroup(); permanent {
		steps = append(steps, setupStep{
			title:  "Background capture",
			status: "fixed",
			detail: detail + " Log out and back in once so the new group applies.",
		})
		return steps
	}

	// Sandbox / Wayland without access — gentle tip, not a scare.
	sandbox := os.Getenv("FLATPAK_ID") != "" || os.Getenv("container") != ""
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		sandbox = true
	}
	detail := "Cliks needs one-time permission to hear keyboard/mouse activity kinds (never key values)."
	if sandbox {
		detail = "Sandboxed session cannot read input devices. Install Cliks on your normal desktop user session."
	}
	steps = append(steps, setupStep{
		title:   "Background capture",
		status:  "action",
		detail:  detail,
		command: "sudo usermod -aG input " + input.username + " && echo 'Then log out and back in once.'",
	})
	steps = append(steps, setupStep{
		title:  "Meanwhile",
		status: "tip",
		detail: "You can still try the room with terminal-only capture: cliks start --terminal --self",
	})
	return steps
}

func tryLinuxInputACL() (bool, string) {
	username := currentUsername()
	if username == "" {
		return false, ""
	}
	// Need setfacl + sudo -n for noninteractive.
	if _, err := exec.LookPath("setfacl"); err != nil {
		return false, ""
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return false, ""
	}
	devices, _ := filepath.Glob("/dev/input/event*")
	if len(devices) == 0 {
		return false, ""
	}
	ok := 0
	for _, device := range devices {
		cmd := exec.Command("sudo", "-n", "setfacl", "-m", "u:"+username+":r", device)
		if err := cmd.Run(); err == nil {
			ok++
		}
	}
	if ok == 0 {
		return false, ""
	}
	return true, fmt.Sprintf("Granted read access to %d input device(s) for this session.", ok)
}

func tryLinuxInputGroup() (bool, string) {
	username := currentUsername()
	if username == "" {
		return false, ""
	}
	// Already in group?
	if inGroup, _ := userInGroup(username, "input"); inGroup {
		return true, "User is already in the input group."
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return false, ""
	}
	cmd := exec.Command("sudo", "-n", "usermod", "-aG", "input", username)
	if err := cmd.Run(); err != nil {
		return false, ""
	}
	return true, "Added your user to the input group for permanent global capture."
}

func currentUsername() string {
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("LOGNAME")); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

func userInGroup(username, groupName string) (bool, error) {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, groupName+":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			return false, nil
		}
		for _, member := range strings.Split(parts[3], ",") {
			if strings.TrimSpace(member) == username {
				return true, nil
			}
		}
	}
	// Also check supplementary groups of current process.
	if u, err := user.Current(); err == nil && u.Username == username {
		gids, _ := u.GroupIds()
		for _, gid := range gids {
			if g, err := user.LookupGroupId(gid); err == nil && g.Name == groupName {
				return true, nil
			}
		}
	}
	return false, nil
}

//go:build linux

package main

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"
)

func platformCaptureSetup() []setupStep {
	if isolatedLinuxCaptureReady() {
		return []setupStep{{
			title: "Private background capture", status: "ok",
			detail: "The isolated Cliks helper is ready. Your terminal and user account cannot read raw input devices.",
		}}
	}
	input := linuxInputStatus()
	if !input.hasInputDir {
		return []setupStep{{title: "Background capture", status: "tip", detail: "No desktop input devices are visible here (common in Termux, containers, and SSH). Terminal-only capture is still available."}}
	}
	detail := "Install the small root-owned Cliks capture helper. It can read input devices, but exposes only keyboard, left-click, and right-click activity kinds over a local socket."
	command := "Re-run the Cliks installer, or install cli/linux-capture-helper/cliks-capture.service with sudo"
	if input.readableCount > 0 || input.inputGroupActive {
		detail += " Your account currently has broad /dev/input access from an older setup. After the helper works, remove your user from the input group and log out/in."
	}
	return []setupStep{
		{title: "Private background capture", status: "action", detail: detail, command: command},
		{title: "Compatibility choices", status: "tip", detail: "Terminal-only is safest without installation. Direct mode is opt-in and broad: cliks set capture.mode direct. Undo it with: cliks set capture.mode isolated"},
	}
}

func isolatedLinuxCaptureReady() bool {
	conn, err := net.DialTimeout("unix", linuxCaptureSocket(), 250*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	line, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(line) == "ready"
}

// Kept for migration diagnostics only. Cliks no longer grants ACLs or adds the
// desktop user to input automatically.
func tryLinuxInputACL() (bool, string)   { return false, "disabled in favor of isolated capture" }
func tryLinuxInputGroup() (bool, string) { return false, "disabled in favor of isolated capture" }

func currentUsername() string {
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

func userInGroup(username, groupName string) (bool, error) {
	username = strings.TrimSpace(username)
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return false, nil
	}
	if username == "" {
		username = currentUsername()
	}

	// 1. Resolve via system Name Service Switch runtime APIs
	if u, err := user.Lookup(username); err == nil {
		if g, err := user.LookupGroup(groupName); err == nil {
			if u.Gid == g.Gid {
				return true, nil
			}
			if gids, err := u.GroupIds(); err == nil {
				for _, gid := range gids {
					if gid == g.Gid {
						return true, nil
					}
				}
			}
		} else if gids, err := u.GroupIds(); err == nil {
			for _, gid := range gids {
				if grp, err := user.LookupGroupId(gid); err == nil && grp.Name == groupName {
					return true, nil
				}
			}
		}
	} else if u, err := user.Current(); err == nil && (username == "" || u.Username == username) {
		if g, err := user.LookupGroup(groupName); err == nil {
			if u.Gid == g.Gid {
				return true, nil
			}
			if gids, err := u.GroupIds(); err == nil {
				for _, gid := range gids {
					if gid == g.Gid {
						return true, nil
					}
				}
			}
		}
	}

	// Also check process active groups (os.Getgroups)
	if processGids, err := os.Getgroups(); err == nil {
		if g, err := user.LookupGroup(groupName); err == nil {
			if targetGid, err := strconv.Atoi(g.Gid); err == nil {
				for _, pg := range processGids {
					if pg == targetGid {
						return true, nil
					}
				}
			}
		}
		for _, pg := range processGids {
			if grp, err := user.LookupGroupId(strconv.Itoa(pg)); err == nil && grp.Name == groupName {
				return true, nil
			}
		}
	}

	// 2. Fall back to command-line user lookup utilities
	var idCmd *exec.Cmd
	if username != "" {
		idCmd = exec.Command("id", "-Gn", username)
	} else {
		idCmd = exec.Command("id", "-Gn")
	}
	if out, err := idCmd.Output(); err == nil {
		for _, g := range strings.Fields(string(out)) {
			if g == groupName {
				return true, nil
			}
		}
	}

	if out, err := exec.Command("getent", "group", groupName).Output(); err == nil {
		line := strings.TrimSpace(string(out))
		parts := strings.Split(line, ":")
		if len(parts) >= 4 && parts[0] == groupName {
			if username != "" {
				for _, member := range strings.Split(parts[3], ",") {
					if strings.TrimSpace(member) == username {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

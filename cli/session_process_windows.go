//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type localStartProcess struct {
	PID     int
	Command string
}

var siblingProcessFinder = discoverSiblingStartProcesses

func findSiblingStartProcesses(excludePIDs ...int) []localStartProcess {
	return siblingProcessFinder(excludePIDs...)
}

func discoverSiblingStartProcesses(excludePIDs ...int) []localStartProcess {
	current := currentExecutable()
	if !shouldScanSiblingProcesses(current) {
		return nil
	}
	exclude := excludedPIDSet(excludePIDs...)
	script := "Get-CimInstance Win32_Process | ForEach-Object { if ($_.CommandLine) { \"$($_.ProcessId)`t$($_.CommandLine)\" } }"
	output, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil
	}
	var processes []localStartProcess
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || exclude[pid] {
			continue
		}
		command := strings.TrimSpace(parts[1])
		if isLocalStartCommandLine(command, current) {
			processes = append(processes, localStartProcess{PID: pid, Command: command})
		}
	}
	return processes
}

func excludedPIDSet(pids ...int) map[int]bool {
	exclude := map[int]bool{os.Getpid(): true}
	for _, pid := range pids {
		if pid > 0 {
			exclude[pid] = true
		}
	}
	return exclude
}

func shouldScanSiblingProcesses(current string) bool {
	base := strings.ToLower(filepath.Base(strings.Trim(current, "\"'")))
	return base == "cliks.exe" || strings.HasPrefix(base, "cliks-")
}

func isLocalStartCommandLine(command string, current string) bool {
	normalizedCommand := strings.ToLower(strings.ReplaceAll(command, "/", `\`))
	normalizedCurrent := strings.ToLower(strings.ReplaceAll(strings.Trim(current, "\"'"), "/", `\`))
	if !strings.Contains(normalizedCommand, normalizedCurrent) {
		return false
	}
	return strings.Contains(normalizedCommand, " start") || strings.Contains(normalizedCommand, " --")
}

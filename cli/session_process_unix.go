//go:build !windows

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
	if processes := discoverProcSiblingStarts(current, exclude); processes != nil {
		return processes
	}
	return discoverPSSiblingStarts(current, exclude)
}

func discoverProcSiblingStarts(current string, exclude map[int]bool) []localStartProcess {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var processes []localStartProcess
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || exclude[pid] {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		args := splitNullArgs(data)
		if isLocalStartCommand(args, current) {
			processes = append(processes, localStartProcess{PID: pid, Command: strings.Join(args, " ")})
		}
	}
	return processes
}

func discoverPSSiblingStarts(current string, exclude map[int]bool) []localStartProcess {
	output, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil
	}
	var processes []localStartProcess
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || exclude[pid] {
			continue
		}
		command := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		if isLocalStartCommand(fields[1:], current) {
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

func splitNullArgs(data []byte) []string {
	parts := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			args = append(args, part)
		}
	}
	return args
}

func shouldScanSiblingProcesses(current string) bool {
	base := strings.ToLower(filepath.Base(normalizeExecutablePath(current)))
	return base == "cliks" || strings.HasPrefix(base, "cliks-")
}

func isLocalStartCommand(args []string, current string) bool {
	if len(args) < 2 {
		return false
	}
	if !sameExecutable(args[0], current) {
		return false
	}
	first := args[1]
	return first == "start" || strings.HasPrefix(first, "-")
}

func sameExecutable(candidate string, current string) bool {
	candidate = normalizeExecutablePath(candidate)
	current = normalizeExecutablePath(current)
	if candidate == current {
		return true
	}
	if filepath.IsAbs(candidate) || filepath.IsAbs(current) {
		return false
	}
	return filepath.Base(candidate) == filepath.Base(current)
}

func normalizeExecutablePath(path string) string {
	path = strings.Trim(path, "\"'")
	path = strings.TrimSuffix(path, " (deleted)")
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

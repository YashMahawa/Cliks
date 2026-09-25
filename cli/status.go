package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func cmdStatus(args []string) error {
	cfg := loadConfig()
	fmt.Print(unifiedStatusText(cfg))
	return nil
}

func unifiedStatusText(cfg CliksConfig) string {
	active, activeOK := activeSession()
	autostartInfo := getAutostartDetails(cfg)

	var lines []string
	lines = append(lines, "Cliks Status Summary", "")

	// 1. Daemon Status
	if activeOK {
		daemonDetail := fmt.Sprintf("running (pid %d, %s)", active.PID, modeLabel(active.Mode))
		lines = append(lines, fmt.Sprintf("Daemon:     %s", daemonDetail))
	} else {
		lines = append(lines, "Daemon:     stopped")
	}

	// 2. Team
	code := cfg.CurrentTeamCode
	if activeOK && active.TeamCode != "" {
		code = active.TeamCode
	}
	if code != "" {
		name := teamNameForCode(cfg, code)
		lines = append(lines, fmt.Sprintf("Team:       %s", formatTeamLabel(name, code)))
	} else {
		lines = append(lines, "Team:       not joined")
	}

	// 3. Connection Status
	apiURL := valuePlain(cfg.APIURL, productionAPIURL)
	if activeOK {
		connStatus := valuePlain(active.ConnectionStatus, "starting")
		lines = append(lines, fmt.Sprintf("Connection: %s (%s)", connStatus, apiURL))
	} else {
		lines = append(lines, fmt.Sprintf("Connection: stopped (%s)", apiURL))
	}

	// 4. Autostart Status
	autostartState := "disabled"
	if autostartInfo.Enabled {
		autostartState = "enabled"
	}
	if autostartInfo.Label != "" && autostartInfo.Path != "" {
		lines = append(lines, fmt.Sprintf("Autostart:  %s (%s: %s)", autostartState, autostartInfo.Label, autostartInfo.Path))
	} else if autostartInfo.Label != "" {
		lines = append(lines, fmt.Sprintf("Autostart:  %s (%s)", autostartState, autostartInfo.Label))
	} else {
		lines = append(lines, fmt.Sprintf("Autostart:  %s", autostartState))
	}

	// 5. Active Users & Activity Metrics (if daemon is running)
	if activeOK {
		userLabel := "users"
		if active.ActiveCount == 1 {
			userLabel = "user"
		}
		lines = append(lines, fmt.Sprintf("Active:     %d %s", active.ActiveCount, userLabel))
		lines = append(lines, fmt.Sprintf("Activity:   %d captured, %d sent", active.LocalCapturedEvents, active.LocalSentEvents))
	}

	// 6. Log File
	logPath := filepath.Join(stateDir(), "background.log")
	lines = append(lines, fmt.Sprintf("Log:        %s", logPath))

	return strings.Join(lines, "\n") + "\n"
}

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("31")).Padding(0, 1)
	styleAccent   = lipgloss.NewStyle().Foreground(lipgloss.Color("38")).Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	stylePanel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("31")).Padding(1, 2)
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("31")).Bold(true)
)

type homeAction string

const (
	actionNone      homeAction = ""
	actionStart     homeAction = "start"
	actionDoctor    homeAction = "doctor"
	actionSoundTest homeAction = "sound-test"
)

type homeModel struct {
	cfg            CliksConfig
	cursor         int
	mode           string
	message        string
	action         homeAction
	settingsCursor int
	width          int
	height         int
}

func runHomeTUI(cfg CliksConfig) error {
	if !isInteractiveTerminal() {
		printHelp("cliks")
		return nil
	}
	model := homeModel{cfg: cfg, mode: "home", message: "Enter selects. Mouse wheel or arrows move. q quits."}
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	result, _ := finalModel.(homeModel)
	switch result.action {
	case actionStart:
		return startSession(result.cfg, StartOptions{CaptureMode: "auto", SelfMonitor: result.cfg.Listening.Self})
	case actionDoctor:
		return runDoctor()
	case actionSoundTest:
		return runSoundTest()
	default:
		return nil
	}
}

func (m homeModel) Init() tea.Cmd { return nil }

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp {
			m.move(-1)
		}
		if msg.Type == tea.MouseWheelDown {
			m.move(1)
		}
		if msg.Type == tea.MouseLeft && msg.Y >= 7 {
			if m.mode == "home" {
				m.cursor = clampInt(msg.Y-7, 0, len(homeItems())-1)
				return m.activate()
			}
			m.settingsCursor = clampInt(msg.Y-7, 0, len(settingsRows(m.cfg))-1)
			m.changeSetting(1)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if m.mode == "settings" {
				m.mode = "home"
				return m, nil
			}
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "left", "h":
			if m.mode == "settings" {
				m.changeSetting(-1)
			}
		case "right", "l":
			if m.mode == "settings" {
				m.changeSetting(1)
			}
		case "enter", " ":
			return m.activate()
		case "s":
			if m.mode == "settings" {
				if err := saveConfig(m.cfg); err != nil {
					m.message = err.Error()
				} else {
					m.message = "Saved settings."
				}
			}
		}
	}
	return m, nil
}

func (m homeModel) View() string {
	var body string
	if m.mode == "settings" {
		body = m.settingsView()
	} else {
		body = m.homeView()
	}
	return lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("Cliks"), body)
}

func (m *homeModel) move(delta int) {
	if m.mode == "settings" {
		m.settingsCursor = clampInt(m.settingsCursor+delta, 0, len(settingsRows(m.cfg))-1)
		return
	}
	m.cursor = clampInt(m.cursor+delta, 0, len(homeItems())-1)
}

func (m homeModel) activate() (tea.Model, tea.Cmd) {
	if m.mode == "settings" {
		m.changeSetting(1)
		return m, nil
	}
	item := homeItems()[m.cursor]
	switch item.key {
	case "start":
		m.action = actionStart
		return m, tea.Quit
	case "settings":
		m.mode = "settings"
		m.message = "Adjust with left/right. s saves. q returns."
	case "doctor":
		m.action = actionDoctor
		return m, tea.Quit
	case "sound":
		m.action = actionSoundTest
		return m, tea.Quit
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}

func (m *homeModel) changeSetting(delta int) {
	rows := settingsRows(m.cfg)
	if len(rows) == 0 {
		return
	}
	row := rows[m.settingsCursor]
	row.apply(&m.cfg, delta)
	_ = saveConfig(m.cfg)
	m.message = "Saved."
}

func (m homeModel) homeView() string {
	items := homeItems()
	var lines []string
	lines = append(lines, "")
	lines = append(lines, styleAccent.Render("Ambient coworking, no keystrokes shared."))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Team: %s", valueOr(m.cfg.CurrentTeamCode, "not joined")))
	lines = append(lines, fmt.Sprintf("Nickname: %s", valueOr(m.cfg.Nickname, "not set")))
	lines = append(lines, "")
	for i, item := range items {
		line := fmt.Sprintf("%-12s %s", item.label, item.help)
		if i == m.cursor {
			line = styleSelected.Render(" " + line + " ")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, styleDim.Render(m.message))
	return stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n"))
}

func (m homeModel) settingsView() string {
	rows := settingsRows(m.cfg)
	var lines []string
	lines = append(lines, styleAccent.Render("Settings"))
	lines = append(lines, "")
	for i, row := range rows {
		line := fmt.Sprintf("%-18s %s", row.label, row.value(m.cfg))
		if i == m.settingsCursor {
			line = styleSelected.Render(" " + line + " ")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, styleDim.Render("Left/right adjusts. Enter toggles. s saves. q returns."))
	lines = append(lines, styleDim.Render(m.message))
	return stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n"))
}

type homeItem struct {
	key   string
	label string
	help  string
}

func homeItems() []homeItem {
	return []homeItem{
		{key: "start", label: "Start", help: "join the room and open the live ambience dashboard"},
		{key: "settings", label: "Settings", help: "volume, density, spatial audio, sharing, and team"},
		{key: "doctor", label: "Doctor", help: "check audio, capture, permissions, and privacy"},
		{key: "sound", label: "Sound Test", help: "play the bundled keyboard and mouse samples"},
		{key: "quit", label: "Quit", help: "close Cliks"},
	}
}

type settingRow struct {
	label string
	value func(CliksConfig) string
	apply func(*CliksConfig, int)
}

func settingsRows(cfg CliksConfig) []settingRow {
	return []settingRow{
		{"Volume", func(c CliksConfig) string { return bar(c.Listening.Volume) }, func(c *CliksConfig, d int) {
			c.Listening.Volume = clamp(c.Listening.Volume+float64(d)*0.05, 0, 1)
			c.Listening.Muted = false
		}},
		{"Density", func(c CliksConfig) string { return bar(c.Listening.Density) }, func(c *CliksConfig, d int) { c.Listening.Density = clamp(c.Listening.Density+float64(d)*0.05, 0.15, 1) }},
		{"Muted", func(c CliksConfig) string { return onOff(c.Listening.Muted) }, func(c *CliksConfig, _ int) { c.Listening.Muted = !c.Listening.Muted }},
		{"Spatial audio", func(c CliksConfig) string { return onOff(c.Listening.Spatial) }, func(c *CliksConfig, _ int) { c.Listening.Spatial = !c.Listening.Spatial }},
		{"Fatigue fade", func(c CliksConfig) string { return onOff(c.Listening.FatigueProtection) }, func(c *CliksConfig, _ int) { c.Listening.FatigueProtection = !c.Listening.FatigueProtection }},
		{"Hear keyboard", func(c CliksConfig) string { return onOff(c.Listening.Keyboard) }, func(c *CliksConfig, _ int) { c.Listening.Keyboard = !c.Listening.Keyboard }},
		{"Hear mouse", func(c CliksConfig) string { return onOff(c.Listening.Mouse) }, func(c *CliksConfig, _ int) { c.Listening.Mouse = !c.Listening.Mouse }},
		{"Self monitor", func(c CliksConfig) string { return onOff(c.Listening.Self) }, func(c *CliksConfig, _ int) { c.Listening.Self = !c.Listening.Self }},
		{"Share keyboard", func(c CliksConfig) string { return onOff(c.Sharing.Keyboard) }, func(c *CliksConfig, _ int) { c.Sharing.Keyboard = !c.Sharing.Keyboard }},
		{"Share mouse", func(c CliksConfig) string { return onOff(c.Sharing.Mouse) }, func(c *CliksConfig, _ int) { c.Sharing.Mouse = !c.Sharing.Mouse }},
		{"Current team", func(c CliksConfig) string { return valueOr(c.CurrentTeamCode, "not set") }, func(c *CliksConfig, d int) { cycleTeam(c, d) }},
	}
}

func cycleTeam(cfg *CliksConfig, delta int) {
	if len(cfg.Teams) == 0 {
		return
	}
	index := 0
	for i, team := range cfg.Teams {
		if team.Code == cfg.CurrentTeamCode {
			index = i
			break
		}
	}
	index = (index + delta + len(cfg.Teams)) % len(cfg.Teams)
	cfg.CurrentTeamCode = cfg.Teams[index].Code
}

type sessionUpdateMsg SessionViewState

type sessionModel struct {
	controller *sessionController
	state      SessionViewState
	width      int
	height     int
}

func newSessionModel(controller *sessionController) sessionModel {
	return sessionModel{controller: controller, state: controller.viewState()}
}

func (m sessionModel) Init() tea.Cmd {
	return waitForSessionUpdate(m.controller)
}

func (m sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case sessionUpdateMsg:
		m.state = SessionViewState(msg)
		return m, waitForSessionUpdate(m.controller)
	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.controller.adjustVolume(0.05)
		case tea.MouseWheelDown:
			m.controller.adjustVolume(-0.05)
		case tea.MouseLeft:
			if m.state.CaptureMode == "terminal" && m.state.Listening.Mouse {
				m.controller.recordLocalActivity(LocalActivityEvent{Kind: "mouse", Button: "left", At: time.Now()})
			}
			if msg.Y >= maxInt(10, m.height-7) {
				switch {
				case msg.X < 12:
					m.controller.adjustVolume(-0.05)
				case msg.X < 24:
					m.controller.adjustVolume(0.05)
				case msg.X < 38:
					m.controller.adjustDensity(-0.1)
				case msg.X < 52:
					m.controller.adjustDensity(0.1)
				case msg.X < 64:
					m.controller.toggle("muted")
				case msg.X < 78:
					m.controller.toggle("spatial")
				default:
					m.controller.toggle("fade")
				}
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.controller.stop()
			return m, tea.Quit
		case "up", "+":
			m.controller.adjustVolume(0.05)
		case "down", "-":
			m.controller.adjustVolume(-0.05)
		case "right", "]":
			m.controller.adjustDensity(0.1)
		case "left", "[":
			m.controller.adjustDensity(-0.1)
		case "m":
			m.controller.toggle("muted")
		case "s":
			m.controller.toggle("spatial")
		case "f":
			m.controller.toggle("fade")
		}
		if m.state.CaptureMode == "terminal" && m.state.Listening.Keyboard && msg.String() != "ctrl+c" && msg.String() != "q" {
			m.controller.recordLocalActivity(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
		}
	}
	return m, nil
}

func (m sessionModel) View() string {
	state := m.state
	var peers []string
	for _, peer := range state.Peers {
		if peer.Nickname != "" {
			peers = append(peers, peer.Nickname)
		} else {
			peers = append(peers, shortPeer(peer.PeerID))
		}
	}
	if len(peers) == 0 {
		peers = append(peers, "just you for now")
	}
	left := []string{
		styleAccent.Render(state.TeamName),
		"",
		"Connection: " + connectionStyle(state.ConnectionStatus),
		fmt.Sprintf("Active: %d", state.ActiveCount),
		"Capture: " + state.CaptureMode,
		"Peers: " + strings.Join(peers, ", "),
		"",
		fmt.Sprintf("Captured: %d", state.LocalCapturedEvents),
		fmt.Sprintf("Sent:     %d", state.LocalSentEvents),
	}
	if state.PermissionHint != "" {
		left = append(left, "", styleWarn.Render(state.PermissionHint))
	}
	right := []string{
		"Sound",
		"",
		"Volume  " + muteAwareBar(state.Listening),
		"Density " + bar(state.Listening.Density),
		"Muted   " + onOff(state.Listening.Muted),
		"Spatial " + onOff(state.Listening.Spatial),
		"Fade    " + onOff(state.Listening.FatigueProtection),
		"",
		"Keys: ↑/↓ volume  ←/→ density",
		"m mute  s spatial  f fade  q quit",
		"Mouse: wheel volume, click controls",
	}
	width := panelWidth(m.width)
	colWidth := (width - 6) / 2
	room := stylePanel.Width(colWidth).Render(strings.Join(left, "\n"))
	sound := stylePanel.Width(colWidth).Render(strings.Join(right, "\n"))
	controls := stylePanel.Width(width).Render("[ Vol - ] [ Vol + ] [ Density - ] [ Density + ] [ Mute ] [ Spatial ] [ Fade ]")
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Cliks Live"),
		lipgloss.JoinHorizontal(lipgloss.Top, room, "  ", sound),
		controls,
	)
}

func waitForSessionUpdate(controller *sessionController) tea.Cmd {
	return func() tea.Msg {
		state, ok := <-controller.updates
		if !ok {
			return nil
		}
		return sessionUpdateMsg(state)
	}
}

func panelWidth(width int) int {
	if width <= 0 {
		return 82
	}
	if width < 60 {
		return width - 4
	}
	if width > 110 {
		return 104
	}
	return width - 4
}

func bar(value float64) string {
	width := 16
	filled := int(clamp(value, 0, 1)*float64(width) + 0.5)
	return styleAccent.Render(strings.Repeat("█", filled)) + styleDim.Render(strings.Repeat("░", width-filled)) + fmt.Sprintf(" %d%%", int(value*100+0.5))
}

func muteAwareBar(listening ListeningConfig) string {
	if listening.Muted {
		return styleWarn.Render("muted")
	}
	return bar(listening.Volume)
}

func onOff(value bool) string {
	if value {
		return styleOK.Render("on")
	}
	return styleDim.Render("off")
}

func connectionStyle(value string) string {
	if value == "connected" {
		return styleOK.Render(value)
	}
	if strings.Contains(value, "error") || strings.Contains(value, "offline") {
		return styleWarn.Render(value)
	}
	return styleAccent.Render(value)
}

func shortPeer(peerID string) string {
	if len(peerID) <= 6 {
		return peerID
	}
	return peerID[:6]
}

func valueOr(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return styleDim.Render(fallback)
	}
	return value
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

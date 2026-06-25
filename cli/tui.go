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
	actionNone             homeAction = ""
	actionStart            homeAction = "start"
	actionCreate           homeAction = "create"
	actionDelete           homeAction = "delete"
	actionDoctor           homeAction = "doctor"
	actionSoundTest        homeAction = "sound-test"
	actionBackgroundStart  homeAction = "background-start"
	actionBackgroundStop   homeAction = "background-stop"
	actionBackgroundStatus homeAction = "background-status"
	actionAutostartEnable  homeAction = "autostart-enable"
	actionAutostartDisable homeAction = "autostart-disable"
	actionAutostartStatus  homeAction = "autostart-status"
)

type homeModel struct {
	cfg            CliksConfig
	cursor         int
	mode           string
	message        string
	action         homeAction
	settingsCursor int
	formCursor     int
	createName     string
	createPassword string
	deleteCode     string
	deletePassword string
	busy           bool
	width          int
	height         int
}

func runHomeTUI(cfg CliksConfig) error {
	if !isInteractiveTerminal() {
		printHelp("cliks")
		return nil
	}
	model := homeModel{cfg: cfg, mode: "home", message: welcomeMessage(cfg)}
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	result, _ := finalModel.(homeModel)
	switch result.action {
	case actionStart:
		return startSession(result.cfg, StartOptions{CaptureMode: "auto", SelfMonitor: result.cfg.Listening.Self})
	case actionCreate:
		return cmdCreate(nil)
	case actionDelete:
		return cmdDelete(nil)
	case actionDoctor:
		return runDoctor()
	case actionSoundTest:
		return runSoundTest()
	case actionBackgroundStart:
		return cmdBackground([]string{"start", result.cfg.CurrentTeamCode})
	case actionBackgroundStop:
		return cmdBackground([]string{"stop"})
	case actionBackgroundStatus:
		return cmdBackground([]string{"status"})
	case actionAutostartEnable:
		return cmdAutostart([]string{"enable", result.cfg.CurrentTeamCode})
	case actionAutostartDisable:
		return cmdAutostart([]string{"disable"})
	case actionAutostartStatus:
		return cmdAutostart([]string{"status"})
	default:
		return nil
	}
}

func (m homeModel) Init() tea.Cmd { return nil }

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case formDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.cfg = msg.cfg
		m.mode = "home"
		if msg.kind == "create" {
			m.message = fmt.Sprintf("Created %s. Press Enter on Start when ready.", msg.code)
		} else {
			m.message = fmt.Sprintf("Deleted %s.", msg.code)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.MouseMsg:
		if m.mode == "create" || m.mode == "delete" {
			return m, nil
		}
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
		if m.mode == "create" || m.mode == "delete" {
			return m.updateForm(msg)
		}
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
	} else if m.mode == "create" || m.mode == "delete" {
		body = m.formView()
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
	case "create":
		m.mode = "create"
		m.formCursor = 0
		m.createName = ""
		m.createPassword = ""
		m.message = "Name the room and set a delete password."
	case "delete":
		m.mode = "delete"
		m.formCursor = 0
		m.deleteCode = m.cfg.CurrentTeamCode
		m.deletePassword = ""
		m.message = "Delete closes the live room for everyone using this code."
	case "settings":
		m.mode = "settings"
		m.message = "Adjust with left/right. s saves. q returns."
	case "doctor":
		m.action = actionDoctor
		return m, tea.Quit
	case "sound":
		m.action = actionSoundTest
		return m, tea.Quit
	case "background-start":
		m.action = actionBackgroundStart
		return m, tea.Quit
	case "background-stop":
		m.action = actionBackgroundStop
		return m, tea.Quit
	case "background-status":
		m.action = actionBackgroundStatus
		return m, tea.Quit
	case "autostart-enable":
		m.action = actionAutostartEnable
		return m, tea.Quit
	case "autostart-disable":
		m.action = actionAutostartDisable
		return m, tea.Quit
	case "autostart-status":
		m.action = actionAutostartStatus
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
	lines = append(lines, fmt.Sprintf("Background: %s", backgroundSummary()))
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

type formDoneMsg struct {
	kind string
	code string
	cfg  CliksConfig
	err  error
}

func homeItems() []homeItem {
	return []homeItem{
		{key: "start", label: "Start", help: "join the room and open the live ambience dashboard"},
		{key: "create", label: "Create", help: "make a new team code from the terminal"},
		{key: "delete", label: "Delete", help: "delete the selected team with its password"},
		{key: "settings", label: "Settings", help: "volume, density, spatial audio, sharing, and team"},
		{key: "background-start", label: "BG Start", help: "keep this team connected after closing the terminal"},
		{key: "background-status", label: "BG Status", help: "see whether background Cliks is running"},
		{key: "background-stop", label: "BG Stop", help: "disconnect the background room"},
		{key: "autostart-enable", label: "Boot On", help: "auto-connect this team when you sign in"},
		{key: "autostart-status", label: "Boot Status", help: "show login-time autoconnect status"},
		{key: "autostart-disable", label: "Boot Off", help: "disable login-time autoconnect"},
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

func (m homeModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.busy {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = "home"
		m.message = "Cancelled."
		return m, nil
	case "up", "shift+tab":
		m.formCursor = clampInt(m.formCursor-1, 0, m.formFieldCount()-1)
		return m, nil
	case "down", "tab":
		m.formCursor = clampInt(m.formCursor+1, 0, m.formFieldCount()-1)
		return m, nil
	case "enter":
		if m.formCursor < m.formFieldCount()-1 {
			m.formCursor++
			return m, nil
		}
		return m.submitForm()
	case "backspace", "ctrl+h":
		m.trimFormValue()
		return m, nil
	case "ctrl+u":
		m.setFormValue("")
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.setFormValue(m.formValue() + string(msg.Runes))
	}
	return m, nil
}

func (m homeModel) submitForm() (tea.Model, tea.Cmd) {
	if m.mode == "create" {
		name := strings.TrimSpace(m.createName)
		if name == "" {
			name = "Cliks Room"
		}
		password := strings.TrimSpace(m.createPassword)
		if len(password) < 6 {
			m.message = "Delete password must be at least 6 characters."
			m.formCursor = 1
			return m, nil
		}
		m.busy = true
		m.message = "Creating team..."
		return m, createTeamCmd(name, password)
	}
	code := strings.ToUpper(strings.TrimSpace(m.deleteCode))
	if code == "" {
		m.message = "Team code is required."
		m.formCursor = 0
		return m, nil
	}
	password := strings.TrimSpace(m.deletePassword)
	if password == "" {
		m.message = "Delete password is required."
		m.formCursor = 1
		return m, nil
	}
	m.busy = true
	m.message = "Deleting team..."
	return m, deleteTeamCmd(code, password)
}

func (m homeModel) formFieldCount() int {
	return 2
}

func (m homeModel) formValue() string {
	switch m.mode {
	case "create":
		if m.formCursor == 0 {
			return m.createName
		}
		return m.createPassword
	case "delete":
		if m.formCursor == 0 {
			return m.deleteCode
		}
		return m.deletePassword
	default:
		return ""
	}
}

func (m *homeModel) setFormValue(value string) {
	switch m.mode {
	case "create":
		if m.formCursor == 0 {
			m.createName = value
		} else {
			m.createPassword = value
		}
	case "delete":
		if m.formCursor == 0 {
			m.deleteCode = strings.ToUpper(value)
		} else {
			m.deletePassword = value
		}
	}
}

func (m *homeModel) trimFormValue() {
	value := []rune(m.formValue())
	if len(value) == 0 {
		return
	}
	m.setFormValue(string(value[:len(value)-1]))
}

func (m homeModel) formView() string {
	var title string
	var rows []string
	if m.mode == "create" {
		title = "Create Team"
		rows = []string{
			formLine("Team name", valueOr(m.createName, "Cliks Room"), m.formCursor == 0),
			formLine("Delete password", maskSecret(m.createPassword), m.formCursor == 1),
		}
	} else {
		title = "Delete Team"
		rows = []string{
			formLine("Team code", valueOr(m.deleteCode, "CLIK-XXXXXX"), m.formCursor == 0),
			formLine("Delete password", maskSecret(m.deletePassword), m.formCursor == 1),
		}
	}
	lines := []string{styleAccent.Render(title), ""}
	lines = append(lines, rows...)
	lines = append(lines, "")
	if m.busy {
		lines = append(lines, styleAccent.Render(m.message))
	} else {
		lines = append(lines, styleDim.Render("Enter moves forward/submits. Tab changes fields. Esc cancels."))
		lines = append(lines, styleDim.Render(m.message))
	}
	return stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n"))
}

func formLine(label string, value string, selected bool) string {
	line := fmt.Sprintf("%-18s %s", label, value)
	if selected {
		return styleSelected.Render(" " + line + " ")
	}
	return line
}

func maskSecret(value string) string {
	if value == "" {
		return styleDim.Render("not set")
	}
	return strings.Repeat("*", len([]rune(value)))
}

func createTeamCmd(name string, password string) tea.Cmd {
	return func() tea.Msg {
		cfg := loadConfig()
		team, err := createTeamViaAPI(cfg, name, password)
		if err != nil {
			return formDoneMsg{kind: "create", err: err}
		}
		next, err := rememberTeam(team.Code, team.Name)
		if err != nil {
			return formDoneMsg{kind: "create", err: err}
		}
		return formDoneMsg{kind: "create", code: team.Code, cfg: next}
	}
}

func deleteTeamCmd(code string, password string) tea.Cmd {
	return func() tea.Msg {
		cfg := loadConfig()
		if err := deleteTeamViaAPI(cfg, code, password); err != nil {
			return formDoneMsg{kind: "delete", code: code, err: err}
		}
		cfg.Teams = filterTeams(cfg.Teams, code)
		if cfg.CurrentTeamCode == code {
			cfg.CurrentTeamCode = ""
			if len(cfg.Teams) > 0 {
				cfg.CurrentTeamCode = cfg.Teams[0].Code
			}
		}
		if err := saveConfig(cfg); err != nil {
			return formDoneMsg{kind: "delete", code: code, err: err}
		}
		return formDoneMsg{kind: "delete", code: code, cfg: cfg}
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
	controller     *sessionController
	state          SessionViewState
	mode           string
	settingsCursor int
	width          int
	height         int
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
		if m.mode == "settings" {
			if msg.Type == tea.MouseWheelUp {
				m.settingsCursor = clampInt(m.settingsCursor-1, 0, len(settingsRows(m.controller.cfg))-1)
			}
			if msg.Type == tea.MouseWheelDown {
				m.settingsCursor = clampInt(m.settingsCursor+1, 0, len(settingsRows(m.controller.cfg))-1)
			}
			if msg.Type == tea.MouseLeft && msg.Y >= 5 {
				m.settingsCursor = clampInt(msg.Y-5, 0, len(settingsRows(m.controller.cfg))-1)
				m.applyLiveSetting(1)
			}
			return m, nil
		}
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
		if m.mode == "settings" {
			switch msg.String() {
			case "esc", "tab", "q", "S":
				m.mode = ""
			case "up", "k":
				m.settingsCursor = clampInt(m.settingsCursor-1, 0, len(settingsRows(m.controller.cfg))-1)
			case "down", "j":
				m.settingsCursor = clampInt(m.settingsCursor+1, 0, len(settingsRows(m.controller.cfg))-1)
			case "left", "h":
				m.applyLiveSetting(-1)
			case "right", "l", "enter", " ":
				m.applyLiveSetting(1)
			}
			return m, nil
		}
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
		case "tab", "S":
			m.mode = "settings"
		}
		if m.state.CaptureMode == "terminal" && m.state.Listening.Keyboard && isTerminalCaptureKey(msg.String()) {
			m.controller.recordLocalActivity(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
		}
	}
	return m, nil
}

func (m sessionModel) View() string {
	if m.mode == "settings" {
		return m.sessionSettingsView()
	}
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
		"m mute  s spatial  f fade  Tab settings  q quit",
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

func (m *sessionModel) applyLiveSetting(delta int) {
	rows := settingsRows(m.controller.cfg)
	if len(rows) == 0 {
		return
	}
	row := rows[m.settingsCursor]
	row.apply(&m.controller.cfg, delta)
	_ = saveConfig(m.controller.cfg)
	m.controller.set(func(state *SessionViewState) {
		state.Listening = m.controller.cfg.Listening
		state.HearingSelf = m.controller.cfg.Listening.Self
	})
	m.controller.audio.updateListening(m.controller.cfg.Listening)
	m.state = m.controller.viewState()
}

func (m sessionModel) sessionSettingsView() string {
	cfg := m.controller.cfg
	rows := settingsRows(cfg)
	var lines []string
	lines = append(lines, styleAccent.Render("Live Settings"))
	lines = append(lines, "")
	for i, row := range rows {
		line := fmt.Sprintf("%-18s %s", row.label, row.value(cfg))
		if i == m.settingsCursor {
			line = styleSelected.Render(" " + line + " ")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, styleDim.Render("Left/right adjusts. Enter toggles. Tab/Esc returns to the live room. q also returns here."))
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Cliks Live"),
		stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n")),
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

func welcomeMessage(cfg CliksConfig) string {
	if cfg.CurrentTeamCode == "" {
		return "Desk is warm. Create or join a team to start hearing the room."
	}
	return fmt.Sprintf("Desk is warm for %s. Press Enter to start.", cfg.CurrentTeamCode)
}

func backgroundSummary() string {
	pid, ok := readBackgroundPID()
	if !ok {
		return styleDim.Render("stopped")
	}
	if processLooksAlive(pid) {
		return styleOK.Render(fmt.Sprintf("running (pid %d)", pid))
	}
	return styleWarn.Render(fmt.Sprintf("stale pid %d", pid))
}

func isTerminalCaptureKey(key string) bool {
	switch key {
	case "ctrl+c", "q", "tab", "S", "up", "down", "left", "right", "+", "-", "[", "]", "m", "s", "f":
		return false
	default:
		return true
	}
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

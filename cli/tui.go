package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#006D7D", Dark: "#33D6E8"}
	colorDim    = lipgloss.AdaptiveColor{Light: "#5B5751", Dark: "#A9A39A"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#9A4D00", Dark: "#FFB454"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#18743A", Dark: "#55D98B"}
	colorPanel  = lipgloss.AdaptiveColor{Light: "#007487", Dark: "#159BB5"}
	colorSelect = lipgloss.AdaptiveColor{Light: "#007487", Dark: "#33D6E8"}
	colorOnPick = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#071013"}

	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorOnPick).Background(colorSelect).Padding(0, 1)
	styleAccent   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(colorDim)
	styleWarn     = lipgloss.NewStyle().Foreground(colorWarn)
	styleOK       = lipgloss.NewStyle().Foreground(colorOK)
	stylePanel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPanel).Padding(1, 2)
	styleSelected = lipgloss.NewStyle().Foreground(colorOnPick).Background(colorSelect).Bold(true)
	styleFocused  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
)

type shortcutHelp struct {
	keys        string
	description string
}

func shortcutEntries(context string) []shortcutHelp {
	switch context {
	case "preferences", "live-preferences":
		return []shortcutHelp{
			{"Up/k, Down/j", "move between preferences"},
			{"Left/h, Right/l", "adjust the selected preference"},
			{"Enter/Space", "toggle the selected preference"},
			{"Tab/Esc/q", "return to the previous screen"},
			{"?", "close this shortcut guide"},
		}
	case "live":
		return []shortcutHelp{
			{"Up/+, Down/-", "raise or lower volume"},
			{"Right/], Left/[", "raise or lower sound density"},
			{"m / s / f", "toggle mute, spatial audio, or fatigue fade"},
			{"Tab/Shift+S", "open live preferences"},
			{"Esc/q/b", "return to the main control screen"},
			{"x/Ctrl+C", "stop and disconnect this session"},
			{"Mouse wheel", "adjust volume"},
			{"?", "close this shortcut guide"},
		}
	default:
		return []shortcutHelp{
			{"Up/k, Down/j", "move between actions"},
			{"Enter/Space", "run the highlighted action"},
			{"Esc/q", "go back or close the control screen"},
			{"Mouse", "hover and click an action"},
			{"?", "close this shortcut guide"},
		}
	}
}

func shortcutHelpView(context string, width int) string {
	lines := []string{styleAccent.Render("Keyboard & mouse"), ""}
	for _, entry := range shortcutEntries(context) {
		lines = append(lines, fmt.Sprintf("%-18s %s", entry.keys, entry.description))
	}
	lines = append(lines, "", styleDim.Render("Press ? or Esc to return."))
	return stylePanel.Width(panelWidth(width)).Render(strings.Join(lines, "\n"))
}

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
	cfg              CliksConfig
	active           ActiveSessionState
	activeOK         bool
	cursor           int
	mouseOver        bool
	mode             string
	message          string
	action           homeAction
	settingsCursor   int
	formCursor       int
	formTextCursor   int
	formReturnMode   string
	createName       string
	createPassword   string
	joinCode         string
	deleteCode       string
	deletePassword   string
	nicknameValue    string
	audioDeviceValue string
	batchWindowValue string
	stopActiveOnExit bool
	busy             bool
	width            int
	height           int
	helpOpen         bool
}

func runHomeTUI(cfg CliksConfig) error {
	if !isInteractiveTerminal() {
		printHelp("cliks")
		return nil
	}
	ctx, stopSignals := tuiSignalContext(context.Background())
	defer stopSignals()
	deferredMessage := consumeDeferredStopIfNeeded()
	active, activeOK := activeSession()
	message := welcomeMessage(cfg)
	if deferredMessage != "" {
		message = deferredMessage
	} else if activeOK {
		message = "Already connected. Use Stop to disconnect, or Quit to leave it running."
		if stopped := stopDuplicateLocalSessions(active); stopped > 0 {
			message = fmt.Sprintf("Cleaned up %d older duplicate Cliks session(s).", stopped)
			active, activeOK = activeSession()
		}
	}
	model := homeModel{cfg: cfg, active: active, activeOK: activeOK, mode: "home", message: message, stopActiveOnExit: activeOK && deferredStopMatches(active)}
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	result, ok := finalModel.(homeModel)
	if !ok {
		result = model
	}
	if result.stopActiveOnExit {
		_, _ = stopActiveSession()
		_ = clearDeferredStop()
	} else {
		_ = clearDeferredStop()
	}
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

func tuiSignalContext(parent context.Context) (context.Context, func()) {
	exitSignals := tuiExitSignals()
	if len(exitSignals) == 0 {
		return parent, func() {}
	}
	return signal.NotifyContext(parent, exitSignals...)
}

func (m homeModel) Init() tea.Cmd { return homeTick() }

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case homeTickMsg:
		m.refreshRuntime()
		return m, homeTick()
	case commandDoneMsg:
		m.busy = false
		m.refreshRuntime()
		m.cfg = loadConfig()
		if msg.err != nil {
			m.message = msg.err.Error()
		} else {
			m.message = msg.message
		}
		return m, nil
	case formDoneMsg:
		m.busy = false
		m.refreshRuntime()
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.cfg = msg.cfg
		m.mode = "home"
		m.mouseOver = false
		if msg.kind == "create" {
			m.message = fmt.Sprintf("Created %s. %s", teamLabel(msg.cfg, msg.code), msg.message)
		} else if msg.kind == "join" {
			m.message = fmt.Sprintf("Joined %s. Opening live...", teamLabel(msg.cfg, msg.code))
			if !m.activeOK {
				m.action = actionStart
				return m, tea.Quit
			}
		} else {
			m.message = fmt.Sprintf("Deleted %s.", msg.code)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.MouseMsg:
		if m.helpOpen {
			return m, nil
		}
		if isFormMode(m.mode) {
			if msg.Type == tea.MouseLeft {
				if index := m.formHit(msg.X, msg.Y); index >= 0 {
					m.formCursor = index
					m.moveFormTextCursorToEnd()
				}
			}
			return m, nil
		}
		if msg.Type == tea.MouseWheelUp {
			m.move(-1)
			m.mouseOver = false
		}
		if msg.Type == tea.MouseWheelDown {
			m.move(1)
			m.mouseOver = false
		}
		if msg.Type == tea.MouseMotion {
			m.mouseOver = m.hover(msg.Y)
		}
		if msg.Type == tea.MouseLeft {
			if !m.hover(msg.Y) {
				m.mouseOver = false
				return m, nil
			}
			m.mouseOver = true
			if m.mode == "preferences" {
				m.changeSetting(1)
				return m, nil
			}
			return m.activate()
		}
	case tea.KeyMsg:
		if m.helpOpen {
			switch msg.String() {
			case "?", "esc", "q":
				m.helpOpen = false
			}
			return m, nil
		}
		if isFormMode(m.mode) {
			return m.updateForm(msg)
		}
		switch msg.String() {
		case "?":
			m.helpOpen = true
			return m, nil
		case "ctrl+c", "q", "esc":
			if m.mode != "home" {
				m.back()
				return m, nil
			}
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
			m.mouseOver = false
		case "down", "j":
			m.move(1)
			m.mouseOver = false
		case "left", "h":
			if m.mode == "preferences" {
				m.changeSetting(-1)
			}
		case "right", "l":
			if m.mode == "preferences" {
				m.changeSetting(1)
			}
		case "enter", " ":
			return m.activate()
		case "s":
			if m.mode == "preferences" {
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
	if m.helpOpen {
		context := m.mode
		if context != "preferences" {
			context = "home"
		}
		body = shortcutHelpView(context, m.width)
	} else if m.mode == "preferences" {
		body = m.preferencesView()
	} else if isFormMode(m.mode) {
		body = m.formView()
	} else {
		body = m.itemView()
	}
	return lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("Cliks"), body)
}

func (m *homeModel) move(delta int) {
	if m.mode == "preferences" {
		m.settingsCursor = clampInt(m.settingsCursor+delta, 0, len(settingsRows(m.cfg))-1)
		return
	}
	m.cursor = clampInt(m.cursor+delta, 0, len(m.items())-1)
}

func (m homeModel) activate() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	if m.mode == "preferences" {
		m.changeSetting(1)
		return m, nil
	}
	item := m.items()[m.cursor]
	switch item.key {
	case "start":
		if m.cfg.CurrentTeamCode == "" {
			m.message = "Create or join a team first."
			return m, nil
		}
		if m.activeOK {
			m.message = fmt.Sprintf("Already running as %s. Use Stop to disconnect first.", modeLabel(m.active.Mode))
			return m, nil
		}
		m.action = actionStart
		return m, tea.Quit
	case "menu":
		m.mode = "menu"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Choose what to adjust."
	case "back":
		m.back()
	case "create":
		returnMode := m.mode
		if returnMode != "home" {
			returnMode = "team"
		}
		m.mode = "create"
		m.formCursor = 0
		m.formReturnMode = returnMode
		m.mouseOver = false
		m.createName = ""
		m.createPassword = ""
		m.moveFormTextCursorToEnd()
		m.message = "Name the room and set a delete password."
	case "join":
		returnMode := m.mode
		if returnMode != "home" {
			returnMode = "team"
		}
		m.mode = "join"
		m.formCursor = 0
		m.formReturnMode = returnMode
		m.mouseOver = false
		m.joinCode = ""
		m.moveFormTextCursorToEnd()
		m.message = "Paste or type a team code. Join opens live automatically."
	case "delete":
		m.mode = "delete"
		m.formCursor = 0
		m.formReturnMode = "team"
		m.mouseOver = false
		m.deleteCode = m.cfg.CurrentTeamCode
		m.deletePassword = ""
		m.moveFormTextCursorToEnd()
		m.message = "Delete closes the live room for everyone using this code."
	case "nickname":
		returnMode := m.mode
		if returnMode != "advanced" {
			returnMode = "team"
		}
		m.mode = "nickname"
		m.formCursor = 0
		m.formReturnMode = returnMode
		m.mouseOver = false
		m.nicknameValue = m.cfg.Nickname
		m.moveFormTextCursorToEnd()
		m.message = "Set the short name teammates see in the live room. Max 10 characters."
	case "advanced":
		m.mode = "advanced"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Less common local controls. Backend overrides stay in cliks set --list."
	case "audio-device":
		m.mode = "audio-device"
		m.formCursor = 0
		m.formReturnMode = "advanced"
		m.audioDeviceValue = valuePlain(m.cfg.Listening.AudioDevice, "default")
		m.moveFormTextCursorToEnd()
		m.message = "Use default, or an output name supported by mpv/PulseAudio/PipeWire."
	case "batch-window":
		m.mode = "batch-window"
		m.formCursor = 0
		m.formReturnMode = "advanced"
		m.batchWindowValue = fmt.Sprintf("%d", m.cfg.BatchWindowMs)
		m.moveFormTextCursorToEnd()
		m.message = "100-2000 ms. The default 500 ms balances latency and network use."
	case "preferences":
		m.mode = "preferences"
		m.settingsCursor = 0
		m.mouseOver = false
		m.message = "Adjust with left/right. s saves. q returns."
	case "team":
		m.mode = "team"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Manage the selected team."
	case "connection":
		m.mode = "connection"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Control this device's single Cliks connection."
	case "diagnostics":
		m.mode = "diagnostics"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Check sound and setup."
	case "switch-team":
		cycleTeam(&m.cfg, 1)
		_ = saveConfig(m.cfg)
		m.message = fmt.Sprintf("Selected %s.", valuePlain(teamLabel(m.cfg, m.cfg.CurrentTeamCode), "no team"))
	case "doctor":
		m.busy = true
		m.message = "Checking setup..."
		return m, doctorSummaryCmd()
	case "sound":
		m.busy = true
		m.message = "Playing test sounds..."
		return m, soundTestCmd()
	case "background-toggle":
		if m.activeOK {
			m.stopActiveOnExit = !m.stopActiveOnExit
			m.cfg.KeepRunning = !m.stopActiveOnExit
			_ = saveConfig(m.cfg)
			if m.stopActiveOnExit {
				_ = scheduleDeferredStop(m.active.PID)
				if m.active.Mode == runModeBoot {
					m.message = "Keep Running is off. This connection will stop when this screen closes; launch-at-login remains on for the next login."
				} else {
					m.message = "Keep Running is off. This connection will stop when this control screen closes."
				}
			} else {
				_ = clearDeferredStop()
				m.message = "Keep Running is on. This connection will stay alive after this screen closes."
			}
			return m, nil
		}
		m.cfg.KeepRunning = !m.cfg.KeepRunning
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		if m.cfg.KeepRunning {
			m.message = "Keep Running is on. Future live sessions may stay connected after this terminal closes."
		} else {
			m.message = "Keep Running is off. Future live sessions stop with their terminal unless started in background."
		}
		return m, nil
	case "stop-connection":
		m.busy = true
		m.message = "Stopping this device's connection..."
		return m, stopConnectionCmd(m.active, m.activeOK)
	case "autostart-toggle":
		m.busy = true
		m.message = "Updating launch-at-login..."
		return m, toggleAutostartCmd(m.cfg.CurrentTeamCode)
	case "autostart-status":
		m.busy = true
		m.message = "Checking launch-at-login..."
		return m, autostartStatusCmd()
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

func (m homeModel) itemView() string {
	items := m.items()
	lines := m.itemPrefixLines()
	for i, item := range items {
		line := fmt.Sprintf("%-12s %s", item.label, item.help)
		if i == m.cursor {
			if m.mouseOver {
				line = styleSelected.Render(" " + line + " ")
			} else {
				line = styleFocused.Render("> " + line)
			}
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, styleDim.Render("? shortcuts"))
	lines = append(lines, styleDim.Render(m.message))
	return stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n"))
}

func (m homeModel) itemPrefixLines() []string {
	title, intro := m.viewHeader()
	lines := []string{styleAccent.Render(title)}
	if intro != "" {
		lines = append(lines, intro)
	}
	if m.mode == "home" {
		lines = append(lines, "")
		teamText := teamLabel(m.cfg, m.cfg.CurrentTeamCode)
		if m.activeOK && m.activeTeamLabel() != "" {
			teamText = m.activeTeamLabel()
		}
		lines = append(lines, fmt.Sprintf("Team: %s", valueOr(teamText, "not joined")))
		lines = append(lines, "Connection: "+m.connectionSummary())
		if m.activeOK {
			lines = append(lines, fmt.Sprintf("People: %s", peopleSummary(m.active.ActiveCount)))
			lines = append(lines, fmt.Sprintf("Activity: %d captured, %d sent", m.active.LocalCapturedEvents, m.active.LocalSentEvents))
		}
	}
	return append(lines, "")
}

func (m homeModel) preferencesView() string {
	rows := settingsRows(m.cfg)
	start, end := settingsWindow(len(rows), m.settingsCursor, m.height)
	var lines []string
	lines = append(lines, styleAccent.Render("Preferences"))
	lines = append(lines, "")
	for i := start; i < end; i++ {
		row := rows[i]
		line := fmt.Sprintf("%-18s %-24s %s", row.label, row.value(m.cfg), styleDim.Render(row.help))
		if i == m.settingsCursor {
			if m.mouseOver {
				line = styleSelected.Render(" " + line + " ")
			} else {
				line = styleFocused.Render("> " + line)
			}
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	footer := "Left/right adjusts. Enter toggles. s saves. q returns."
	if start > 0 || end < len(rows) {
		footer = fmt.Sprintf("Showing %d-%d of %d. Up/down reveals more. q returns.", start+1, end, len(rows))
	}
	lines = append(lines, styleDim.Render(footer))
	lines = append(lines, styleDim.Render(m.message))
	return stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n"))
}

type homeItem struct {
	key   string
	label string
	help  string
}

type formDoneMsg struct {
	kind    string
	code    string
	message string
	cfg     CliksConfig
	err     error
}

type commandDoneMsg struct {
	message string
	err     error
}

type homeTickMsg time.Time

func (m homeModel) items() []homeItem {
	switch m.mode {
	case "menu":
		return []homeItem{
			{key: "preferences", label: "Preferences", help: "sound, sharing, spatial audio, and fatigue fade"},
			{key: "advanced", label: "Advanced", help: "nickname, audio output, and batching"},
			{key: "team", label: "Team", help: "create, delete, or switch the selected team"},
			{key: "connection", label: "Connection", help: "background mode and launch-at-login"},
			{key: "diagnostics", label: "Diagnostics", help: "sound test and setup check"},
			{key: "back", label: "Back", help: "return to the greeting screen"},
		}
	case "team":
		return []homeItem{
			{key: "nickname", label: "Nickname", help: valuePlain(m.cfg.Nickname, "set a short name")},
			{key: "join", label: "Join", help: "save a team code and open live"},
			{key: "create", label: "Create", help: "make a new team code"},
			{key: "delete", label: "Delete", help: "delete the selected team with its password"},
			{key: "switch-team", label: "Switch", help: "cycle through saved teams"},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	case "connection":
		return []homeItem{
			{key: "background-toggle", label: "Keep Running", help: m.backgroundToggleHelp()},
			{key: "stop-connection", label: "Stop", help: m.stopConnectionHelp()},
			{key: "autostart-toggle", label: "Launch Login", help: autostartToggleHelp()},
			{key: "autostart-status", label: "Login Status", help: "show where launch-at-login is configured"},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	case "diagnostics":
		return []homeItem{
			{key: "sound", label: "Sound Test", help: "play keyboard and mouse samples"},
			{key: "doctor", label: "Doctor", help: "quick setup and permission check"},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	case "advanced":
		return []homeItem{
			{key: "nickname", label: "Nickname", help: valuePlain(m.cfg.Nickname, "anonymous") + " (CLI key: nickname)"},
			{key: "audio-device", label: "Audio Output", help: valuePlain(m.cfg.Listening.AudioDevice, "default") + " (CLI key: audio.device)"},
			{key: "batch-window", label: "Batch Window", help: fmt.Sprintf("%d ms (CLI key: batch.ms)", m.cfg.BatchWindowMs)},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	default:
		if m.cfg.CurrentTeamCode == "" {
			return []homeItem{
				{key: "join", label: "Join Team", help: "paste a team code and open live"},
				{key: "create", label: "Create Team", help: "make a room and copy its code"},
				{key: "sound", label: "Sound Check", help: "play keyboard and mouse samples"},
				{key: "doctor", label: "Setup Check", help: "check audio and input permissions"},
				{key: "menu", label: "More", help: "preferences, diagnostics, and connection options"},
				{key: "quit", label: "Quit", help: "close this control screen"},
			}
		}
		items := []homeItem{
			{key: "start", label: "Open Live", help: m.startHelp()},
			{key: "background-toggle", label: "Keep Running", help: m.backgroundToggleHelp()},
		}
		if m.activeOK {
			items = append(items, homeItem{key: "stop-connection", label: "Stop", help: m.stopConnectionHelp()})
		}
		items = append(items,
			homeItem{key: "menu", label: "More", help: "teams, preferences, diagnostics, and boot options"},
			homeItem{key: "quit", label: "Quit", help: "close this control screen"},
		)
		return items
	}
}

func (m homeModel) viewHeader() (string, string) {
	switch m.mode {
	case "menu":
		return "More", "Everything here stays in this control screen."
	case "team":
		return "Team", fmt.Sprintf("Selected: %s", valuePlain(teamLabel(m.cfg, m.cfg.CurrentTeamCode), "not joined"))
	case "connection":
		return "Connection", "Cliks allows one local connection per device."
	case "diagnostics":
		return "Diagnostics", "Quick checks without leaving the TUI."
	case "advanced":
		return "Advanced", "These controls stay on this device. Run cliks set --list for every scriptable key."
	default:
		if m.cfg.CurrentTeamCode == "" {
			return "Set up Cliks", "Join a team first, then Cliks opens the live room automatically."
		}
		return "Welcome back", "Ambient coworking, no keystrokes shared."
	}
}

func (m homeModel) startHelp() string {
	if m.activeOK {
		return "already running"
	}
	if m.cfg.CurrentTeamCode == "" {
		return "join or create a team first"
	}
	return "open the live room in this terminal; stops when this terminal closes"
}

func (m homeModel) backgroundToggleHelp() string {
	if m.activeOK {
		if m.stopActiveOnExit {
			return "off after close; press Enter to keep running"
		}
		return fmt.Sprintf("on (%s); use Stop to disconnect", modeLabel(m.active.Mode))
	}
	if m.cfg.KeepRunning {
		return "on for future sessions; press Enter to turn off"
	}
	return "off for future sessions; press Enter to turn on"
}

func (m homeModel) stopConnectionHelp() string {
	if !m.activeOK {
		return "no active local connection"
	}
	return fmt.Sprintf("disconnect %s pid %d", modeLabel(m.active.Mode), m.active.PID)
}

func (m homeModel) connectionSummary() string {
	if !m.activeOK {
		return styleDim.Render("stopped")
	}
	return fmt.Sprintf("%s for %s (%s, pid %d)",
		connectionStyle(valuePlain(m.active.ConnectionStatus, "starting")),
		valuePlain(m.activeTeamLabel(), m.cfg.CurrentTeamCode),
		modeLabel(m.active.Mode),
		m.active.PID,
	)
}

func (m homeModel) activeTeamLabel() string {
	return formatTeamLabel(valuePlain(m.active.TeamName, teamNameForCode(m.cfg, m.active.TeamCode)), m.active.TeamCode)
}

func (m *homeModel) refreshRuntime() {
	m.active, m.activeOK = activeSession()
}

func (m *homeModel) hover(y int) bool {
	if m.mode == "preferences" {
		rows := settingsRows(m.cfg)
		start, end := settingsWindow(len(rows), m.settingsCursor, m.height)
		index := start + y - settingsRowsStartY()
		if index >= start && index < end {
			m.settingsCursor = index
			return true
		}
		return false
	}
	index := y - m.itemStartY()
	if index >= 0 && index < len(m.items()) {
		m.cursor = index
		return true
	}
	return false
}

func (m homeModel) itemStartY() int {
	return panelContentStartY() + len(m.itemPrefixLines())
}

func panelContentStartY() int {
	return 3
}

func settingsRowsStartY() int {
	return panelContentStartY() + 2
}

func formRowsStartY() int {
	return panelContentStartY() + 2
}

func settingsWindow(total int, cursor int, height int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	visible := total
	if height > 0 {
		visible = clampInt(height-10, 3, total)
	}
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start, start + visible
}

func (m *homeModel) back() {
	switch m.mode {
	case "team", "connection", "diagnostics", "preferences", "advanced":
		m.mode = "menu"
		m.cursor = 0
	default:
		m.mode = "home"
		m.cursor = 0
	}
	m.mouseOver = false
	m.message = welcomeMessage(m.cfg)
}

func peopleSummary(activeCount int) string {
	if activeCount <= 1 {
		return "just you"
	}
	return fmt.Sprintf("you + %d teammate(s)", activeCount-1)
}

func roomPeopleSummary(state SessionViewState) string {
	if state.ActiveCount <= 1 {
		return "just you"
	}
	if state.ActiveCount > 6 {
		return fmt.Sprintf("%d people here", state.ActiveCount)
	}
	names := peerDisplayNames(state)
	if len(names) == 0 {
		return peopleSummary(state.ActiveCount)
	}
	return joinNames(append([]string{"you"}, names...))
}

func typingSummary(state SessionViewState, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	nameByPeer := peerDisplayNameMap(state)
	var names []string
	for _, item := range state.RecentPeerActivity {
		if now.Sub(item.LastActivityAt) > 1800*time.Millisecond {
			continue
		}
		name := sanitizeNickname(item.Nickname)
		if name == "" {
			name = nameByPeer[item.PeerID]
		}
		if name == "" {
			name = "Someone"
		}
		names = append(names, name)
	}
	names = uniqueStrings(names)
	if len(names) == 0 {
		return "quiet"
	}
	if state.ActiveCount > 6 || len(names) > 3 {
		return fmt.Sprintf("%d people typing", len(names))
	}
	if len(names) == 1 {
		return names[0] + " is typing"
	}
	return joinNames(names) + " are typing"
}

func flowBadge(state SessionViewState, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	if state.LastLocalActivityAt.IsZero() || now.Sub(state.LastLocalActivityAt) > 8*time.Second {
		return styleDim.Render("idle")
	}
	if state.LocalBurstCount >= 24 {
		return styleOK.Render("deep flow")
	}
	if state.LocalBurstCount >= 5 {
		return styleAccent.Render("active")
	}
	return styleAccent.Render("warming")
}

func healthSummary(state SessionViewState, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	status := strings.ToLower(state.ConnectionStatus)
	if strings.Contains(status, "offline") || strings.Contains(status, "disconnected") || strings.Contains(status, "stopped") {
		return styleWarn.Render("connection " + valuePlain(state.ConnectionStatus, "offline"))
	}
	pulse := "live"
	if now.Unix()%2 == 0 {
		pulse = "live."
	}
	last := latestTime(state.LastLocalActivityAt, state.LastPeerActivityAt)
	if last.IsZero() {
		return styleOK.Render(pulse) + styleDim.Render(" | waiting for activity")
	}
	return styleOK.Render(pulse) + styleDim.Render(" | last activity "+relativeAge(now, last))
}

func latestTime(left time.Time, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}

func relativeAge(now time.Time, then time.Time) string {
	if then.IsZero() {
		return "never"
	}
	age := now.Sub(then)
	if age < 0 {
		age = 0
	}
	if age < time.Second {
		return "now"
	}
	if age < time.Minute {
		return fmt.Sprintf("%ds ago", int(age.Seconds()))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(age.Hours()))
}

func peerDisplayNames(state SessionViewState) []string {
	peers := sortedRemotePeers(state)
	names := make([]string, 0, len(peers))
	for index, peer := range peers {
		name := sanitizeNickname(peer.Nickname)
		if name == "" {
			name = fmt.Sprintf("Teammate %d", index+1)
		}
		names = append(names, name)
	}
	return names
}

func peerDisplayNameMap(state SessionViewState) map[string]string {
	peers := sortedRemotePeers(state)
	names := map[string]string{}
	for index, peer := range peers {
		name := sanitizeNickname(peer.Nickname)
		if name == "" {
			name = fmt.Sprintf("Teammate %d", index+1)
		}
		names[peer.PeerID] = name
	}
	return names
}

func sortedRemotePeers(state SessionViewState) []PeerPresence {
	peers := make([]PeerPresence, 0, len(state.Peers))
	for _, peer := range state.Peers {
		if peer.PeerID == "" || peer.PeerID == state.OwnPeerID {
			continue
		}
		peers = append(peers, peer)
	}
	sort.SliceStable(peers, func(i, j int) bool {
		if peers[i].JoinedAt == peers[j].JoinedAt {
			return peers[i].PeerID < peers[j].PeerID
		}
		return peers[i].JoinedAt < peers[j].JoinedAt
	})
	return peers
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ", ")
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	next := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		next = append(next, value)
	}
	return next
}

func autostartToggleHelp() string {
	if autostartEnabled() {
		return "on; press Enter to disable launch-at-login"
	}
	return "off; press Enter to connect this team when you sign in"
}

func homeTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return homeTickMsg(t)
	})
}

type settingRow struct {
	label string
	help  string
	value func(CliksConfig) string
	apply func(*CliksConfig, int)
}

func settingsRows(cfg CliksConfig) []settingRow {
	return []settingRow{
		{"Volume", "overall loudness", func(c CliksConfig) string { return bar(c.Listening.Volume) }, func(c *CliksConfig, d int) {
			c.Listening.Volume = clamp(c.Listening.Volume+float64(d)*0.05, 0, 1)
			c.Listening.Muted = false
		}},
		{"Density", "hear fewer or more activity sounds", func(c CliksConfig) string { return bar(c.Listening.Density) }, func(c *CliksConfig, d int) { c.Listening.Density = clamp(c.Listening.Density+float64(d)*0.05, 0.15, 1) }},
		{"Muted", "silence local playback", func(c CliksConfig) string { return onOff(c.Listening.Muted) }, func(c *CliksConfig, _ int) { c.Listening.Muted = !c.Listening.Muted }},
		{"Spatial audio", "pan teammates around your desk", func(c CliksConfig) string { return onOff(c.Listening.Spatial) }, func(c *CliksConfig, _ int) { c.Listening.Spatial = !c.Listening.Spatial }},
		{"Dynamic circle", "move active teammates closer locally", func(c CliksConfig) string { return onOff(c.Listening.DynamicPlacement) }, func(c *CliksConfig, _ int) { c.Listening.DynamicPlacement = !c.Listening.DynamicPlacement }},
		{"Shuffle mins", "dynamic circle refresh interval", func(c CliksConfig) string { return fmt.Sprintf("%d min", c.Listening.ShuffleMinutes) }, func(c *CliksConfig, d int) {
			c.Listening.ShuffleMinutes = clampInt(c.Listening.ShuffleMinutes+d, 1, 60)
		}},
		{"Fatigue fade", "soften long typing bursts", func(c CliksConfig) string { return onOff(c.Listening.FatigueProtection) }, func(c *CliksConfig, _ int) { c.Listening.FatigueProtection = !c.Listening.FatigueProtection }},
		{"Hear keyboard", "play teammate keyboard events", func(c CliksConfig) string { return onOff(c.Listening.Keyboard) }, func(c *CliksConfig, _ int) { c.Listening.Keyboard = !c.Listening.Keyboard }},
		{"Hear mouse", "play teammate click events", func(c CliksConfig) string { return onOff(c.Listening.Mouse) }, func(c *CliksConfig, _ int) { c.Listening.Mouse = !c.Listening.Mouse }},
		{"Self monitor", "hear your own local test events", func(c CliksConfig) string { return onOff(c.Listening.Self) }, func(c *CliksConfig, _ int) { c.Listening.Self = !c.Listening.Self }},
		{"Share keyboard", "send keyboard activity kind only", func(c CliksConfig) string { return onOff(c.Sharing.Keyboard) }, func(c *CliksConfig, _ int) { c.Sharing.Keyboard = !c.Sharing.Keyboard }},
		{"Share mouse", "send left/right click activity only", func(c CliksConfig) string { return onOff(c.Sharing.Mouse) }, func(c *CliksConfig, _ int) { c.Sharing.Mouse = !c.Sharing.Mouse }},
		{"Keep Running", "saved terminal-close preference", func(c CliksConfig) string { return onOff(c.KeepRunning) }, func(c *CliksConfig, _ int) { c.KeepRunning = !c.KeepRunning }},
		{"Current team", "cycle saved teams", func(c CliksConfig) string { return valueOr(teamLabel(c, c.CurrentTeamCode), "not set") }, func(c *CliksConfig, d int) { cycleTeam(c, d) }},
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
		m.mode = valuePlain(m.formReturnMode, "home")
		m.cursor = 0
		m.mouseOver = false
		m.message = "Cancelled."
		return m, nil
	case "up", "shift+tab":
		m.formCursor = clampInt(m.formCursor-1, 0, m.formFieldCount()-1)
		m.moveFormTextCursorToEnd()
		return m, nil
	case "down", "tab":
		m.formCursor = clampInt(m.formCursor+1, 0, m.formFieldCount()-1)
		m.moveFormTextCursorToEnd()
		return m, nil
	case "left", "ctrl+b":
		m.formTextCursor = clampInt(m.formTextCursor-1, 0, len([]rune(m.formValue())))
		return m, nil
	case "right", "ctrl+f":
		m.formTextCursor = clampInt(m.formTextCursor+1, 0, len([]rune(m.formValue())))
		return m, nil
	case "home", "ctrl+a":
		m.formTextCursor = 0
		return m, nil
	case "end", "ctrl+e":
		m.moveFormTextCursorToEnd()
		return m, nil
	case "enter":
		if m.formCursor < m.formFieldCount()-1 {
			m.formCursor++
			m.moveFormTextCursorToEnd()
			return m, nil
		}
		return m.submitForm()
	case "backspace", "ctrl+h":
		m.trimFormValue()
		return m, nil
	case "delete":
		m.deleteFormValueAtCursor()
		return m, nil
	case "ctrl+u":
		m.setFormValue("")
		m.formTextCursor = 0
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.insertFormRunes(msg.Runes)
	}
	return m, nil
}

func (m homeModel) submitForm() (tea.Model, tea.Cmd) {
	if m.mode == "nickname" {
		name := sanitizeNickname(m.nicknameValue)
		m.cfg.Nickname = name
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.mode = valuePlain(m.formReturnMode, "team")
		m.cursor = 0
		m.mouseOver = false
		m.message = fmt.Sprintf("Nickname set to %s.", valuePlain(name, "anonymous"))
		return m, nil
	}
	if m.mode == "audio-device" {
		device := strings.TrimSpace(m.audioDeviceValue)
		if strings.EqualFold(device, "default") {
			device = ""
		}
		m.cfg.Listening.AudioDevice = device
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.mode = "advanced"
		m.cursor = 1
		m.message = fmt.Sprintf("Audio output set to %s.", valuePlain(device, "default"))
		return m, nil
	}
	if m.mode == "batch-window" {
		window, err := parseBatchWindow(m.batchWindowValue)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.cfg.BatchWindowMs = window
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.mode = "advanced"
		m.cursor = 2
		m.message = fmt.Sprintf("Batch window set to %d ms.", window)
		return m, nil
	}
	if m.mode == "join" {
		code := strings.ToUpper(strings.TrimSpace(m.joinCode))
		if code == "" {
			m.message = "Team code is required."
			m.formCursor = 0
			return m, nil
		}
		m.busy = true
		m.message = "Joining team..."
		return m, joinTeamCmd(code)
	}
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
	if m.mode == "nickname" || m.mode == "join" || m.mode == "audio-device" || m.mode == "batch-window" {
		return 1
	}
	return 2
}

func (m homeModel) formValue() string {
	switch m.mode {
	case "create":
		if m.formCursor == 0 {
			return m.createName
		}
		return m.createPassword
	case "join":
		return m.joinCode
	case "delete":
		if m.formCursor == 0 {
			return m.deleteCode
		}
		return m.deletePassword
	case "nickname":
		return m.nicknameValue
	case "audio-device":
		return m.audioDeviceValue
	case "batch-window":
		return m.batchWindowValue
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
	case "join":
		m.joinCode = strings.ToUpper(value)
	case "delete":
		if m.formCursor == 0 {
			m.deleteCode = strings.ToUpper(value)
		} else {
			m.deletePassword = value
		}
	case "nickname":
		m.nicknameValue = value
	case "audio-device":
		m.audioDeviceValue = value
	case "batch-window":
		m.batchWindowValue = value
	}
}

func (m *homeModel) trimFormValue() {
	value := []rune(m.formValue())
	if len(value) == 0 || m.formTextCursor == 0 {
		return
	}
	index := clampInt(m.formTextCursor, 0, len(value))
	value = append(value[:index-1], value[index:]...)
	m.setFormValue(string(value))
	m.formTextCursor = index - 1
}

func (m *homeModel) insertFormRunes(inserted []rune) {
	value := []rune(m.formValue())
	index := clampInt(m.formTextCursor, 0, len(value))
	next := make([]rune, 0, len(value)+len(inserted))
	next = append(next, value[:index]...)
	next = append(next, inserted...)
	next = append(next, value[index:]...)
	m.setFormValue(string(next))
	m.formTextCursor = clampInt(index+len(inserted), 0, len([]rune(m.formValue())))
}

func (m *homeModel) deleteFormValueAtCursor() {
	value := []rune(m.formValue())
	index := clampInt(m.formTextCursor, 0, len(value))
	if index >= len(value) {
		return
	}
	value = append(value[:index], value[index+1:]...)
	m.setFormValue(string(value))
	m.formTextCursor = clampInt(index, 0, len([]rune(m.formValue())))
}

func (m *homeModel) moveFormTextCursorToEnd() {
	m.formTextCursor = len([]rune(m.formValue()))
}

func (m homeModel) formHit(x int, y int) int {
	if x < 3 || x > panelWidth(m.width)-3 {
		return -1
	}
	index := y - formRowsStartY()
	if index < 0 || index >= m.formFieldCount() {
		return -1
	}
	return index
}

func isFormMode(mode string) bool {
	switch mode {
	case "create", "join", "delete", "nickname", "audio-device", "batch-window":
		return true
	default:
		return false
	}
}

func parseBatchWindow(value string) (int, error) {
	window, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || window < 100 || window > 2000 {
		return 0, fmt.Errorf("batch window must be a whole number from 100 to 2000 ms")
	}
	return window, nil
}

func (m homeModel) formView() string {
	var title string
	var rows []string
	if m.mode == "create" {
		title = "Create Team"
		rows = []string{
			formLine("Team name", m.createName, "Cliks Room", m.formCursor == 0, m.formTextCursor, false),
			formLine("Delete password", m.createPassword, "not set", m.formCursor == 1, m.formTextCursor, true),
		}
	} else if m.mode == "join" {
		title = "Join Team"
		rows = []string{
			formLine("Team code", m.joinCode, "CLIK-XXXXXX", true, m.formTextCursor, false),
		}
	} else if m.mode == "delete" {
		title = "Delete Team"
		rows = []string{
			formLine("Team code", m.deleteCode, "CLIK-XXXXXX", m.formCursor == 0, m.formTextCursor, false),
			formLine("Delete password", m.deletePassword, "not set", m.formCursor == 1, m.formTextCursor, true),
		}
	} else if m.mode == "audio-device" {
		title = "Audio Output"
		rows = []string{
			formLine("Device", m.audioDeviceValue, "default", true, m.formTextCursor, false),
		}
	} else if m.mode == "batch-window" {
		title = "Batch Window"
		rows = []string{
			formLine("Milliseconds", m.batchWindowValue, "500", true, m.formTextCursor, false),
		}
	} else {
		title = "Nickname"
		rows = []string{
			formLine("Display name", m.nicknameValue, "anonymous", true, m.formTextCursor, false),
		}
	}
	lines := []string{styleAccent.Render(title), ""}
	lines = append(lines, rows...)
	lines = append(lines, "")
	if m.busy {
		lines = append(lines, styleAccent.Render(m.message))
	} else {
		lines = append(lines, styleDim.Render("Left/right edits at the cursor. Enter submits. Tab changes fields. Esc cancels."))
		lines = append(lines, styleDim.Render(m.message))
	}
	return stylePanel.Width(panelWidth(m.width)).Render(strings.Join(lines, "\n"))
}

func formLine(label string, value string, placeholder string, selected bool, cursor int, secret bool) string {
	display := value
	if secret {
		display = strings.Repeat("*", len([]rune(value)))
	}
	if selected {
		runes := []rune(display)
		cursor = clampInt(cursor, 0, len(runes))
		display = string(runes[:cursor]) + "|" + string(runes[cursor:])
	} else if display == "" {
		display = placeholder
	}
	line := fmt.Sprintf("%-18s %s", label, display)
	if selected {
		return styleSelected.Render(" " + line + " ")
	}
	return line
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
		return formDoneMsg{kind: "create", code: team.Code, message: clipboardStatus(team.Code), cfg: next}
	}
}

func joinTeamCmd(code string) tea.Cmd {
	return func() tea.Msg {
		cfg := loadConfig()
		team, err := getTeamViaAPI(cfg, code)
		if err != nil {
			return formDoneMsg{kind: "join", code: code, err: err}
		}
		next, err := rememberTeam(team.Code, team.Name)
		if err != nil {
			return formDoneMsg{kind: "join", code: team.Code, err: err}
		}
		return formDoneMsg{kind: "join", code: team.Code, cfg: next}
	}
}

func toggleBackgroundCmd(code string, active ActiveSessionState, activeOK bool) tea.Cmd {
	return func() tea.Msg {
		if activeOK {
			return commandDoneMsg{message: "Keep Running is already on. Use Stop to disconnect this device."}
		}
		if code == "" {
			return commandDoneMsg{err: fmt.Errorf("no team selected. Create or join a team first")}
		}
		message, err := startBackgroundForTeam(code)
		return commandDoneMsg{message: message, err: err}
	}
}

func stopConnectionCmd(active ActiveSessionState, activeOK bool) tea.Cmd {
	return func() tea.Msg {
		if !activeOK {
			return commandDoneMsg{message: "No active local connection."}
		}
		message, err := stopActiveSession()
		return commandDoneMsg{message: message, err: err}
	}
}

func toggleAutostartCmd(code string) tea.Cmd {
	return func() tea.Msg {
		if autostartEnabled() {
			message, err := autostartAction([]string{"disable"})
			return commandDoneMsg{message: message, err: err}
		}
		if code == "" {
			return commandDoneMsg{err: fmt.Errorf("no team selected. Create or join a team first")}
		}
		message, err := autostartAction([]string{"enable", code})
		return commandDoneMsg{message: message, err: err}
	}
}

func autostartStatusCmd() tea.Cmd {
	return func() tea.Msg {
		message, err := autostartAction([]string{"status"})
		return commandDoneMsg{message: message, err: err}
	}
}

func soundTestCmd() tea.Cmd {
	return func() tea.Msg {
		audio := newAudioEngine(loadConfig().Listening)
		if err := audio.preview(); err != nil {
			return commandDoneMsg{err: err}
		}
		return commandDoneMsg{message: "Played keyboard and mouse test sounds."}
	}
}

func doctorSummaryCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := loadConfig()
		player, spatial, hint, _ := getAudioPlayerStatus(cfg.Listening.AudioDevice)
		if player == "" {
			return commandDoneMsg{message: "Audio player missing: " + hint}
		}
		if hint != "" {
			return commandDoneMsg{message: "Audio setup warning: " + hint}
		}
		if os.Getenv("CLIKS_SKIP_INPUT_DOCTOR") == "" && runtime.GOOS == "linux" {
			input := linuxInputStatus()
			if input.hasInputDir && input.eventCount > 0 && input.readableCount == 0 {
				return commandDoneMsg{message: "Input permission needed: sudo usermod -aG input " + input.username + "; then log out and back in."}
			}
		}
		if cfg.CurrentTeamCode == "" {
			return commandDoneMsg{message: "Setup needs a team. Create or join one first."}
		}
		if spatial {
			return commandDoneMsg{message: fmt.Sprintf("Doctor OK. Audio: %s with stereo spatial support.", player)}
		}
		return commandDoneMsg{message: fmt.Sprintf("Doctor OK. Audio: %s with basic/distance playback.", player)}
	}
}

func deleteTeamCmd(code string, password string) tea.Cmd {
	return func() tea.Msg {
		cfg := loadConfig()
		if err := deleteTeamViaAPI(cfg, code, password); err != nil {
			return formDoneMsg{kind: "delete", code: code, err: err}
		}
		cfg, err := forgetTeam(code)
		if err != nil {
			return formDoneMsg{kind: "delete", code: code, err: err}
		}
		stopDeletedTeamSession(code)
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
type sessionTickMsg time.Time

type sessionModel struct {
	controller     *sessionController
	state          SessionViewState
	mode           string
	settingsCursor int
	settingsHover  bool
	buttonHover    int
	codeHover      bool
	message        string
	exit           sessionExitAction
	width          int
	height         int
	now            time.Time
	helpOpen       bool
}

func newSessionModel(controller *sessionController) sessionModel {
	return sessionModel{controller: controller, state: controller.viewState(), buttonHover: -1, now: time.Now()}
}

func (m sessionModel) Init() tea.Cmd {
	return tea.Batch(waitForSessionUpdate(m.controller), sessionTick())
}

func (m sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case sessionUpdateMsg:
		m.state = SessionViewState(msg)
		return m, waitForSessionUpdate(m.controller)
	case sessionTickMsg:
		m.now = time.Time(msg)
		m.state = m.controller.viewState()
		return m, sessionTick()
	case tea.MouseMsg:
		if m.helpOpen {
			return m, nil
		}
		if m.mode == "settings" {
			rows := settingsRows(m.controller.cfg)
			if msg.Type == tea.MouseWheelUp {
				m.settingsCursor = clampInt(m.settingsCursor-1, 0, len(rows)-1)
				m.settingsHover = false
			}
			if msg.Type == tea.MouseWheelDown {
				m.settingsCursor = clampInt(m.settingsCursor+1, 0, len(rows)-1)
				m.settingsHover = false
			}
			if msg.Type == tea.MouseMotion {
				if index := m.settingsHit(msg.X, msg.Y); index >= 0 && index < len(rows) {
					m.settingsCursor = index
					m.settingsHover = true
				} else {
					m.settingsHover = false
				}
			}
			if msg.Type == tea.MouseLeft {
				index := m.settingsHit(msg.X, msg.Y)
				if index < 0 || index >= len(rows) {
					m.settingsHover = false
					return m, nil
				}
				m.settingsCursor = index
				m.settingsHover = true
				m.applyLiveSetting(1)
			}
			return m, nil
		}
		switch msg.Type {
		case tea.MouseMotion:
			m.buttonHover = m.controlButtonHit(msg.X, msg.Y)
			m.codeHover = m.teamCodeHit(msg.X, msg.Y)
		case tea.MouseWheelUp:
			m.controller.adjustVolume(0.05)
			m.buttonHover = -1
		case tea.MouseWheelDown:
			m.controller.adjustVolume(-0.05)
			m.buttonHover = -1
		case tea.MouseLeft:
			if m.teamCodeHit(msg.X, msg.Y) {
				m.codeHover = true
				m.message = clipboardStatus(m.state.TeamCode)
				return m, nil
			}
			if index := m.controlButtonHit(msg.X, msg.Y); index >= 0 {
				m.buttonHover = index
				return m.activateLiveButton(index)
			}
			if m.state.CaptureMode == "terminal" && m.state.Listening.Mouse {
				m.controller.recordLocalActivity(LocalActivityEvent{Kind: "mouse", Button: "left", At: time.Now()})
			}
		}
	case tea.KeyMsg:
		if m.helpOpen {
			switch msg.String() {
			case "?", "esc", "q":
				m.helpOpen = false
			}
			return m, nil
		}
		if msg.String() == "?" {
			m.helpOpen = true
			return m, nil
		}
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
		case "ctrl+c":
			m.exit = sessionExitStop
			return m, tea.Quit
		case "q", "esc":
			m.exit = sessionExitBack
			return m, tea.Quit
		case "b":
			m.exit = sessionExitBack
			return m, tea.Quit
		case "x":
			m.exit = sessionExitStop
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
	if m.helpOpen {
		context := "live"
		if m.mode == "settings" {
			context = "live-preferences"
		}
		return lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("Cliks Shortcuts"), shortcutHelpView(context, m.width))
	}
	if m.mode == "settings" {
		return m.sessionSettingsView()
	}
	left, right, _ := m.livePanelLines()
	width := panelWidth(m.width)
	colWidth := (width - 6) / 2
	room := stylePanel.Width(colWidth).Render(strings.Join(left, "\n"))
	sound := stylePanel.Width(colWidth).Render(strings.Join(right, "\n"))
	controls := m.liveControlsStyle().Width(width).Render(m.liveControlsLine())
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Cliks Live"),
		lipgloss.JoinHorizontal(lipgloss.Top, room, "  ", sound),
		controls,
	)
}

func (m sessionModel) livePanelLines() ([]string, []string, int) {
	state := m.state
	teamName := strings.TrimSpace(state.TeamName)
	if teamName == "" || strings.EqualFold(teamName, state.TeamCode) {
		teamName = teamNameForCode(m.controller.cfg, state.TeamCode)
	}
	if teamName == "" {
		teamName = "Team"
	}
	code := valuePlain(state.TeamCode, m.controller.cfg.CurrentTeamCode)
	codeValue := code
	if m.codeHover {
		codeValue = styleSelected.Render(" " + code + " ")
	}
	if m.height > 0 && m.height < 24 {
		left := []string{
			"Team: " + styleAccent.Render(teamName),
			"Code: " + codeValue,
			"You:  " + valuePlain(m.controller.cfg.Nickname, "anonymous"),
			"Connection: " + connectionStyle(state.ConnectionStatus),
			"People: " + roomPeopleSummary(state),
			"Typing: " + typingSummary(state, m.now),
			"Flow: " + flowBadge(state, m.now),
			"Health: " + healthSummary(state, m.now),
			fmt.Sprintf("Activity: %d captured, %d sent", state.LocalCapturedEvents, state.LocalSentEvents),
		}
		if state.Notice != "" {
			left = append(left, styleWarn.Render(state.Notice))
		} else if state.PermissionHint != "" {
			left = append(left, styleWarn.Render(state.PermissionHint))
		}
		right := []string{
			"Sound",
			"Volume  " + muteAwareBar(state.Listening),
			"Density " + bar(state.Listening.Density),
			fmt.Sprintf("Mute %s  Spatial %s", onOff(state.Listening.Muted), onOff(state.Listening.Spatial)),
			fmt.Sprintf("Fade %s  Keep %s", onOff(state.Listening.FatigueProtection), onOff(m.controller.cfg.KeepRunning)),
			"↑/↓ volume  ←/→ density  ? help",
		}
		return left, right, 1
	}
	left := []string{
		"Team: " + styleAccent.Render(teamName),
		"Code: " + codeValue,
		"You:  " + valuePlain(m.controller.cfg.Nickname, "anonymous"),
		"",
		"Connection: " + connectionStyle(state.ConnectionStatus),
		"People: " + roomPeopleSummary(state),
		"Typing: " + typingSummary(state, m.now),
		"Flow: " + flowBadge(state, m.now),
		"Health: " + healthSummary(state, m.now),
		"Capture: " + state.CaptureMode,
		"",
		fmt.Sprintf("Captured: %d", state.LocalCapturedEvents),
		fmt.Sprintf("Sent:     %d", state.LocalSentEvents),
	}
	if state.Notice != "" {
		left = append(left, "", styleWarn.Render(state.Notice))
	}
	if m.message != "" {
		left = append(left, styleDim.Render(m.message))
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
		"Keep    " + onOff(m.controller.cfg.KeepRunning),
		"",
		"Keys: ↑/↓ volume  ←/→ density",
		"m mute  s spatial  f fade  Tab prefs",
		"Esc/q/back returns  x stop  ? shortcuts",
		"Mouse: wheel volume; click hovered controls",
	}
	return left, right, 1
}

type liveButton struct {
	label  string
	action string
}

func liveButtons() []liveButton {
	return []liveButton{
		{label: "Back", action: "back"},
		{label: "Prefs", action: "prefs"},
		{label: "Vol-", action: "vol-down"},
		{label: "Vol+", action: "vol-up"},
		{label: "Den-", action: "density-down"},
		{label: "Den+", action: "density-up"},
		{label: "Mute", action: "mute"},
		{label: "Spatial", action: "spatial"},
		{label: "Fade", action: "fade"},
		{label: "Stop", action: "stop"},
	}
}

func (m sessionModel) liveControlsLine() string {
	parts := []string{}
	for index, button := range liveButtons() {
		part := "[ " + button.label + " ]"
		if index == m.buttonHover {
			part = styleSelected.Render(part)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
}

func (m sessionModel) liveControlsStyle() lipgloss.Style {
	if m.height > 0 && m.height < 24 {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPanel).Padding(0, 2)
	}
	return stylePanel
}

func (m sessionModel) activateLiveButton(index int) (tea.Model, tea.Cmd) {
	buttons := liveButtons()
	if index < 0 || index >= len(buttons) {
		return m, nil
	}
	switch buttons[index].action {
	case "back":
		m.exit = sessionExitBack
		return m, tea.Quit
	case "prefs":
		m.mode = "settings"
		m.settingsCursor = 0
		m.settingsHover = false
	case "vol-down":
		m.controller.adjustVolume(-0.05)
	case "vol-up":
		m.controller.adjustVolume(0.05)
	case "density-down":
		m.controller.adjustDensity(-0.1)
	case "density-up":
		m.controller.adjustDensity(0.1)
	case "mute":
		m.controller.toggle("muted")
	case "spatial":
		m.controller.toggle("spatial")
	case "fade":
		m.controller.toggle("fade")
	case "stop":
		m.exit = sessionExitStop
		return m, tea.Quit
	}
	return m, nil
}

func (m sessionModel) livePanelHeight() int {
	left, right, _ := m.livePanelLines()
	width := panelWidth(m.width)
	colWidth := (width - 6) / 2
	room := stylePanel.Width(colWidth).Render(strings.Join(left, "\n"))
	sound := stylePanel.Width(colWidth).Render(strings.Join(right, "\n"))
	return maxInt(lipgloss.Height(room), lipgloss.Height(sound))
}

func (m sessionModel) controlsContentY() int {
	padding := 2
	if m.height > 0 && m.height < 24 {
		padding = 1
	}
	return 1 + m.livePanelHeight() + padding
}

func (m sessionModel) controlButtonHit(x int, y int) int {
	if y != m.controlsContentY() {
		return -1
	}
	cursor := 3
	for index, button := range liveButtons() {
		width := len("[ " + button.label + " ]")
		if x >= cursor && x < cursor+width {
			return index
		}
		cursor += width + 1
	}
	return -1
}

func (m sessionModel) teamCodeHit(x int, y int) bool {
	code := valuePlain(m.state.TeamCode, m.controller.cfg.CurrentTeamCode)
	if code == "" {
		return false
	}
	_, _, codeLine := m.livePanelLines()
	codeY := 3 + codeLine
	codeX := 3 + len("Code: ")
	return y == codeY && x >= codeX && x < codeX+len(code)
}

func (m sessionModel) settingsHit(x int, y int) int {
	if x < 3 || x > panelWidth(m.width)-3 {
		return -1
	}
	rows := settingsRows(m.controller.cfg)
	start, end := settingsWindow(len(rows), m.settingsCursor, m.height)
	index := start + y - settingsRowsStartY()
	if index < start || index >= end {
		return -1
	}
	return index
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
	start, end := settingsWindow(len(rows), m.settingsCursor, m.height)
	var lines []string
	lines = append(lines, styleAccent.Render("Live Settings"))
	lines = append(lines, "")
	for i := start; i < end; i++ {
		row := rows[i]
		line := fmt.Sprintf("%-18s %-24s %s", row.label, row.value(cfg), styleDim.Render(row.help))
		if i == m.settingsCursor {
			if m.settingsHover {
				line = styleSelected.Render(" " + line + " ")
			} else {
				line = styleFocused.Render("> " + line)
			}
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	footer := "Left/right adjusts. Enter toggles. Tab/Esc/q returns to the live room."
	if start > 0 || end < len(rows) {
		footer = fmt.Sprintf("Showing %d-%d of %d. Up/down reveals more. Tab/Esc/q returns.", start+1, end, len(rows))
	}
	lines = append(lines, styleDim.Render(footer))
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Cliks Preferences"),
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

func sessionTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return sessionTickMsg(t)
	})
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
	return fmt.Sprintf("Desk is warm for %s. Press Enter to start.", teamLabel(cfg, cfg.CurrentTeamCode))
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
	case "ctrl+c", "q", "esc", "b", "x", "tab", "S", "up", "down", "left", "right", "+", "-", "[", "]", "m", "s", "f":
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

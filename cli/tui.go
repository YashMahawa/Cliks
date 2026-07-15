package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#9A4D00", Dark: "#F2A65A"}
	colorDim    = lipgloss.AdaptiveColor{Light: "#5B5751", Dark: "#A9A39A"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#9A4D00", Dark: "#FFB454"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#18743A", Dark: "#55D98B"}
	colorPanel  = lipgloss.AdaptiveColor{Light: "#B65E2E", Dark: "#D97746"}
	colorSelect = lipgloss.AdaptiveColor{Light: "#9A4D00", Dark: "#D97746"}
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

func applyTheme(theme string) {
	switch theme {
	case "ocean":
		colorAccent = lipgloss.AdaptiveColor{Light: "#006D7D", Dark: "#33D6E8"}
		colorPanel = lipgloss.AdaptiveColor{Light: "#007487", Dark: "#159BB5"}
		colorSelect = lipgloss.AdaptiveColor{Light: "#007487", Dark: "#33D6E8"}
		colorOnPick = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#071013"}
	case "mono":
		colorAccent = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#EEEEEE"}
		colorPanel = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#8A8A8A"}
		colorSelect = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#EEEEEE"}
		colorOnPick = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111111"}
	default:
		colorAccent = lipgloss.AdaptiveColor{Light: "#9A4D00", Dark: "#F2A65A"}
		colorPanel = lipgloss.AdaptiveColor{Light: "#B65E2E", Dark: "#D97746"}
		colorSelect = lipgloss.AdaptiveColor{Light: "#9A4D00", Dark: "#D97746"}
		colorOnPick = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#11100F"}
	}
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colorOnPick).Background(colorSelect).Padding(0, 1)
	styleAccent = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleDim = lipgloss.NewStyle().Foreground(colorDim)
	styleWarn = lipgloss.NewStyle().Foreground(colorWarn)
	styleOK = lipgloss.NewStyle().Foreground(colorOK)
	stylePanel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPanel).Padding(1, 2)
	styleSelected = lipgloss.NewStyle().Foreground(colorOnPick).Background(colorSelect).Bold(true)
	styleFocused = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
}

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
			{"1 / 2 / 3 / 4 / 5", "wave, nice, coffee, celebrate, or suggest a break"},
			{"p", "cycle available, focus, break, and do not disturb"},
			{"Up/+, Down/-", "raise or lower volume"},
			{"Right/], Left/[", "raise or lower sound density"},
			{"m / s / f", "toggle mute, spatial audio, or fatigue fade"},
			{"Tab/Shift+S", "open live preferences"},
			{"Esc/q/b", "return to the main control screen"},
			{"x/Ctrl+C", "stop and disconnect this session"},
			{"Mouse wheel", "adjust volume"},
			{"?", "close this shortcut guide"},
		}
	case "doctor-report":
		return []shortcutHelp{
			{"Up/k, Down/j", "scroll through the setup report"},
			{"PageUp/PageDown", "scroll one report page"},
			{"r", "run every setup check again"},
			{"Esc/q/b", "return to Diagnostics"},
			{"Mouse wheel", "scroll the report"},
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
	actionSetup            homeAction = "setup"
	actionDoctor           homeAction = "doctor"
	actionSoundTest        homeAction = "sound-test"
	actionSolo             homeAction = "solo"
	actionBackgroundStart  homeAction = "background-start"
	actionBackgroundStop   homeAction = "background-stop"
	actionBackgroundStatus homeAction = "background-status"
	actionAutostartEnable  homeAction = "autostart-enable"
	actionAutostartDisable homeAction = "autostart-disable"
	actionAutostartStatus  homeAction = "autostart-status"
)

type homeModel struct {
	cfg                   CliksConfig
	active                ActiveSessionState
	activeOK              bool
	cursor                int
	mouseOver             bool
	mode                  string
	message               string
	action                homeAction
	settingsCursor        int
	formCursor            int
	formTextCursor        int
	formReturnMode        string
	createName            string
	createPassword        string
	joinCode              string
	deleteCode            string
	deletePassword        string
	nicknameValue         string
	audioDeviceValue      string
	batchWindowValue      string
	backendURLValue       string
	stopActiveOnExit      bool
	busy                  bool
	width                 int
	height                int
	helpOpen              bool
	doctorLines           []string
	doctorOffset          int
	doctorHover           string
	doctorReturnMode      string
	preferencesReturnMode string
	codeHover             bool
	launchStartedAt       time.Time
	launchUntil           time.Time
	launchSoundPhase      int
	firstLaunch           bool
	onboardingPending     bool
	onboardingStep        int
	onboardingQuip        int
	onboardingSuggestion  string
}

func runHomeTUI(cfg CliksConfig) error {
	if !isInteractiveTerminal() {
		printHelp("cliks")
		return nil
	}
	applyTheme(cfg.Theme)
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
	now := time.Now()
	firstLaunch := !cfg.LaunchSeen
	if firstLaunch {
		cfg.LaunchSeen = true
		_ = saveConfig(cfg)
	}
	launchDuration := 3 * time.Second
	if firstLaunch {
		launchDuration = 10 * time.Second
	}
	model := homeModel{cfg: cfg, active: active, activeOK: activeOK, mode: "home", message: message, stopActiveOnExit: activeOK && deferredStopMatches(active), launchStartedAt: now, launchUntil: now.Add(launchDuration), firstLaunch: firstLaunch, onboardingPending: !cfg.OnboardingSeen, onboardingQuip: int(now.UnixNano() % 7), onboardingSuggestion: randomFunnyNickname()}
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
	case actionSetup:
		return cmdSetup(nil)
	case actionDoctor:
		return runDoctor()
	case actionSoundTest:
		return runSoundTest()
	case actionSolo:
		return runSoloTUI(result.cfg)
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

type launchTickMsg time.Time

func (m homeModel) Init() tea.Cmd {
	commands := []tea.Cmd{homeTick(), launchTick()}
	commands = append(commands, welcomeSoundCmd(m.cfg.Listening))
	return tea.Batch(commands...)
}

func launchTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg { return launchTickMsg(t) })
}

func welcomeSoundCmd(listening ListeningConfig) tea.Cmd {
	return func() tea.Msg {
		audio := newAudioEngine(listening)
		defer audio.Close()
		_ = audio.preview()
		return nil
	}
}

func ambientPreviewCmd(listening ListeningConfig) tea.Cmd {
	return func() tea.Msg {
		audio := newAudioEngine(listening)
		time.Sleep(1800 * time.Millisecond)
		audio.Close()
		return nil
	}
}

func launchSoundCuePhase(elapsed time.Duration) int {
	if elapsed >= 7*time.Second {
		return 2
	}
	if elapsed >= 3500*time.Millisecond {
		return 1
	}
	return 0
}

func (m *homeModel) finishLaunch() {
	m.launchUntil = time.Now()
	if m.onboardingPending {
		m.mode = "first-setup"
		m.cursor = 0
		m.mouseOver = false
		if m.onboardingSuggestion == "" {
			m.onboardingSuggestion = randomFunnyNickname()
		}
		m.message = "One choice at a time. Nothing here is permanent."
	}
}

type onboardingPermissionMsg struct{ message string }

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case launchTickMsg:
		if time.Now().Before(m.launchUntil) {
			commands := []tea.Cmd{launchTick()}
			if m.firstLaunch {
				phase := launchSoundCuePhase(time.Since(m.launchStartedAt))
				if phase > m.launchSoundPhase {
					m.launchSoundPhase = phase
					commands = append(commands, welcomeSoundCmd(m.cfg.Listening))
				}
			}
			return m, tea.Batch(commands...)
		}
		m.finishLaunch()
		return m, nil
	case homeTickMsg:
		m.refreshRuntime()
		return m, homeTick()
	case commandDoneMsg:
		m.busy = false
		m.refreshRuntime()
		m.cfg = loadConfig()
		if msg.err != nil {
			m.message = msg.err.Error()
		} else if len(msg.report) > 0 {
			m.doctorLines = msg.report
			m.doctorOffset = 0
			m.doctorHover = ""
			m.mode = "doctor-report"
			m.message = msg.message
		} else {
			m.message = msg.message
		}
		return m, nil
	case onboardingPermissionMsg:
		m.busy = false
		m.advanceOnboarding(msg.message)
		return m, nil
	case notificationTestMsg:
		if msg.err != nil {
			m.message = "Notification test needs attention: " + msg.err.Error()
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
			if msg.message != "" {
				m.message += " " + msg.message
			}
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
		if time.Now().Before(m.launchUntil) {
			if msg.Type == tea.MouseLeft {
				m.finishLaunch()
			}
			return m, nil
		}
		if m.helpOpen {
			return m, nil
		}
		if m.mode == "doctor-report" {
			switch msg.Type {
			case tea.MouseWheelUp:
				m.scrollDoctor(-3)
			case tea.MouseWheelDown:
				m.scrollDoctor(3)
			case tea.MouseMotion:
				m.doctorHover = m.doctorActionAt(msg.X, msg.Y)
			case tea.MouseLeft:
				switch m.doctorActionAt(msg.X, msg.Y) {
				case "back":
					m.back()
				case "refresh":
					m.busy = true
					m.message = "Checking setup..."
					return m, doctorSummaryCmd()
				}
			}
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
			m.codeHover = m.homeCodeHit(msg.X, msg.Y)
			if m.codeHover {
				m.mouseOver = false
				return m, nil
			}
			m.mouseOver = m.hover(msg.Y)
		}
		if msg.Type == tea.MouseLeft {
			if m.homeCodeHit(msg.X, msg.Y) {
				m.message = clipboardStatus(m.cfg.CurrentTeamCode)
				m.codeHover = true
				m.mouseOver = false
				return m, nil
			}
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
		if time.Now().Before(m.launchUntil) {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "enter", " ", "esc":
				m.finishLaunch()
			}
			return m, nil
		}
		if m.helpOpen {
			switch msg.String() {
			case "?", "esc", "q":
				m.helpOpen = false
			}
			return m, nil
		}
		if m.mode == "first-setup" {
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				return m, tea.Quit
			case "left", "h", "backspace":
				if m.onboardingStep > 0 {
					m.onboardingStep--
					m.cursor = 0
					m.mouseOver = false
					m.message = "Previous choice. Change anything you like."
				}
				return m, nil
			}
		}
		if isFormMode(m.mode) {
			return m.updateForm(msg)
		}
		if m.mode == "doctor-report" {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				m.scrollDoctor(-1)
			case "down", "j":
				m.scrollDoctor(1)
			case "pgup":
				m.scrollDoctor(-m.doctorVisibleCount())
			case "pgdown":
				m.scrollDoctor(m.doctorVisibleCount())
			case "r":
				m.busy = true
				m.message = "Checking setup..."
				return m, doctorSummaryCmd()
			case "esc", "q", "b":
				m.back()
			case "?":
				m.helpOpen = true
			}
			return m, nil
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
	if time.Now().Before(m.launchUntil) {
		return m.launchView()
	}
	var body string
	if m.helpOpen {
		context := m.mode
		if context != "preferences" && context != "doctor-report" {
			context = "home"
		}
		body = shortcutHelpView(context, m.width)
	} else if m.mode == "first-setup" {
		body = m.itemView()
		return lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("Cliks  /  first light"), body)
	} else if m.mode == "preferences" {
		body = m.preferencesView()
	} else if m.mode == "doctor-report" {
		body = m.doctorReportView()
	} else if isFormMode(m.mode) {
		body = m.formView()
	} else {
		body = m.itemView()
	}
	return lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("Cliks"), body, m.statusFooterView())
}

func (m homeModel) launchView() string {
	width := maxInt(40, panelWidth(m.width))
	height := maxInt(14, m.height-7)
	elapsed := time.Since(m.launchStartedAt)
	canvas := renderLaunchCanvas(maxInt(34, width-6), maxInt(10, height-4), elapsed, m.firstLaunch, teamLabel(m.cfg, m.cfg.CurrentTeamCode))
	return stylePanel.Width(width).Height(height).Render(styleAccent.Render(canvas))
}

func renderLaunchCanvas(width int, height int, elapsed time.Duration, firstLaunch bool, team string) string {
	width = maxInt(34, width)
	height = maxInt(10, height)
	grid := make([][]rune, height)
	for y := range grid {
		grid[y] = []rune(strings.Repeat(" ", width))
	}
	put := func(x int, y int, value string) {
		if y < 0 || y >= height {
			return
		}
		for index, char := range []rune(value) {
			px := x + index
			if px >= 0 && px < width {
				grid[y][px] = char
			}
		}
	}
	center := func(y int, value string) {
		put((width-len([]rune(value)))/2, y, value)
	}
	cx, cy := width/2, (height+1)/2
	frame := int(elapsed.Milliseconds()/180) % 4
	ringProgress := clamp(elapsed.Seconds()/1.8, 0, 1)
	steps := int(180 * ringProgress)
	rx := float64(width) * 0.39
	ry := float64(height) * 0.28
	for step := 0; step < 180; step++ {
		angle := -math.Pi/2 + float64(step)*2*math.Pi/180
		glyph := "·"
		if step < steps && step%11 == 0 {
			glyph = "✦"
		} else if step < steps {
			glyph = "•"
		}
		put(cx+int(math.Cos(angle)*rx), cy+int(math.Sin(angle)*ry), glyph)
	}
	pulse := []string{"·   ·", "• · •", "✦ · ✦", "• ✦ •"}[frame]
	center(cy-1, pulse)
	center(cy, "[ YOU ]")

	if !firstLaunch {
		name := valuePlain(team, "your desk")
		center(0, "CLIKS  /  WARM DESK")
		center(maxInt(1, cy-3), "Opening "+name)
		beat := []string{"·", "✦", "•", "✦"}[frame]
		center(minInt(height-2, cy+2), beat+"  listening  "+beat)
		center(height-1, launchProgress(elapsed, 3*time.Second, width)+"  Enter skips")
		return launchGridString(grid)
	}

	phaseTitle := "A quiet room is taking shape"
	if elapsed >= 2*time.Second {
		phaseTitle = "Your people find their places"
	}
	if elapsed >= 4*time.Second {
		phaseTitle = "Activity becomes ambience"
	}
	if elapsed >= 6*time.Second {
		phaseTitle = "Small signals cross the room"
	}
	if elapsed >= 8500*time.Millisecond {
		phaseTitle = "Your desk is ready"
	}
	center(0, "CLIKS  /  YOUR FIRST WARM DESK")
	center(1, phaseTitle)

	type launchPeer struct {
		name      string
		x         int
		y         int
		appearsAt time.Duration
	}
	peers := []launchPeer{
		{name: "Mira", x: cx, y: cy - int(ry), appearsAt: 1200 * time.Millisecond},
		{name: "Sam", x: cx + int(rx), y: cy, appearsAt: 2100 * time.Millisecond},
		{name: "Kai", x: cx, y: cy + int(ry), appearsAt: 2900 * time.Millisecond},
		{name: "Noor", x: cx - int(rx), y: cy, appearsAt: 3600 * time.Millisecond},
	}
	for index, peer := range peers {
		if elapsed < peer.appearsAt {
			continue
		}
		marker := "○"
		if elapsed >= 4*time.Second && (index == 0 || index == 3) && frame%2 == 0 {
			marker = "◆"
		}
		label := marker + " " + peer.name
		put(peer.x-len([]rune(label))/2, peer.y, label)
	}
	if elapsed >= 4*time.Second && elapsed < 6*time.Second {
		center(minInt(height-3, cy+2), "Mira and Noor are typing  ◆  ◆")
	}
	if elapsed >= 6*time.Second && elapsed < 8500*time.Millisecond {
		burst := []string{"· 👋 ✦", "✦ 👋 ·", "· 👋 ·", "✦ 👋 ✦"}[frame]
		put(cx+int(rx)-len([]rune(burst))/2, maxInt(1, cy-2), burst)
		center(minInt(height-3, cy+2), "Sam  👋  Hey there!  •  a signal, not a meeting")
	}
	if elapsed >= 8500*time.Millisecond {
		burst := []string{"✦   ·   ✦", "·  ✦✦✦  ·", "✦  ·✹·  ✦", "·   ✦   ·"}[frame]
		center(maxInt(1, cy-2), burst)
		center(minInt(height-3, cy+2), "Hear the room  •  never share key values")
	}
	center(height-2, launchProgress(elapsed, 10*time.Second, width))
	center(height-1, "Enter skips  •  this is a local demo  •  nothing is sent")
	return launchGridString(grid)
}

func launchProgress(elapsed time.Duration, duration time.Duration, width int) string {
	barWidth := clampInt(width/5, 8, 20)
	filled := clampInt(int(float64(barWidth)*clamp(float64(elapsed)/float64(duration), 0, 1)), 0, barWidth)
	return "[" + strings.Repeat("━", filled) + strings.Repeat("─", barWidth-filled) + "]"
}

func launchGridString(grid [][]rune) string {
	lines := make([]string, len(grid))
	for index := range grid {
		lines[index] = strings.TrimRight(string(grid[index]), " ")
	}
	return strings.Join(lines, "\n")
}

func (m homeModel) fullPanel() lipgloss.Style {
	return stylePanel.Width(panelWidth(m.width)).Height(maxInt(10, m.height-8))
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
	case "solo":
		m.action = actionSolo
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
		if returnMode != "advanced" && returnMode != "first-setup" {
			returnMode = "team"
		}
		m.mode = "nickname"
		m.formCursor = 0
		m.formReturnMode = returnMode
		m.mouseOver = false
		m.nicknameValue = m.cfg.Nickname
		m.moveFormTextCursorToEnd()
		m.message = "Set the short name teammates see in the live room. Max 10 characters."
	case "onboarding-random-name":
		m.cfg.Nickname = m.onboardingSuggestion
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("You are now " + m.cfg.Nickname + ". Extremely credible.")
	case "onboarding-anonymous":
		m.cfg.Nickname = ""
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("Anonymous it is. Mysterious, but punctual.")
	case "onboarding-sound-balanced", "onboarding-sound-quiet", "onboarding-sound-lively":
		switch item.key {
		case "onboarding-sound-quiet":
			m.cfg.Listening.Volume, m.cfg.Listening.Density = 0.42, 0.48
		case "onboarding-sound-lively":
			m.cfg.Listening.Volume, m.cfg.Listening.Density = 0.78, 1
		default:
			m.cfg.Listening.Volume, m.cfg.Listening.Density = 0.7, 0.8
		}
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("Your listening mix is set.")
		return m, welcomeSoundCmd(m.cfg.Listening)
	case "onboarding-permission":
		m.busy = true
		m.message = "Opening the one OS permission Cliks needs…"
		return m, onboardingPermissionCmd()
	case "onboarding-permission-later":
		m.advanceOnboarding("Skipped for now. Setup Check can reopen it anytime.")
	case "onboarding-notify-sound", "onboarding-notify-quiet", "onboarding-notify-off":
		m.cfg.Notifications.Enabled = item.key != "onboarding-notify-off"
		m.cfg.Notifications.Sound = item.key == "onboarding-notify-sound"
		m.cfg.Notifications.Configured = true
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("Notification preference saved.")
		if m.cfg.Notifications.Enabled {
			cfg := m.cfg
			return m, func() tea.Msg {
				return notificationTestMsg{err: sendNativeNotification("Mira 👋 Welcome to Cliks", "A quick signal looks like this.", cfg.Notifications.Sound)}
			}
		}
	case "onboarding-background-on", "onboarding-background-off":
		m.cfg.KeepRunning = item.key == "onboarding-background-on"
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("Background preference saved.")
	case "onboarding-autostart-on", "onboarding-autostart-off":
		m.cfg.AutostartWanted = item.key == "onboarding-autostart-on"
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("Launch-at-login preference saved. It activates after you join a team.")
	case "onboarding-ambient-off", "onboarding-ambient-rain", "onboarding-ambient-cafe", "onboarding-ambient-deep":
		m.cfg.Listening.Ambient = strings.TrimPrefix(item.key, "onboarding-ambient-")
		_ = saveConfig(m.cfg)
		m.advanceOnboarding("Room tone saved. It stays private to your headphones.")
		if m.cfg.Listening.Ambient != "off" {
			return m, ambientPreviewCmd(m.cfg.Listening)
		}
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
		if usesPublicBackend(m.cfg) {
			m.message = "The public relay is fixed at 500 ms. Choose Server and use your own backend to tune 100-2000 ms."
			return m, nil
		}
		m.mode = "batch-window"
		m.formCursor = 0
		m.formReturnMode = "advanced"
		m.batchWindowValue = fmt.Sprintf("%d", m.cfg.BatchWindowMs)
		m.moveFormTextCursorToEnd()
		m.message = "100-2000 ms. The default 500 ms balances latency and network use."
	case "backend-url":
		returnMode := m.mode
		if returnMode != "menu" && returnMode != "advanced" && returnMode != "preferences" && returnMode != "first-setup" {
			returnMode = "advanced"
		}
		m.mode = "backend-url"
		m.formCursor = 0
		m.formReturnMode = returnMode
		m.backendURLValue = m.cfg.APIURL
		m.moveFormTextCursorToEnd()
		m.message = "Use public/default, or paste your self-hosted http(s) server. WebSocket is derived automatically."
	case "factory-reset":
		m.mode = "factory-reset"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Choose Reset Device to clear this device only."
	case "reset-confirm":
		if err := factoryResetDevice(); err != nil {
			m.message = "Could not reset: " + err.Error()
			return m, nil
		}
		m.cfg = defaultConfig()
		m.activeOK = false
		m.mode = "home"
		m.cursor = 0
		m.firstLaunch = true
		m.onboardingPending = true
		m.onboardingStep = 0
		m.onboardingSuggestion = randomFunnyNickname()
		m.onboardingQuip = int(time.Now().UnixNano() % 7)
		m.launchSoundPhase = 0
		m.launchStartedAt = time.Now()
		m.launchUntil = m.launchStartedAt.Add(10 * time.Second)
		m.message = "Factory reset complete. Welcome back to the beginning."
		return m, tea.Batch(launchTick(), welcomeSoundCmd(m.cfg.Listening))
	case "first-setup-done":
		m.cfg.OnboardingSeen = true
		_ = saveConfig(m.cfg)
		m.onboardingPending = false
		m.mode = "home"
		m.cursor = 0
		m.message = "Setup saved. Open Live when you are ready."
	case "preferences":
		m.preferencesReturnMode = m.mode
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
	case "setup":
		m.doctorReturnMode = m.mode
		m.busy = true
		m.message = "Running easy setup..."
		return m, setupSummaryCmd()
	case "notification-test":
		m.busy = true
		m.message = "Sending a native test notification..."
		return m, notificationSetupCmd()
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
	if row.label == "Server" {
		m.mode = "backend-url"
		m.formCursor = 0
		m.formReturnMode = "preferences"
		m.backendURLValue = m.cfg.APIURL
		m.moveFormTextCursorToEnd()
		m.message = "Paste a self-hosted http(s) URL, or type public."
		return
	}
	row.apply(&m.cfg, delta)
	applyTheme(m.cfg.Theme)
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
		if m.mode == "first-setup" {
			line = centerTerminalText(line, maxInt(30, panelWidth(m.width)-8))
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	if m.mode == "first-setup" {
		lines = append(lines, centerTerminalText(styleDim.Render("↑/↓ choose  •  Enter confirms  •  ← previous  •  Esc exits safely"), maxInt(30, panelWidth(m.width)-8)))
	} else {
		lines = append(lines, styleDim.Render("? shortcuts"))
	}
	lines = append(lines, styleDim.Render(m.message))
	return m.fullPanel().Render(strings.Join(lines, "\n"))
}

func (m homeModel) itemPrefixLines() []string {
	if m.mode == "first-setup" {
		return m.firstSetupPrefixLines()
	}
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
		if m.cfg.CurrentTeamCode != "" {
			code := "[ " + m.cfg.CurrentTeamCode + "  COPY ]"
			if m.codeHover {
				code = styleSelected.Render(code)
			} else {
				code = styleFocused.Render(code)
			}
			lines = append(lines, code)
		}
		lines = append(lines, "Connection: "+m.connectionSummary())
		if m.activeOK {
			lines = append(lines, fmt.Sprintf("People: %s", peopleSummary(m.active.ActiveCount)))
			lines = append(lines, fmt.Sprintf("Activity: %d captured, %d sent", m.active.LocalCapturedEvents, m.active.LocalSentEvents))
		}
	}
	if m.mode == "team" && m.cfg.CurrentTeamCode != "" {
		code := "[ " + m.cfg.CurrentTeamCode + "  COPY ]"
		if m.codeHover {
			code = styleSelected.Render(code)
		} else {
			code = styleFocused.Render(code)
		}
		lines = append(lines, "", code)
	}
	return append(lines, "")
}

func (m homeModel) firstSetupPrefixLines() []string {
	step := clampInt(m.onboardingStep, 0, onboardingStepCount-1)
	titles := []string{"What should the room call you?", "How present should the room feel?", "Let Cliks notice activity—not keys", "How should quick signals arrive?", "Should your chair stay warm?", "Wake Cliks after sign-in?", "Add a private room tone?"}
	details := []string{
		"Your name is shared as plain presence text. It never comes from your OS account.",
		"This changes only what you hear. Your teammates keep their own mix.",
		"The OS permission exposes local events; Cliks immediately reduces them to keyboard or left/right click.",
		"Every signal includes who sent it and the fixed message. Mute silences them locally.",
		"Background mode keeps one small session alive after this terminal closes.",
		"One launcher, one session, no surprise duplicate coworkers pretending to be you.",
		"Room tones are generated and played locally. The team never hears your choice.",
	}
	quips := []string{
		"No stand-up meeting was scheduled to configure this.",
		"A volume slider should not become a personality test.",
		"Your password manager may relax; we are not collecting letters.",
		"The notification has one job, then it leaves.",
		"Tiny daemon. Normal chair. Absolutely no blockchain.",
		"Computers are excellent at remembering the boring part.",
		"Like office air-conditioning, except everyone controls their own.",
	}
	width := maxInt(30, panelWidth(m.width)-8)
	filled := step + 1
	progress := strings.Repeat("━", filled) + strings.Repeat("─", onboardingStepCount-filled)
	art := []string{
		"             ·       ○       ·",
		"         ·       [ YOU ]       ·",
		"             ·       ○       ·",
	}
	lines := make([]string, 0, 20)
	for index := 0; index < clampInt((m.height-22)/4, 0, 5); index++ {
		lines = append(lines, "")
	}
	lines = append(lines,
		centerTerminalText(styleDim.Render(fmt.Sprintf("SETUP  %d/%d   %s", step+1, onboardingStepCount, progress)), width),
		"",
	)
	for _, line := range art {
		lines = append(lines, centerTerminalText(styleAccent.Render(line), width))
	}
	lines = append(lines, "", centerTerminalText(styleAccent.Render(titles[step]), width))
	for _, line := range strings.Split(ansi.Wordwrap(details[step], maxInt(28, width-12), ""), "\n") {
		lines = append(lines, centerTerminalText(line, width))
	}
	quip := quips[(step+m.onboardingQuip)%len(quips)]
	lines = append(lines, centerTerminalText(styleDim.Render(quip), width), "")
	return lines
}

func centerTerminalText(value string, width int) string {
	padding := maxInt(0, (width-ansi.StringWidth(value))/2)
	return strings.Repeat(" ", padding) + value
}

func (m homeModel) homeCodeHit(x int, y int) bool {
	if m.cfg.CurrentTeamCode == "" || (m.mode != "home" && m.mode != "team") {
		return false
	}
	for i, line := range m.itemPrefixLines() {
		if strings.Contains(ansi.Strip(line), "COPY ]") {
			return y == panelContentStartY()+i && x >= 3 && x < 3+len(m.cfg.CurrentTeamCode)+10
		}
	}
	return false
}

func factoryResetDevice() error {
	_, _ = stopActiveSession()
	_, _ = autostartAction([]string{"disable"})
	if err := os.Remove(configPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	legacy := legacyConfigPath()
	if legacy != "" && legacy != configPath() {
		_ = os.Remove(legacy)
	}
	return os.RemoveAll(stateDir())
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
	return m.fullPanel().Render(strings.Join(lines, "\n"))
}

func (m homeModel) doctorReportView() string {
	displayLines := m.doctorDisplayLines()
	start, end := m.doctorWindow()
	lines := []string{styleAccent.Render("Setup Check"), ""}
	for _, line := range displayLines[start:end] {
		if strings.HasSuffix(line, ":") {
			lines = append(lines, styleAccent.Render(line))
		} else {
			lines = append(lines, line)
		}
	}
	back := "[ Back ]"
	refresh := "[ Refresh ]"
	if m.doctorHover == "back" {
		back = styleSelected.Render(" Back ")
	}
	if m.doctorHover == "refresh" {
		refresh = styleSelected.Render(" Refresh ")
	}
	lines = append(lines, "", back+"  "+refresh)
	position := fmt.Sprintf("Lines %d-%d of %d", start+1, end, len(displayLines))
	if len(displayLines) == 0 {
		position = "No report data"
	}
	lines = append(lines, styleDim.Render(position+". Wheel or up/down scrolls; r refreshes; q returns."), styleDim.Render(m.message))
	return m.fullPanel().Render(strings.Join(lines, "\n"))
}

func (m homeModel) doctorVisibleCount() int {
	visible := 14
	if m.height > 0 {
		visible = m.height - 12
	}
	if visible < 5 {
		visible = 5
	}
	displayCount := len(m.doctorDisplayLines())
	if displayCount > 0 && visible > displayCount {
		visible = displayCount
	}
	return visible
}

func (m homeModel) doctorWindow() (int, int) {
	displayCount := len(m.doctorDisplayLines())
	if displayCount == 0 {
		return 0, 0
	}
	visible := m.doctorVisibleCount()
	start := clampInt(m.doctorOffset, 0, maxInt(0, displayCount-visible))
	return start, minInt(displayCount, start+visible)
}

func (m *homeModel) scrollDoctor(delta int) {
	visible := m.doctorVisibleCount()
	displayCount := len(m.doctorDisplayLines())
	m.doctorOffset = clampInt(m.doctorOffset+delta, 0, maxInt(0, displayCount-visible))
	m.doctorHover = ""
}

func (m homeModel) doctorDisplayLines() []string {
	width := panelWidth(m.width) - 6
	if width < 24 {
		width = 24
	}
	var wrapped []string
	for _, line := range m.doctorLines {
		parts := strings.Split(ansi.Wordwrap(line, width, ""), "\n")
		wrapped = append(wrapped, parts...)
	}
	return wrapped
}

func (m homeModel) doctorActionAt(x, y int) string {
	start, end := m.doctorWindow()
	buttonY := panelContentStartY() + (end - start) + 3
	if y != buttonY {
		return ""
	}
	contentX := 3
	if x >= contentX && x < contentX+8 {
		return "back"
	}
	refreshX := contentX + 10
	if x >= refreshX && x < refreshX+11 {
		return "refresh"
	}
	return ""
}

func (m homeModel) statusFooterView() string {
	team := valuePlain(teamLabel(m.cfg, m.cfg.CurrentTeamCode), "no team")
	status := "stopped"
	people := ""
	if m.activeOK {
		team = valuePlain(m.activeTeamLabel(), team)
		status = valuePlain(m.active.ConnectionStatus, "starting")
		people = fmt.Sprintf(" | %d here", maxInt(1, m.active.ActiveCount))
	}
	muted := ""
	if m.cfg.Listening.Muted {
		muted = " | muted"
	}
	line := fmt.Sprintf("%s | %s | volume %d%%%s%s", team, status, int(m.cfg.Listening.Volume*100+0.5), muted, people)
	if m.width > 0 {
		runes := []rune(line)
		if len(runes) > m.width {
			if m.width > 3 {
				line = string(runes[:m.width-3]) + "..."
			} else {
				line = string(runes[:m.width])
			}
		}
	}
	return styleDim.Render(line)
}

type homeItem struct {
	key   string
	label string
	help  string
}

const onboardingStepCount = 7

func (m homeModel) onboardingItems() []homeItem {
	switch clampInt(m.onboardingStep, 0, onboardingStepCount-1) {
	case 0:
		return []homeItem{
			{key: "nickname", label: "Type my own", help: "up to 10 characters"},
			{key: "onboarding-random-name", label: m.onboardingSuggestion, help: "a freshly generated internet citizen"},
			{key: "onboarding-anonymous", label: "Stay anonymous", help: "the classic mysterious coworker"},
		}
	case 1:
		return []homeItem{
			{key: "onboarding-sound-balanced", label: "Balanced", help: "warm, present, easy to forget"},
			{key: "onboarding-sound-quiet", label: "Quiet", help: "soft enough for deep work"},
			{key: "onboarding-sound-lively", label: "Lively", help: "more of the room, still fatigue-protected"},
		}
	case 2:
		return []homeItem{
			{key: "onboarding-permission", label: "Open permission", help: "Cliks guides the exact OS step"},
			{key: "onboarding-permission-later", label: "Do it later", help: "the room can wait without judging"},
		}
	case 3:
		return []homeItem{
			{key: "onboarding-notify-sound", label: "Banners + sound", help: "signals may make one small sound"},
			{key: "onboarding-notify-quiet", label: "Quiet banners", help: "sender and message, no alert sound"},
			{key: "onboarding-notify-off", label: "No banners", help: "signals stay inside the room"},
		}
	case 4:
		return []homeItem{
			{key: "onboarding-background-on", label: "Keep it running", help: "closing this screen will not empty your chair"},
			{key: "onboarding-background-off", label: "Only while open", help: "terminal closes, Cliks stops"},
		}
	case 5:
		return []homeItem{
			{key: "onboarding-autostart-on", label: "Start after sign-in", help: "activates after your first team join"},
			{key: "onboarding-autostart-off", label: "I will start it", help: "no login launcher"},
		}
	default:
		return []homeItem{
			{key: "onboarding-ambient-off", label: "No room tone", help: "only coworker keyboard and click ambience"},
			{key: "onboarding-ambient-rain", label: "Rain window", help: "private, low, and steady"},
			{key: "onboarding-ambient-cafe", label: "Cafe hum", help: "soft room texture without voices"},
			{key: "onboarding-ambient-deep", label: "Deep focus", help: "a slow low-frequency bed"},
		}
	}
}

func (m *homeModel) advanceOnboarding(message string) {
	if m.onboardingStep >= onboardingStepCount-1 {
		m.cfg.OnboardingSeen = true
		_ = saveConfig(m.cfg)
		m.onboardingPending = false
		m.mode = "home"
		m.cursor = 0
		m.mouseOver = false
		m.message = "Your desk is ready. " + message
		return
	}
	m.onboardingStep++
	m.cursor = 0
	m.mouseOver = false
	m.message = message
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
	report  []string
	err     error
}

type homeTickMsg time.Time

func (m homeModel) items() []homeItem {
	switch m.mode {
	case "menu":
		return []homeItem{
			{key: "preferences", label: "Preferences", help: "sound, sharing, spatial audio, and fatigue fade"},
			{key: "backend-url", label: "Server", help: backendSummary(m.cfg)},
			{key: "advanced", label: "Advanced", help: "nickname, audio output, and batching"},
			{key: "team", label: "Team", help: "create, delete, or switch the selected team"},
			{key: "connection", label: "Connection", help: "background mode and launch-at-login"},
			{key: "diagnostics", label: "Diagnostics", help: "sound test and setup check"},
			{key: "back", label: "Back", help: "return to the greeting screen"},
		}
	case "first-setup":
		return m.onboardingItems()
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
			{key: "autostart-status", label: "Login Status", help: "show launch-at-login config path"},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	case "diagnostics":
		return []homeItem{
			{key: "setup", label: "Easy Setup", help: "one-time sound + capture setup"},
			{key: "sound", label: "Sound Test", help: "play keyboard and mouse samples"},
			{key: "notification-test", label: "Notification Test", help: "send one native notification through this OS"},
			{key: "doctor", label: "Doctor", help: "quick setup and permission check"},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	case "advanced":
		return []homeItem{
			{key: "backend-url", label: "Server", help: backendSummary(m.cfg)},
			{key: "nickname", label: "Nickname", help: valuePlain(m.cfg.Nickname, "anonymous") + " (CLI key: nickname)"},
			{key: "audio-device", label: "Audio Output", help: valuePlain(m.cfg.Listening.AudioDevice, "default") + " (CLI key: audio.device)"},
			{key: "batch-window", label: "Batch Window", help: batchWindowHelp(m.cfg)},
			{key: "factory-reset", label: "Factory Reset", help: "clear this device and replay first-run welcome"},
			{key: "back", label: "Back", help: "return to the menu"},
		}
	case "factory-reset":
		return []homeItem{
			{key: "reset-confirm", label: "Reset Device", help: "stop Cliks, clear local settings, and restart fresh"},
			{key: "back", label: "Cancel", help: "keep everything as it is"},
		}
	default:
		if m.cfg.CurrentTeamCode == "" {
			return []homeItem{
				{key: "solo", label: "Solo Desk", help: "an offline simulated room, made only on this device"},
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
			{key: "solo", label: "Solo Desk", help: "open your private offline soundscape"},
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
	case "first-setup":
		return "Make Cliks yours", "A quick setup. No meeting about the setup."
	case "team":
		return "Team", fmt.Sprintf("Selected: %s", valuePlain(teamLabel(m.cfg, m.cfg.CurrentTeamCode), "not joined"))
	case "connection":
		return "Connection", "Cliks allows one local connection per device."
	case "diagnostics":
		return "Diagnostics", "Quick checks without leaving the TUI."
	case "advanced":
		return "Advanced", "These controls stay on this device. Run cliks set --list for every scriptable key."
	case "factory-reset":
		return "Factory Reset", "This clears only this device. It does not delete your team from the server."
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

func backendSummary(cfg CliksConfig) string {
	if usesPublicBackend(cfg) {
		return "Cliks public · 20 people · protected 500 ms batching"
	}
	return "self-hosted · " + cfg.APIURL
}

func batchWindowHelp(cfg CliksConfig) string {
	if usesPublicBackend(cfg) {
		return "500 ms · locked on the public relay; self-host to tune"
	}
	return fmt.Sprintf("%d ms · self-hosted range 100-2000 ms", cfg.BatchWindowMs)
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
	if !isFormMode(m.mode) {
		m.cfg = loadConfig()
	}
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
	fromDoctorReport := m.mode == "doctor-report"
	switch m.mode {
	case "doctor-report":
		m.mode = valuePlain(m.doctorReturnMode, "diagnostics")
		m.cursor = 0
	case "preferences":
		m.mode = valuePlain(m.preferencesReturnMode, "menu")
		m.cursor = 0
	case "team", "connection", "diagnostics", "advanced", "factory-reset":
		m.mode = "menu"
		m.cursor = 0
	default:
		m.mode = "home"
		m.cursor = 0
	}
	m.mouseOver = false
	m.message = welcomeMessage(m.cfg)
	if fromDoctorReport {
		m.message = "Setup check complete. You can continue or adjust another option."
	}
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
		{"Server", "open URL editor; reconnect required", func(c CliksConfig) string {
			if usesPublicBackend(c) {
				return "public"
			}
			return "self-hosted"
		}, func(_ *CliksConfig, _ int) {}},
		{"Volume", "overall loudness", func(c CliksConfig) string { return bar(c.Listening.Volume) }, func(c *CliksConfig, d int) {
			c.Listening.Volume = clamp(c.Listening.Volume+float64(d)*0.05, 0, 1)
			c.Listening.Muted = false
		}},
		{"Density", "hear fewer or more activity sounds", func(c CliksConfig) string { return bar(c.Listening.Density) }, func(c *CliksConfig, d int) { c.Listening.Density = clamp(c.Listening.Density+float64(d)*0.05, 0.15, 1) }},
		{"Room tone", "private ambient layer: off, rain, cafe, or deep", func(c CliksConfig) string { return ambientLabel(c.Listening.Ambient) }, func(c *CliksConfig, d int) { c.Listening.Ambient = nextAmbient(c.Listening.Ambient, d) }},
		{"Room tone volume", "private ambient layer loudness", func(c CliksConfig) string { return bar(c.Listening.AmbientVolume) }, func(c *CliksConfig, d int) {
			c.Listening.AmbientVolume = clamp(c.Listening.AmbientVolume+float64(d)*0.05, 0.05, 0.6)
		}},
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
		{"Notifications", "native alerts with sender and signal message", func(c CliksConfig) string { return onOff(c.Notifications.Enabled) }, func(c *CliksConfig, _ int) {
			c.Notifications.Enabled = !c.Notifications.Enabled
			c.Notifications.Configured = true
		}},
		{"Notify sound", "sound with native wave alerts", func(c CliksConfig) string { return onOff(c.Notifications.Sound) }, func(c *CliksConfig, _ int) {
			c.Notifications.Sound = !c.Notifications.Sound
			c.Notifications.Configured = true
		}},
		{"Presence", "available, focus, break, or dnd", func(c CliksConfig) string { return presenceLabel(c.PresenceStatus) }, func(c *CliksConfig, d int) { c.PresenceStatus = nextPresence(c.PresenceStatus, d) }},
		{"Theme", "ember, ocean, or mono", func(c CliksConfig) string { return c.Theme }, func(c *CliksConfig, d int) { c.Theme = nextTheme(c.Theme, d) }},
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
	if m.mode == "backend-url" {
		backend, err := normalizeBackendURL(m.backendURLValue)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.cfg.APIURL = backend
		m.cfg.WSURL = toWSURL(backend)
		if usesPublicBackend(m.cfg) {
			m.cfg.BatchWindowMs = 500
		}
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.mode = valuePlain(m.formReturnMode, "advanced")
		m.cursor = 0
		m.message = "Server saved. " + backendSummary(m.cfg) + ". Reconnect Live to use it."
		return m, nil
	}
	if m.mode == "nickname" {
		name := sanitizeNickname(m.nicknameValue)
		m.cfg.Nickname = name
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		returnMode := valuePlain(m.formReturnMode, "team")
		m.mode = returnMode
		m.cursor = 0
		m.mouseOver = false
		m.message = fmt.Sprintf("Nickname set to %s.", valuePlain(name, "anonymous"))
		if returnMode == "first-setup" {
			m.advanceOnboarding(m.message)
		}
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
		m.cursor = 2
		m.message = fmt.Sprintf("Audio output set to %s.", valuePlain(device, "default"))
		return m, nil
	}
	if m.mode == "batch-window" {
		window, err := parseBatchWindow(m.batchWindowValue)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		if usesPublicBackend(m.cfg) && window != 500 {
			m.message = "The public relay is fixed at 500 ms. Set Server to your own backend before tuning batching."
			return m, nil
		}
		m.cfg.BatchWindowMs = window
		if err := saveConfig(m.cfg); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.mode = "advanced"
		m.cursor = 3
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
	if m.mode == "nickname" || m.mode == "join" || m.mode == "audio-device" || m.mode == "batch-window" || m.mode == "backend-url" {
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
	case "backend-url":
		return m.backendURLValue
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
	case "backend-url":
		m.backendURLValue = value
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
	case "create", "join", "delete", "nickname", "audio-device", "batch-window", "backend-url":
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
	} else if m.mode == "backend-url" {
		title = "Server"
		rows = []string{
			formLine("HTTP URL", m.backendURLValue, productionAPIURL, true, m.formTextCursor, false),
			styleDim.Render("Type public to restore Cliks. Self-hosting unlocks larger room limits and 100-2000 ms batching."),
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
	return m.fullPanel().Render(strings.Join(lines, "\n"))
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
		message := clipboardStatus(team.Code)
		if autostart := enableWantedAutostart(next); autostart != "" {
			message += " " + autostart
		}
		return formDoneMsg{kind: "create", code: team.Code, message: message, cfg: next}
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
		return formDoneMsg{kind: "join", code: team.Code, message: enableWantedAutostart(next), cfg: next}
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
			if err == nil {
				cfg := loadConfig()
				cfg.AutostartWanted = false
				_ = saveConfig(cfg)
			}
			return commandDoneMsg{message: message, err: err}
		}
		if code == "" {
			return commandDoneMsg{err: fmt.Errorf("no team selected. Create or join a team first")}
		}
		message, err := autostartAction([]string{"enable", code})
		if err == nil {
			cfg := loadConfig()
			cfg.AutostartWanted = true
			_ = saveConfig(cfg)
		}
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
		defer audio.Close()
		if err := audio.preview(); err != nil {
			return commandDoneMsg{err: err}
		}
		return commandDoneMsg{message: "Played keyboard and mouse test sounds."}
	}
}

func onboardingPermissionCmd() tea.Cmd {
	return func() tea.Msg {
		steps := ensureCaptureReady()
		for _, step := range steps {
			if step.status == "action" {
				return onboardingPermissionMsg{message: "Permission pane opened. Enable the requested switch, then restart Cliks if the OS asks."}
			}
		}
		return onboardingPermissionMsg{message: "Capture permission already looks ready."}
	}
}

func notificationSetupCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := loadConfig()
		cfg.Notifications.Enabled = true
		cfg.Notifications.Configured = true
		if err := saveConfig(cfg); err != nil {
			return commandDoneMsg{err: err}
		}
		title, body := reactionNotificationContent("Mira", "wave")
		if err := sendNativeNotification(title, "Example: "+body, cfg.Notifications.Sound); err != nil {
			return commandDoneMsg{err: fmt.Errorf("notification test failed: %w", err)}
		}
		return commandDoneMsg{message: "Notification delivered. Incoming signals are enabled."}
	}
}

func doctorSummaryCmd() tea.Cmd {
	return func() tea.Msg {
		report := buildDoctorReport(loadConfig())
		return commandDoneMsg{message: doctorSummary(report), report: doctorReportLines(report)}
	}
}

func setupSummaryCmd() tea.Cmd {
	return func() tea.Msg {
		steps := runSetupChecks(true)
		return commandDoneMsg{
			message: setupSummaryMessage(steps),
			report:  setupReportLines(steps),
		}
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
type notificationTestMsg struct{ err error }

type sessionModel struct {
	controller     *sessionController
	state          SessionViewState
	mode           string
	settingsCursor int
	settingsHover  bool
	codeHover      bool
	message        string
	exit           sessionExitAction
	width          int
	height         int
	now            time.Time
	helpOpen       bool
	welcomeUntil   time.Time
	hoverAction    string
}

func newSessionModel(controller *sessionController) sessionModel {
	now := time.Now()
	model := sessionModel{controller: controller, state: controller.viewState(), now: now}
	if !controller.cfg.WelcomeSeen {
		model.welcomeUntil = now.Add(6 * time.Second)
		controller.cfg.WelcomeSeen = true
		_ = saveConfig(controller.cfg)
	}
	return model
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
	case notificationTestMsg:
		if msg.err != nil {
			m.message = "Notification failed: " + msg.err.Error() + ". Run cliks notification-test."
		} else {
			m.message = "Notification delivered. Incoming signals will appear here too."
		}
		return m, nil
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
			m.hoverAction = m.liveHit(msg.X, msg.Y)
			m.codeHover = m.hoverAction == "copy-code"
		case tea.MouseWheelUp:
			m.controller.adjustVolume(0.05)
			m.hoverAction = ""
		case tea.MouseWheelDown:
			m.controller.adjustVolume(-0.05)
			m.hoverAction = ""
		case tea.MouseLeft:
			if action := m.liveHit(msg.X, msg.Y); action != "" {
				m.hoverAction = action
				return m.activateLiveAction(action)
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
		case "1":
			m.sendLiveReaction("wave")
		case "2":
			m.sendLiveReaction("nice")
		case "3":
			m.sendLiveReaction("coffee")
		case "4":
			m.sendLiveReaction("celebrate")
		case "5":
			m.sendLiveReaction("break")
		case "p":
			m.controller.cyclePresence()
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
	width := maxInt(44, panelWidth(m.width))
	bodyHeight := maxInt(12, m.height-7)
	header := m.liveHeader(width)
	footer := styleDim.Render(" ? Help   ↑/↓ Volume   ←/→ Density   Tab Preferences   Esc Back   Ctrl+C Stop")
	if width < 74 {
		desk := stylePanel.Width(width).Height(bodyHeight).Render(m.renderSpatialDesk(width-6, bodyHeight-3))
		return lipgloss.JoinVertical(lipgloss.Left, header, desk, footer)
	}
	mapWidth := int(float64(width) * 0.68)
	infoWidth := width - mapWidth - 2
	desk := stylePanel.Width(mapWidth).Height(bodyHeight).Render(m.renderSpatialDesk(mapWidth-6, bodyHeight-3))
	activity := stylePanel.Width(infoWidth).Height(bodyHeight).Render(m.liveActivityView(infoWidth - 5))
	return lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, desk, "  ", activity), footer)
}

func (m sessionModel) liveHeader(width int) string {
	team := valuePlain(m.state.TeamName, teamNameForCode(m.controller.cfg, m.state.TeamCode))
	code := valuePlain(m.state.TeamCode, m.controller.cfg.CurrentTeamCode)
	left := "Cliks  /  " + valuePlain(team, "Team")
	if code != "" {
		left += "  ·  " + code
	}
	right := connectionStyle(m.state.ConnectionStatus) + "  ·  " + fmt.Sprintf("%d here", maxInt(1, m.state.ActiveCount))
	// Lip Gloss Width includes padding. Budget the actual inner width so the
	// header cannot gain a surprise second row and shift every panel below it.
	contentWidth := maxInt(20, width-styleTitle.GetPaddingLeft()-styleTitle.GetPaddingRight())
	rightWidth := ansi.StringWidth(right)
	left = ansi.Truncate(left, maxInt(8, contentWidth-rightWidth-2), "…")
	gap := maxInt(2, contentWidth-ansi.StringWidth(left)-rightWidth)
	return styleTitle.Width(width).MaxWidth(width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m sessionModel) renderSpatialDesk(width int, height int) string {
	width = maxInt(32, width)
	height = maxInt(10, height)
	grid := make([][]rune, height)
	for y := range grid {
		grid[y] = []rune(strings.Repeat(" ", width))
	}
	put := func(x int, y int, value string) {
		if y < 0 || y >= height {
			return
		}
		for i, ch := range []rune(value) {
			px := x + i
			if px >= 0 && px < width {
				grid[y][px] = ch
			}
		}
	}
	cx, cy := width/2, height/2
	for ring := 1; ring <= 2; ring++ {
		rx := float64(width) * (0.20 + float64(ring)*0.12)
		ry := float64(height) * (0.18 + float64(ring)*0.12)
		for step := 0; step < 72; step++ {
			a := float64(step) * 2 * math.Pi / 72
			put(cx+int(math.Cos(a)*rx), cy+int(math.Sin(a)*ry), "·")
		}
	}
	put(cx-3, cy, "[ YOU ]")
	var latestReaction *PeerReactionStatus
	if len(m.state.RecentReactions) > 0 {
		reaction := &m.state.RecentReactions[len(m.state.RecentReactions)-1]
		if m.now.Sub(reaction.At) < 5*time.Second {
			latestReaction = reaction
			name := valuePlain(sanitizeNickname(reaction.Nickname), "Someone")
			if reaction.PeerID == m.state.OwnPeerID {
				name = "You"
			}
			message := name + "  " + reactionGlyph(reaction.Reaction) + "  " + reactionPhrase(reaction.Reaction)
			put(cx-len([]rune(message))/2, 1, message)
			if reaction.PeerID == m.state.OwnPeerID {
				burst := reactionGlyph(reaction.Reaction)
				if (m.now.UnixMilli()/250)%2 == 0 {
					burst = "✦ " + burst + " ✦"
				}
				put(cx-len([]rune(burst))/2, maxInt(2, cy-2), burst)
			}
		}
	}
	peers := sortedRemotePeers(m.state)
	if len(peers) == 0 && !m.welcomeUntil.IsZero() && m.now.Before(m.welcomeUntil) {
		peers = []PeerPresence{{PeerID: "demo-1", Nickname: "Mira", JoinedAt: m.now.Add(-time.Second).UnixMilli()}, {PeerID: "demo-2", Nickname: "Sam", JoinedAt: m.now.Add(-2 * time.Second).UnixMilli()}, {PeerID: "demo-3", Nickname: "Noor", JoinedAt: m.now.Add(-3 * time.Second).UnixMilli()}}
		put(maxInt(1, cx-10), 0, "Welcome to your desk")
	}
	visible := minInt(len(peers), 12)
	for i := 0; i < visible; i++ {
		peer := peers[i]
		ring := 1
		seat := i
		capacity := 4
		if i >= 4 {
			ring, seat, capacity = 2, i-4, 8
		}
		a := -math.Pi/2 + float64(seat)*2*math.Pi/float64(capacity) + float64(ring-1)*0.22
		rx := float64(width) * (0.31 + float64(ring-1)*0.11)
		ry := float64(height) * (0.28 + float64(ring-1)*0.10)
		x, y := cx+int(math.Cos(a)*rx), cy+int(math.Sin(a)*ry)
		name := sanitizeNickname(peer.Nickname)
		if name == "" {
			name = fmt.Sprintf("Peer %d", i+1)
		}
		marker := peerStatusMarker(peer.Status)
		if peerTyping(m.state, peer.PeerID, m.now) {
			marker = "◆"
		}
		if m.now.Sub(time.UnixMilli(peer.JoinedAt)) < 1600*time.Millisecond && (m.now.UnixMilli()/250)%2 == 0 {
			marker = "◌"
		}
		label := marker + " " + truncateRunes(name, 10)
		put(x-len([]rune(label))/2, y, label)
		if latestReaction != nil && latestReaction.PeerID == peer.PeerID {
			burst := reactionGlyph(latestReaction.Reaction)
			if (m.now.UnixMilli()/250)%2 == 0 {
				burst = "· " + burst + " ✦"
			} else {
				burst = "✦ " + burst + " ·"
			}
			put(x-len([]rune(burst))/2, maxInt(2, y-1), burst)
		}
	}
	if len(peers) > visible {
		extra := len(peers) - visible
		dots := strings.Repeat("•", minInt(extra, 8))
		put(cx-len([]rune(dots))/2, height-1, dots+fmt.Sprintf(" +%d nearby", extra))
	}
	lines := make([]string, height)
	for i := range grid {
		lines[i] = strings.TrimRight(string(grid[i]), " ")
	}
	return strings.Join(lines, "\n")
}

func peerStatusMarker(status string) string {
	switch status {
	case "focus":
		return "◎"
	case "break":
		return "◒"
	case "dnd":
		return "×"
	default:
		return "○"
	}
}

func peerTyping(state SessionViewState, peerID string, now time.Time) bool {
	for _, activity := range state.RecentPeerActivity {
		if activity.PeerID == peerID && now.Sub(activity.LastActivityAt) < 1800*time.Millisecond {
			return true
		}
	}
	return false
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func reactionGlyph(value string) string {
	switch value {
	case "wave":
		return "👋"
	case "nice":
		return "👍"
	case "coffee":
		return "☕"
	case "focus":
		return "🎯"
	case "celebrate":
		return "🎉"
	case "break":
		return "🧘"
	}
	return "•"
}

func (m sessionModel) liveActivityView(width int) string {
	team := valuePlain(m.state.TeamName, teamNameForCode(m.controller.cfg, m.state.TeamCode))
	code := valuePlain(m.state.TeamCode, m.controller.cfg.CurrentTeamCode)
	navigation := m.liveActionLine("prefs", "Preferences") + "   " + m.liveActionLine("back", "Back") + "   " + m.liveActionLine("stop", "Stop")
	navigationLines := []string{navigation}
	if ansi.StringWidth(navigation) > width {
		navigationLines = []string{
			m.liveActionLine("prefs", "Preferences") + "   " + m.liveActionLine("back", "Back"),
			m.liveActionLine("stop", "Stop"),
		}
	}
	lines := []string{
		styleAccent.Render(valuePlain(team, "Your room")),
		m.liveActionLine("copy-code", code+"  COPY"),
		connectionStyle(m.state.ConnectionStatus) + "  ·  " + roomPeopleSummary(m.state),
		typingSummary(m.state, m.now),
		"Flow: " + flowBadge(m.state, m.now),
		"Health: " + styleDim.Render(healthSummary(m.state, m.now)),
		"",
		"Volume  " + compactListeningBar(m.state.Listening),
		"Density " + compactBar(m.state.Listening.Density),
		m.liveActionLine("notifications", "Notifications  "+onOff(m.controller.cfg.Notifications.Enabled)),
		m.liveActionLine("notification-sound", "Notify sound   "+onOff(m.controller.cfg.Notifications.Sound)),
		m.liveActionLine("mute", "Mute "+onOff(m.state.Listening.Muted)) + "   " + m.liveActionLine("spatial", "Spatial "+onOff(m.state.Listening.Spatial)),
		"",
		styleAccent.Render("Quick signals"),
		m.liveActionLine("reaction-wave", "👋 Wave") + "    " + m.liveActionLine("reaction-nice", "👍 Nice"),
		m.liveActionLine("reaction-coffee", "☕ Coffee") + "  " + m.liveActionLine("reaction-celebrate", "🎉 Celebrate"),
		m.liveActionLine("reaction-break", "🧘 Break"),
		"",
	}
	lines = append(lines, navigationLines...)
	if m.message != "" {
		lines = append(lines, "", styleDim.Render(m.message))
	}
	return strings.Join(lines, "\n")
}

func (m sessionModel) liveActionLine(action string, label string) string {
	text := "[ " + label + " ]"
	if m.hoverAction == action {
		return styleSelected.Render(text)
	}
	return styleFocused.Render(text)
}

func (m *sessionModel) sendLiveReaction(reaction string) {
	m.controller.sendReaction(reaction)
	m.message = "Shared with the room  " + reactionGlyph(reaction) + "  " + reactionPhrase(reaction)
}

func (m sessionModel) liveHit(x int, y int) string {
	for _, region := range m.liveHitRegions() {
		if y == region.y && x >= region.x && x < region.x+region.width {
			return region.action
		}
	}
	return ""
}

type liveHitRegion struct {
	action string
	x      int
	y      int
	width  int
}

// liveHitTarget and inlineLiveHit are used by compact single-line rails such
// as Solo Desk. The live team room uses rendered regions because its header
// and panel can change height with terminal width.
type liveHitTarget struct {
	action string
	label  string
	gap    int
}

func inlineLiveHit(x int, startX int, targets []liveHitTarget) string {
	cursor := startX
	for _, target := range targets {
		width := ansi.StringWidth("[ " + target.label + " ]")
		if x >= cursor && x < cursor+width {
			return target.action
		}
		cursor += width + target.gap
	}
	return ""
}

func (m sessionModel) liveHitRegions() []liveHitRegion {
	width := maxInt(44, panelWidth(m.width))
	var regions []liveHitRegion
	addRenderedRegion := func(action string, rendered string, needle string, baseX int, baseY int) {
		for localY, styledLine := range strings.Split(rendered, "\n") {
			line := ansi.Strip(styledLine)
			if index := strings.Index(line, needle); index >= 0 {
				regions = append(regions, liveHitRegion{
					action: action,
					x:      baseX + ansi.StringWidth(line[:index]),
					y:      baseY + localY,
					width:  ansi.StringWidth(needle),
				})
				return
			}
		}
	}
	header := m.liveHeader(width)
	code := valuePlain(m.state.TeamCode, m.controller.cfg.CurrentTeamCode)
	if code != "" {
		addRenderedRegion("copy-code", header, code, 0, 0)
	}
	if width < 74 {
		return regions
	}
	mapWidth := int(float64(width) * 0.68)
	infoWidth := width - mapWidth - 2
	bodyHeight := maxInt(12, m.height-7)
	desk := stylePanel.Width(mapWidth).Height(bodyHeight).Render("")
	rail := stylePanel.Width(infoWidth).Height(bodyHeight).Render(m.liveActivityView(infoWidth - 5))
	baseX := lipgloss.Width(desk) + 2
	baseY := lipgloss.Height(header)
	labels := []struct {
		action string
		label  string
	}{
		{"copy-code", code + "  COPY"},
		{"notifications", "Notifications  " + onOff(m.controller.cfg.Notifications.Enabled)},
		{"notification-sound", "Notify sound   " + onOff(m.controller.cfg.Notifications.Sound)},
		{"mute", "Mute " + onOff(m.state.Listening.Muted)},
		{"spatial", "Spatial " + onOff(m.state.Listening.Spatial)},
		{"reaction-wave", "👋 Wave"},
		{"reaction-nice", "👍 Nice"},
		{"reaction-coffee", "☕ Coffee"},
		{"reaction-celebrate", "🎉 Celebrate"},
		{"reaction-break", "🧘 Break"},
		{"prefs", "Preferences"},
		{"back", "Back"},
		{"stop", "Stop"},
	}
	for _, item := range labels {
		if item.label != "" {
			addRenderedRegion(item.action, rail, "[ "+item.label+" ]", baseX, baseY)
		}
	}
	return regions
}

func (m sessionModel) activateLiveAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "copy-code":
		m.message = clipboardStatus(valuePlain(m.state.TeamCode, m.controller.cfg.CurrentTeamCode))
	case "notifications":
		m.controller.cfg.Notifications.Enabled = !m.controller.cfg.Notifications.Enabled
		m.controller.cfg.Notifications.Configured = true
		_ = saveConfig(m.controller.cfg)
		m.message = "Notifications " + onOff(m.controller.cfg.Notifications.Enabled)
		if m.controller.cfg.Notifications.Enabled {
			cfg := m.controller.cfg
			return m, func() tea.Msg {
				return notificationTestMsg{err: sendNativeNotification("Cliks test 👋 Notifications are on", "Quick signals will appear here.", cfg.Notifications.Sound)}
			}
		}
	case "notification-sound":
		m.controller.cfg.Notifications.Sound = !m.controller.cfg.Notifications.Sound
		m.controller.cfg.Notifications.Configured = true
		_ = saveConfig(m.controller.cfg)
		m.message = "Notification sound " + onOff(m.controller.cfg.Notifications.Sound)
	case "mute":
		m.controller.toggle("muted")
	case "spatial":
		m.controller.toggle("spatial")
	case "reaction-wave":
		m.sendLiveReaction("wave")
	case "reaction-nice":
		m.sendLiveReaction("nice")
	case "reaction-coffee":
		m.sendLiveReaction("coffee")
	case "reaction-celebrate":
		m.sendLiveReaction("celebrate")
	case "reaction-break":
		m.sendLiveReaction("break")
	case "prefs":
		m.mode = "settings"
		m.settingsCursor = 0
		m.settingsHover = false
	case "back":
		m.exit = sessionExitBack
		return m, tea.Quit
	case "stop":
		m.exit = sessionExitStop
		return m, tea.Quit
	}
	m.state = m.controller.viewState()
	return m, nil
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
	previousStatus := m.controller.cfg.PresenceStatus
	row := rows[m.settingsCursor]
	if row.label == "Server" {
		m.message = "Server changes reconnect the room. Return to the main screen → More → Server."
		return
	}
	row.apply(&m.controller.cfg, delta)
	applyTheme(m.controller.cfg.Theme)
	_ = saveConfig(m.controller.cfg)
	m.controller.set(func(state *SessionViewState) {
		state.Listening = m.controller.cfg.Listening
		state.HearingSelf = m.controller.cfg.Listening.Self
	})
	m.controller.audio.updateListening(m.controller.cfg.Listening)
	if previousStatus != m.controller.cfg.PresenceStatus {
		m.controller.sendProfile(sanitizeNickname(m.controller.cfg.Nickname), m.controller.cfg.PresenceStatus)
	}
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
	if m.message != "" {
		lines = append(lines, styleDim.Render(m.message))
	}
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
	return maxInt(36, width-4)
}

func bar(value float64) string {
	width := 16
	filled := int(clamp(value, 0, 1)*float64(width) + 0.5)
	return styleAccent.Render(strings.Repeat("█", filled)) + styleDim.Render(strings.Repeat("░", width-filled)) + fmt.Sprintf(" %d%%", int(value*100+0.5))
}

func compactBar(value float64) string {
	width := 10
	filled := int(clamp(value, 0, 1)*float64(width) + 0.5)
	return styleAccent.Render(strings.Repeat("█", filled)) + styleDim.Render(strings.Repeat("░", width-filled)) + fmt.Sprintf(" %d%%", int(value*100+0.5))
}

func compactListeningBar(listening ListeningConfig) string {
	if listening.Muted {
		return styleWarn.Render("muted")
	}
	return compactBar(listening.Volume)
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

func presenceLabel(value string) string {
	switch value {
	case "focus":
		return "focus"
	case "break":
		return "on a break"
	case "dnd":
		return "do not disturb"
	default:
		return "available"
	}
}

func nextPresence(current string, delta int) string {
	values := []string{"available", "focus", "break", "dnd"}
	index := 0
	for i, value := range values {
		if value == current {
			index = i
			break
		}
	}
	index = (index + delta + len(values)) % len(values)
	return values[index]
}

func nextTheme(current string, delta int) string {
	values := []string{"ember", "ocean", "mono"}
	index := 0
	for i, value := range values {
		if value == current {
			index = i
			break
		}
	}
	return values[(index+delta+len(values))%len(values)]
}

func connectionStyle(value string) string {
	lower := strings.ToLower(value)
	if value == "connected" {
		return styleOK.Render(value)
	}
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "error") || strings.Contains(lower, "offline") || strings.Contains(lower, "stopped") {
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

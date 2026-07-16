package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type soloTickMsg time.Time

type soloModel struct {
	cfg         CliksConfig
	audio       *AudioEngine
	state       SessionViewState
	now         time.Time
	width       int
	height      int
	hoverAction string
	message     string
}

var soloNames = []string{"Mira", "Noor", "Sam", "Juniper", "Otter", "Pixel", "Toast", "Mochi", "Orbit", "Noodle", "Pebble", "Basil"}

func runSoloTUI(cfg CliksConfig) error {
	if !term.IsTerminal(int(stdinFD())) || !term.IsTerminal(int(stdoutFD())) {
		return fmt.Errorf("Solo Desk needs an interactive terminal; run cliks in a terminal and choose Solo Desk")
	}
	applyTheme(cfg.Theme)
	model := newSoloModel(cfg)
	defer model.audio.Close()
	_, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseAllMotion()).Run()
	return err
}

// Kept as tiny seams so tests can exercise the model without replacing os.Stdin.
var stdinFD = func() uintptr { return 0 }
var stdoutFD = func() uintptr { return 1 }

func newSoloModel(cfg CliksConfig) soloModel {
	now := time.Now()
	state := SessionViewState{
		TeamName:         "Solo Desk",
		OwnPeerID:        "solo-you",
		ConnectionStatus: "offline · private",
		CaptureMode:      "simulation",
		Listening:        cfg.Listening,
	}
	m := soloModel{cfg: cfg, audio: newAudioEngine(cfg.Listening), state: state, now: now, message: "A local room. No account, server, capture, or internet."}
	m.setPeople(cfg.Solo.People)
	return m
}

func (m soloModel) Init() tea.Cmd { return soloTick() }

func soloTick() tea.Cmd {
	return tea.Tick(220*time.Millisecond, func(t time.Time) tea.Msg { return soloTickMsg(t) })
}

func (m soloModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case soloTickMsg:
		m.now = time.Time(msg)
		m.simulatePulse()
		return m, soloTick()
	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseMotion:
			m.hoverAction = m.hit(msg.X, msg.Y)
		case tea.MouseLeft:
			if action := m.hit(msg.X, msg.Y); action != "" {
				return m.activate(action)
			}
		case tea.MouseWheelUp:
			m.adjustVolume(.05)
		case tea.MouseWheelDown:
			m.adjustVolume(-.05)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "b", "ctrl+c":
			return m, tea.Quit
		case "+", "=", "right":
			m.setPeople(m.cfg.Solo.People + 1)
		case "-", "left":
			m.setPeople(m.cfg.Solo.People - 1)
		case "k":
			m.cfg.Solo.Keyboard = !m.cfg.Solo.Keyboard
			m.persist()
		case "c":
			m.cfg.Solo.Mouse = !m.cfg.Solo.Mouse
			m.persist()
		case "a":
			m.cfg.Listening.Ambient = nextAmbient(m.cfg.Listening.Ambient, 1)
			m.applyListening()
		case "m":
			m.cfg.Listening.Muted = !m.cfg.Listening.Muted
			m.applyListening()
		case "up":
			m.adjustVolume(.05)
		case "down":
			m.adjustVolume(-.05)
		case " ", "enter":
			m.spark()
		}
	}
	return m, nil
}

func (m *soloModel) simulatePulse() {
	if len(m.state.Peers) == 0 || rand.Float64() > .32 {
		return
	}
	peer := m.state.Peers[rand.Intn(len(m.state.Peers))]
	kind := ""
	switch {
	case m.cfg.Solo.Keyboard && m.cfg.Solo.Mouse:
		if rand.Float64() < .82 {
			kind = "keyboard"
		} else {
			kind = "mouse"
		}
	case m.cfg.Solo.Keyboard:
		kind = "keyboard"
	case m.cfg.Solo.Mouse:
		kind = "mouse"
	default:
		return
	}
	event := RemoteActivityEvent{Kind: kind}
	if kind == "mouse" {
		event.Button = "left"
	}
	m.state.LastPeerActivityAt = m.now
	m.state.RecentPeerActivity = markPeerActive(m.state.RecentPeerActivity, peer.PeerID, peer.Nickname, m.now)
	sourceGain := m.cfg.Solo.KeyboardVolume
	if kind == "mouse" {
		sourceGain = m.cfg.Solo.MouseVolume
	}
	m.audio.scheduleBatchScaled(peer.PeerID, []RemoteActivityEvent{event}, sourceGain)
}

func (m *soloModel) spark() {
	for index := 0; index < minInt(3, len(m.state.Peers)); index++ {
		peer := m.state.Peers[index]
		m.state.RecentPeerActivity = markPeerActive(m.state.RecentPeerActivity, peer.PeerID, peer.Nickname, m.now)
		if m.cfg.Solo.Keyboard {
			m.audio.scheduleBatchScaled(peer.PeerID, []RemoteActivityEvent{{Kind: "keyboard"}}, m.cfg.Solo.KeyboardVolume)
		}
	}
	m.message = "A tiny productivity conspiracy has begun."
}

func (m *soloModel) setPeople(count int) {
	count = clampInt(count, 1, 12)
	m.cfg.Solo.People = count
	peers := make([]PeerPresence, count)
	for index := range peers {
		peers[index] = PeerPresence{PeerID: fmt.Sprintf("solo-%02d", index), Nickname: soloNames[index%len(soloNames)], JoinedAt: m.now.Add(-time.Duration(index+1) * time.Second).UnixMilli(), Status: []string{"available", "focus", "available", "break"}[index%4]}
	}
	m.state.Peers = peers
	m.state.ActiveCount = count + 1
	m.audio.updatePeers(peers, m.state.OwnPeerID)
	m.persist()
}

func (m *soloModel) adjustVolume(delta float64) {
	m.cfg.Listening.Volume = clamp(m.cfg.Listening.Volume+delta, 0, 1)
	m.applyListening()
}

func percent(value float64) string {
	return fmt.Sprintf("%3.0f%%", clamp(value, 0, 1)*100)
}

func (m *soloModel) adjustSoloVolume(kind string, delta float64) {
	switch kind {
	case "keyboard":
		m.cfg.Solo.KeyboardVolume = clamp(m.cfg.Solo.KeyboardVolume+delta, 0.05, 1)
	case "mouse":
		m.cfg.Solo.MouseVolume = clamp(m.cfg.Solo.MouseVolume+delta, 0.05, 1)
	case "ambient":
		m.cfg.Listening.AmbientVolume = clamp(m.cfg.Listening.AmbientVolume+delta, 0.05, 0.6)
		m.applyListening()
		return
	}
	m.persist()
}

func (m *soloModel) applyListening() {
	m.state.Listening = m.cfg.Listening
	m.audio.updateListening(m.cfg.Listening)
	m.persist()
}

func (m *soloModel) persist() { _ = saveConfig(m.cfg) }

func (m soloModel) activate(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "less":
		m.setPeople(m.cfg.Solo.People - 1)
	case "more":
		m.setPeople(m.cfg.Solo.People + 1)
	case "keyboard":
		m.cfg.Solo.Keyboard = !m.cfg.Solo.Keyboard
		m.persist()
	case "keyboard-quieter":
		m.adjustSoloVolume("keyboard", -0.05)
	case "keyboard-louder":
		m.adjustSoloVolume("keyboard", 0.05)
	case "mouse":
		m.cfg.Solo.Mouse = !m.cfg.Solo.Mouse
		m.persist()
	case "mouse-quieter":
		m.adjustSoloVolume("mouse", -0.05)
	case "mouse-louder":
		m.adjustSoloVolume("mouse", 0.05)
	case "ambient":
		m.cfg.Listening.Ambient = nextAmbient(m.cfg.Listening.Ambient, 1)
		m.applyListening()
	case "ambient-quieter":
		m.adjustSoloVolume("ambient", -0.05)
	case "ambient-louder":
		m.adjustSoloVolume("ambient", 0.05)
	case "master-quieter":
		m.adjustVolume(-0.05)
	case "master-louder":
		m.adjustVolume(0.05)
	case "mute":
		m.cfg.Listening.Muted = !m.cfg.Listening.Muted
		m.applyListening()
	case "spark":
		m.spark()
	case "back":
		return m, tea.Quit
	}
	return m, nil
}

func (m soloModel) View() string {
	width := maxInt(44, panelWidth(m.width))
	bodyHeight := maxInt(12, m.height-7)
	header := m.header(width)
	deskModel := sessionModel{state: m.state, now: m.now}
	footer := styleDim.Render(" Click a control   ·   ↑/↓ master volume   ·   +/- people   ·   Space adds a burst   ·   Esc back")
	if width < 74 {
		deskHeight := maxInt(9, bodyHeight-20)
		controlHeight := maxInt(13, bodyHeight-deskHeight)
		desk := stylePanel.Width(width).Height(deskHeight).Render(deskModel.renderSpatialDesk(width-6, deskHeight-3))
		controls := stylePanel.Width(width).Height(controlHeight).Render(m.controlView())
		return lipgloss.JoinVertical(lipgloss.Left, header, desk, controls, footer)
	}
	mapWidth := int(float64(width) * .68)
	infoWidth := width - mapWidth - 2
	desk := stylePanel.Width(mapWidth).Height(bodyHeight).Render(deskModel.renderSpatialDesk(mapWidth-6, bodyHeight-3))
	controls := stylePanel.Width(infoWidth).Height(bodyHeight).Render(m.controlView())
	return lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, desk, "  ", controls), footer)
}

func (m soloModel) header(width int) string {
	return styleTitle.Width(maxInt(20, width-2)).Render(fmt.Sprintf(" Cliks  /  Solo Desk   ·   %d simulated coworkers   ·   OFFLINE ", m.cfg.Solo.People))
}

func (m soloModel) controlView() string {
	lines := []string{
		styleAccent.Render("Your private soundscape"),
		m.button("less", "− person") + "   " + m.button("more", "+ person"),
		m.button("master-quieter", "−") + "  Master  " + percent(m.cfg.Listening.Volume) + "  " + m.button("master-louder", "+"),
		m.button("keyboard", "Keyboard  "+onOff(m.cfg.Solo.Keyboard)),
		m.button("keyboard-quieter", "−") + "  Keyboard level  " + percent(m.cfg.Solo.KeyboardVolume) + "  " + m.button("keyboard-louder", "+"),
		m.button("mouse", "Mouse clicks  "+onOff(m.cfg.Solo.Mouse)),
		m.button("mouse-quieter", "−") + "  Click level     " + percent(m.cfg.Solo.MouseVolume) + "  " + m.button("mouse-louder", "+"),
		m.button("ambient", "Room tone  "+ambientLabel(m.cfg.Listening.Ambient)),
		m.button("ambient-quieter", "−") + "  Room tone level " + percent(m.cfg.Listening.AmbientVolume) + "  " + m.button("ambient-louder", "+"),
		m.button("mute", "Mute  "+onOff(m.cfg.Listening.Muted)),
		"",
		m.button("spark", "Wake the room"),
		"",
		styleDim.Render("Personal and played locally."),
		styleDim.Render("No network. Nothing is sent."),
		"",
		m.button("back", "Back"),
	}
	if m.message != "" {
		lines = append(lines, "", styleDim.Render(m.message))
	}
	return strings.Join(lines, "\n")
}

func (m soloModel) button(action, label string) string {
	text := "[ " + label + " ]"
	if m.hoverAction == action {
		return styleSelected.Render(ansi.Strip(text))
	}
	return styleFocused.Render(text)
}

func (m soloModel) hit(x, y int) string {
	for _, region := range m.hitRegions() {
		if y == region.y && x >= region.x && x < region.x+region.width {
			return region.action
		}
	}
	return ""
}

func (m soloModel) hitRegions() []liveHitRegion {
	width := maxInt(44, panelWidth(m.width))
	bodyHeight := maxInt(12, m.height-7)
	baseX := 0
	baseY := lipgloss.Height(m.header(width))
	var rail string
	if width < 74 {
		deskHeight := maxInt(9, bodyHeight-20)
		controlHeight := maxInt(13, bodyHeight-deskHeight)
		desk := stylePanel.Width(width).Height(deskHeight).Render("")
		rail = stylePanel.Width(width).Height(controlHeight).Render(m.controlView())
		baseY += lipgloss.Height(desk)
	} else {
		mapWidth := int(float64(width) * .68)
		infoWidth := width - mapWidth - 2
		desk := stylePanel.Width(mapWidth).Height(bodyHeight).Render("")
		rail = stylePanel.Width(infoWidth).Height(bodyHeight).Render(m.controlView())
		baseX = lipgloss.Width(desk) + 2
	}
	labels := []struct{ action, label string }{
		{"less", "− person"}, {"more", "+ person"},
		{"master-quieter", "−"}, {"master-louder", "+"},
		{"keyboard", "Keyboard  " + onOff(m.cfg.Solo.Keyboard)},
		{"keyboard-quieter", "−"}, {"keyboard-louder", "+"},
		{"mouse", "Mouse clicks  " + onOff(m.cfg.Solo.Mouse)},
		{"mouse-quieter", "−"}, {"mouse-louder", "+"},
		{"ambient", "Room tone  " + ambientLabel(m.cfg.Listening.Ambient)},
		{"ambient-quieter", "−"}, {"ambient-louder", "+"},
		{"mute", "Mute  " + onOff(m.cfg.Listening.Muted)},
		{"spark", "Wake the room"}, {"back", "Back"},
	}
	var regions []liveHitRegion
	used := map[string]int{}
	for _, item := range labels {
		needle := "[ " + ansi.Strip(item.label) + " ]"
		occurrence := used[needle]
		used[needle]++
		found := 0
		for localY, styledLine := range strings.Split(rail, "\n") {
			line := ansi.Strip(styledLine)
			start := 0
			for {
				index := strings.Index(line[start:], needle)
				if index < 0 {
					break
				}
				index += start
				if found == occurrence {
					regions = append(regions, liveHitRegion{action: item.action, x: baseX + ansi.StringWidth(line[:index]), y: baseY + localY, width: ansi.StringWidth(needle)})
					goto nextTarget
				}
				found++
				start = index + len(needle)
			}
		}
	nextTarget:
	}
	return regions
}

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
	cfg          CliksConfig
	audio        *AudioEngine
	state        SessionViewState
	now          time.Time
	width        int
	height       int
	hoverAction  string
	sliderCursor int
	message      string
	typingBursts map[string]int
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
	m := soloModel{cfg: cfg, audio: newAudioEngine(cfg.Listening), state: state, now: now, message: "A local room. No account, server, capture, or internet.", typingBursts: map[string]int{}}
	m.setPeople(cfg.Solo.People)
	return m
}

func (m soloModel) Init() tea.Cmd { return soloTick() }

func soloTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return soloTickMsg(t) })
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
			m.rememberHoveredSlider()
		case tea.MouseLeft:
			if region, ok := m.hitRegion(msg.X, msg.Y); ok && isSoloSlider(region.action) {
				m.setSliderFromPointer(region, msg.X)
				return m, nil
			}
			if action := m.hit(msg.X, msg.Y); action != "" {
				return m.activate(action)
			}
		case tea.MouseWheelUp:
			m.adjustActiveSlider(-.05)
		case tea.MouseWheelDown:
			m.adjustActiveSlider(.05)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "b", "ctrl+c":
			return m, tea.Quit
		case "+", "=":
			m.setPeople(m.cfg.Solo.People + 1)
		case "-":
			m.setPeople(m.cfg.Solo.People - 1)
		case "tab":
			m.sliderCursor = (m.sliderCursor + 1) % len(soloSliderActions)
			m.hoverAction = ""
		case "right", "up":
			m.adjustActiveSlider(.05)
		case "left", "down":
			m.adjustActiveSlider(-.05)
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
		case " ", "enter":
			m.spark()
		}
	}
	return m, nil
}

func (m *soloModel) simulatePulse() {
	if len(m.state.Peers) == 0 {
		return
	}
	if m.typingBursts == nil {
		m.typingBursts = map[string]int{}
	}
	emitted := false
	if m.cfg.Solo.Keyboard {
		for _, peer := range m.state.Peers {
			remaining := m.typingBursts[peer.PeerID]
			if remaining <= 0 {
				continue
			}
			m.emitSoloEvent(peer, "keyboard")
			emitted = true
			remaining--
			if remaining == 0 {
				delete(m.typingBursts, peer.PeerID)
			} else {
				m.typingBursts[peer.PeerID] = remaining
			}
		}
	} else {
		clear(m.typingBursts)
	}
	// Let an existing sentence finish before usually starting another one. This
	// creates recognizable coworkers and quiet gaps instead of a click metronome.
	if !emitted && m.cfg.Solo.Keyboard && rand.Float64() < .11 {
		peer := m.state.Peers[rand.Intn(len(m.state.Peers))]
		m.typingBursts[peer.PeerID] = 2 + rand.Intn(9)
		m.emitSoloEvent(peer, "keyboard")
		return
	}
	if m.cfg.Solo.Mouse && rand.Float64() < .035 {
		peer := m.state.Peers[rand.Intn(len(m.state.Peers))]
		m.emitSoloEvent(peer, "mouse")
	}
}

func (m *soloModel) emitSoloEvent(peer PeerPresence, kind string) {
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
	for peerID := range m.typingBursts {
		if !strings.HasPrefix(peerID, "solo-") {
			delete(m.typingBursts, peerID)
			continue
		}
		var index int
		if _, err := fmt.Sscanf(peerID, "solo-%02d", &index); err != nil || index >= count {
			delete(m.typingBursts, peerID)
		}
	}
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
		m.cfg.Listening.AmbientVolume = clamp(m.cfg.Listening.AmbientVolume+delta, 0.05, 1)
		m.applyListening()
		return
	}
	m.persist()
}

var soloSliderActions = []string{"master-slider", "keyboard-slider", "mouse-slider", "ambient-slider"}

func isSoloSlider(action string) bool {
	for _, candidate := range soloSliderActions {
		if action == candidate {
			return true
		}
	}
	return false
}

func (m *soloModel) rememberHoveredSlider() {
	for index, action := range soloSliderActions {
		if m.hoverAction == action {
			m.sliderCursor = index
			return
		}
	}
}

func (m soloModel) activeSlider() string {
	if isSoloSlider(m.hoverAction) {
		return m.hoverAction
	}
	return soloSliderActions[clampInt(m.sliderCursor, 0, len(soloSliderActions)-1)]
}

func (m *soloModel) adjustActiveSlider(delta float64) {
	switch m.activeSlider() {
	case "keyboard-slider":
		m.adjustSoloVolume("keyboard", delta)
	case "mouse-slider":
		m.adjustSoloVolume("mouse", delta)
	case "ambient-slider":
		m.adjustSoloVolume("ambient", delta)
	default:
		m.adjustVolume(delta)
	}
}

func (m *soloModel) setSliderFromPointer(region liveHitRegion, x int) {
	trackWidth := maxInt(1, region.width-2)
	value := clamp(float64(x-region.x-1)/float64(trackWidth-1), 0, 1)
	switch region.action {
	case "keyboard-slider":
		m.cfg.Solo.KeyboardVolume = value
		m.persist()
	case "mouse-slider":
		m.cfg.Solo.MouseVolume = value
		m.persist()
	case "ambient-slider":
		m.cfg.Listening.AmbientVolume = clamp(value, .05, 1)
		m.applyListening()
	default:
		m.cfg.Listening.Volume = value
		m.applyListening()
	}
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
	footerText := " Hover slider + arrows/scroll adjust    Tab next slider    +/- people    Space wakes room    Esc back"
	if width < 110 {
		footerText = " Sliders: hover + arrows/scroll    Tab next    +/- people    Esc back"
	}
	footer := styleDim.Render(ansi.Truncate(footerText, width, ""))
	if width < 96 {
		if m.height < 46 {
			controls := stylePanel.Width(width).Height(bodyHeight).Render(m.controlView(width-6, true))
			return lipgloss.JoinVertical(lipgloss.Left, header, controls, footer)
		}
		controlHeight := minInt(24, bodyHeight-10)
		deskHeight := maxInt(10, bodyHeight-controlHeight-1)
		desk := stylePanel.Width(width).Height(deskHeight).Render(deskModel.renderSpatialDesk(width-6, deskHeight-3))
		controls := stylePanel.Width(width).Height(controlHeight).Render(m.controlView(width-6, false))
		return lipgloss.JoinVertical(lipgloss.Left, header, desk, controls, footer)
	}
	mapWidth := maxInt(50, width-52)
	infoWidth := width - mapWidth - 2
	desk := stylePanel.Width(mapWidth).Height(bodyHeight).Render(deskModel.renderSpatialDesk(mapWidth-6, bodyHeight-3))
	controls := stylePanel.Width(infoWidth).Height(bodyHeight).Render(m.controlView(infoWidth-6, false))
	return lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, desk, "  ", controls), footer)
}

func (m soloModel) header(width int) string {
	return styleTitle.Width(width).MaxWidth(width).Render(fmt.Sprintf("Cliks  /  Solo Desk    %d coworkers    OFFLINE + PRIVATE", m.cfg.Solo.People))
}

func (m soloModel) controlView(width int, compact bool) string {
	width = maxInt(32, width)
	lines := []string{
		styleAccent.Render("SOLO DESK"),
		styleDim.Render("A private room mixed only for you."),
		"",
		"Coworkers    " + m.button("less", "−") + "  " + styleSecond.Render(fmt.Sprintf("%2d", m.cfg.Solo.People)) + "  " + m.button("more", "+"),
		"",
		styleSecond.Render("SOUND MIX"),
		m.sliderLine("master-slider", "Master", m.cfg.Listening.Volume, width),
		m.button("keyboard", "Keyboard  "+onOff(m.cfg.Solo.Keyboard)),
		m.sliderLine("keyboard-slider", "Keyboard", m.cfg.Solo.KeyboardVolume, width),
		m.button("mouse", "Clicks    "+onOff(m.cfg.Solo.Mouse)),
		m.sliderLine("mouse-slider", "Clicks", m.cfg.Solo.MouseVolume, width),
		"",
		styleThird.Render("ROOM TONE"),
		m.button("ambient", ambientLabel(m.cfg.Listening.Ambient)),
		m.sliderLine("ambient-slider", "Room level", m.cfg.Listening.AmbientVolume, width),
		"",
		m.button("mute", "Mute  "+onOff(m.cfg.Listening.Muted)) + "    " + m.button("spark", "Wake the room"),
	}
	if compact {
		lines = append([]string{styleDim.Render("○   ○   [ YOU ]   ○   ○"), ""}, lines...)
	}
	if m.message != "" {
		lines = append(lines, "", styleDim.Render(ansi.Truncate(m.message, width, "…")))
	}
	lines = append(lines, "", styleDim.Render("Local only · no capture · no network"), m.button("back", "Back"))
	return strings.Join(lines, "\n")
}

func (m soloModel) sliderLine(action string, label string, value float64, width int) string {
	trackWidth := clampInt(width-20, 10, 24)
	filled := clampInt(int(value*float64(trackWidth)+.5), 0, trackWidth)
	track := "[" + styleAccent.Render(strings.Repeat("━", filled)) + styleDim.Render(strings.Repeat("─", trackWidth-filled)) + "]"
	prefix := "  "
	if action == m.activeSlider() {
		prefix = styleSecond.Render("› ")
	}
	line := prefix + fmt.Sprintf("%-11s", label) + track + fmt.Sprintf(" %3.0f%%", value*100)
	if m.hoverAction == action {
		return styleSelected.Render(ansi.Strip(line))
	}
	return line
}

func (m soloModel) button(action, label string) string {
	text := "[ " + label + " ]"
	if m.hoverAction == action {
		return styleSelected.Render(ansi.Strip(text))
	}
	return styleFocused.Render(text)
}

func (m soloModel) hit(x, y int) string {
	if region, ok := m.hitRegion(x, y); ok {
		return region.action
	}
	return ""
}

func (m soloModel) hitRegion(x, y int) (liveHitRegion, bool) {
	for _, region := range m.hitRegions() {
		if y == region.y && x >= region.x && x < region.x+region.width {
			return region, true
		}
	}
	return liveHitRegion{}, false
}

func (m soloModel) hitRegions() []liveHitRegion {
	rendered := m.View()
	labels := []struct{ action, label string }{
		{"less", "−"}, {"more", "+"},
		{"keyboard", "Keyboard  " + onOff(m.cfg.Solo.Keyboard)},
		{"mouse", "Clicks    " + onOff(m.cfg.Solo.Mouse)},
		{"ambient", ambientLabel(m.cfg.Listening.Ambient)},
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
		for localY, styledLine := range strings.Split(rendered, "\n") {
			line := ansi.Strip(styledLine)
			start := 0
			for {
				index := strings.Index(line[start:], needle)
				if index < 0 {
					break
				}
				index += start
				if found == occurrence {
					regions = append(regions, liveHitRegion{action: item.action, x: ansi.StringWidth(line[:index]), y: localY, width: ansi.StringWidth(needle)})
					goto nextTarget
				}
				found++
				start = index + len(needle)
			}
		}
	nextTarget:
	}
	for index, label := range []string{"Master", "Keyboard", "Clicks", "Room level"} {
		action := soloSliderActions[index]
		for y, styledLine := range strings.Split(rendered, "\n") {
			line := ansi.Strip(styledLine)
			labelAt := strings.Index(line, label)
			if labelAt < 0 {
				continue
			}
			trackAt := strings.Index(line[labelAt+len(label):], "[")
			if trackAt < 0 {
				continue
			}
			trackAt += labelAt + len(label)
			trackEnd := strings.Index(line[trackAt:], "]")
			if trackEnd < 0 {
				continue
			}
			regions = append(regions, liveHitRegion{action: action, x: ansi.StringWidth(line[:trackAt]), y: y, width: ansi.StringWidth(line[trackAt : trackAt+trackEnd+1])})
			break
		}
	}
	return regions
}

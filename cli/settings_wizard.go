package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsWizardModel struct {
	cfg            CliksConfig
	originalCfg    CliksConfig
	categoryIndex  int
	itemIndex      int
	width          int
	height         int
	textEditing    bool
	textBuffer     string
	message        string
	isError        bool
	saved          bool
	focusArea      int // 0: Category Bar, 1: Settings List, 2: Action Bar
	actionIndex    int // 0: Save & Apply, 1: Reset Defaults, 2: Cancel
	reconnect      bool
}

func runSettingsWizardTUI(cfg CliksConfig) error {
	if !isInteractiveTerminal() {
		printSettingCatalog()
		return nil
	}
	applyTheme(cfg.Theme)
	model := newSettingsWizardModel(cfg)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseAllMotion())
	finalModel, err := program.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if result, ok := finalModel.(settingsWizardModel); ok && result.saved {
		fmt.Println("Saved settings to disk.")
	}
	return nil
}

func newSettingsWizardModel(cfg CliksConfig) settingsWizardModel {
	return settingsWizardModel{
		cfg:         cfg,
		originalCfg: cfg,
		focusArea:   1, // start with focus in settings list
		message:     "Use ←/→ or 1-5 for categories, ↑/↓ for settings, Enter/←/→ to adjust, 's' to save.",
	}
}

func (m settingsWizardModel) Init() tea.Cmd {
	return nil
}

func (m *settingsWizardModel) currentCategory() SettingCategory {
	if m.categoryIndex < 0 {
		m.categoryIndex = 0
	}
	if m.categoryIndex >= len(settingCategories) {
		m.categoryIndex = len(settingCategories) - 1
	}
	return settingCategories[m.categoryIndex]
}

func (m *settingsWizardModel) currentItems() []configSettingMetadata {
	return settingsForCategory(m.currentCategory().ID)
}

func (m *settingsWizardModel) currentItem() (configSettingMetadata, bool) {
	items := m.currentItems()
	if len(items) == 0 {
		return configSettingMetadata{}, false
	}
	if m.itemIndex < 0 {
		m.itemIndex = 0
	}
	if m.itemIndex >= len(items) {
		m.itemIndex = len(items) - 1
	}
	return items[m.itemIndex], true
}

func formatOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m *settingsWizardModel) getSettingValueString(key string) string {
	switch key {
	case "volume":
		return fmt.Sprintf("%.2f", m.cfg.Listening.Volume)
	case "density":
		return fmt.Sprintf("%.2f", m.cfg.Listening.Density)
	case "hear.muted":
		return formatOnOff(m.cfg.Listening.Muted)
	case "hear.spatial":
		return formatOnOff(m.cfg.Listening.Spatial)
	case "hear.fade":
		return formatOnOff(m.cfg.Listening.FatigueProtection)
	case "hear.keyboard":
		return formatOnOff(m.cfg.Listening.Keyboard)
	case "hear.mouse":
		return formatOnOff(m.cfg.Listening.Mouse)
	case "hear.self":
		return formatOnOff(m.cfg.Listening.Self)
	case "ambient":
		return m.cfg.Listening.Ambient
	case "ambient.volume":
		return fmt.Sprintf("%.2f", m.cfg.Listening.AmbientVolume)
	case "audio.device":
		if m.cfg.Listening.AudioDevice == "" {
			return "default"
		}
		return m.cfg.Listening.AudioDevice
	case "share.keyboard":
		return formatOnOff(m.cfg.Sharing.Keyboard)
	case "share.mouse":
		return formatOnOff(m.cfg.Sharing.Mouse)
	case "capture.mode":
		return m.cfg.Capture.Mode
	case "nickname", "name":
		return m.cfg.Nickname
	case "presence":
		return m.cfg.PresenceStatus
	case "theme":
		return m.cfg.Theme
	case "spatial.dynamic":
		return formatOnOff(m.cfg.Listening.DynamicPlacement)
	case "spatial.shuffleMinutes":
		return strconv.Itoa(m.cfg.Listening.ShuffleMinutes)
	case "solo.people":
		return strconv.Itoa(m.cfg.Solo.People)
	case "solo.keyboard":
		return formatOnOff(m.cfg.Solo.Keyboard)
	case "solo.mouse":
		return formatOnOff(m.cfg.Solo.Mouse)
	case "solo.keyboardVolume":
		return fmt.Sprintf("%.2f", m.cfg.Solo.KeyboardVolume)
	case "solo.mouseVolume":
		return fmt.Sprintf("%.2f", m.cfg.Solo.MouseVolume)
	case "autostart":
		return formatOnOff(m.cfg.AutostartWanted)
	case "keep.running":
		return formatOnOff(m.cfg.KeepRunning)
	case "notifications":
		return formatOnOff(m.cfg.Notifications.Enabled)
	case "notifications.sound":
		return formatOnOff(m.cfg.Notifications.Sound)
	case "batch.ms":
		return strconv.Itoa(m.cfg.BatchWindowMs)
	case "api.url":
		return m.cfg.APIURL
	case "ws.url":
		return m.cfg.WSURL
	default:
		return ""
	}
}

func (m *settingsWizardModel) applyVal(key string, proposed string) error {
	tempCfg := m.cfg
	reconnectNeeded, err := applyConfigSetting(&tempCfg, key, proposed)
	if err != nil {
		m.message = fmt.Sprintf("Invalid value for %s: %v", key, err)
		m.isError = true
		return err
	}
	m.cfg = tempCfg
	if reconnectNeeded {
		m.reconnect = true
	}
	m.message = fmt.Sprintf("Set %s to %s", key, proposed)
	m.isError = false
	applyTheme(m.cfg.Theme)
	return nil
}

func (m *settingsWizardModel) saveAndApply() tea.Cmd {
	if err := saveConfig(m.cfg); err != nil {
		m.message = "Failed to save settings: " + err.Error()
		m.isError = true
		return nil
	}
	if m.reconnect || m.originalCfg.APIURL != m.cfg.APIURL || m.originalCfg.WSURL != m.cfg.WSURL {
		if _, ok := activeSession(); ok {
			_ = enqueueSessionCommand(localSessionCommand{Type: "reload_connection"})
		}
	}
	m.saved = true
	m.message = "Settings saved successfully!"
	m.isError = false
	return tea.Quit
}

func (m settingsWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Text Editing mode
		if m.textEditing {
			switch msg.Type {
			case tea.KeyEnter:
				item, ok := m.currentItem()
				if ok {
					_ = m.applyVal(item.Key, m.textBuffer)
				}
				m.textEditing = false
				return m, nil
			case tea.KeyEsc:
				m.textEditing = false
				return m, nil
			case tea.KeyBackspace:
				if len(m.textBuffer) > 0 {
					m.textBuffer = m.textBuffer[:len(m.textBuffer)-1]
				}
				return m, nil
			case tea.KeyRunes:
				m.textBuffer += string(msg.Runes)
				return m, nil
			default:
				return m, nil
			}
		}

		// Normal keyboard handling
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.focusArea != 1 {
				m.focusArea = 1
			} else {
				return m, tea.Quit
			}
			return m, nil
		case "s", "ctrl+s":
			return m, m.saveAndApply()
		case "tab":
			m.focusArea = (m.focusArea + 1) % 3
			return m, nil
		case "shift+tab":
			m.focusArea = (m.focusArea + 2) % 3
			return m, nil
		case "1", "2", "3", "4", "5":
			idx := int(msg.Runes[0] - '1')
			if idx >= 0 && idx < len(settingCategories) {
				m.categoryIndex = idx
				m.itemIndex = 0
				m.focusArea = 1
			}
			return m, nil
		case "[":
			if m.categoryIndex > 0 {
				m.categoryIndex--
				m.itemIndex = 0
			}
			return m, nil
		case "]":
			if m.categoryIndex < len(settingCategories)-1 {
				m.categoryIndex++
				m.itemIndex = 0
			}
			return m, nil
		}

		switch m.focusArea {
		case 0: // Category Bar
			switch msg.String() {
			case "left", "h":
				if m.categoryIndex > 0 {
					m.categoryIndex--
					m.itemIndex = 0
				}
			case "right", "l":
				if m.categoryIndex < len(settingCategories)-1 {
					m.categoryIndex++
					m.itemIndex = 0
				}
			case "down", "j", "enter":
				m.focusArea = 1
				m.itemIndex = 0
			}

		case 1: // Settings List
			items := m.currentItems()
			switch msg.String() {
			case "up", "k":
				if m.itemIndex > 0 {
					m.itemIndex--
				} else {
					m.focusArea = 0
				}
			case "down", "j":
				if m.itemIndex < len(items)-1 {
					m.itemIndex++
				} else {
					m.focusArea = 2
					m.actionIndex = 0
				}
			case "left", "h":
				m.adjustItemValue(-1)
			case "right", "l":
				m.adjustItemValue(1)
			case "enter", "space":
				m.activateItem()
			}

		case 2: // Action Bar
			switch msg.String() {
			case "left", "h":
				if m.actionIndex > 0 {
					m.actionIndex--
				}
			case "right", "l":
				if m.actionIndex < 2 {
					m.actionIndex++
				}
			case "up", "k":
				m.focusArea = 1
				items := m.currentItems()
				m.itemIndex = len(items) - 1
			case "enter", "space":
				switch m.actionIndex {
				case 0: // Save & Apply
					return m, m.saveAndApply()
				case 1: // Reset Defaults
					m.cfg = defaultConfig()
					m.message = "Reset settings to safe defaults."
					m.isError = false
				case 2: // Cancel
					return m, tea.Quit
				}
			}
		}

	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			// Basic mouse support: check category tabs
			y := msg.Y
			if y == 3 {
				// Category bar row
				x := msg.X
				col := 2
				for i, cat := range settingCategories {
					tabWidth := len(cat.Name) + 4
					if x >= col && x < col+tabWidth {
						m.categoryIndex = i
						m.itemIndex = 0
						m.focusArea = 1
						break
					}
					col += tabWidth + 1
				}
			}
		}
	}

	return m, nil
}

func (m *settingsWizardModel) adjustItemValue(direction int) {
	item, ok := m.currentItem()
	if !ok {
		return
	}
	currentVal := m.getSettingValueString(item.Key)

	switch item.ControlType {
	case "toggle":
		if strings.EqualFold(currentVal, "on") || currentVal == "true" || currentVal == "1" {
			_ = m.applyVal(item.Key, "off")
		} else {
			_ = m.applyVal(item.Key, "on")
		}

	case "range":
		parsed, _ := strconv.ParseFloat(currentVal, 64)
		step := item.Step
		if step == 0 {
			step = 0.05
		}
		newVal := parsed + float64(direction)*step
		if newVal < item.Min {
			newVal = item.Min
		}
		if newVal > item.Max {
			newVal = item.Max
		}
		var valStr string
		if item.Step >= 1 {
			valStr = strconv.Itoa(int(newVal + 0.5))
		} else {
			valStr = fmt.Sprintf("%.2f", newVal)
		}
		_ = m.applyVal(item.Key, valStr)

	case "select":
		if len(item.Options) == 0 {
			return
		}
		currIdx := 0
		for i, opt := range item.Options {
			if strings.EqualFold(opt, currentVal) {
				currIdx = i
				break
			}
		}
		nextIdx := (currIdx + direction + len(item.Options)) % len(item.Options)
		_ = m.applyVal(item.Key, item.Options[nextIdx])

	case "text":
		m.textEditing = true
		m.textBuffer = currentVal
	}
}

func (m *settingsWizardModel) activateItem() {
	item, ok := m.currentItem()
	if !ok {
		return
	}
	if item.ControlType == "text" {
		m.textEditing = true
		m.textBuffer = m.getSettingValueString(item.Key)
	} else {
		m.adjustItemValue(1)
	}
}

func (m settingsWizardModel) View() string {
	var b strings.Builder

	// Header
	header := styleTitle.Render(" Cliks Settings Wizard ") + " " + styleDim.Render("Interactive Configuration")
	b.WriteString(header + "\n\n")

	// Category Nav Bar
	var catTabs []string
	for i, cat := range settingCategories {
		label := fmt.Sprintf(" %d. %s ", i+1, cat.Name)
		if i == m.categoryIndex {
			if m.focusArea == 0 {
				catTabs = append(catTabs, styleSelected.Render(label))
			} else {
				catTabs = append(catTabs, styleFocused.Render("["+label+"]"))
			}
		} else {
			catTabs = append(catTabs, styleDim.Render(label))
		}
	}
	b.WriteString("  " + strings.Join(catTabs, "  ") + "\n\n")

	// Content Panel
	items := m.currentItems()
	var contentLines []string
	for i, item := range items {
		isSelected := (i == m.itemIndex && m.focusArea == 1)
		prefix := "  "
		if isSelected {
			prefix = "► "
		}

		valStr := m.getSettingValueString(item.Key)
		widgetStr := m.renderWidget(item, valStr, isSelected)

		labelStyle := lipgloss.NewStyle().Bold(isSelected)
		if isSelected {
			labelStyle = styleAccent
		}

		line := fmt.Sprintf("%s%-22s %s", prefix, labelStyle.Render(item.Label), widgetStr)
		descLine := fmt.Sprintf("    %s", styleDim.Render(item.Description))
		contentLines = append(contentLines, line, descLine)
	}

	b.WriteString(strings.Join(contentLines, "\n") + "\n\n")

	// Status / Message Bar
	if m.message != "" {
		msgStyle := styleOK
		if m.isError {
			msgStyle = styleWarn
		}
		b.WriteString("  " + msgStyle.Render(m.message) + "\n\n")
	}

	// Action Bar
	var actionBtns []string
	actions := []string{"Save & Apply", "Reset Defaults", "Cancel"}
	for i, act := range actions {
		label := " " + act + " "
		if i == m.actionIndex && m.focusArea == 2 {
			actionBtns = append(actionBtns, styleSelected.Render("["+label+"]"))
		} else {
			actionBtns = append(actionBtns, styleDim.Render("["+label+"]"))
		}
	}
	b.WriteString("  " + strings.Join(actionBtns, "   ") + "\n\n")

	// Footer Help
	helpText := styleDim.Render("  1-5/← → Categories • ↑ ↓ Select • ← → Adjust • Enter Edit • 's' Save • Esc Exit")
	b.WriteString(helpText)

	return b.String()
}

func (m settingsWizardModel) renderWidget(item configSettingMetadata, currentVal string, isSelected bool) string {
	switch item.ControlType {
	case "toggle":
		isOn := strings.EqualFold(currentVal, "on") || currentVal == "true" || currentVal == "1"
		if isOn {
			return styleOK.Render("[● On ]")
		}
		return styleDim.Render("[○ Off]")

	case "range":
		parsed, _ := strconv.ParseFloat(currentVal, 64)
		pct := 0.0
		if item.Max > item.Min {
			pct = (parsed - item.Min) / (item.Max - item.Min)
		}
		if pct < 0 {
			pct = 0
		}
		if pct > 1 {
			pct = 1
		}
		barWidth := 10
		filled := int(pct * float64(barWidth))
		if filled < 0 {
			filled = 0
		}
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		displayVal := currentVal
		if item.Step >= 1 {
			displayVal = fmt.Sprintf("%d", int(parsed+0.5))
		}
		if item.Key == "batch.ms" {
			displayVal += " ms"
		} else if item.Key == "spatial.shuffleMinutes" {
			displayVal += " min"
		} else if item.Key == "solo.people" {
			displayVal += " coworkers"
		}

		return fmt.Sprintf("[%s] %s", styleAccent.Render(bar), displayVal)

	case "select":
		return fmt.Sprintf("< %s >", styleAccent.Render(currentVal))

	case "text":
		if m.textEditing && isSelected {
			return styleSelected.Render("[" + m.textBuffer + "│]")
		}
		if currentVal == "" {
			return styleDim.Render("[ (none) ]")
		}
		return fmt.Sprintf("[ %s ]", currentVal)

	default:
		return currentVal
	}
}

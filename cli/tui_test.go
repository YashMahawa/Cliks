package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeClickOutsideHighlightedRowDoesNotActivate(t *testing.T) {
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := homeModel{
		cfg:       cfg,
		mode:      "home",
		cursor:    2,
		mouseOver: false,
	}

	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 1, Y: 0})
	got := updated.(homeModel)
	if cmd != nil {
		t.Fatalf("outside click returned command")
	}
	if got.mode != "home" {
		t.Fatalf("mode = %q, want home", got.mode)
	}
	if got.action != "" {
		t.Fatalf("action = %q, want none", got.action)
	}
	if got.cursor != model.cursor {
		t.Fatalf("cursor = %d, want %d", got.cursor, model.cursor)
	}
}

func TestPreferencesClickOutsideRowDoesNotChangeSetting(t *testing.T) {
	cfg := defaultConfig()
	model := homeModel{
		cfg:            cfg,
		mode:           "preferences",
		settingsCursor: 2,
		mouseOver:      false,
	}

	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 1, Y: 0})
	got := updated.(homeModel)
	if cmd != nil {
		t.Fatalf("outside preferences click returned command")
	}
	if got.cfg.Listening.Muted != cfg.Listening.Muted {
		t.Fatalf("muted = %v, want %v", got.cfg.Listening.Muted, cfg.Listening.Muted)
	}
	if got.settingsCursor != model.settingsCursor {
		t.Fatalf("settingsCursor = %d, want %d", got.settingsCursor, model.settingsCursor)
	}
}

func TestKeepRunningToggleWithoutActiveSessionDoesNotStart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := homeModel{
		cfg:    cfg,
		mode:   "home",
		cursor: 1,
	}

	updated, cmd := model.activate()
	got := updated.(homeModel)
	if cmd != nil {
		t.Fatalf("keep-running toggle returned command")
	}
	if got.action != "" {
		t.Fatalf("action = %q, want none", got.action)
	}
	if !got.cfg.KeepRunning {
		t.Fatalf("KeepRunning = false, want true")
	}
	if saved := loadConfig(); !saved.KeepRunning {
		t.Fatalf("saved KeepRunning = false, want true")
	}
}

func TestLiveEscapeReturnsToHomeInsteadOfStopping(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(sessionModel)
	if cmd == nil {
		t.Fatalf("escape did not request quit/back transition")
	}
	if got.exit != sessionExitBack {
		t.Fatalf("exit = %q, want back", got.exit)
	}
}

func TestLiveActionRailHoverAndClickBack(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)
	model.width = 120
	model.height = 32
	width := panelWidth(model.width)
	mapWidth := int(float64(width) * 0.68)
	railX := mapWidth + 4
	railWidth := maxInt(18, width-mapWidth-2)
	y := 21
	x := railX + railWidth/2

	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseMotion, X: x, Y: y})
	got := updated.(sessionModel)
	if cmd != nil {
		t.Fatalf("hover returned command")
	}
	if got.hoverAction != "back" {
		t.Fatalf("hoverAction = %q, want back", got.hoverAction)
	}

	updated, cmd = got.Update(tea.MouseMsg{Type: tea.MouseLeft, X: x, Y: y})
	got = updated.(sessionModel)
	if cmd == nil {
		t.Fatalf("back click did not request transition")
	}
	if got.exit != sessionExitBack {
		t.Fatalf("exit = %q, want back", got.exit)
	}
}

func TestLiveMiddleDoesNotHighlightDetachedControls(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := newSessionModel(newSessionController(cfg, StartOptions{}, nil))
	model.width = 120
	model.height = 32

	updated, _ := model.Update(tea.MouseMsg{Type: tea.MouseMotion, X: 12, Y: 16})
	got := updated.(sessionModel)
	if got.hoverAction != "" {
		t.Fatalf("middle of desk highlighted %q", got.hoverAction)
	}
}

func TestLiveNotificationRowTogglesDirectly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := newSessionModel(newSessionController(cfg, StartOptions{}, nil))
	model.width = 120
	model.height = 32
	width := panelWidth(model.width)
	railX := int(float64(width)*0.68) + 5

	updated, _ := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: railX, Y: 13})
	got := updated.(sessionModel)
	if !got.controller.cfg.Notifications.Enabled {
		t.Fatal("clicking Notifications did not toggle it on")
	}
}

func TestLiveTabOpensUnifiedPreferences(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(sessionModel)
	if got.mode != "settings" {
		t.Fatalf("mode = %q, want settings", got.mode)
	}
	view := got.View()
	if !strings.Contains(view, "Dynamic circle") || !strings.Contains(view, "Keep Running") {
		t.Fatalf("settings view does not include unified preference rows:\n%s", view)
	}
}

func TestLiveViewShowsHealthAndFlowDuringQuietPeriods(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)
	now := time.Now()
	model.now = now
	model.state.ConnectionStatus = "connected"
	model.state.LastLocalActivityAt = now.Add(-2 * time.Second)
	model.state.LocalBurstCount = 30

	view := model.View()
	if !strings.Contains(view, "Flow:") || !strings.Contains(view, "deep flow") || !strings.Contains(view, "Health:") {
		t.Fatalf("live view missing health/flow status:\n%s", view)
	}
}

func TestFirstLiveViewTeachesSpatialDeskWithoutNetworkPeers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	controller := newSessionController(cfg, StartOptions{}, nil)
	model := newSessionModel(controller)
	model.width = 100
	model.height = 28

	view := model.View()
	for _, want := range []string{"Welcome to your desk", "Mira", "Sam", "Noor", "[ YOU ]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("first live view is missing %q:\n%s", want, view)
		}
	}
	if !loadConfig().WelcomeSeen {
		t.Fatal("first live view did not persist the welcome marker")
	}
}

func TestHomeShortcutGuideTogglesWithoutChangingSelection(t *testing.T) {
	model := homeModel{cfg: defaultConfig(), mode: "home", cursor: 1}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(homeModel)
	if !got.helpOpen || got.cursor != 1 {
		t.Fatalf("helpOpen = %v, cursor = %d; want open with cursor preserved", got.helpOpen, got.cursor)
	}
	if view := got.View(); !strings.Contains(view, "Up/k, Down/j") || !strings.Contains(view, "Mouse") {
		t.Fatalf("shortcut guide is incomplete:\n%s", view)
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(homeModel).helpOpen {
		t.Fatal("escape did not close the shortcut guide")
	}
}

func TestLiveShortcutGuideDocumentsSessionControls(t *testing.T) {
	controller := newSessionController(defaultConfig(), StartOptions{}, nil)
	model := newSessionModel(controller)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(sessionModel)
	view := got.View()
	for _, text := range []string{"m / s / f", "Tab/Shift+S", "x/Ctrl+C", "Mouse wheel"} {
		if !strings.Contains(view, text) {
			t.Fatalf("live shortcut guide is missing %q:\n%s", text, view)
		}
	}
}

func TestFirstRunSurfacesJoinCreateAndSoundCheck(t *testing.T) {
	model := homeModel{cfg: defaultConfig(), mode: "home"}
	items := model.items()
	want := []string{"join", "create", "sound", "doctor"}
	for index, key := range want {
		if items[index].key != key {
			t.Fatalf("first-run item %d = %q, want %q", index, items[index].key, key)
		}
	}
	view := model.View()
	if !strings.Contains(view, "Set up Cliks") || !strings.Contains(view, "Join Team") {
		t.Fatalf("first-run view does not expose setup actions:\n%s", view)
	}
}

func TestFormEditingUsesCursorInsteadOfAppendOnlyInput(t *testing.T) {
	model := homeModel{
		cfg:            defaultConfig(),
		mode:           "join",
		joinCode:       "CLIK-ABCDEF",
		formTextCursor: len([]rune("CLIK-ABCDEF")),
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated, _ = updated.(homeModel).Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated, _ = updated.(homeModel).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated, _ = updated.(homeModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}})
	got := updated.(homeModel)
	if got.joinCode != "CLIK-ABCZEF" {
		t.Fatalf("join code = %q, want cursor edit CLIK-ABCZEF", got.joinCode)
	}
	if !strings.Contains(got.View(), "|") {
		t.Fatal("form view does not render the text cursor")
	}
}

func TestFormMouseClickFocusesOnlyTheClickedField(t *testing.T) {
	model := homeModel{
		cfg:            defaultConfig(),
		mode:           "create",
		width:          100,
		formCursor:     0,
		createPassword: "secret",
	}
	updated, cmd := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 8, Y: formRowsStartY() + 1})
	got := updated.(homeModel)
	if cmd != nil || got.formCursor != 1 || got.formTextCursor != len([]rune("secret")) {
		t.Fatalf("form click result: cursor=%d textCursor=%d cmd=%v", got.formCursor, got.formTextCursor, cmd)
	}
}

func TestSettingsWindowKeepsSelectionVisibleInShortTerminal(t *testing.T) {
	rows := settingsRows(defaultConfig())
	start, end := settingsWindow(len(rows), len(rows)-1, 15)
	if end != len(rows) || start >= end || end-start > 5 {
		t.Fatalf("settings window = [%d,%d) for %d rows", start, end, len(rows))
	}
}

func TestAdvancedBatchWindowValidation(t *testing.T) {
	if got, err := parseBatchWindow("500"); err != nil || got != 500 {
		t.Fatalf("parseBatchWindow(500) = %d, %v", got, err)
	}
	for _, value := range []string{"99", "2001", "nope"} {
		if _, err := parseBatchWindow(value); err == nil {
			t.Fatalf("parseBatchWindow(%q) should fail", value)
		}
	}
}

func TestAdvancedServerFormSwitchesToSelfHostedBackend(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	model := homeModel{cfg: defaultConfig(), mode: "advanced"}
	updated, _ := model.activate()
	got := updated.(homeModel)
	if got.mode != "backend-url" {
		t.Fatalf("mode = %q, want backend-url", got.mode)
	}
	got.backendURLValue = "https://cliks.example.com"
	updated, _ = got.submitForm()
	got = updated.(homeModel)
	if got.cfg.APIURL != "https://cliks.example.com" || got.cfg.WSURL != "wss://cliks.example.com/ws" {
		t.Fatalf("backend = %q, %q", got.cfg.APIURL, got.cfg.WSURL)
	}
}

func TestPublicServerBatchWindowOpensWithPolicyMessageInsteadOfForm(t *testing.T) {
	model := homeModel{cfg: defaultConfig(), mode: "advanced", cursor: 3}
	updated, _ := model.activate()
	got := updated.(homeModel)
	if got.mode != "advanced" || !strings.Contains(got.message, "public relay is fixed at 500 ms") {
		t.Fatalf("public batch action = mode %q, message %q", got.mode, got.message)
	}
}

func TestDoctorCommandOpensScrollableReportInsideTUI(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	model := homeModel{cfg: defaultConfig(), mode: "diagnostics", height: 16}
	updated, cmd := model.Update(commandDoneMsg{
		message: "Found 1 setup item.",
		report:  []string{"Cliks doctor", "System:", "- Audio player: missing", "Fixes:", "Install audio:"},
	})
	got := updated.(homeModel)
	if cmd != nil || got.mode != "doctor-report" {
		t.Fatalf("mode = %q, cmd = %v; want doctor-report", got.mode, cmd)
	}
	view := got.View()
	if !strings.Contains(view, "Audio player: missing") || !strings.Contains(view, "[ Back ]") || !strings.Contains(view, "[ Refresh ]") {
		t.Fatalf("doctor report view is incomplete:\n%s", view)
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(homeModel).mode != "diagnostics" {
		t.Fatalf("escape returned to %q, want diagnostics", updated.(homeModel).mode)
	}
}

func TestHomeFooterKeepsConnectionAndVolumeVisible(t *testing.T) {
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	cfg.Listening.Volume = 0.65
	model := homeModel{
		cfg:      cfg,
		mode:     "preferences",
		activeOK: true,
		active: ActiveSessionState{
			TeamCode:         "CLIK-LOCAL",
			TeamName:         "Study",
			ConnectionStatus: "connected",
			ActiveCount:      3,
		},
	}
	footer := model.statusFooterView()
	for _, want := range []string{"Study (CLIK-LOCAL)", "connected", "volume 65%", "3 here"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("status footer missing %q: %s", want, footer)
		}
	}
}

func TestNewUsersGetDynamicCircleByDefault(t *testing.T) {
	if !defaultConfig().Listening.DynamicPlacement {
		t.Fatal("DynamicPlacement = false, want true for new configurations")
	}
}

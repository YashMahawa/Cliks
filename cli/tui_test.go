package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
		cursor: 2,
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
	x, y := renderedTextPosition(t, model.View(), "[ Back ]")
	x++

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
	x, y := renderedTextPosition(t, model.View(), "[ Notifications")
	updated, _ := model.Update(tea.MouseMsg{Type: tea.MouseLeft, X: x + 2, Y: y})
	got := updated.(sessionModel)
	if !got.controller.cfg.Notifications.Enabled {
		t.Fatal("clicking Notifications did not toggle it on")
	}
}

func TestLiveReactionButtonsUseExactRenderedHitboxes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.CurrentTeamCode = "CLIK-LOCAL"
	model := newSessionModel(newSessionController(cfg, StartOptions{}, nil))
	model.width = 120
	model.height = 32
	niceX, niceY := renderedTextPosition(t, model.View(), "[ 👍 Nice ]")
	breakX, breakY := renderedTextPosition(t, model.View(), "[ 🧘 Break ]")
	waveX, _ := renderedTextPosition(t, model.View(), "[ 👋 Wave ]")

	if got := model.liveHit(niceX+2, niceY); got != "reaction-nice" {
		t.Fatalf("nice hit = %q, want reaction-nice", got)
	}
	if got := model.liveHit(breakX+2, breakY); got != "reaction-break" {
		t.Fatalf("break hit = %q, want reaction-break", got)
	}
	if got := model.liveHit(waveX+ansi.StringWidth("[ 👋 Wave ]")+1, niceY); got != "" {
		t.Fatalf("gap between reaction buttons hit %q", got)
	}
}

func renderedTextPosition(t *testing.T, view string, needle string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(view), "\n") {
		if index := strings.Index(line, needle); index >= 0 {
			return ansi.StringWidth(line[:index]), y
		}
	}
	t.Fatalf("rendered view is missing %q:\n%s", needle, view)
	return 0, 0
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

func TestFirstSetupShowsOneDecisionAtATime(t *testing.T) {
	model := homeModel{cfg: defaultConfig(), mode: "home", onboardingPending: true, onboardingSuggestion: "CozyOtter"}
	model.finishLaunch()
	if model.mode != "first-setup" {
		t.Fatalf("mode = %q, want first-setup", model.mode)
	}
	items := model.items()
	if len(items) != 3 || items[0].key != "nickname" || items[1].key != "onboarding-random-name" {
		t.Fatalf("nickname step items = %#v", items)
	}
	view := model.View()
	for _, want := range []string{"SETUP  1/7", "What should the room call you?", "CozyOtter"} {
		if !strings.Contains(view, want) {
			t.Fatalf("first setup view is missing %q:\n%s", want, view)
		}
	}
}

func TestFirstSetupNicknameReturnsToOnboarding(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	model := homeModel{cfg: defaultConfig(), mode: "first-setup", cursor: 0}
	updated, _ := model.activate()
	form := updated.(homeModel)
	if form.mode != "nickname" || form.formReturnMode != "first-setup" {
		t.Fatalf("nickname form mode=%q return=%q", form.mode, form.formReturnMode)
	}
	form.nicknameValue = "Mira"
	updated, _ = form.submitForm()
	got := updated.(homeModel)
	if got.mode != "first-setup" || got.cfg.Nickname != "Mira" || got.onboardingStep != 1 {
		t.Fatalf("after nickname mode=%q nickname=%q step=%d", got.mode, got.cfg.Nickname, got.onboardingStep)
	}
}

func TestFirstLaunchChoreographyTeachesPresenceTypingAndSignals(t *testing.T) {
	tests := []struct {
		elapsed time.Duration
		wants   []string
	}{
		{500 * time.Millisecond, []string{"A quiet room is taking shape", "[ YOU ]"}},
		{2500 * time.Millisecond, []string{"Your people find their places", "Mira", "Sam"}},
		{4500 * time.Millisecond, []string{"Activity becomes ambience", "Mira and Noor are typing"}},
		{6500 * time.Millisecond, []string{"Small signals cross the room", "Sam", "Hey there!"}},
		{9 * time.Second, []string{"Your desk is ready", "never share key values", "Enter skips"}},
	}
	for _, test := range tests {
		view := renderLaunchCanvas(100, 24, test.elapsed, true, "Sol")
		for _, want := range test.wants {
			if !strings.Contains(view, want) {
				t.Fatalf("at %s launch view is missing %q:\n%s", test.elapsed, want, view)
			}
		}
	}
}

func TestNormalLaunchUsesResponsiveDeskAndOneSoundPhase(t *testing.T) {
	view := renderLaunchCanvas(90, 20, 2*time.Second, false, "Sol")
	if !strings.Contains(view, "WARM DESK") || !strings.Contains(view, "Opening Sol") || !strings.Contains(view, "[ YOU ]") || !strings.Contains(view, "listening") || !strings.Contains(view, "Enter skips") {
		t.Fatalf("normal launch view is incomplete:\n%s", view)
	}
	if launchSoundCuePhase(3*time.Second) != 0 || launchSoundCuePhase(4*time.Second) != 1 || launchSoundCuePhase(8*time.Second) != 2 {
		t.Fatal("first-launch sound cue phases changed")
	}
}

func TestReactionAnimationNamesSenderInsideDesk(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := defaultConfig()
	model := newSessionModel(newSessionController(cfg, StartOptions{}, nil))
	now := time.Now()
	model.now = now
	model.state.Peers = []PeerPresence{{PeerID: "peer-mira", Nickname: "Mira", JoinedAt: now.Add(-time.Minute).UnixMilli()}}
	model.state.RecentReactions = []PeerReactionStatus{{PeerID: "peer-mira", Nickname: "Mira", Reaction: "break", At: now}}
	view := model.renderSpatialDesk(80, 22)
	if !strings.Contains(view, "Mira") || !strings.Contains(view, "🧘") || !strings.Contains(view, "Let’s take a break.") {
		t.Fatalf("reaction animation does not identify the sender and fixed message:\n%s", view)
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
	want := []string{"solo", "join", "create", "sound", "doctor"}
	for index, key := range want {
		if items[index].key != key {
			t.Fatalf("first-run item %d = %q, want %q", index, items[index].key, key)
		}
	}
	view := model.View()
	if !strings.Contains(view, "Set up Cliks") || !strings.Contains(view, "Solo Desk") || !strings.Contains(view, "Join Team") {
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

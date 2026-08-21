package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDoctorReportLinesIncludeChecksAndFixCommands(t *testing.T) {
	report := doctorReport{
		privacy: []string{"Only activity kinds are sent."},
		checks:  []doctorCheck{{label: "Audio player", status: "missing"}},
		issues: []doctorIssue{{
			title:    "Install audio",
			detail:   "Playback needs a supported player.",
			commands: []string{"cliks sound-test"},
		}},
	}
	text := strings.Join(doctorReportLines(report), "\n")
	for _, want := range []string{"Privacy:", "Audio player: missing", "Install audio:", "cliks sound-test"} {
		if !strings.Contains(text, want) {
			t.Fatalf("doctor report missing %q:\n%s", want, text)
		}
	}
}

func TestPassiveDoctorWarningSkipsMissingTeamOnly(t *testing.T) {
	report := doctorReport{issues: []doctorIssue{{title: "Join a team"}, {title: "Install audio"}}}
	got := passiveDoctorWarning(report)
	if strings.Contains(got, "Join a team") || !strings.Contains(got, "Install audio") {
		t.Fatalf("passive warning = %q", got)
	}
}

func TestDoctorReportUnreachableAudioEndpoint(t *testing.T) {
	originalRunner := audioCommandRunner
	originalDetector := detectAudioPlayerForDevice
	defer func() {
		audioCommandRunner = originalRunner
		detectAudioPlayerForDevice = originalDetector
	}()

	detectAudioPlayerForDevice = func(device string) *audioPlayer {
		return &audioPlayer{
			Command:       "mpv",
			DeviceRouting: true,
		}
	}

	audioCommandRunner = func(_ context.Context, _ *audioPlayer, job playbackJob) error {
		if job.Device == "missing_speaker" {
			return errors.New("device not found")
		}
		return nil
	}

	cfg := CliksConfig{}
	cfg.Listening.AudioDevice = "missing_speaker"

	report := buildDoctorReportOptions(cfg, false)
	foundUnreachableCheck := false
	foundUnreachableIssue := false

	for _, check := range report.checks {
		if check.label == "Audio output" && strings.Contains(check.status, "unreachable") {
			foundUnreachableCheck = true
		}
	}
	for _, issue := range report.issues {
		if strings.Contains(issue.title, "unreachable") {
			foundUnreachableIssue = true
		}
	}

	if !foundUnreachableCheck {
		t.Fatalf("expected 'Audio output' check to report unreachable, got checks: %#v", report.checks)
	}
	if !foundUnreachableIssue {
		t.Fatalf("expected issue for unreachable endpoint, got issues: %#v", report.issues)
	}
}


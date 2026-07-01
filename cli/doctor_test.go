package main

import (
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

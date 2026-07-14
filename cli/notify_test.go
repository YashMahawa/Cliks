package main

import "testing"

func TestReactionNotificationCopyIsFixedAndDistinct(t *testing.T) {
	tests := []struct {
		reaction string
		glyph    string
		phrase   string
	}{
		{"wave", "👋", "Hey there!"},
		{"nice", "👍", "Nice work!"},
		{"coffee", "☕", "Coffee time?"},
		{"celebrate", "🎉", "That deserves a celebration!"},
		{"break", "🧘", "Let’s take a break."},
	}
	for _, test := range tests {
		t.Run(test.reaction, func(t *testing.T) {
			if got := reactionGlyph(test.reaction); got != test.glyph {
				t.Fatalf("glyph = %q, want %q", got, test.glyph)
			}
			if got := reactionPhrase(test.reaction); got != test.phrase {
				t.Fatalf("phrase = %q, want %q", got, test.phrase)
			}
		})
	}
}

func TestReactionNotificationTitleContainsSenderAndMessage(t *testing.T) {
	title, body := reactionNotificationContent("Mira", "break")
	if title != "Mira 🧘 Let’s take a break." {
		t.Fatalf("title = %q", title)
	}
	if body != "Cliks quick signal" {
		t.Fatalf("body = %q", body)
	}
	title, _ = reactionNotificationContent("", "wave")
	if title != "A teammate 👋 Hey there!" {
		t.Fatalf("anonymous title = %q", title)
	}
}

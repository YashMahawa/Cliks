package main

import "testing"

func TestNormalizeNicknameStripsTerminalSequencesBeforeTruncating(t *testing.T) {
	input := "\x1b[31mAlice\x1b[0m\x1b]0;owned\x07 Long Name"
	if got := normalizeNickname(input); got != "Alice Long" {
		t.Fatalf("nickname = %q, want Alice Long", got)
	}
}

func TestNormalizeNicknameRemovesControlAndFormatCharacters(t *testing.T) {
	if got := normalizeNickname("Ali\x00ce\u202e Bob"); got != "Alice Bob" {
		t.Fatalf("nickname = %q, want Alice Bob", got)
	}
}

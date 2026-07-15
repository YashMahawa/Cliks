package main

import "testing"

func TestRandomFunnyNicknameIsSafeAndShort(t *testing.T) {
	for i := 0; i < 50; i++ {
		name := randomFunnyNickname()
		if name == "" || len([]rune(name)) > 10 || sanitizeNickname(name) != name {
			t.Fatalf("invalid generated nickname %q", name)
		}
	}
}

//go:build linux

package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestUserInGroupDynamicAndFallback(t *testing.T) {
	username := currentUsername()
	if username == "" {
		t.Skip("no current username found")
	}

	// Fetch actual groups using id -Gn
	out, err := exec.Command("id", "-Gn", username).Output()
	if err != nil {
		t.Skip("id -Gn command unavailable")
	}
	groups := strings.Fields(string(out))
	if len(groups) == 0 {
		t.Skip("no groups returned by id -Gn")
	}

	knownGroup := groups[0]
	inGroup, err := userInGroup(username, knownGroup)
	if err != nil {
		t.Fatalf("userInGroup(%q, %q) error: %v", username, knownGroup, err)
	}
	if !inGroup {
		t.Fatalf("expected user %q to be in group %q", username, knownGroup)
	}

	// Test non-existent group
	inNonExistent, err := userInGroup(username, "nonexistent_group_99999")
	if err != nil {
		t.Fatalf("userInGroup for non-existent group returned error: %v", err)
	}
	if inNonExistent {
		t.Fatalf("user %q unexpectedly reported in nonexistent_group_99999", username)
	}
}

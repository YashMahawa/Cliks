package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActivityWireEventCannotSerializeKeyContent(t *testing.T) {
	data, err := json.Marshal(RemoteActivityEvent{Kind: "keyboard", OffsetMs: 150})
	if err != nil {
		t.Fatal(err)
	}
	wire := string(data)
	if wire != `{"kind":"keyboard","offsetMs":150}` {
		t.Fatalf("unexpected activity wire shape: %s", wire)
	}
	for _, forbidden := range []string{"keyCode", "scanCode", "rune", "text", "window", "app", "coordinate"} {
		if strings.Contains(strings.ToLower(wire), strings.ToLower(forbidden)) {
			t.Fatalf("wire event exposed forbidden field %q: %s", forbidden, wire)
		}
	}
}

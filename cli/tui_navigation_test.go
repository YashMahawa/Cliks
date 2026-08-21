package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStandardWheelAdjustDelta(t *testing.T) {
	step := 0.05

	deltaUp, okUp := StandardWheelAdjustDelta(tea.MouseMsg{Type: tea.MouseWheelUp}, step)
	if !okUp || deltaUp <= 0 {
		t.Fatalf("expected positive delta for MouseWheelUp, got delta=%v ok=%v", deltaUp, okUp)
	}

	deltaDown, okDown := StandardWheelAdjustDelta(tea.MouseMsg{Type: tea.MouseWheelDown}, step)
	if !okDown || deltaDown >= 0 {
		t.Fatalf("expected negative delta for MouseWheelDown, got delta=%v ok=%v", deltaDown, okDown)
	}

	_, okMotion := StandardWheelAdjustDelta(tea.MouseMsg{Type: tea.MouseMotion}, step)
	if okMotion {
		t.Fatal("expected ok=false for MouseMotion")
	}
}

func TestStandardValueAdjustKeyDelta(t *testing.T) {
	step := 0.05

	deltaRight, okRight := StandardValueAdjustKeyDelta("right", step)
	if !okRight || deltaRight <= 0 {
		t.Fatalf("expected positive delta for right key, got delta=%v ok=%v", deltaRight, okRight)
	}

	deltaLeft, okLeft := StandardValueAdjustKeyDelta("left", step)
	if !okLeft || deltaLeft >= 0 {
		t.Fatalf("expected negative delta for left key, got delta=%v ok=%v", deltaLeft, okLeft)
	}

	_, okOther := StandardValueAdjustKeyDelta("up", step)
	if okOther {
		t.Fatal("expected ok=false for non-horizontal key")
	}
}

func TestNavigateFocusRow(t *testing.T) {
	total := 4

	// Down moves down (0 -> 1)
	next, ok := NavigateFocusRow("down", 0, total)
	if !ok || next != 1 {
		t.Fatalf("down from 0 want 1, got %v (ok=%v)", next, ok)
	}

	// Down clamps at total-1
	next, ok = NavigateFocusRow("down", 3, total)
	if !ok || next != 3 {
		t.Fatalf("down from 3 want 3, got %v (ok=%v)", next, ok)
	}

	// Up moves up (1 -> 0)
	next, ok = NavigateFocusRow("up", 1, total)
	if !ok || next != 0 {
		t.Fatalf("up from 1 want 0, got %v (ok=%v)", next, ok)
	}

	// Up clamps at 0
	next, ok = NavigateFocusRow("up", 0, total)
	if !ok || next != 0 {
		t.Fatalf("up from 0 want 0, got %v (ok=%v)", next, ok)
	}
}

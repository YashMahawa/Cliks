package main

import tea "github.com/charmbracelet/bubbletea"

// StandardWheelAdjustDelta returns the value or volume adjustment delta for a mouse wheel event.
// MouseWheelUp produces a positive delta (+step) to increase value/volume.
// MouseWheelDown produces a negative delta (-step) to decrease value/volume.
func StandardWheelAdjustDelta(msg tea.MouseMsg, step float64) (float64, bool) {
	switch msg.Type {
	case tea.MouseWheelUp:
		return step, true
	case tea.MouseWheelDown:
		return -step, true
	default:
		return 0, false
	}
}

// StandardValueAdjustKeyDelta returns the value adjustment delta for Left/Right or +/- keys.
// Right, +, or = produces a positive delta (+step).
// Left or - produces a negative delta (-step).
func StandardValueAdjustKeyDelta(key string, step float64) (float64, bool) {
	switch key {
	case "right", "+", "=":
		return step, true
	case "left", "-":
		return -step, true
	default:
		return 0, false
	}
}

// NavigateFocusRow returns the new cursor index when navigating focus vertically up/down between control rows.
// Up decreases the cursor index (moves focus upward).
// Down increases the cursor index (moves focus downward).
func NavigateFocusRow(key string, current int, total int) (int, bool) {
	if total <= 0 {
		return 0, false
	}
	switch key {
	case "up":
		return clampInt(current-1, 0, total-1), true
	case "down":
		return clampInt(current+1, 0, total-1), true
	default:
		return current, false
	}
}

// NavigateFocusRowWithVim returns the new cursor index when navigating focus vertically including vim keys (k/j).
// Up or k decreases the cursor index (moves focus upward).
// Down or j increases the cursor index (moves focus downward).
func NavigateFocusRowWithVim(key string, current int, total int) (int, bool) {
	if total <= 0 {
		return 0, false
	}
	switch key {
	case "up", "k":
		return clampInt(current-1, 0, total-1), true
	case "down", "j":
		return clampInt(current+1, 0, total-1), true
	default:
		return current, false
	}
}

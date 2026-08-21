package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDeclarativeFormsAllModes(t *testing.T) {
	modes := []string{"create", "join", "delete", "nickname", "audio-device", "batch-window", "backend-url"}
	for _, mode := range modes {
		initial := map[string]string{
			"name":     "Test Room",
			"password": "secretpassword",
			"code":     "CLIK-TEST12",
			"nickname": "TestUser",
			"device":   "default",
			"window":   "500",
			"url":      "https://cliks.example.com",
		}
		form := newDeclarativeForm(mode, initial)
		if form == nil {
			t.Fatalf("newDeclarativeForm(%q) returned nil", mode)
		}
		if form.Mode != mode {
			t.Fatalf("form.Mode = %q, want %q", form.Mode, mode)
		}
		if len(form.Fields) == 0 {
			t.Fatalf("mode %q has 0 fields", mode)
		}
		viewLines := form.View()
		if len(viewLines) == 0 {
			t.Fatalf("mode %q View() produced 0 lines", mode)
		}
	}
}

func TestFormPasswordMasking(t *testing.T) {
	form := newDeclarativeForm("create", map[string]string{
		"name":     "Secret Room",
		"password": "supersecretpassword",
	})
	if form == nil {
		t.Fatal("form is nil")
	}

	// Field 1 is password
	if !form.Fields[1].Secret {
		t.Fatal("create password field is not marked secret")
	}

	view := strings.Join(form.View(), "\n")
	if strings.Contains(view, "supersecretpassword") {
		t.Fatalf("form view revealed secret password in plain text:\n%s", view)
	}
}

func TestFormValidation(t *testing.T) {
	// Create form validation
	createForm := newDeclarativeForm("create", map[string]string{
		"name":     "Room",
		"password": "123", // too short
	})
	if err := createForm.ValidateAll(); err == nil {
		t.Fatal("createForm with short password should fail validation")
	}

	// Batch window validation
	batchForm := newDeclarativeForm("batch-window", map[string]string{
		"window": "50", // out of range
	})
	if err := batchForm.ValidateAll(); err == nil {
		t.Fatal("batchForm with window < 100 should fail validation")
	}

	// Join form validation
	joinForm := newDeclarativeForm("join", map[string]string{
		"code": "",
	})
	if err := joinForm.ValidateAll(); err == nil {
		t.Fatal("joinForm with empty code should fail validation")
	}
}

func TestFormFieldFocusTraversal(t *testing.T) {
	form := newDeclarativeForm("delete", map[string]string{
		"code":     "CLIK-DELETE",
		"password": "secretpassword",
	})
	if form.FocusedIdx != 0 {
		t.Fatalf("initial focus = %d, want 0", form.FocusedIdx)
	}

	form.FocusNext()
	if form.FocusedIdx != 1 {
		t.Fatalf("after FocusNext focus = %d, want 1", form.FocusedIdx)
	}

	form.FocusPrev()
	if form.FocusedIdx != 0 {
		t.Fatalf("after FocusPrev focus = %d, want 0", form.FocusedIdx)
	}
}

func TestFormStructuredValueMap(t *testing.T) {
	form := newDeclarativeForm("create", map[string]string{
		"name":     "My Room",
		"password": "mysecretpassword",
	})
	values := form.ValueMap()
	if values["name"] != "My Room" || values["password"] != "mysecretpassword" {
		t.Fatalf("ValueMap = %#v", values)
	}
}

func TestFormUpdateEditing(t *testing.T) {
	form := newDeclarativeForm("nickname", map[string]string{
		"nickname": "Alex",
	})
	form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if form.Value("nickname") != "Alexa" {
		t.Fatalf("nickname = %q, want Alexa", form.Value("nickname"))
	}
}

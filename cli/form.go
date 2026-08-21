package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DeclarativeFormField struct {
	Key         string
	Label       string
	Placeholder string
	Secret      bool
	Validate    func(string) error
	Input       textinput.Model
}

type DeclarativeForm struct {
	Mode       string
	Title      string
	HelpText   string
	Fields     []DeclarativeFormField
	FocusedIdx int
}

func newDeclarativeForm(mode string, initialValues map[string]string) *DeclarativeForm {
	var fields []DeclarativeFormField
	var title, help string

	switch mode {
	case "create":
		title = "Create Team"
		fields = []DeclarativeFormField{
			createFormField("name", "Team name", "Cliks Room", false, initialValues["name"], nil),
			createFormField("password", "Delete password", "not set", true, initialValues["password"], func(val string) error {
				if len(strings.TrimSpace(val)) < 6 {
					return errors.New("Delete password must be at least 6 characters.")
				}
				return nil
			}),
		}
	case "join":
		title = "Join Team"
		fields = []DeclarativeFormField{
			createFormField("code", "Team code", "CLIK-XXXXXX", false, initialValues["code"], func(val string) error {
				if strings.TrimSpace(val) == "" {
					return errors.New("Team code is required.")
				}
				return nil
			}),
		}
	case "delete":
		title = "Delete Team"
		fields = []DeclarativeFormField{
			createFormField("code", "Team code", "CLIK-XXXXXX", false, initialValues["code"], func(val string) error {
				if strings.TrimSpace(val) == "" {
					return errors.New("Team code is required.")
				}
				return nil
			}),
			createFormField("password", "Delete password", "not set", true, initialValues["password"], func(val string) error {
				if strings.TrimSpace(val) == "" {
					return errors.New("Delete password is required.")
				}
				return nil
			}),
		}
	case "nickname":
		title = "Nickname"
		fields = []DeclarativeFormField{
			createFormField("nickname", "Display name", "anonymous", false, initialValues["nickname"], nil),
		}
	case "audio-device":
		title = "Audio Output"
		fields = []DeclarativeFormField{
			createFormField("device", "Device", "default", false, initialValues["device"], nil),
		}
	case "batch-window":
		title = "Batch Window"
		fields = []DeclarativeFormField{
			createFormField("window", "Milliseconds", "500", false, initialValues["window"], func(val string) error {
				_, err := parseBatchWindow(val)
				return err
			}),
		}
	case "backend-url":
		title = "Server"
		help = "Type public to restore Cliks. Self-hosting unlocks larger room limits and 100-2000 ms batching."
		fields = []DeclarativeFormField{
			createFormField("url", "HTTP URL", productionAPIURL, false, initialValues["url"], func(val string) error {
				_, err := normalizeBackendURL(val)
				return err
			}),
		}
	default:
		return nil
	}

	form := &DeclarativeForm{
		Mode:       mode,
		Title:      title,
		HelpText:   help,
		Fields:     fields,
		FocusedIdx: 0,
	}
	form.FocusField(0)
	return form
}

func createFormField(key, label, placeholder string, secret bool, initialVal string, validate func(string) error) DeclarativeFormField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle()
	ti.PlaceholderStyle = styleDim
	ti.CharLimit = 128
	if key == "code" {
		ti.CharLimit = 32
	} else if key == "nickname" {
		ti.CharLimit = 32
	}
	if secret {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
	}
	ti.SetValue(initialVal)
	return DeclarativeFormField{
		Key:         key,
		Label:       label,
		Placeholder: placeholder,
		Secret:      secret,
		Validate:    validate,
		Input:       ti,
	}
}

func (f *DeclarativeForm) FocusField(idx int) {
	if len(f.Fields) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	} else if idx >= len(f.Fields) {
		idx = len(f.Fields) - 1
	}
	f.FocusedIdx = idx
	for i := range f.Fields {
		if i == idx {
			f.Fields[i].Input.Focus()
			f.Fields[i].Input.CursorEnd()
		} else {
			f.Fields[i].Input.Blur()
		}
	}
}

func (f *DeclarativeForm) FocusNext() {
	if f.FocusedIdx < len(f.Fields)-1 {
		f.FocusField(f.FocusedIdx + 1)
	}
}

func (f *DeclarativeForm) FocusPrev() {
	if f.FocusedIdx > 0 {
		f.FocusField(f.FocusedIdx - 1)
	}
}

func (f *DeclarativeForm) Update(msg tea.Msg) {
	if f.FocusedIdx >= 0 && f.FocusedIdx < len(f.Fields) {
		var cmd tea.Cmd
		f.Fields[f.FocusedIdx].Input, cmd = f.Fields[f.FocusedIdx].Input.Update(msg)
		_ = cmd
		if f.Fields[f.FocusedIdx].Key == "code" {
			f.Fields[f.FocusedIdx].Input.SetValue(strings.ToUpper(f.Fields[f.FocusedIdx].Input.Value()))
		}
	}
}

func (f *DeclarativeForm) Value(key string) string {
	for _, field := range f.Fields {
		if field.Key == key {
			return field.Input.Value()
		}
	}
	return ""
}

func (f *DeclarativeForm) SetValue(key string, value string) {
	for i := range f.Fields {
		if f.Fields[i].Key == key {
			if f.Fields[i].Key == "code" {
				value = strings.ToUpper(value)
			}
			f.Fields[i].Input.SetValue(value)
			return
		}
	}
}

func (f *DeclarativeForm) ValueMap() map[string]string {
	res := make(map[string]string)
	for _, field := range f.Fields {
		res[field.Key] = field.Input.Value()
	}
	return res
}

func (f *DeclarativeForm) ValidateCurrent() error {
	if f.FocusedIdx >= 0 && f.FocusedIdx < len(f.Fields) {
		field := f.Fields[f.FocusedIdx]
		if field.Validate != nil {
			return field.Validate(field.Input.Value())
		}
	}
	return nil
}

func (f *DeclarativeForm) ValidateAll() error {
	for _, field := range f.Fields {
		if field.Validate != nil {
			if err := field.Validate(field.Input.Value()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *DeclarativeForm) View() []string {
	var rows []string
	for i, field := range f.Fields {
		isFocused := i == f.FocusedIdx
		line := fmt.Sprintf("%-18s %s", field.Label, field.Input.View())
		if isFocused {
			rows = append(rows, styleSelected.Render(" "+line+" "))
		} else {
			rows = append(rows, line)
		}
	}
	if f.HelpText != "" {
		rows = append(rows, styleDim.Render(f.HelpText))
	}
	return rows
}

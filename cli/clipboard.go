package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

type clipboardCommand struct {
	name string
	args []string
}

func copyTextToClipboard(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("nothing to copy")
	}
	for _, candidate := range clipboardCandidates() {
		cmd := exec.Command(candidate.name, candidate.args...)
		cmd.Stdin = strings.NewReader(value)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return errors.New("no clipboard command found")
}

func clipboardCandidates() []clipboardCommand {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardCommand{{name: "pbcopy"}}
	case "windows":
		return []clipboardCommand{{name: "clip"}}
	case "android":
		return []clipboardCommand{{name: "termux-clipboard-set"}}
	default:
		return []clipboardCommand{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
			{name: "termux-clipboard-set"},
		}
	}
}

func clipboardStatus(value string) string {
	if err := copyTextToClipboard(value); err == nil {
		return "Copied code to clipboard."
	}
	return "Clipboard unavailable; copy the code manually."
}

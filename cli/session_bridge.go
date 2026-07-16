package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type localSessionCommand struct {
	Type     string `json:"type"`
	Reaction string `json:"reaction,omitempty"`
}

func sessionCommandDir() string {
	return filepath.Join(stateDir(), "commands")
}

func enqueueSessionCommand(command localSessionCommand) error {
	dir := sessionCommandDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(command)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%020d-%d.json", time.Now().UnixNano(), os.Getpid())
	return atomicWriteFile(filepath.Join(dir, name), append(data, '\n'), 0o600)
}

func consumeSessionCommands(handle func(localSessionCommand)) {
	entries, err := os.ReadDir(sessionCommandDir())
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(sessionCommandDir(), entry.Name())
		data, readErr := os.ReadFile(path)
		_ = os.Remove(path)
		if readErr != nil {
			continue
		}
		var command localSessionCommand
		if json.Unmarshal(data, &command) == nil {
			handle(command)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileReplacesWithoutTruncatingTargetFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("old-content-that-must-survive-partial-failure\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := []byte("{\"ok\":true}\n")
	if err := atomicWriteFile(path, next, 0o644); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(next) {
		t.Fatalf("got %q, want %q", got, next)
	}
}

func TestAtomicWriteFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "a", "config.json")
	if err := atomicWriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x\n" {
		t.Fatalf("got %q", got)
	}
}

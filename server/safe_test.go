package main

import "testing"

func TestRunSafelyRecoversPanics(t *testing.T) {
	if recovered := runSafely("test task", func() { panic("boom") }); !recovered {
		t.Fatal("runSafely did not report the recovered panic")
	}

	run := false
	if recovered := runSafely("healthy task", func() { run = true }); recovered {
		t.Fatal("runSafely reported a panic for a healthy task")
	}
	if !run {
		t.Fatal("healthy task did not run")
	}
}

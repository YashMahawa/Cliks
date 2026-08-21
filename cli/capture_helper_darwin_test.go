//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacCaptureHelperPath_ValidOverride(t *testing.T) {
	tempDir := t.TempDir()
	overridePath := filepath.Join(tempDir, "cliks-capture")
	if err := os.WriteFile(overridePath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create temp override file: %v", err)
	}

	t.Setenv("CLIKS_CAPTURE_HELPER", overridePath)

	origVerifier := macCodeSignatureVerifier
	defer func() { macCodeSignatureVerifier = origVerifier }()

	macCodeSignatureVerifier = func(path string) error {
		if path == overridePath {
			return nil
		}
		return fmt.Errorf("unexpected path: %s", path)
	}

	got := macCaptureHelperPath()
	if got != overridePath {
		t.Errorf("macCaptureHelperPath() = %q, want %q", got, overridePath)
	}
	if warn := getMacHelperOverrideWarning(); warn != "" {
		t.Errorf("expected empty override warning, got %q", warn)
	}
}

func TestMacCaptureHelperPath_InvalidOverride_Fallback(t *testing.T) {
	tempDir := t.TempDir()
	overridePath := filepath.Join(tempDir, "bad-cliks-capture")
	if err := os.WriteFile(overridePath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create bad override file: %v", err)
	}

	t.Setenv("CLIKS_CAPTURE_HELPER", overridePath)

	origVerifier := macCodeSignatureVerifier
	defer func() { macCodeSignatureVerifier = origVerifier }()

	macCodeSignatureVerifier = func(path string) error {
		if path == overridePath {
			return fmt.Errorf("code signature invalid: adhoc signature missing")
		}
		return nil
	}

	got := macCaptureHelperPath()
	if got == overridePath {
		t.Errorf("macCaptureHelperPath() returned rejected override path %q", got)
	}

	warn := getMacHelperOverrideWarning()
	if !strings.Contains(warn, "rejected") || !strings.Contains(warn, "code signature invalid") {
		t.Errorf("expected warning containing rejection details, got %q", warn)
	}
}

func TestMacCaptureHelperPath_InvalidStandard_Fallback(t *testing.T) {
	t.Setenv("CLIKS_CAPTURE_HELPER", "")

	origVerifier := macCodeSignatureVerifier
	defer func() { macCodeSignatureVerifier = origVerifier }()

	macCodeSignatureVerifier = func(path string) error {
		return fmt.Errorf("signature verification failed")
	}

	got := macCaptureHelperPath()
	if got != "" {
		t.Errorf("expected empty path for invalid standard candidate, got %q", got)
	}
}

func TestParseBundleIdentifierFromCodesignOutput(t *testing.T) {
	sampleOutput := `Executable=/Applications/Cliks Capture.app/Contents/MacOS/cliks-capture
Identifier=io.cliks.capture
Format=app bundle with Mach-O universal (x86_64 arm64)
CodeDirectory v=20400 size=741 flags=0x2(adhoc) hashes=15+2 location=embedded
`
	got := parseBundleIdentifierFromCodesignOutput(sampleOutput)
	if got != "io.cliks.capture" {
		t.Errorf("parseBundleIdentifierFromCodesignOutput = %q, want %q", got, "io.cliks.capture")
	}

	missing := parseBundleIdentifierFromCodesignOutput("Executable=/path\nFormat=macho\n")
	if missing != "" {
		t.Errorf("expected empty bundle id for output missing Identifier, got %q", missing)
	}
}

func TestParseBundleIdentifierFromInfoPlist(t *testing.T) {
	tempDir := t.TempDir()
	macOSDir := filepath.Join(tempDir, "Cliks Capture.app", "Contents", "MacOS")
	if err := os.MkdirAll(macOSDir, 0755); err != nil {
		t.Fatalf("failed to create app bundle dir: %v", err)
	}

	execPath := filepath.Join(macOSDir, "cliks-capture")
	if err := os.WriteFile(execPath, []byte("binary"), 0755); err != nil {
		t.Fatalf("failed to write exec binary: %v", err)
	}

	infoPlistPath := filepath.Join(tempDir, "Cliks Capture.app", "Contents", "Info.plist")
	infoPlistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>cliks-capture</string>
  <key>CFBundleIdentifier</key><string>io.cliks.capture</string>
</dict>
</plist>`
	if err := os.WriteFile(infoPlistPath, []byte(infoPlistContent), 0644); err != nil {
		t.Fatalf("failed to write Info.plist: %v", err)
	}

	got := parseBundleIdentifierFromInfoPlist(execPath)
	if got != "io.cliks.capture" {
		t.Errorf("parseBundleIdentifierFromInfoPlist() = %q, want %q", got, "io.cliks.capture")
	}
}

func TestAppendPlatformCaptureChecks_OverrideRejected(t *testing.T) {
	setMacHelperOverrideWarning("CLIKS_CAPTURE_HELPER override \"/usr/local/bin/bad\" rejected: code signature invalid")

	report := doctorReport{}
	appendPlatformCaptureChecks(&report, false)

	foundCheck := false
	for _, check := range report.checks {
		if check.label == "Custom capture helper override" && check.status == "rejected" {
			foundCheck = true
			break
		}
	}
	if !foundCheck {
		t.Errorf("doctorReport checks missing Custom capture helper override check")
	}

	foundIssue := false
	for _, issue := range report.issues {
		if issue.title == "Custom capture helper override rejected" && strings.Contains(issue.detail, "code signature invalid") {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Errorf("doctorReport issues missing Custom capture helper override rejected issue with details")
	}
}

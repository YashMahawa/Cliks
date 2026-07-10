//go:build !linux && !darwin && !windows

package main

func appendPlatformCaptureChecks(report *doctorReport, thorough bool) {
	_ = thorough
	report.issues = append(report.issues, doctorIssue{
		title:    "Global capture is limited on this platform",
		detail:   "Cliks supports global capture on Linux, macOS, and Windows. Use terminal capture for a local self-test.",
		commands: []string{"cliks start --terminal --self"},
	})
	report.recommendation = []string{"Recommended local test:", "cliks start --terminal --self"}
}

func platformStartupCaptureNotice() string {
	return ""
}

//go:build !linux && !darwin && !windows

package main

func platformCaptureSetup() []setupStep {
	return []setupStep{{
		title:  "Background capture",
		status: "tip",
		detail: "Global capture is best supported on Linux, macOS, and Windows. Try: cliks start --terminal --self",
	}}
}

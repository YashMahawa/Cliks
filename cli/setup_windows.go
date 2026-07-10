//go:build windows

package main

func platformCaptureSetup() []setupStep {
	steps := []setupStep{}
	// Global hooks work for standard-user apps without a special permission dialog.
	// UIPI only pauses capture while an elevated window is focused — not a setup blocker.
	elevated, _ := windowsElevationStatus()
	if elevated {
		steps = append(steps, setupStep{
			title:  "Background capture",
			status: "ok",
			detail: "Running elevated. Global capture covers normal and Administrator windows.",
		})
		return steps
	}
	steps = append(steps, setupStep{
		title:  "Background capture",
		status: "ok",
		detail: "Ready. Cliks captures keyboard/mouse activity kinds system-wide with no extra Windows permission.",
	})
	steps = append(steps, setupStep{
		title:  "Elevated windows",
		status: "tip",
		detail: "If you focus Task Manager or another Administrator app, capture pauses until you leave that window (Windows security). Everyday apps are fine.",
	})
	return steps
}

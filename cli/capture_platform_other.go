//go:build !darwin && !windows && !linux

package main

func (c *ActivityCapture) platformProviders(mode string) []InputCaptureProvider {
	return []InputCaptureProvider{&TerminalCaptureProvider{capture: c}}
}

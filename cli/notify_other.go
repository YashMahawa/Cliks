//go:build !linux && !darwin && !windows

package main

import "fmt"

func sendNativeNotification(title string, body string, sound bool) error {
	return fmt.Errorf("native notifications are not supported on this platform")
}

func nativeNotificationPlatformReady() bool           { return false }
func repairNativeNotificationService() (bool, string) { return false, "" }

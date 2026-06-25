package main

import (
	"context"
	"fmt"
	"time"
)

func runCaptureTest(cfg CliksConfig, mode string, seconds int) error {
	if seconds <= 0 {
		seconds = 8
	}
	fmt.Println("Cliks capture test")
	fmt.Println("")
	fmt.Println("Privacy: this counts only keyboard/mouse event types locally. It does not print or send key values.")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	capture := newActivityCapture()
	state := capture.start(ctx, cfg.Sharing, mode)
	defer capture.stop()
	fmt.Printf("Mode: %s\n", state.Mode)
	if state.PermissionHint != "" {
		fmt.Printf("Permission: %s\n", state.PermissionHint)
	}
	fmt.Printf("Type and click for %d seconds...\n", seconds)
	keyboard := 0
	mouse := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Println("")
			fmt.Printf("Keyboard events: %d\n", keyboard)
			fmt.Printf("Mouse events: %d\n", mouse)
			if keyboard+mouse == 0 {
				fmt.Println("")
				fmt.Println("Nothing captured.")
				if state.PermissionHint != "" {
					fmt.Println(state.PermissionHint)
				}
				fmt.Println("Try: cliks doctor")
			}
			return nil
		case event := <-capture.Events:
			if event.Kind == "keyboard" {
				keyboard++
			}
			if event.Kind == "mouse" {
				mouse++
			}
		}
	}
}

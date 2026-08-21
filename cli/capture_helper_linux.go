//go:build linux

package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type LinuxIsolatedHelperProvider struct {
	capture *ActivityCapture
}

func (p *LinuxIsolatedHelperProvider) Name() string { return "linux-isolated-helper" }

func (p *LinuxIsolatedHelperProvider) Probe(ctx context.Context) ProbeResult {
	conn, err := net.DialTimeout("unix", linuxCaptureSocket(), 300*time.Millisecond)
	if err != nil {
		return ProbeResult{Name: "linux-isolated-helper", Available: false, State: DriverStateFailed, Mode: "off", PermissionHint: "Isolated Linux capture socket not ready."}
	}
	_ = conn.Close()
	return ProbeResult{Name: "linux-isolated-helper", Available: true, State: DriverStateActive, Mode: "linux-isolated-helper"}
}

func (p *LinuxIsolatedHelperProvider) Start(ctx context.Context, sharing SharingConfig, events chan<- LocalActivityEvent) (CaptureState, error) {
	state := p.capture.startLinuxCaptureHelper(ctx, sharing)
	if state.Mode == "off" {
		return state, fmt.Errorf("linux capture helper failed: %s", state.PermissionHint)
	}
	return state, nil
}

func (p *LinuxIsolatedHelperProvider) Stop() error { return nil }

type LinuxEvdevProvider struct {
	capture *ActivityCapture
}

func (p *LinuxEvdevProvider) Name() string { return "evdev" }

func (p *LinuxEvdevProvider) Probe(ctx context.Context) ProbeResult {
	input := linuxInputStatus()
	if !input.hasInputDir || input.readableCount == 0 {
		return ProbeResult{Name: "evdev", Available: false, State: DriverStateFailed, Mode: "off", PermissionHint: "Direct Linux evdev capture permission denied for /dev/input."}
	}
	return ProbeResult{Name: "evdev", Available: true, State: DriverStateActive, Mode: "evdev"}
}

func (p *LinuxEvdevProvider) Start(ctx context.Context, sharing SharingConfig, events chan<- LocalActivityEvent) (CaptureState, error) {
	state := p.capture.startEvdev(ctx, sharing)
	if state.Mode == "off" {
		return state, fmt.Errorf("linux evdev capture failed: %s", state.PermissionHint)
	}
	return state, nil
}

func (p *LinuxEvdevProvider) Stop() error { return nil }

func (c *ActivityCapture) platformProviders(mode string) []InputCaptureProvider {
	if mode == "direct" || mode == "evdev" {
		return []InputCaptureProvider{&LinuxEvdevProvider{capture: c}, &LinuxIsolatedHelperProvider{capture: c}, &TerminalCaptureProvider{capture: c}}
	}
	return []InputCaptureProvider{&LinuxIsolatedHelperProvider{capture: c}, &LinuxEvdevProvider{capture: c}, &TerminalCaptureProvider{capture: c}}
}

func linuxCaptureSocket() string {
	if value := strings.TrimSpace(os.Getenv("CLIKS_CAPTURE_SOCKET")); value != "" {
		return value
	}
	return "/run/cliks/capture.sock"
}

func (c *ActivityCapture) startLinuxCaptureHelper(ctx context.Context, sharing SharingConfig) CaptureState {
	conn, err := net.DialTimeout("unix", linuxCaptureSocket(), 1200*time.Millisecond)
	if err != nil {
		return CaptureState{Mode: "off", PermissionHint: "Isolated Linux capture is not ready. Run cliks setup. Direct /dev/input access is available only as the explicitly less-safe fallback: cliks set capture.mode direct"}
	}
	go func() {
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			switch scanner.Text() {
			case "k":
				if sharing.Keyboard {
					c.emit(LocalActivityEvent{Kind: "keyboard", At: time.Now()})
				}
			case "l":
				if sharing.Mouse {
					c.emit(LocalActivityEvent{Kind: "mouse", Button: "left", At: time.Now()})
				}
			case "r":
				if sharing.Mouse {
					c.emit(LocalActivityEvent{Kind: "mouse", Button: "right", At: time.Now()})
				}
			}
		}
	}()
	return CaptureState{Mode: "linux-isolated-helper", PermissionHint: "Isolated helper sends only keyboard, left-click, and right-click activity kinds."}
}

package main

import (
	"context"
)

// HardwareDriverState represents the operational health of a hardware provider.
type HardwareDriverState string

const (
	DriverStateActive   HardwareDriverState = "active"
	DriverStateDegraded HardwareDriverState = "degraded"
	DriverStateFailed   HardwareDriverState = "failed"
	DriverStateOff      HardwareDriverState = "off"
)

// ProbeResult holds the output of dynamic capability probing for a hardware component.
type ProbeResult struct {
	Name           string              `json:"name"`
	Available      bool                `json:"available"`
	State          HardwareDriverState `json:"state"`
	Mode           string              `json:"mode"`
	PermissionHint string              `json:"permissionHint,omitempty"`
	Err            error               `json:"-"`
}

// AudioCapabilities describes the operational capabilities of an audio backend.
type AudioCapabilities struct {
	Spatial       bool `json:"spatial"`
	DeviceRouting bool `json:"deviceRouting"`
	VolumeCapable bool `json:"volumeCapable"`
	IsNative      bool `json:"isNative"`
}

// InputCaptureProvider provides a unified contract for input capture backends.
type InputCaptureProvider interface {
	Name() string
	Probe(ctx context.Context) ProbeResult
	Start(ctx context.Context, sharing SharingConfig, events chan<- LocalActivityEvent) (CaptureState, error)
	Stop() error
}

// AudioBackendProvider provides a unified contract for audio playback engines.
type AudioBackendProvider interface {
	Name() string
	Probe(ctx context.Context) ProbeResult
	Play(ctx context.Context, job playbackJob) error
	Capabilities() AudioCapabilities
}

// HardwareSubsystemStatus holds diagnostic health and runtime fallback details for hardware subsystems.
type HardwareSubsystemStatus struct {
	InputDriver       string              `json:"inputDriver"`
	InputState        HardwareDriverState `json:"inputState"`
	InputMode         string              `json:"inputMode"`
	InputHint         string              `json:"inputHint,omitempty"`
	InputProbes       []ProbeResult       `json:"inputProbes"`
	AudioDriver       string              `json:"audioDriver"`
	AudioState       HardwareDriverState `json:"audioState"`
	AudioSpatial      bool                `json:"audioSpatial"`
	AudioRouting      bool                `json:"audioRouting"`
	AudioHint         string              `json:"audioHint,omitempty"`
	AudioProbes       []ProbeResult       `json:"audioProbes"`
	DegradedFallback  bool                `json:"degradedFallback"`
}

// probeInputProviders executes capability probing across input capture candidates for the given OS and mode.
func probeInputProviders(ctx context.Context, requestedMode string) []ProbeResult {
	capture := newActivityCapture()
	candidates := capture.getProvidersForMode(requestedMode)
	results := make([]ProbeResult, 0, len(candidates))
	for _, provider := range candidates {
		res := provider.Probe(ctx)
		results = append(results, res)
	}
	return results
}

// probeAudioProviders executes capability probing across audio backend candidates.
func probeAudioProviders(ctx context.Context, device string) []ProbeResult {
	candidates := getAudioBackendCandidates(device)
	results := make([]ProbeResult, 0, len(candidates))
	for _, candidate := range candidates {
		res := candidate.Probe(ctx)
		results = append(results, res)
	}
	return results
}

// ProbeHardwareSubsystem executes dynamic capability probing across capture and audio drivers.
func ProbeHardwareSubsystem(ctx context.Context, cfg CliksConfig) HardwareSubsystemStatus {
	status := HardwareSubsystemStatus{
		InputState:  DriverStateOff,
		AudioState:  DriverStateOff,
		InputProbes: probeInputProviders(ctx, cfg.Capture.Mode),
		AudioProbes: probeAudioProviders(ctx, cfg.Listening.AudioDevice),
	}

	// Determine active input state from probes
	for _, p := range status.InputProbes {
		if p.Available {
			status.InputDriver = p.Name
			status.InputState = p.State
			status.InputMode = p.Mode
			status.InputHint = p.PermissionHint
			break
		}
	}

	// Determine active audio state from probes
	for _, a := range status.AudioProbes {
		if a.Available {
			status.AudioDriver = a.Name
			status.AudioState = a.State
			status.AudioHint = a.PermissionHint
			break
		}
	}

	player, spatial, hint, _ := getAudioPlayerStatus(cfg.Listening.AudioDevice)
	if player != "" {
		status.AudioDriver = player
		status.AudioSpatial = spatial
		if hint != "" {
			status.AudioHint = hint
		}
	}

	if status.InputState == DriverStateDegraded || status.AudioState == DriverStateDegraded {
		status.DegradedFallback = true
	}

	return status
}

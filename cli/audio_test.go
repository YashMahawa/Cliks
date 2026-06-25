package main

import "testing"

func TestFFmpegSpatialFilterUsesMonoSampleForStereoPan(t *testing.T) {
	filter := ffmpegSpatialFilter(0.5, 0.5)
	want := "pan=stereo|c0=0.250*c0|c1=0.500*c0"
	if filter != want {
		t.Fatalf("filter = %q, want %q", filter, want)
	}
}

func TestFFmpegSpatialFilterClampsGainAndPan(t *testing.T) {
	filter := ffmpegSpatialFilter(2, -2)
	want := "pan=stereo|c0=1.000*c0|c1=0.000*c0"
	if filter != want {
		t.Fatalf("filter = %q, want %q", filter, want)
	}
}

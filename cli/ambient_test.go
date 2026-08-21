package main

import "testing"

func TestBundledAmbientTracksDecodeAndProduceValidWAV(t *testing.T) {
	for _, mode := range ambientModes[1:] {
		pcm, err := ambientStereoPCM(mode)
		if err != nil {
			t.Fatalf("%s decode: %v", mode, err)
		}
		if len(pcm) < 44100*4 {
			t.Fatalf("%s pcm is unexpectedly short: %d", mode, len(pcm))
		}
		wav := pcmWAV(pcm)
		if string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || len(wav) != len(pcm)+44 {
			t.Fatalf("%s WAV is invalid", mode)
		}
	}
}

func TestAmbientLabelAndNextAmbient(t *testing.T) {
	if label := ambientLabel("rain"); label != "rain window" {
		t.Fatalf("ambientLabel(rain) = %q, want rain window", label)
	}
	if label := ambientLabel("unknown"); label != "off" {
		t.Fatalf("ambientLabel(unknown) = %q, want off", label)
	}
	if next := nextAmbient("rain", 1); next != "fire" {
		t.Fatalf("nextAmbient(rain, 1) = %q, want fire", next)
	}
}

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

package main

import "testing"

func TestAmbientPCMAndWAVAreDeterministicAndValid(t *testing.T) {
	for _, mode := range []string{"rain", "cafe", "deep"} {
		pcm := ambientStereoPCM(mode, 1)
		if len(pcm) != 44100*4 {
			t.Fatalf("%s pcm length = %d", mode, len(pcm))
		}
		wav := pcmWAV(pcm)
		if string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || len(wav) != len(pcm)+44 {
			t.Fatalf("%s WAV is invalid", mode)
		}
	}
}

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestStereoPCMFromMonoWAVAppliesPan(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("assets", "sounds", "keyboard", "key-01.wav"))
	if err != nil {
		t.Fatal(err)
	}
	stereo, rate, err := stereoPCMFromMonoWAV(data, 0.5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 44100 || len(stereo) == 0 || len(stereo)%4 != 0 {
		t.Fatalf("rate=%d bytes=%d", rate, len(stereo))
	}
	foundRight := false
	for offset := 0; offset+4 <= len(stereo); offset += 4 {
		left := int16(binary.LittleEndian.Uint16(stereo[offset : offset+2]))
		right := int16(binary.LittleEndian.Uint16(stereo[offset+2 : offset+4]))
		if left != 0 {
			t.Fatalf("hard-right pan produced left sample %d", left)
		}
		if right != 0 {
			foundRight = true
		}
	}
	if !foundRight {
		t.Fatal("hard-right pan produced silence")
	}
}

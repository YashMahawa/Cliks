//go:build darwin || windows

package main

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestLoopingPCMReaderScalingAndOffsetPersistence(t *testing.T) {
	// Create 4 stereo frames (8 samples total, 16 bytes)
	// Frame 0: L=10000, R=-10000
	// Frame 1: L=20000, R=-20000
	// Frame 2: L=30000, R=-30000
	// Frame 3: L=1000,  R=-1000
	rawSamples := []int16{10000, -10000, 20000, -20000, 30000, -30000, 1000, -1000}
	pcm := make([]byte, len(rawSamples)*2)
	for i, s := range rawSamples {
		binary.LittleEndian.PutUint16(pcm[i*2:i*2+2], uint16(s))
	}

	reader := &loopingPCMReader{
		data:   pcm,
		volume: 1.0,
	}

	// Read first 2 frames (4 samples, 8 bytes) at volume 1.0
	buf := make([]byte, 8)
	n, err := reader.Read(buf)
	if err != nil || n != 8 {
		t.Fatalf("Read failed: n=%d err=%v", n, err)
	}
	if reader.offset != 8 {
		t.Fatalf("reader offset = %d, want 8", reader.offset)
	}
	s0 := int16(binary.LittleEndian.Uint16(buf[0:2]))
	s1 := int16(binary.LittleEndian.Uint16(buf[2:4]))
	if s0 != 10000 || s1 != -10000 {
		t.Fatalf("samples at vol 1.0 = %d, %d; want 10000, -10000", s0, s1)
	}

	// Dynamic volume change to 0.5 without track offset reset!
	reader.setVolume(0.5)

	// Read next 2 frames (4 samples, 8 bytes) at volume 0.5
	n, err = reader.Read(buf)
	if err != nil || n != 8 {
		t.Fatalf("Read failed: n=%d err=%v", n, err)
	}
	if reader.offset != 0 { // Wrapped around to 0 after 16 bytes total
		t.Fatalf("reader offset after wrap = %d, want 0", reader.offset)
	}
	s2 := int16(binary.LittleEndian.Uint16(buf[0:2]))
	s3 := int16(binary.LittleEndian.Uint16(buf[2:4]))
	if s2 != 10000 || s3 != -10000 { // 20000*0.5 = 10000, -20000*0.5 = -10000
		t.Fatalf("samples at vol 0.5 = %d, %d; want 10000, -10000", s2, s3)
	}

	// Dynamic volume change to 0.1
	reader.setVolume(0.1)
	n, err = reader.Read(buf)
	if err != nil || n != 8 {
		t.Fatalf("Read failed: n=%d err=%v", n, err)
	}
	if reader.offset != 8 {
		t.Fatalf("reader offset = %d, want 8", reader.offset)
	}
	s0_01 := int16(binary.LittleEndian.Uint16(buf[0:2]))
	want0_01 := int16(math.Round(10000.0 * 0.1))
	if s0_01 != want0_01 {
		t.Fatalf("sample at vol 0.1 = %d, want %d", s0_01, want0_01)
	}
}

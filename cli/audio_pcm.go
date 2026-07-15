package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

func stereoPCMFromMonoWAV(data []byte, gain float64, pan float64) ([]byte, int, error) {
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("not a WAV file")
	}
	format, channels, sampleRate, bits := 0, 0, 0, 0
	var mono []byte
	for offset := 12; offset+8 <= len(data); {
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		end := start + chunkSize
		if end > len(data) {
			return nil, 0, fmt.Errorf("invalid WAV chunk")
		}
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if chunkSize < 16 {
				return nil, 0, fmt.Errorf("invalid WAV format chunk")
			}
			format = int(binary.LittleEndian.Uint16(data[start : start+2]))
			channels = int(binary.LittleEndian.Uint16(data[start+2 : start+4]))
			sampleRate = int(binary.LittleEndian.Uint32(data[start+4 : start+8]))
			bits = int(binary.LittleEndian.Uint16(data[start+14 : start+16]))
		case "data":
			mono = data[start:end]
		}
		offset = end
		if chunkSize%2 != 0 {
			offset++
		}
	}
	if format != 1 || channels != 1 || bits != 16 || len(mono) == 0 || len(mono)%2 != 0 {
		return nil, 0, fmt.Errorf("built-in audio supports 16-bit mono PCM WAV files")
	}
	gain = clamp(gain, 0, 1)
	pan = clamp(pan, -1, 1)
	leftGain, rightGain := gain, gain
	if pan < 0 {
		rightGain *= 1 + pan
	} else if pan > 0 {
		leftGain *= 1 - pan
	}
	stereo := make([]byte, len(mono)*2)
	for src, dst := 0, 0; src < len(mono); src, dst = src+2, dst+4 {
		sample := float64(int16(binary.LittleEndian.Uint16(mono[src : src+2])))
		left := int16(math.Round(clamp(sample*leftGain, -32768, 32767)))
		right := int16(math.Round(clamp(sample*rightGain, -32768, 32767)))
		binary.LittleEndian.PutUint16(stereo[dst:dst+2], uint16(left))
		binary.LittleEndian.PutUint16(stereo[dst+2:dst+4], uint16(right))
	}
	return stereo, sampleRate, nil
}

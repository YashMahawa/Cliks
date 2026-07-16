package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/hajimehoshi/go-mp3"
)

//go:embed assets/ambient/*.mp3
var ambientAssets embed.FS

var ambientModes = []string{"off", "rain", "fire", "cafe", "cloud", "contemplation", "downtempo"}

func ambientMP3(mode string) ([]byte, error) {
	if mode == "off" {
		return nil, nil
	}
	data, err := ambientAssets.ReadFile("assets/ambient/" + mode + ".mp3")
	if err != nil {
		return nil, fmt.Errorf("bundled room tone %q is unavailable: %w", mode, err)
	}
	return data, nil
}

func ambientStereoPCM(mode string) ([]byte, error) {
	data, err := ambientMP3(mode)
	if err != nil {
		return nil, err
	}
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode bundled room tone %q: %w", mode, err)
	}
	if decoder.SampleRate() != 44100 {
		return nil, fmt.Errorf("bundled room tone %q uses unsupported %d Hz audio", mode, decoder.SampleRate())
	}
	pcm, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("read bundled room tone %q: %w", mode, err)
	}
	return pcm, nil
}

func pcmWAV(stereoPCM []byte) []byte {
	data := make([]byte, 44+len(stereoPCM))
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+len(stereoPCM)))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 2)
	binary.LittleEndian.PutUint32(data[24:28], 44100)
	binary.LittleEndian.PutUint32(data[28:32], 44100*4)
	binary.LittleEndian.PutUint16(data[32:34], 4)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(stereoPCM)))
	copy(data[44:], stereoPCM)
	return data
}

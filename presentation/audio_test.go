package presentation

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestStereoResamplerUsesDeterministicLinearTimestamps(t *testing.T) {
	var output []int16
	resampler := NewStereoResampler(4, 1, 8, func(left, right int16) {
		output = append(output, left, right)
	})
	resampler.Push(0, 0)
	resampler.Push(10, -10)
	resampler.Push(20, -20)
	want := []int16{0, 0, 5, -5, 10, -10, 15, -15, 20, -20}
	if !reflect.DeepEqual(output, want) {
		t.Fatalf("output=%v, want %v", output, want)
	}
}

func TestEncodePCM16WAVHeaderAndPayload(t *testing.T) {
	pcm := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	var output bytes.Buffer
	if err := EncodePCM16WAV(&output, 48000, pcm); err != nil {
		t.Fatal(err)
	}
	wav := output.Bytes()
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || string(wav[36:40]) != "data" {
		t.Fatalf("header=%q/%q/%q", wav[0:4], wav[8:12], wav[36:40])
	}
	if binary.LittleEndian.Uint32(wav[24:28]) != 48000 || binary.LittleEndian.Uint32(wav[40:44]) != uint32(len(pcm)) {
		t.Fatalf("rate=%d data=%d", binary.LittleEndian.Uint32(wav[24:28]), binary.LittleEndian.Uint32(wav[40:44]))
	}
	if !bytes.Equal(wav[44:], pcm) {
		t.Fatalf("payload=%v", wav[44:])
	}
	if err := EncodePCM16WAV(&output, 48000, pcm[:3]); err == nil {
		t.Fatal("unaligned stereo PCM accepted")
	}
}

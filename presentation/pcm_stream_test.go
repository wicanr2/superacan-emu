package presentation

import (
	"bytes"
	"testing"
)

func TestPCM16StereoStreamProvidesSamplesAndSilence(t *testing.T) {
	stream := NewPCM16StereoStream(2)
	stream.Push(0x1234, -2)
	output := make([]byte, 8)
	if n, err := stream.Read(output); err != nil || n != len(output) {
		t.Fatalf("Read() = %d, %v", n, err)
	}
	want := []byte{0x34, 0x12, 0xfe, 0xff, 0, 0, 0, 0}
	if !bytes.Equal(output, want) {
		t.Fatalf("PCM = % x, want % x", output, want)
	}
}

func TestPCM16StereoStreamDropsOldestFrameAtCapacity(t *testing.T) {
	stream := NewPCM16StereoStream(2)
	stream.Push(1, 2)
	stream.Push(3, 4)
	stream.Push(5, 6)
	output := make([]byte, 8)
	_, _ = stream.Read(output)
	want := []byte{3, 0, 4, 0, 5, 0, 6, 0}
	if !bytes.Equal(output, want) {
		t.Fatalf("PCM = % x, want % x", output, want)
	}
}

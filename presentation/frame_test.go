package presentation

import (
	"bytes"
	"image/color"
	"image/png"
	"reflect"
	"testing"
)

func TestARGBToRGBAReusesBuffer(t *testing.T) {
	buffer := make([]byte, 0, 8)
	buffer = ARGBToRGBA(buffer, []uint32{0xff12_3456, 0x8044_88cc})
	want := []byte{0x12, 0x34, 0x56, 0xff, 0x44, 0x88, 0xcc, 0x80}
	if !reflect.DeepEqual(buffer, want) || cap(buffer) != 8 {
		t.Fatalf("RGBA=% X capacity=%d", buffer, cap(buffer))
	}
}

func TestEncodePNGPreservesPixelChannels(t *testing.T) {
	var encoded bytes.Buffer
	if err := EncodePNG(&encoded, 2, 1, []uint32{0xff12_3456, 0x8044_88cc}); err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(1, 0)).(color.NRGBA)
	if got != (color.NRGBA{R: 0x44, G: 0x88, B: 0xcc, A: 0x80}) {
		t.Fatalf("pixel=%v", got)
	}
}

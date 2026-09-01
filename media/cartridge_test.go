package media

import (
	"archive/zip"
	"bytes"
	"testing"
)

func buildZIP(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, payload := range members {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestDecodeCartridgeAcceptsRawImage(t *testing.T) {
	image, err := DecodeCartridge("raw", []byte{0x12, 0x34, 0x56, 0x78})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(image.Bytes, []byte{0x34, 0x12, 0x78, 0x56}) {
		t.Fatalf("bytes=%x", image.Bytes)
	}
}

func TestDecodeCartridgeJoinsTwoPartsLowThenHigh(t *testing.T) {
	low := bytes.Repeat([]byte{0xaa, 0xbb}, twoPartLowSize/2)
	high := bytes.Repeat([]byte{0xcc, 0xdd}, twoPartHighSize/2)
	// 成員順序刻意與尺寸順序相反，確認排序依尺寸而不是檔名或壓縮順序。
	archive := buildZIP(t, map[string][]byte{"08007.1": high, "16007.0": low})

	image, err := DecodeCartridge("two-part", archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(image.Bytes) != twoPartLowSize+twoPartHighSize {
		t.Fatalf("size=%d, want %d", len(image.Bytes), twoPartLowSize+twoPartHighSize)
	}
	if image.Bytes[0] != 0xbb || image.Bytes[1] != 0xaa {
		t.Fatalf("低位部分沒有放在 $000000：%02X%02X", image.Bytes[0], image.Bytes[1])
	}
	if image.Bytes[twoPartLowSize] != 0xdd || image.Bytes[twoPartLowSize+1] != 0xcc {
		t.Fatalf("高位部分沒有接在 $200000：%02X%02X",
			image.Bytes[twoPartLowSize], image.Bytes[twoPartLowSize+1])
	}
	if image.RawSize != len(archive) {
		t.Fatalf("身分應是原始 ZIP，RawSize=%d want %d", image.RawSize, len(archive))
	}
}

func TestDecodeCartridgeRejectsUnexpectedZIPShapes(t *testing.T) {
	three := buildZIP(t, map[string][]byte{"a": {1, 2}, "b": {3, 4}, "c": {5, 6}})
	if _, err := DecodeCartridge("three", three); err == nil {
		t.Fatal("三個成員的 ZIP 必須拒絕")
	}
	wrongSizes := buildZIP(t, map[string][]byte{"a": {1, 2}, "b": {3, 4}})
	if _, err := DecodeCartridge("sizes", wrongSizes); err == nil {
		t.Fatal("尺寸不符的雙成員 ZIP 必須拒絕")
	}
}

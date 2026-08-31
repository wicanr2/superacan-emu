package media

import (
	"crypto/sha256"
	"strings"
	"testing"
)

func TestDecodeWordSwapped(t *testing.T) {
	raw := []byte{0x71, 0x4e, 0xb8, 0x4e}
	image, err := DecodeWordSwapped("ipl", raw, 4)
	if err != nil {
		t.Fatal(err)
	}
	if got := image.Bytes; len(got) != 4 || got[0] != 0x4e || got[1] != 0x71 || got[2] != 0x4e || got[3] != 0xb8 {
		t.Fatalf("decoded bytes: % X", got)
	}
	if image.RawSHA256 != sha256.Sum256(raw) || image.Transform == "" {
		t.Fatalf("manifest: %+v", image)
	}
	raw[0] = 0
	if image.Bytes[1] != 0x71 {
		t.Fatal("decoded image aliases caller input")
	}
}

func TestDecodeWordSwappedRejectsInvalidShape(t *testing.T) {
	for _, test := range []struct {
		raw  []byte
		size int
		want string
	}{
		{nil, 0, "empty"},
		{[]byte{1, 2}, 4, "size"},
		{[]byte{1, 2, 3}, 0, "odd"},
	} {
		if _, err := DecodeWordSwapped("bad", test.raw, test.size); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("raw=%v size=%d: err=%v", test.raw, test.size, err)
		}
	}
}

package session

import (
	"testing"

	"github.com/wicanr2/superacan-emu/media"
)

// decodeForTest 走的是正式路徑的解碼，測試不另寫一套規則。
func decodeForTest(t *testing.T, path string, raw []byte) []byte {
	t.Helper()
	image, err := media.DecodeCartridge(path, raw)
	if err != nil {
		t.Fatal(err)
	}
	return image.Bytes
}

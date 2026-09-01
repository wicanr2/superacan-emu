package capture

import (
	"bytes"
	"image/jpeg"
)

// decodeJPEG 只是把 image/jpeg 的解碼包起來，供測試確認寫出去的真的是 JPEG。
func decodeJPEG(raw []byte) error {
	_, err := jpeg.Decode(bytes.NewReader(raw))
	return err
}

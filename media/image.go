package media

import (
	"crypto/sha256"
	"fmt"
)

// Image records both the supplied dump identity and its CPU-visible bytes.
// Super A'Can ROM dumps store every 16-bit word with its two bytes swapped.
type Image struct {
	Name      string
	RawSize   int
	RawSHA256 [32]byte
	Bytes     []byte
	Transform string
}

// DecodeWordSwapped validates and converts a ROM dump without retaining the
// caller's buffer. expectedSize == 0 accepts any non-empty even size.
func DecodeWordSwapped(name string, raw []byte, expectedSize int) (Image, error) {
	if len(raw) == 0 {
		return Image{}, fmt.Errorf("media %q: empty image", name)
	}
	if expectedSize != 0 && len(raw) != expectedSize {
		return Image{}, fmt.Errorf("media %q: size %d, want %d", name, len(raw), expectedSize)
	}
	if len(raw)&1 != 0 {
		return Image{}, fmt.Errorf("media %q: odd size %d cannot be word-swapped", name, len(raw))
	}

	decoded := make([]byte, len(raw))
	for i := 0; i < len(raw); i += 2 {
		decoded[i], decoded[i+1] = raw[i+1], raw[i]
	}
	return Image{
		Name: name, RawSize: len(raw), RawSHA256: sha256.Sum256(raw),
		Bytes: decoded, Transform: "swap-bytes-in-each-16-bit-word",
	}, nil
}

// DecodeLinear validates a byte-oriented dump such as the UMC6650 key.
func DecodeLinear(name string, raw []byte, expectedSize int) (Image, error) {
	if len(raw) != expectedSize {
		return Image{}, fmt.Errorf("media %q: size %d, want %d", name, len(raw), expectedSize)
	}
	return Image{
		Name: name, RawSize: len(raw), RawSHA256: sha256.Sum256(raw),
		Bytes: append([]byte(nil), raw...), Transform: "none",
	}, nil
}

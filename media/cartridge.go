package media

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

// 雙部分卡帶的固定尺寸。目前唯一已知的雙部分卡帶是 F007 Super Light Saga：
// 2 MiB 的部分放在 $000000，1 MiB 的部分接在 $200000，合計 3 MiB，沒有 mapper。
// 來源：acan 知識庫 emulator-analysis.md §4.5（Bcan 內建白名單與 CRC 驗證）。
const (
	twoPartLowSize  = 2 << 20
	twoPartHighSize = 1 << 20
)

// zipBombLimit 限制單一成員解壓後的大小。卡帶最大就是 3 MiB，留兩倍餘裕即可。
const zipBombLimit = 8 << 20

// DecodeCartridge 讀入卡帶映像。raw 是 ZIP 時依成員數決定單一或雙部分卡帶，
// 其餘一律視為 raw 卡帶。回傳的 Image 記錄的是「餵進來的位元組」的雜湊，
// ZIP 的情況下就是整個壓縮檔的雜湊，可直接回查使用者手上的檔案。
func DecodeCartridge(name string, raw []byte) (Image, error) {
	if !isZIP(raw) {
		return DecodeWordSwapped(name, raw, 0)
	}
	combined, transform, err := extractCartridgeZIP(name, raw)
	if err != nil {
		return Image{}, err
	}
	image, err := DecodeWordSwapped(name, combined, 0)
	if err != nil {
		return Image{}, err
	}
	// 對外的身分是原始 ZIP，不是解出來的內容。
	image.RawSize = len(raw)
	image.RawSHA256 = sha256.Sum256(raw)
	image.Transform = transform + "+" + image.Transform
	return image, nil
}

func isZIP(raw []byte) bool {
	return len(raw) >= 4 && raw[0] == 'P' && raw[1] == 'K' && raw[2] == 3 && raw[3] == 4
}

func extractCartridgeZIP(name string, raw []byte) ([]byte, string, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, "", fmt.Errorf("media %q: read zip: %w", name, err)
	}
	var members []*zip.File
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		members = append(members, file)
	}

	switch len(members) {
	case 1:
		payload, err := readMember(name, members[0])
		if err != nil {
			return nil, "", err
		}
		return payload, "zip-single-part", nil
	case 2:
		low, high, err := orderTwoPartMembers(name, members)
		if err != nil {
			return nil, "", err
		}
		combined := make([]byte, 0, len(low)+len(high))
		combined = append(combined, low...)
		combined = append(combined, high...)
		return combined, "zip-two-part-low-then-high", nil
	default:
		return nil, "", fmt.Errorf("media %q: zip has %d cartridge members, want 1 or 2", name, len(members))
	}
}

// orderTwoPartMembers 依尺寸決定順序，不依檔名。已知的雙部分 ZIP 在流通版本裡
// 檔名被改過（本機的 "Super Dragon Force" ZIP 其實是 Super Light Saga 的兩個部分），
// 所以檔名不是可靠的排序依據；尺寸則由 Bcan 的驗證規則固定下來。
func orderTwoPartMembers(name string, members []*zip.File) ([]byte, []byte, error) {
	first, err := readMember(name, members[0])
	if err != nil {
		return nil, nil, err
	}
	second, err := readMember(name, members[1])
	if err != nil {
		return nil, nil, err
	}
	low, high := first, second
	if len(low) != twoPartLowSize {
		low, high = second, first
	}
	if len(low) != twoPartLowSize || len(high) != twoPartHighSize {
		return nil, nil, fmt.Errorf(
			"media %q: two-part cartridge members are %d and %d bytes, want %d and %d",
			name, len(first), len(second), twoPartLowSize, twoPartHighSize)
	}
	return low, high, nil
}

func readMember(name string, file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > zipBombLimit {
		return nil, fmt.Errorf("media %q: member %q claims %d bytes, over the %d limit",
			name, file.Name, file.UncompressedSize64, zipBombLimit)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("media %q: open member %q: %w", name, file.Name, err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, zipBombLimit+1))
	if err != nil {
		return nil, fmt.Errorf("media %q: read member %q: %w", name, file.Name, err)
	}
	if len(payload) > zipBombLimit {
		return nil, fmt.Errorf("media %q: member %q expands past the %d limit", name, file.Name, zipBombLimit)
	}
	return payload, nil
}

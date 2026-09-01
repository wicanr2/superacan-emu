package ui

import "testing"

// PokeWorkRAM 是 UI 唯一的寫入通道，範圍檢查必須在型別上就成立，
// 不能寄望呼叫端記得檢查。
func TestPokeWorkRAMRange(t *testing.T) {
	for _, c := range []struct {
		addr  uint32
		width uint8
		want  bool
	}{
		{0xfc0000, 1, true},
		{0xfcfffe, 2, true},
		{0xfcffff, 1, true},
		{0xfcffff, 2, false},
		{0xfbffff, 1, false},
		{0xfd0000, 1, false},
		{0xfc0000, 3, false},
		{0x000000, 1, false},
	} {
		if got := (PokeWorkRAM{Addr: c.addr, Width: c.width, Value: 1}).Valid(); got != c.want {
			t.Errorf("$%06X 寬度 %d：得到 %v，want %v", c.addr, c.width, got, c.want)
		}
	}
}

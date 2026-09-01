package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/superacan-emu/cheat"
	"github.com/wicanr2/superacan-emu/ui"
)

// 越界的 PokeWorkRAM 由入口拒絕，而且不得寫入任何位元組。
func TestPokeRejectsOutOfRangeAddresses(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)
	for _, address := range []uint32{0xf40000, 0xfbffff, 0xfd0000} {
		before, err := s.System.Bus.Read8(address)
		if err != nil {
			before = 0
		}
		if err := s.apply(ui.PokeWorkRAM{Addr: address, Width: 1, Value: 0x5a}); err == nil {
			t.Fatalf("$%06X 必須被拒絕", address)
		}
		after, err := s.System.Bus.Read8(address)
		if err == nil && after != before {
			t.Fatalf("$%06X 被拒絕之後不得改變記憶體", address)
		}
	}
	if s.PokeCount() != 0 {
		t.Fatalf("被拒絕的寫入不該計數，得到 %d", s.PokeCount())
	}
}

// 金手指關閉時，UI 通道不存在任何寫入。這是 C10 的前提：回歸基準必須在金手指
// 關閉下取得。
func TestNoPokesWhenCheatsAreDisabled(t *testing.T) {
	s := newTestSession(t)
	s.cheat.entries = []cheat.Entry{
		{Name: "test", Address: 0xfc1a20, Width: 16, Value: 99, Locked: true},
	}
	s.cheat.enabled = false
	advance(t, s, 30)
	if s.PokeCount() != 0 {
		t.Fatalf("金手指關閉時不得寫入，得到 %d 次", s.PokeCount())
	}
	if s.Cheats().Wrote {
		t.Fatal("沒寫過就不該標記寫過")
	}
}

// 啟用之後鎖定項每個 frame 邊界寫入一次，而且標記會亮起來。
func TestLockedCheatsWriteOncePerFrame(t *testing.T) {
	s := newTestSession(t)
	s.cheat.entries = []cheat.Entry{
		{Name: "test", Address: 0xfc1a20, Width: 16, Value: 0x1234, Locked: true},
	}
	if err := s.apply(ui.Cheat{Command: ui.CheatSetEnabled, Flag: true}); err != nil {
		t.Fatal(err)
	}
	advance(t, s, 5)
	if s.PokeCount() != 5 {
		t.Fatalf("五個 frame 應該寫五次，得到 %d", s.PokeCount())
	}
	value, err := s.System.Bus.Read16(0xfc1a20)
	if err != nil || value != 0x1234 {
		t.Fatalf("寫入結果 $%04X err=%v", value, err)
	}
	state := s.Cheats()
	if !state.Wrote || !state.Enabled {
		t.Fatalf("標記狀態 %+v", state)
	}
}

// 搜尋在 frame 邊界取快照，並且能一路縮小到單一位址。
func TestSearchThroughSessionNarrowsToOneAddress(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)
	if err := s.System.Bus.Write16(0xfc1a20, 99); err != nil {
		t.Fatal(err)
	}
	if err := s.System.Bus.Write16(0xfc3e88, 99); err != nil {
		t.Fatal(err)
	}
	if err := s.apply(ui.Cheat{Command: ui.CheatNewSearch, Width: 16,
		Compare: uint8(cheat.CompareEqual), Value: 99}); err != nil {
		t.Fatal(err)
	}
	first := len(s.cheat.search.Candidates())
	if first < 2 {
		t.Fatalf("第一次搜尋只找到 %d 個候選", first)
	}

	if err := s.System.Bus.Write16(0xfc3e88, 98); err != nil {
		t.Fatal(err)
	}
	if err := s.apply(ui.Cheat{Command: ui.CheatRefine, Width: 16,
		Compare: uint8(cheat.CompareEqual), Value: 99}); err != nil {
		t.Fatal(err)
	}
	for _, address := range s.cheat.search.Candidates() {
		if address == 0xfc3e88 {
			t.Fatal("值已改變的位址應該被縮掉")
		}
	}
	if len(s.cheat.search.Candidates()) >= first {
		t.Fatal("縮小之後候選應該變少")
	}
}

// 匯入 Bcan 的 .cht：項目數與內容要與來源相同。
func TestImportBcanCheatFile(t *testing.T) {
	s := newTestSession(t)
	path := filepath.Join(t.TempDir(), "game.cht")
	document := "; Bcan per-game cheat file\nBCAN_CHT_1\nLives\t$FC1A20\t16\t99\tdec\nMoney\t$FC3E88\t16\t9999\tbcd\n"
	if err := os.WriteFile(path, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.apply(ui.Cheat{Command: ui.CheatImport, Path: path}); err != nil {
		t.Fatal(err)
	}
	if len(s.cheat.entries) != 2 {
		t.Fatalf("匯入 %d 筆", len(s.cheat.entries))
	}
	if s.cheat.entries[0].Name != "Lives" || s.cheat.entries[0].Value != 99 {
		t.Fatalf("第一筆 %+v", s.cheat.entries[0])
	}

	// 匯出再匯入要一樣。
	exported := filepath.Join(t.TempDir(), "out.cht")
	if err := s.apply(ui.Cheat{Command: ui.CheatExport, Path: exported}); err != nil {
		t.Fatal(err)
	}
	other := newTestSession(t)
	if err := other.apply(ui.Cheat{Command: ui.CheatImport, Path: exported}); err != nil {
		t.Fatal(err)
	}
	if len(other.cheat.entries) != len(s.cheat.entries) {
		t.Fatalf("往返之後 %d 筆", len(other.cheat.entries))
	}
	for i := range s.cheat.entries {
		if other.cheat.entries[i] != s.cheat.entries[i] {
			t.Fatalf("第 %d 筆 %+v，want %+v", i, other.cheat.entries[i], s.cheat.entries[i])
		}
	}
}

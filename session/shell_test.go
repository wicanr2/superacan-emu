package session

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

// writeTestCartridges 造一個卡帶目錄：一個 raw 檔與一個雙部分 ZIP。內容是自製的
// 位元組，不是商業 ROM，所以這條測試在任何機器上都能跑。
func writeTestCartridges(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	raw := make([]byte, 0x10000)
	for i := range raw {
		raw[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.bin"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	// 雙部分卡帶依尺寸排序：2 MiB 在前、1 MiB 在後。
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, part := range []struct {
		name string
		size int
	}{{"second.bin", 1 << 20}, {"first.bin", 2 << 20}} {
		entry, err := writer.Create(part.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(make([]byte, part.size)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.zip"), archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func testLoader(t *testing.T) func(string) (*machine.System, string, error) {
	t.Helper()
	ipl := make([]byte, machine.IPLSize)
	ipl[0], ipl[1], ipl[2], ipl[3] = 0x00, 0x00, 0x10, 0x00
	ipl[4], ipl[5], ipl[6], ipl[7] = 0x00, 0x00, 0x04, 0x00
	ipl[0x400], ipl[0x401] = 0x60, 0xfe
	key := make([]byte, 16)
	return func(path string) (*machine.System, string, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, "", err
		}
		system, err := machine.NewSystem(ipl, decodeForTest(t, path, raw), key)
		if err != nil {
			return nil, "", err
		}
		if err := system.Reset(); err != nil {
			return nil, "", err
		}
		return system, TitleFromPath(path), nil
	}
}

// 卡帶瀏覽器要列出 raw 與 ZIP 兩種，而且 ZIP 展開成子項但只有整包可選。
func TestLibraryListsRawAndZip(t *testing.T) {
	dir := writeTestCartridges(t)
	library := NewLibrary([]string{dir}, nil, "", "")
	entries := library.Cartridges()
	if len(entries) != 2 {
		t.Fatalf("得到 %d 筆", len(entries))
	}
	if entries[0].Name != "alpha" || entries[0].Kind != "bin" || len(entries[0].Parts) != 0 {
		t.Fatalf("raw 卡帶 %+v", entries[0])
	}
	if entries[1].Name != "beta" || entries[1].Kind != "zip" || len(entries[1].Parts) != 2 {
		t.Fatalf("ZIP 卡帶 %+v", entries[1])
	}
	// 自製卡帶當然不在已驗證清單裡，那正是「未驗證」要顯示的情況。
	if entries[0].Verified || entries[1].Verified {
		t.Fatal("自製卡帶不應被標成已驗證")
	}
}

// 沒有卡帶時介面停在啟動畫面，走完瀏覽器就能載入——整條路在 headless 跑完。
func TestShellBrowsesAndLoadsHeadless(t *testing.T) {
	dir := writeTestCartridges(t)
	library := NewLibrary([]string{dir}, nil, t.TempDir(), "")
	firmware := FirmwareSet{
		{Kind: ui.FirmwareIPL, Loaded: true},
		{Kind: ui.FirmwareKey, Loaded: true},
		{Kind: ui.FirmwareSoundA, Loaded: true},
		{Kind: ui.FirmwareSoundB, Loaded: true},
	}
	s := New(Options{
		Surface:     ui.Surface{W: 960, H: 720, Scale: 1, Profile: ui.ProfileCompact},
		Config:      ui.DefaultConfig(),
		Library:     library,
		FirmwareSet: firmware,
	})
	s.Loader = testLoader(t)
	s.StateRoot = t.TempDir()

	if s.UI.Mode() != ui.ModeShell {
		t.Fatal("沒有卡帶時應停在啟動畫面")
	}
	if _, err := s.Advance(0); err != nil {
		t.Fatal(err)
	}
	if s.System != nil {
		t.Fatal("啟動畫面不應該有機器在跑")
	}

	// 啟動畫面：設定主機韌體(0) → 選擇卡帶(1)。
	s.Handle(ui.Nav{Dir: ui.DirDown})
	s.Handle(ui.Action{Kind: ui.ActConfirm})
	s.Handle(ui.Action{Kind: ui.ActConfirm}) // 瀏覽器第一筆 alpha.bin
	if _, err := s.Advance(time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if s.System == nil || s.UI.Mode() != ui.ModeGame {
		t.Fatalf("載入之後應該進入遊戲：system=%v mode=%v", s.System != nil, s.UI.Mode())
	}
	if s.Title != "ALPHA" {
		t.Fatalf("標題 %q", s.Title)
	}

	// 載入之後模擬時間要真的前進。
	before := s.System.Instructions
	advanced, err := s.Advance(2 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !advanced || s.System.Instructions <= before {
		t.Fatal("載入之後模擬時間應該前進")
	}

	// 退出卡帶回到啟動畫面。
	s.UI.Handle(ui.Action{Kind: ui.ActMenu})
	if _, err := s.Advance(3 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	s.apply(ui.UnloadCartridge{})
	if s.System != nil || s.UI.Mode() != ui.ModeShell {
		t.Fatal("退出卡帶應該回到啟動畫面")
	}
}

// 韌體不齊時瀏覽器不得載入卡帶。
func TestIncompleteFirmwareBlocksBrowserLoad(t *testing.T) {
	dir := writeTestCartridges(t)
	firmware := FirmwareSet{
		{Kind: ui.FirmwareIPL, Loaded: true},
		{Kind: ui.FirmwareKey, Loaded: true},
		{Kind: ui.FirmwareSoundA, Loaded: true},
		{Kind: ui.FirmwareSoundB}, // 缺這一份
	}
	s := New(Options{
		Surface:     ui.Surface{W: 960, H: 720, Scale: 1, Profile: ui.ProfileCompact},
		Config:      ui.DefaultConfig(),
		Library:     NewLibrary([]string{dir}, nil, "", ""),
		FirmwareSet: firmware,
	})
	s.Loader = testLoader(t)

	s.Handle(ui.Nav{Dir: ui.DirDown})
	s.Handle(ui.Action{Kind: ui.ActConfirm})
	if _, err := s.Advance(0); err != nil {
		t.Fatal(err)
	}
	if s.System != nil {
		t.Fatal("韌體不齊時不得載入卡帶")
	}
}

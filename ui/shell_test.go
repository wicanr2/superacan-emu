package ui

import (
	"testing"
)

// fixedFirmware 是固定的韌體現況，讓 S0 與 S0.1 的畫面可重現。
type fixedFirmware struct{ complete bool }

func (f fixedFirmware) FirmwareEntries() []FirmwareEntry {
	sum := func(seed byte) [32]byte {
		var out [32]byte
		for i := range out {
			out[i] = seed + byte(i)
		}
		return out
	}
	entries := []FirmwareEntry{
		{Kind: FirmwareIPL, Path: "/media/acan/bios/internal_68k.bin", Size: 4096, SHA256: sum(0x2e), Loaded: true, Known: true},
		{Kind: FirmwareKey, Path: "/media/acan/bios/umc6650.bin", Size: 16, SHA256: sum(0xf1), Loaded: true, Known: true},
		{Kind: FirmwareSoundA, Path: "/media/acan/bios/internal_6502_1.bin", Size: 8192, SHA256: sum(0x21), Loaded: true, Known: false},
		{Kind: FirmwareSoundB},
	}
	if f.complete {
		entries[3] = FirmwareEntry{
			Kind: FirmwareSoundB, Path: "/media/acan/bios/internal_6502_2.bin",
			Size: 8192, SHA256: sum(0x98), Loaded: true, Known: true,
		}
	}
	return entries
}

// fixedLibrary 是固定的卡帶清單，含一個雙部分 ZIP 與一個已消失的最近項目。
type fixedLibrary struct{}

func (fixedLibrary) Directory() string { return "/media/acan/roms" }

func (fixedLibrary) Cartridges() []CartridgeEntry {
	sum := func(seed byte) [32]byte {
		var out [32]byte
		for i := range out {
			out[i] = seed ^ byte(i)
		}
		return out
	}
	return []CartridgeEntry{
		{Name: "Boom Zoo", Path: "/media/acan/roms/boomzoo.bin", Size: 4 << 20, SHA256: sum(0x09),
			Kind: "bin", Verified: true, Battery: 32768, SaveSlots: []int{0, 3}},
		{Name: "Formosa Duel", Path: "/media/acan/roms/formosa.bin", Size: 4 << 20, SHA256: sum(0xd6),
			Kind: "bin", Verified: true},
		{Name: "Super Dragon Force", Path: "/media/acan/roms/sdf.zip", Size: 3 << 20, SHA256: sum(0x1d),
			Kind: "zip", Verified: true, Parts: []CartridgePart{
				{Name: "part1.bin", Size: 2 << 20}, {Name: "part2.bin", Size: 1 << 20},
			}},
		{Name: "Unknown Dump", Path: "/media/acan/roms/unknown.bin", Size: 1 << 20, SHA256: sum(0x77), Kind: "bin"},
	}
}

func (l fixedLibrary) Recent() []CartridgeEntry {
	all := l.Cartridges()
	recent := []CartridgeEntry{all[0], all[1]}
	recent = append(recent, CartridgeEntry{Name: "Sango Fighter", Path: "/gone/sango.bin", Missing: true})
	return recent
}

var fixedAbout = AboutInfo{
	Version: "0.1.0", BuildDate: "2026-09-01", GoVersion: "go1.26.7",
	Platform: "linux/amd64", CGOEnabled: false,
	Dependencies: []Dependency{
		{Path: "github.com/ebitengine/purego", Version: "v0.9.0", License: "Apache-2.0"},
		{Path: "github.com/hajimehoshi/bitmapfont/v4", Version: "v4.1.0", License: "Apache-2.0"},
		{Path: "github.com/jezek/xgb", Version: "v1.1.1", License: "BSD-3-Clause"},
		{Path: "golang.org/x/image", Version: "v0.31.0", License: "BSD-3-Clause"},
	},
}

func newShellUI(surface Surface, complete bool) *UI {
	u := New(Options{
		Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: complete}, About: fixedAbout,
	})
	u.Update(0)
	u.SetMode(ModeShell, "")
	return u
}

func TestStartScreenRenders(t *testing.T) {
	for _, c := range surfaceCases {
		u := newShellUI(c.surface, false)
		checkHash(t, "S0/"+c.name, render(t, "S0/"+c.name, u, c.surface))
	}
	u := newShellUI(surfaceCases[0].surface, true)
	checkHash(t, "S0ready/"+surfaceCases[0].name,
		render(t, "S0ready/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

// 韌體不齊時不得產生載入卡帶的 Intent，而且要說明原因。
func TestIncompleteFirmwareBlocksLoading(t *testing.T) {
	u := newShellUI(surfaceCases[0].surface, false)
	start := u.stack[0].(*startScreen)
	rows := start.rows(u)
	for index, row := range rows {
		if row.label != textChooseCartridge {
			continue
		}
		if !row.disabled {
			t.Fatal("韌體不齊時「選擇卡帶」必須停用")
		}
		start.focus = index
	}
	u.Handle(Action{Kind: ActConfirm})
	if intents := u.TakeIntents(); len(intents) != 0 {
		t.Fatalf("停用項不得產生 Intent，得到 %#v", intents)
	}
	if len(u.toasts) != 1 {
		t.Fatalf("停用項要說明原因，得到 %+v", u.toasts)
	}

	// 最近清單同樣被擋住。
	for index, row := range rows {
		if row.label == "Boom Zoo" {
			start.focus = index
		}
	}
	u.toasts = nil
	u.Handle(Action{Kind: ActConfirm})
	if intents := u.TakeIntents(); len(intents) != 0 {
		t.Fatalf("韌體不齊時最近清單不得載入，得到 %#v", intents)
	}
}

// 韌體齊備之後最近清單可以載入，缺檔的那一筆仍然停用。
func TestCompleteFirmwareAllowsRecentLoad(t *testing.T) {
	u := newShellUI(surfaceCases[0].surface, true)
	start := u.stack[0].(*startScreen)
	rows := start.rows(u)
	for index, row := range rows {
		switch row.label {
		case "Boom Zoo":
			start.focus = index
		case "Sango Fighter":
			if !row.disabled {
				t.Fatal("檔案已消失的最近項目必須停用")
			}
		}
	}
	u.Handle(Action{Kind: ActConfirm})
	intents := u.TakeIntents()
	if len(intents) != 1 {
		t.Fatalf("得到 %#v", intents)
	}
	load, ok := intents[0].(LoadCartridge)
	if !ok || load.Path != "/media/acan/roms/boomzoo.bin" {
		t.Fatalf("得到 %#v", intents[0])
	}
}

func TestFirmwareScreenRenders(t *testing.T) {
	u := newShellUI(surfaceCases[0].surface, false)
	u.push(&firmwareScreen{})
	checkHash(t, "S0.1/"+surfaceCases[0].name,
		render(t, "S0.1/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

func TestBrowserRenders(t *testing.T) {
	for _, c := range surfaceCases {
		u := newShellUI(c.surface, true)
		u.push(&browserScreen{})
		checkHash(t, "S1/"+c.name, render(t, "S1/"+c.name, u, c.surface))
	}
}

func TestAboutRenders(t *testing.T) {
	u := newShellUI(surfaceCases[0].surface, true)
	u.push(&aboutScreen{})
	checkHash(t, "S8/"+surfaceCases[0].name,
		render(t, "S8/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

func TestHaltScreenRendersAndCannotBeDismissed(t *testing.T) {
	c := surfaceCases[0]
	u := New(Options{
		Surface: c.surface, Config: DefaultConfig(), Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}, About: fixedAbout,
	})
	u.Update(0)
	u.SetMode(ModeHalt, "68000 在 PC=$00D06A 遇到未實作的編碼 $D06A。")
	checkHash(t, "S9/"+c.name, render(t, "S9/"+c.name, u, c.surface))

	u.Handle(Life{Kind: LifeBack})
	u.Handle(Action{Kind: ActCancel})
	if !u.Visible() || u.stack[0].id() != "S9" {
		t.Fatal("停機畫面不能被返回鍵或取消略過")
	}
}

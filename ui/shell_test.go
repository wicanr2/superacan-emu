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

func newSettingsUI(t *testing.T, mutate func(*Config)) *UI {
	t.Helper()
	config := DefaultConfig()
	config.Input.Players[0].Keyboard = map[string]Binding{
		"up": {Frontend: "x11", Code: 0xff52, Label: "ArrowUp"},
		"a":  {Frontend: "x11", Code: 0x7a, Label: "z"},
		"b":  {Frontend: "x11", Code: 0x78, Label: "x"},
	}
	config.Input.Players[0].Gamepad = map[string]Binding{
		"a": {Frontend: "pad", Code: 0, Label: "Button 0"},
	}
	config.Input.Hotkeys = map[string]Binding{
		"menu":       {Frontend: "x11", Code: 0xffbe, Label: "F1"},
		"save_state": {Frontend: "x11", Code: 0xffc2, Label: "F5"},
	}
	if mutate != nil {
		mutate(&config)
	}
	u := New(Options{
		Surface: surfaceCases[0].surface, Config: config, Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}, About: fixedAbout,
	})
	u.Update(0)
	return u
}

func TestSettingsScreensRender(t *testing.T) {
	u := newSettingsUI(t, nil)
	u.Open()
	u.push(&settingsScreen{})
	checkHash(t, "S5/"+surfaceCases[0].name, render(t, "S5/"+surfaceCases[0].name, u, surfaceCases[0].surface))

	u = newSettingsUI(t, nil)
	u.Open()
	u.push(&bindingScreen{})
	checkHash(t, "S5.1/"+surfaceCases[0].name, render(t, "S5.1/"+surfaceCases[0].name, u, surfaceCases[0].surface))

	u = newSettingsUI(t, nil)
	u.Open()
	u.push(&hotkeyScreen{})
	checkHash(t, "S5.2/"+surfaceCases[0].name, render(t, "S5.2/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

// 衝突要在畫面上看得到：同一個鍵綁兩個動作不是錯誤，但使用者一定要知道，
// 否則會以為某個按鈕壞了。
func TestConflictIsVisible(t *testing.T) {
	u := newSettingsUI(t, func(config *Config) {
		// 「上一個存檔槽」指到與「存檔到目前槽」相同的鍵。
		config.Input.Hotkeys["prev_slot"] = Binding{Frontend: "x11", Code: 0xffc2, Label: "F5"}
	})
	u.Open()
	u.push(&hotkeyScreen{})
	screen := u.stack[len(u.stack)-1].(*hotkeyScreen)
	conflicts := conflictsIn(screen.rows(u), func(r bindingRow) Binding { return r.keyboard })
	if len(conflicts) != 2 {
		t.Fatalf("兩列都要標出衝突，得到 %v", conflicts)
	}
	checkHash(t, "S5.2conflict/"+surfaceCases[0].name,
		render(t, "S5.2conflict/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

// 指定綁定：進入等待狀態，收到 RawKey 才寫入，並要求入口存檔。
func TestBindingCaptureWritesConfig(t *testing.T) {
	u := newSettingsUI(t, nil)
	u.Open()
	u.push(&bindingScreen{})
	u.TakeIntents()
	screen := u.stack[len(u.stack)-1].(*bindingScreen)
	screen.focus = 4 // "a"

	u.Handle(Action{Kind: ActConfirm})
	if !screen.waiting {
		t.Fatal("按下確認應該進入等待輸入")
	}
	// 等待中不得被導覽事件干擾。
	u.Handle(Nav{Dir: DirDown})
	if screen.focus != 4 {
		t.Fatal("等待輸入時焦點不該移動")
	}
	u.Handle(RawKey{Frontend: "x11", Code: 0x71, Label: "q"})
	if screen.waiting {
		t.Fatal("收到按鍵之後應該結束等待")
	}
	if got := u.config.Input.Players[0].Keyboard["a"]; got.Code != 0x71 || got.Frontend != "x11" {
		t.Fatalf("綁定 %+v", got)
	}
	intents := u.TakeIntents()
	if len(intents) != 1 {
		t.Fatalf("應該要求入口存檔，得到 %#v", intents)
	}
	if _, ok := intents[0].(ApplyConfig); !ok {
		t.Fatalf("得到 %#v", intents[0])
	}

	// Esc 取消等待，不改變原綁定。
	screen.focus = 5 // "b"
	u.Handle(Action{Kind: ActConfirm})
	u.Handle(Action{Kind: ActCancel})
	if screen.waiting {
		t.Fatal("Esc 應該取消等待")
	}
	if got := u.config.Input.Players[0].Keyboard["b"]; got.Code != 0x78 {
		t.Fatalf("取消之後綁定不該變：%+v", got)
	}
}

// Del 清除該列的兩組綁定。
func TestDeleteClearsBothBindings(t *testing.T) {
	u := newSettingsUI(t, nil)
	u.Open()
	u.push(&bindingScreen{})
	screen := u.stack[len(u.stack)-1].(*bindingScreen)
	screen.focus = 4 // "a"
	u.Handle(Action{Kind: ActDelete})
	if _, ok := u.config.Input.Players[0].Keyboard["a"]; ok {
		t.Fatal("鍵盤綁定應該被清除")
	}
	if _, ok := u.config.Input.Players[0].Gamepad["a"]; ok {
		t.Fatal("手把綁定應該被清除")
	}
}

// fixedAudioStats 固定播放端的數字，讓 S5.4 的畫面可重現。
type fixedAudioStats struct{}

func (fixedAudioStats) AudioStats() AudioStats {
	return AudioStats{BufferedMS: 96, BufferMS: 200, Underruns: 0}
}

// fixedDiagnostics 固定診斷數字。
type fixedDiagnostics struct{}

func (fixedDiagnostics) Diagnostics() DiagnosticsFacts {
	sum := func(seed byte) [32]byte {
		var out [32]byte
		for i := range out {
			out[i] = seed + byte(i)
		}
		return out
	}
	return DiagnosticsFacts{
		Frame: 12480, M68K: 17369003, M65C02: 5790112,
		IRQ7: 12480, IRQ4: 0, IRQ5: 0, SoundClash: 0,
		HostFPS: 60, Pacing: true, Frontend: "x11", Platform: "linux/amd64",
		CGOEnabled: false, IPL: sum(0x2e), Cartridge: sum(0x09),
	}
}

func newDisplayUI(t *testing.T) *UI {
	t.Helper()
	u := New(Options{
		Surface: surfaceCases[0].surface, Config: DefaultConfig(), Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}, About: fixedAbout,
		AudioStats: fixedAudioStats{}, Diagnostics: fixedDiagnostics{},
	})
	u.Update(0)
	u.Open()
	return u
}

func TestDisplayScreensRender(t *testing.T) {
	u := newDisplayUI(t)
	u.push(&videoScreen{})
	checkHash(t, "S5.3/"+surfaceCases[0].name, render(t, "S5.3/"+surfaceCases[0].name, u, surfaceCases[0].surface))

	u = newDisplayUI(t)
	u.push(&audioScreen{})
	checkHash(t, "S5.4/"+surfaceCases[0].name, render(t, "S5.4/"+surfaceCases[0].name, u, surfaceCases[0].surface))

	u = newDisplayUI(t)
	u.push(&diagnosticsScreen{})
	checkHash(t, "S7/"+surfaceCases[0].name, render(t, "S7/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

// 左右鍵調值而不是移動焦點；停用項按下去要說明原因。
func TestOptionRowsAdjustWithLeftRight(t *testing.T) {
	u := newDisplayUI(t)
	u.push(&videoScreen{})
	u.TakeIntents()
	screen := u.stack[len(u.stack)-1].(*videoScreen)

	before := u.config.Video.Scale
	u.Handle(Nav{Dir: DirRight})
	if u.config.Video.Scale != before+1 {
		t.Fatalf("縮放 %d，want %d", u.config.Video.Scale, before+1)
	}
	if screen.focus != 0 {
		t.Fatal("左右鍵不該移動焦點")
	}
	intents := u.TakeIntents()
	if len(intents) != 1 {
		t.Fatalf("調整之後要求存檔，得到 %#v", intents)
	}

	// 上限之後不再增加。
	for i := 0; i < 20; i++ {
		u.Handle(Nav{Dir: DirRight})
	}
	if u.config.Video.Scale != 8 {
		t.Fatalf("縮放應停在 8，得到 %d", u.config.Video.Scale)
	}

	// 動態平滑是停用項：按下去要說明原因，而且不得改變設定。
	screen.focus = 5
	u.toasts = nil
	blend := u.config.Video.FrameBlend
	u.Handle(Nav{Dir: DirRight})
	if u.config.Video.FrameBlend != blend {
		t.Fatal("停用項不得被調整")
	}
	if len(u.toasts) != 1 {
		t.Fatalf("停用項要說明原因，得到 %+v", u.toasts)
	}
}

// 濾鏡與長寬比要寫回設定檔的字串值，而不是只改畫面上的索引。
func TestVideoChoicesWriteBackStrings(t *testing.T) {
	u := newDisplayUI(t)
	u.push(&videoScreen{})
	screen := u.stack[len(u.stack)-1].(*videoScreen)
	screen.sync(u)
	screen.focus = 3 // 濾鏡
	u.Handle(Nav{Dir: DirRight})
	if u.config.Video.Filter != "scanline25" {
		t.Fatalf("濾鏡 %q", u.config.Video.Filter)
	}
	screen.focus = 2 // 長寬比
	u.Handle(Nav{Dir: DirRight})
	if u.config.Video.Aspect != "1:1" {
		t.Fatalf("長寬比 %q", u.config.Video.Aspect)
	}
}

// 診斷的圖層遮罩只送 intent，不自己動 machine，而且要提醒雜湊不可對帳。
func TestDiagnosticsLayerMaskEmitsIntentAndWarns(t *testing.T) {
	u := newDisplayUI(t)
	u.push(&diagnosticsScreen{})
	u.TakeIntents()
	u.Handle(Action{Kind: ActConfirm})
	intents := u.TakeIntents()
	if len(intents) != 1 {
		t.Fatalf("得到 %#v", intents)
	}
	mask, ok := intents[0].(SetLayerMask)
	if !ok || mask.Mask != AllLayers^LayerTilemap0 {
		t.Fatalf("得到 %#v", intents[0])
	}
	if len(u.toasts) != 1 || u.toasts[0].severity != SeverityWarn {
		t.Fatalf("要以 warn 提醒雜湊不可對帳，得到 %+v", u.toasts)
	}
}

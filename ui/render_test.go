package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSnapshot 是可重現的核心視角：framebuffer 是固定的漸層，其餘數值固定，
// 所以畫面雜湊只受介面本身影響。
type fakeSnapshot struct{}

func (fakeSnapshot) Framebuffer() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			offset := img.PixOffset(x, y)
			img.Pix[offset+0] = uint8(x)
			img.Pix[offset+1] = uint8(y)
			img.Pix[offset+2] = uint8(x ^ y)
			img.Pix[offset+3] = 0xff
		}
	}
	return img
}

func (fakeSnapshot) FrameIndex() uint64                { return 12480 }
func (fakeSnapshot) Instructions() (uint64, uint64)    { return 17369003, 5122334 }
func (fakeSnapshot) Halt() (HaltReason, bool)          { return HaltNone, false }
func (fakeSnapshot) Cartridge() (string, [32]byte, int64) {
	return "BOOM ZOO", [32]byte{0x11, 0x22}, 4 << 20
}
func (fakeSnapshot) Firmware() FirmwareIDs        { return FirmwareIDs{IPLOK: true, KeyOK: true} }
func (fakeSnapshot) ReadWorkRAM(dst []byte, addr uint32) int { return 0 }
func (fakeSnapshot) VideoRegisters() [256]uint16  { return [256]uint16{} }
func (fakeSnapshot) LayerMask() uint32            { return 0x1f }

// fixedSlots 是固定的存檔槽現況：兩個可讀、一個被拒絕、其餘為空。
type fixedSlots struct{}

func (fixedSlots) Slots() []SlotInfo {
	thumb := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			offset := thumb.PixOffset(x, y)
			thumb.Pix[offset+0] = uint8(x * 2)
			thumb.Pix[offset+1] = 0x40
			thumb.Pix[offset+2] = uint8(y)
			thumb.Pix[offset+3] = 0xff
		}
	}
	slots := make([]SlotInfo, SlotCount)
	for i := range slots {
		slots[i] = SlotInfo{Index: i}
	}
	slots[0] = SlotInfo{Index: 0, Present: true, Stamp: "09-01 14:22", Frame: 12480, Thumb: thumb}
	slots[3] = SlotInfo{Index: 3, Present: true, Stamp: "08-31 22:40", Frame: 31900, Thumb: thumb}
	slots[7] = SlotInfo{Index: 7, Rejected: true, Reason: "卡帶不同"}
	return slots
}

type surfaceCase struct {
	name    string
	surface Surface
}

var surfaceCases = []surfaceCase{
	{"960x720/compact", Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}},
	{"1280x720/touch", Surface{W: 1280, H: 720, Scale: 1, Profile: ProfileTouch}},
}

func newTestUI(surface Surface) *UI {
	u := New(Options{Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{}})
	u.Update(0)
	return u
}

// renderImage 把遊戲畫面與覆蓋層畫成一張圖。
func renderImage(u *UI, surface Surface) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, surface.W, surface.H))
	snap := fakeSnapshot{}
	game := snap.Framebuffer()
	c := &canvas{dst: dst, scale: 1, metrics: u.metrics, font: u.font, theme: u.theme}
	c.blitScaled(0, 0, surface.W, surface.H, game)
	u.Draw(dst, snap)
	return dst
}

// render 回傳畫面的 SHA-256，並在 ACAN_UI_DUMP 有設時另存一份 PNG。
func render(t *testing.T, key string, u *UI, surface Surface) string {
	t.Helper()
	img := renderImage(u, surface)
	dump(t, key, img)
	sum := sha256.Sum256(img.Pix)
	return hex.EncodeToString(sum[:])
}

// 記錄於 docs/verify-ui.md。畫面一改動這些值就會變，那正是它們的用途：
// 版面的變更必須是刻意的。
var wantHashes = map[string]string{
	"S3/960x720/compact":       "ff406b886d278a0c70600e20d8b53161713df90cdf58434635ab5ef9546e5ef0",
	"S3/1280x720/touch":        "46eef64f92a9890101d108c61e033ab431b5b08fd9570c3a798bdfc98a84a924",
	"S3+focus/960x720/compact": "73ad19293711c6a1b51c6d86f6f20f63d68fdcf8592f990d8c09ecb14ce243ca",
	"S4/960x720/compact":       "8279ec7eade67dca4b70e3cca03deb3f0bd9cd8573a170119cb0a06f52d007cc",
	"S4/1280x720/touch":        "ce19280911c7abca897d29aacabcd241ac646e01f5a713029a45721b6b8273e0",
	"D1/960x720/compact":       "e1bf95ba55c979941a854d0e07d8a964b630a26e51bf28c5e81b99a6eb4c72e6",
}

// dumpDir 由 ACAN_UI_DUMP 指定；設了就把每張畫面寫成 PNG，供人眼檢查版面。
// 雜湊只能守住「沒有意外變動」，看不出版面本來就畫錯。
func dumpDir() string { return os.Getenv("ACAN_UI_DUMP") }

func dump(t *testing.T, key string, img *image.RGBA) {
	dir := dumpDir()
	if dir == "" {
		return
	}
	name := strings.NewReplacer("/", "_", "+", "-").Replace(key) + ".png"
	file, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func checkHash(t *testing.T, key, got string) {
	t.Helper()
	want, ok := wantHashes[key]
	if !ok {
		t.Fatalf("%s 沒有登記期望雜湊", key)
	}
	if want == "" {
		t.Errorf("%s 尚未登記期望雜湊，本次為 %s", key, got)
		return
	}
	if got != want {
		t.Errorf("%s 畫面雜湊 %s，登記的是 %s", key, got, want)
	}
}

func TestOverlayMenuRendersDeterministically(t *testing.T) {
	for _, c := range surfaceCases {
		u := newTestUI(c.surface)
		u.Open()
		first := render(t, "S3/"+c.name, u, c.surface)
		second := render(t, "S3/"+c.name, u, c.surface)
		if first != second {
			t.Fatalf("%s 同一狀態重畫兩次不一致", c.name)
		}
		checkHash(t, "S3/"+c.name, first)
	}
}

// 固定事件序列之後的畫面也要可重現，否則焦點行為的迴歸沒有辦法被測到。
func TestOverlayFocusSequenceIsReproducible(t *testing.T) {
	c := surfaceCases[0]
	u := newTestUI(c.surface)
	u.Open()
	for i := 0; i < 3; i++ {
		u.Handle(Nav{Dir: DirDown})
	}
	u.Handle(Action{Kind: ActConfirm}) // 進入「重設主機」
	u.Handle(Action{Kind: ActCancel})  // 退回覆蓋選單
	checkHash(t, "S3+focus/"+c.name, render(t, "S3+focus/"+c.name, u, c.surface))
}

func TestSaveSlotsRenderDeterministically(t *testing.T) {
	for _, c := range surfaceCases {
		u := newTestUI(c.surface)
		u.Open()
		u.push(&slotsScreen{mode: slotModeSave})
		checkHash(t, "S4/"+c.name, render(t, "S4/"+c.name, u, c.surface))
	}
}

func TestConfirmDialogRenders(t *testing.T) {
	c := surfaceCases[0]
	u := newTestUI(c.surface)
	u.Open()
	u.push(&slotsScreen{mode: slotModeSave})
	u.Handle(Action{Kind: ActConfirm}) // 槽 0 已有存檔，應跳出覆寫確認
	if u.modal == nil {
		t.Fatal("覆寫既有存檔必須先確認")
	}
	checkHash(t, "D1/"+c.name, render(t, "D1/"+c.name, u, c.surface))
}

// 覆蓋層開著的時候，模擬時間不得前進：UI 只會排出 Intent，不會自己動手。
func TestOverlayOnlyEmitsIntents(t *testing.T) {
	u := newTestUI(surfaceCases[0].surface)
	u.Open()
	intents := u.TakeIntents()
	if len(intents) != 1 {
		t.Fatalf("開啟選單應只排出一個 Intent，得到 %d", len(intents))
	}
	if paused, ok := intents[0].(SetPaused); !ok || !paused.Paused {
		t.Fatalf("開啟選單應要求暫停，得到 %#v", intents[0])
	}
	u.push(&slotsScreen{mode: slotModeSave, focus: 1})
	u.Handle(Action{Kind: ActConfirm})
	got := u.TakeIntents()
	if len(got) != 1 {
		t.Fatalf("存到空槽應只排出一個 Intent，得到 %d", len(got))
	}
	if save, ok := got[0].(SaveState); !ok || save.Slot != 1 {
		t.Fatalf("得到 %#v，want SaveState{Slot:1}", got[0])
	}
}

// 被拒絕的槽按下讀檔要走錯誤列，不是 toast：會自己消失的錯誤等於沒說。
func TestRejectedSlotUsesErrorBar(t *testing.T) {
	u := newTestUI(surfaceCases[0].surface)
	u.Open()
	u.push(&slotsScreen{mode: slotModeLoad, focus: 7})
	u.TakeIntents() // 開啟選單本身會排出 SetPaused，先清掉
	u.Handle(Action{Kind: ActConfirm})
	if u.errorText == "" {
		t.Fatal("被拒絕的存檔槽必須顯示錯誤列")
	}
	if len(u.toasts) != 0 {
		t.Fatalf("錯誤不得用 toast 呈現，得到 %d 則", len(u.toasts))
	}
	if intents := u.TakeIntents(); len(intents) != 0 {
		t.Fatalf("被拒絕的槽不得排出載入 Intent，得到 %#v", intents)
	}
}

// info toast 到時要消失，warn 活得比較久；錯誤列不會自己消失。
func TestToastLifetimes(t *testing.T) {
	u := newTestUI(surfaceCases[0].surface)
	u.toast("info", SeverityInfo)
	u.toast("warn", SeverityWarn)
	u.Update(infoToastLife + time.Millisecond)
	if len(u.toasts) != 1 || u.toasts[0].text != "warn" {
		t.Fatalf("2.5 秒後只該剩下 warn，得到 %+v", u.toasts)
	}
	u.Update(warnToastLife + time.Millisecond)
	if len(u.toasts) != 0 {
		t.Fatalf("4 秒後 warn 也該消失，得到 %+v", u.toasts)
	}
	u.fail("boom")
	u.Update(time.Hour)
	if u.errorText == "" {
		t.Fatal("錯誤列不得自己消失")
	}
}

// 抑制操作訊息時錯誤仍然要顯示。
func TestSuppressedInfoToastsKeepErrors(t *testing.T) {
	u := newTestUI(surfaceCases[0].surface)
	u.config.SuppressInfoToasts = true
	u.toast("info", SeverityInfo)
	u.toast("warn", SeverityWarn)
	if len(u.toasts) != 1 {
		t.Fatalf("info 應被抑制，得到 %+v", u.toasts)
	}
}

// 返回鍵的順序：先關 modal，再退堆疊，最後才關掉覆蓋層。
func TestBackUnwindsInOrder(t *testing.T) {
	u := newTestUI(surfaceCases[0].surface)
	u.Open()
	u.push(&slotsScreen{mode: slotModeSave})
	u.modal = &confirm{title: "x", body: "y", accept: "z"}
	steps := []struct {
		wantModal bool
		wantDepth int
	}{
		{false, 2},
		{false, 1},
		{false, 0},
	}
	for i, want := range steps {
		u.Handle(Life{Kind: LifeBack})
		if (u.modal != nil) != want.wantModal || len(u.stack) != want.wantDepth {
			t.Fatalf("第 %d 步：modal=%v 深度=%d，want modal=%v 深度=%d",
				i+1, u.modal != nil, len(u.stack), want.wantModal, want.wantDepth)
		}
	}
}

// 停用項可以聚焦，按下確認要說明原因而不是靜默無事。
func TestDisabledRowExplainsItself(t *testing.T) {
	u := newTestUI(surfaceCases[0].surface)
	u.Open()
	overlay := u.stack[0].(*overlayScreen)
	rows := overlay.rows(u)
	index := -1
	for i, row := range rows {
		if row.disabled {
			index = i
			break
		}
	}
	if index < 0 {
		t.Skip("目前沒有停用項")
	}
	overlay.focus = index
	u.Handle(Action{Kind: ActConfirm})
	if len(u.toasts) != 1 {
		t.Fatalf("停用項應以 toast 說明原因，得到 %+v", u.toasts)
	}
	want := fmt.Sprintf(textNotYet, rows[index].reason)
	if u.toasts[0].text != want {
		t.Fatalf("toast=%q，want %q", u.toasts[0].text, want)
	}
}

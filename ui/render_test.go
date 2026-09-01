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

func (fakeSnapshot) FrameIndex() uint64             { return 12480 }
func (fakeSnapshot) Instructions() (uint64, uint64) { return 17369003, 5122334 }
func (fakeSnapshot) Halt() (HaltReason, bool)       { return HaltNone, false }
func (fakeSnapshot) Cartridge() (string, [32]byte, int64) {
	return "BOOM ZOO", [32]byte{0x11, 0x22}, 4 << 20
}
func (fakeSnapshot) Firmware() FirmwareIDs                   { return FirmwareIDs{IPLOK: true, KeyOK: true} }
func (fakeSnapshot) ReadWorkRAM(dst []byte, addr uint32) int { return 0 }
func (fakeSnapshot) VideoRegisters() [256]uint16             { return [256]uint16{} }
func (fakeSnapshot) LayerMask() uint32                       { return 0x1f }

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
	"S3/960x720/compact":       "6990880e9083ec950cb87bbade7e1945f95c5040e6776bc9aa13ed5394fc7460",
	"S3/1280x720/touch":        "ee222da403badeba5a8779e7573c763a1ad16b9df25ceaca029987ec1499d345",
	"S3+focus/960x720/compact": "5ca59032ac1c4bcaad23445d9b3b96274c65f1bddc91c5a6b6542804e2fe714c",
	"S4/960x720/compact":       "8279ec7eade67dca4b70e3cca03deb3f0bd9cd8573a170119cb0a06f52d007cc",
	"S4/1280x720/touch":        "ce19280911c7abca897d29aacabcd241ac646e01f5a713029a45721b6b8273e0",
	"D1/960x720/compact":       "e1bf95ba55c979941a854d0e07d8a964b630a26e51bf28c5e81b99a6eb4c72e6",
	"S0/960x720/compact":       "84e731b98b74525e2414a15c7df179c9b787ce415a11b8a12a1bc55e16a1f65a",
	"S0/1280x720/touch":        "fa17df07234c345ca9edeafef255dbe9c1ad6094d7b3588e2b18c3fe21794446",
	"S0ready/960x720/compact":  "8ca7a1e7fbfb1a66eaedae4c2d31498a08da1f6915e698f6e42048b74c092148",
	"S0.1/960x720/compact":     "90adda11beebb2cb6fb7529b7177f8522a8b42b372ba37ca0fc94cc1deef90bf",
	"S1/960x720/compact":       "82cae5f1eceefee32895587a3972ea42c1e5b03a3d4ba14a5cb39fbda23bddca",
	"S1/1280x720/touch":        "64274509659f2a7e7a637d197f9b264efba212fedc72d26eed4816902e41d1f9",
	"S8/960x720/compact":       "fb18139c4ac701d5187e9110dd0e9b9e485382eba23a78775b03e2725ff39b81",
	"S5/960x720/compact":          "c48bf197a895e9b46313a21d23dae7e98577ebcd7231e6afb7799228d16056cc",
	"S5.1/960x720/compact":        "1ef20869b7fb4a4442f58beb90bd1b1f7717e0aec4555344c66ef5e5258fdb62",
	"S5.2/960x720/compact":        "2cccd2b67daf8a5e36be3d14722c3037ab6ed2b87a55384c36350d8e1964d4f8",
	"S5.2conflict/960x720/compact": "107ecbc854d803c375197eec83f6c68097dd35778cc307b06767ad6b07f0ddbf",
	"S9/960x720/compact":       "c06f549d0b21a9caba9422a89cef873cc66bb867a0eef0dc3899991d2533e1bf",
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
	u.config.Interface.SuppressInfoToasts = true
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

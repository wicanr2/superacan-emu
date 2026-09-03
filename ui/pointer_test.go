package ui

import (
	"image"
	"testing"
)

// 指標事件的座標是表面像素，命中區是設計單位；這裡一律用 scale 1 讓兩者相同，
// 縮放另有 TestPointerRespectsSurfaceScale 顧。
func desktopUI(t *testing.T) (*UI, Surface) {
	t.Helper()
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := newTestUI(surface)
	u.Open()
	renderImage(u, surface) // 命中區在 Draw 時才登記
	return u, surface
}

func rowCenter(u *UI, index int) image.Point {
	rect := u.hits[index].rect
	return image.Pt((rect.Min.X+rect.Max.X)/2, (rect.Min.Y+rect.Max.Y)/2)
}

func click(u *UI, at image.Point) {
	u.Handle(Pointer{X: at.X, Y: at.Y, Phase: PhaseDown})
	u.Handle(Pointer{X: at.X, Y: at.Y, Phase: PhaseUp})
}

func TestPointerClickRunsTheRow(t *testing.T) {
	u, surface := desktopUI(t)
	if len(u.hits) == 0 {
		t.Fatal("覆蓋選單沒有登記任何可點區域")
	}
	// 第一列是「繼續遊戲」，點下去覆蓋層就關掉。
	click(u, rowCenter(u, 0))
	if u.Visible() {
		t.Fatal("點第一列之後覆蓋層還開著")
	}
	_ = surface
}

// 按下與放開要落在同一塊才算數：按下去之後滑開再放，使用者的意思是取消。
func TestPointerReleaseElsewhereDoesNothing(t *testing.T) {
	u, _ := desktopUI(t)
	first, second := rowCenter(u, 0), rowCenter(u, 1)
	u.Handle(Pointer{X: first.X, Y: first.Y, Phase: PhaseDown})
	u.Handle(Pointer{X: second.X, Y: second.Y, Phase: PhaseUp})
	if !u.Visible() {
		t.Fatal("在別的列放開卻仍然執行了按下那一列")
	}
}

// 滑鼠與鍵盤共用同一個焦點，不是各有一個：移過去之後按鍵盤的確認鍵，
// 執行的要是滑鼠指著的那一列。
func TestPointerHoverMovesTheKeyboardFocus(t *testing.T) {
	u, _ := desktopUI(t)
	if len(u.hits) < 3 {
		t.Skip("列數不足")
	}
	third := rowCenter(u, 2)
	u.Handle(Pointer{X: third.X, Y: third.Y, Phase: PhaseMove})
	u.Handle(Action{Kind: ActConfirm})

	if !u.Visible() {
		t.Fatal("鍵盤確認關掉了覆蓋層，代表焦點還停在第一列（繼續遊戲）")
	}
	if len(u.stack) < 2 {
		t.Fatal("鍵盤確認沒有進到第三列對應的子畫面")
	}
}

// 覆蓋層沒開時不吃指標事件：遊戲本身沒有滑鼠輸入，吃掉會讓前端誤以為處理過了。
func TestPointerIgnoredWhileOverlayClosed(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := newTestUI(surface)
	if u.Handle(Pointer{X: 10, Y: 10, Phase: PhaseDown}) {
		t.Fatal("覆蓋層沒開卻吃掉了指標事件")
	}
}

// 座標是表面像素，命中區是設計單位，兩者差一個 scale。
func TestPointerRespectsSurfaceScale(t *testing.T) {
	surface := Surface{W: 1920, H: 1440, Scale: 2, Profile: ProfileCompact}
	u := newTestUI(surface)
	u.Open()
	dst := image.NewRGBA(image.Rect(0, 0, surface.W, surface.H))
	u.Draw(dst, fakeSnapshot{})
	if len(u.hits) == 0 {
		t.Fatal("沒有登記可點區域")
	}
	at := rowCenter(u, 0)
	click(u, image.Pt(at.X*surface.Scale, at.Y*surface.Scale))
	if u.Visible() {
		t.Fatal("scale 2 時點第一列沒有生效，座標換算可能漏了 scale")
	}
}

// 卡帶瀏覽器是「怎麼把遊戲跑起來」的入口，也接指標。
func TestPointerLoadsACartridgeFromTheBrowser(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := New(Options{Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}})
	u.Update(0)
	u.push(&browserScreen{})
	renderImage(u, surface)

	var loaded string
	before := len(u.hits)
	if before == 0 {
		t.Fatal("瀏覽器沒有登記可點區域")
	}
	click(u, rowCenter(u, 0))
	for _, intent := range u.TakeIntents() {
		if load, ok := intent.(LoadCartridge); ok {
			loaded = load.Path
		}
	}
	if loaded == "" {
		t.Fatal("點清單第一列沒有送出載入卡帶的 Intent")
	}
}

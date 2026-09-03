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

// firstHitBelowTitle 跳過標題列上的「返回」，回傳內容區的第一塊命中區。
func firstHitBelowTitle(u *UI) image.Point { return contentHit(u, 0) }

// contentHit 回傳內容區第 n 塊命中區的中心。走 page 的畫面會先登記標題列上的
// 「返回」，所以命中區的索引比列的索引多一，這裡把它跳掉。
func contentHit(u *UI, n int) image.Point {
	seen := 0
	for _, hit := range u.hits {
		if hit.rect.Min.Y < u.metrics.TitleBar {
			continue
		}
		if seen == n {
			return image.Pt((hit.rect.Min.X+hit.rect.Max.X)/2, (hit.rect.Min.Y+hit.rect.Max.Y)/2)
		}
		seen++
	}
	return image.Point{}
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
	if len(u.hits) == 0 {
		t.Fatal("瀏覽器沒有登記可點區域")
	}
	click(u, firstHitBelowTitle(u))
	for _, intent := range u.TakeIntents() {
		if load, ok := intent.(LoadCartridge); ok {
			loaded = load.Path
		}
	}
	if loaded == "" {
		t.Fatal("點清單第一列沒有送出載入卡帶的 Intent")
	}
}

// 每個有清單或選項的畫面都要吃指標。這條掃過所有畫面，任何一個沒有登記
// 命中區就會被抓出來——新畫面忘了接指標時，這裡會先紅。
func TestEveryListScreenAcceptsPointer(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	for _, tc := range []struct {
		name   string
		screen screen
	}{
		{"S1 卡帶瀏覽器", &browserScreen{}},
		{"S4 存檔槽", &slotsScreen{}},
		{"S5 設定", &settingsScreen{}},
		{"S5.1 輸入綁定", &bindingScreen{}},
		{"S5.2 熱鍵", &hotkeyScreen{}},
		{"S5.3 影像", &videoScreen{}},
		{"S5.4 音訊", &audioScreen{}},
		{"S5.6 觸控版面", &touchScreen{}},
		{"S6.1 金手指搜尋", &cheatSearchScreen{width: 1}},
		{"S6.2 金手指清單", &cheatListScreen{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := New(Options{Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{},
				Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}, About: fixedAbout})
			u.Update(0)
			u.push(tc.screen)
			renderImage(u, surface)
			content := 0
			for _, hit := range u.hits {
				if hit.rect.Min.Y >= u.metrics.TitleBar {
					content++
				}
			}
			if content == 0 {
				t.Fatalf("%s 沒有在內容區登記任何命中區", tc.name)
			}
		})
	}
}

// 走 page 的畫面共用同一個標題列，返回因此一次都能點。
func TestPointerBackFromSharedTitleBar(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := New(Options{Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}})
	u.Update(0)
	u.Open()
	u.push(&browserScreen{})
	renderImage(u, surface)
	depth := len(u.stack)
	click(u, image.Pt(u.metrics.PanelPad+4, u.metrics.TitleBar/2))
	if len(u.stack) >= depth {
		t.Fatalf("點標題列的返回沒有退堆疊：%d → %d", depth, len(u.stack))
	}
}

// 走一條完整的路：覆蓋選單 →「設定」→「影像」→ 點一列選項，確認值真的變了。
// 這條測的是「點下去有沒有做事」，不只是「有沒有登記命中區」。
func TestPointerWalksIntoSettingsAndChangesAValue(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := New(Options{Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{},
		Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}, About: fixedAbout})
	u.Update(0)
	u.Open()

	clickRowLabelled(t, u, surface, u.s.Settings)
	if id := u.stack[len(u.stack)-1].id(); id != "S5" {
		t.Fatalf("點「設定」之後在 %s，預期 S5", id)
	}
	clickRowLabelled(t, u, surface, u.s.VideoTitle)
	if id := u.stack[len(u.stack)-1].id(); id != "S5.3" {
		t.Fatalf("點「影像」之後在 %s，預期 S5.3", id)
	}

	// 影像設定的第一列是縮放倍率，點一下等同按右鍵，值加一階。
	before := u.config.Video.Scale
	renderImage(u, surface)
	click(u, firstHitBelowTitle(u))
	if u.config.Video.Scale != before+1 {
		t.Fatalf("點第一列選項之後縮放是 %d，預期 %d", u.config.Video.Scale, before+1)
	}
}

// clickRowLabelled 畫一次、找到寫著 label 的那一列、點它。用畫面上的文字定位，
// 測試就不必寫死列的索引，選單增減一列也不會假紅。
func clickRowLabelled(t *testing.T, u *UI, surface Surface, label string) {
	t.Helper()
	renderImage(u, surface)
	rows := rowsOnScreen(u)
	for index, row := range rows {
		if row == label {
			click(u, contentHit(u, index))
			return
		}
	}
	t.Fatalf("畫面上找不到「%s」這一列", label)
}

// rowsOnScreen 取目前最上層畫面的列標題，順序與命中區相同。
func rowsOnScreen(u *UI) []string {
	switch top := u.stack[len(u.stack)-1].(type) {
	case *overlayScreen:
		return labelsOf(top.rows(u))
	case *settingsScreen:
		return labelsOf(top.rows(u))
	}
	return nil
}

func labelsOf(rows []menuRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.label
	}
	return out
}

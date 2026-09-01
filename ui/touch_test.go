package ui

import (
	"image"
	"testing"
)

func newTouchUI(t *testing.T, width, height int) *UI {
	t.Helper()
	config := DefaultConfig()
	u := New(Options{
		Surface: Surface{W: width, H: height, Scale: 1, Profile: ProfileTouch},
		Config:  config, Slots: fixedSlots{}, Library: fixedLibrary{},
		Firmware: fixedFirmware{complete: true}, About: fixedAbout,
		AudioStats: fixedAudioStats{}, Diagnostics: fixedDiagnostics{},
	})
	u.Update(0)
	return u
}

// 每個可操作元件的命中區都不得小於 44 見方，而且每邊要比繪製區大至少 4 單位。
// 手指會遮住按鍵，使用者看不到自己按在哪。
func TestTouchTargetsAreLargeEnough(t *testing.T) {
	for _, size := range [][2]int{{1280, 720}, {720, 1280}} {
		u := newTouchUI(t, size[0], size[1])
		layout := u.TouchLayout()
		if len(layout.Controls) == 0 {
			t.Fatalf("%dx%d 沒有任何元件", size[0], size[1])
		}
		for _, item := range layout.Controls {
			if item.hit.Dx() < TouchMinTarget || item.hit.Dy() < TouchMinTarget {
				t.Errorf("%dx%d 的 %s 命中區只有 %dx%d",
					size[0], size[1], item.name, item.hit.Dx(), item.hit.Dy())
			}
			for _, edge := range []struct {
				name string
				gap  int
			}{
				{"左", item.draw.Min.X - item.hit.Min.X},
				{"上", item.draw.Min.Y - item.hit.Min.Y},
				{"右", item.hit.Max.X - item.draw.Max.X},
				{"下", item.hit.Max.Y - item.draw.Max.Y},
			} {
				if edge.gap < 4 {
					t.Errorf("%s 的%s邊命中區只比繪製區大 %d 單位", item.name, edge.name, edge.gap)
				}
			}
		}
	}
}

// 五個同時觸點要全部生效：方向＋兩鍵＋肩鍵是常見組合。
func TestFiveSimultaneousTouches(t *testing.T) {
	u := newTouchUI(t, 1280, 720)
	layout := u.TouchLayout()

	press := func(id int, name string) {
		for _, item := range layout.Controls {
			if item.name != name {
				continue
			}
			point := image.Pt((item.hit.Min.X+item.hit.Max.X)/2, (item.hit.Min.Y+item.hit.Max.Y)/2)
			if item.dpad {
				// 方向鍵要按在上緣才是「上」，中央是死區。
				point.Y = item.draw.Min.Y + 4
			}
			u.Handle(Pointer{ID: id, X: point.X, Y: point.Y, Phase: PhaseDown})
			return
		}
		t.Fatalf("找不到 %s", name)
	}
	press(0, "up")
	press(1, "a")
	press(2, "b")
	press(3, "l")
	press(4, "r")

	mask := u.TouchPad()
	for _, name := range []string{"up", "a", "b", "l", "r"} {
		if !mask.Pressed(name) {
			t.Errorf("%s 沒有被按到", name)
		}
	}
	for _, name := range []string{"down", "left", "right", "start", "select"} {
		if mask.Pressed(name) {
			t.Errorf("%s 不該被按到", name)
		}
	}

	// 放開其中一個，其餘不受影響。
	u.Handle(Pointer{ID: 1, Phase: PhaseUp})
	if u.TouchPad().Pressed("a") {
		t.Error("放開之後 A 仍然按著")
	}
	if !u.TouchPad().Pressed("b") {
		t.Error("放開 A 不該影響 B")
	}
}

// 方向鍵的中央是死區，對角線是兩個位元。
func TestDPadDeadzoneAndDiagonals(t *testing.T) {
	u := newTouchUI(t, 1280, 720)
	var pad touchControl
	for _, item := range u.TouchLayout().Controls {
		if item.dpad {
			pad = item
		}
	}
	centre := image.Pt((pad.draw.Min.X+pad.draw.Max.X)/2, (pad.draw.Min.Y+pad.draw.Max.Y)/2)

	u.Handle(Pointer{ID: 0, X: centre.X, Y: centre.Y, Phase: PhaseDown})
	if u.TouchPad() != 0 {
		t.Error("死區內不該產生方向")
	}
	u.Handle(Pointer{ID: 0, X: pad.draw.Min.X + 4, Y: pad.draw.Min.Y + 4, Phase: PhaseMove})
	mask := u.TouchPad()
	if !mask.Pressed("up") || !mask.Pressed("left") {
		t.Errorf("左上角應該同時是上與左，得到 %012b", mask)
	}
	if mask.Pressed("down") || mask.Pressed("right") {
		t.Error("左上角不該有下或右")
	}
}

// 覆蓋層開著時虛擬手把要隱藏，而且不再吃觸點。
func TestVirtualPadHidesUnderTheOverlay(t *testing.T) {
	u := newTouchUI(t, 1280, 720)
	layout := u.TouchLayout()
	var start touchControl
	for _, item := range layout.Controls {
		if item.name == "start" {
			start = item
		}
	}
	u.Handle(Pointer{ID: 0, X: start.draw.Min.X + 4, Y: start.draw.Min.Y + 4, Phase: PhaseDown})
	if !u.TouchPad().Pressed("start") {
		t.Fatal("START 應該被按到")
	}

	withPad := renderTouch(u, 1280, 720)
	u.Open()
	if u.TouchPad() != 0 {
		t.Fatal("覆蓋層開著時虛擬手把不得回報按鍵")
	}
	overlay := renderTouch(u, 1280, 720)
	if withPad == overlay {
		t.Fatal("覆蓋層開著與關著的畫面不該相同")
	}

	// 覆蓋層開著時的觸點不再進虛擬手把。
	u.Handle(Pointer{ID: 1, X: start.draw.Min.X + 4, Y: start.draw.Min.Y + 4, Phase: PhaseDown})
	if u.TouchPad() != 0 {
		t.Fatal("覆蓋層開著時觸點不得變成手把輸入")
	}
}

// 按選單鍵要叫出覆蓋選單，而且那一個觸點不能同時算成手把輸入。
func TestMenuTouchOpensOverlay(t *testing.T) {
	u := newTouchUI(t, 1280, 720)
	var menu touchControl
	for _, item := range u.TouchLayout().Controls {
		if item.name == "menu" {
			menu = item
		}
	}
	u.Handle(Pointer{ID: 0, X: menu.draw.Min.X + 4, Y: menu.draw.Min.Y + 4, Phase: PhaseDown})
	if !u.Visible() {
		t.Fatal("按 ☰ 應該叫出覆蓋選單")
	}
	if u.TouchPad() != 0 {
		t.Fatal("選單鍵不得同時算成手把輸入")
	}
}

// 兩個方向的版面各渲染一次並記錄雜湊。
func TestTouchLayoutsRender(t *testing.T) {
	for _, c := range []struct {
		name          string
		width, height int
	}{
		{"landscape/1280x720", 1280, 720},
		{"portrait/720x1280", 720, 1280},
	} {
		u := newTouchUI(t, c.width, c.height)
		checkHash(t, "touch/"+c.name, renderTouchNamed(t, "touch/"+c.name, u, c.width, c.height))
	}
}

// 左右手互換要把每個元件鏡像到另一邊。
func TestSwapHandsMirrorsTheLayout(t *testing.T) {
	u := newTouchUI(t, 1280, 720)
	before := map[string]int{}
	for _, item := range u.TouchLayout().Controls {
		before[item.name] = item.draw.Min.X
	}
	u.config.Input.Touch.SwapHands = true
	u.touch.layout = TouchLayout{}
	after := map[string]int{}
	for _, item := range u.TouchLayout().Controls {
		after[item.name] = item.draw.Min.X
	}
	if before["up"] >= after["up"] {
		t.Errorf("方向鍵應該移到右邊：%d → %d", before["up"], after["up"])
	}
	if before["a"] <= after["a"] {
		t.Errorf("A 鍵應該移到左邊：%d → %d", before["a"], after["a"])
	}
}

// renderTouch 先畫遊戲畫面再畫介面，與實際執行時的合成順序相同——
// 虛擬手把是半透明的，底下是什麼會改變合成結果。
func renderTouch(u *UI, width, height int) string {
	dst := touchFrame(u, width, height)
	return hashPix(dst)
}

func renderTouchNamed(t *testing.T, key string, u *UI, width, height int) string {
	t.Helper()
	dst := touchFrame(u, width, height)
	dump(t, key, dst)
	return hashPix(dst)
}

func touchFrame(u *UI, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	snap := fakeSnapshot{}
	c := &canvas{dst: dst, scale: 1, metrics: u.metrics, font: u.font, theme: u.theme}
	layout := u.TouchLayout()
	c.blitScaled(layout.Screen.Min.X, layout.Screen.Min.Y,
		layout.Screen.Dx(), layout.Screen.Dy(), snap.Framebuffer())
	u.Draw(dst, snap)
	return dst
}

func TestTouchSettingsRender(t *testing.T) {
	u := newSettingsUI(t, nil)
	u.Open()
	u.push(&touchScreen{})
	checkHash(t, "S5.6/"+surfaceCases[0].name,
		render(t, "S5.6/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

// 改觸控參數要立刻重算版面，不能等到轉向才生效。
func TestTouchSettingsRecomputeLayout(t *testing.T) {
	u := newTouchUI(t, 1280, 720)
	before := u.TouchLayout().Controls[0].draw.Min.X
	u.push(&touchScreen{})
	screen := u.stack[len(u.stack)-1].(*touchScreen)
	screen.focus = 3 // 左右手互換
	u.Handle(Action{Kind: ActConfirm})
	u.Close()
	if u.TouchLayout().Controls[0].draw.Min.X == before {
		t.Fatal("改設定之後版面應該重算")
	}
}

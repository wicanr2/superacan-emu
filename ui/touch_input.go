package ui

import "image"

// touchState 追蹤同時按下的觸點。至少要能同時追蹤五個：方向＋兩鍵＋肩鍵是
// 常見組合，只追一個會讓斜向移動時按不出動作。
type touchInput struct {
	layout       TouchLayout
	layoutWidth  int
	layoutHeight int
	pointers     map[int]PadMask
	mask         PadMask
}

// TouchPad 回傳目前虛擬手把按住的按鈕。入口把它與實體輸入合併之後再送給 machine。
func (u *UI) TouchPad() PadMask {
	if u.surface.Profile != ProfileTouch || u.Visible() {
		// 覆蓋層開著時虛擬手把隱藏：兩套控制同時存在會互相搶觸點。
		return 0
	}
	return u.touch.mask
}

// TouchLayout 回傳目前的虛擬手把版面，供測試與版面編輯畫面使用。
func (u *UI) TouchLayout() TouchLayout {
	u.ensureTouchLayout()
	return u.touch.layout
}

func (u *UI) ensureTouchLayout() {
	width, height := u.surface.W/u.surface.Scale, u.surface.H/u.surface.Scale
	if u.touch.layout.Screen.Empty() ||
		u.touch.layout.Portrait != (height > width) ||
		u.touch.layoutWidth != width || u.touch.layoutHeight != height {
		u.touch.layout = touchLayoutFor(width, height, u.config.Input.Touch.SwapHands)
		u.touch.layoutWidth, u.touch.layoutHeight = width, height
	}
	if u.touch.pointers == nil {
		u.touch.pointers = map[int]PadMask{}
	}
}

// handleTouch 把一個觸點事件變成虛擬手把的按鍵狀態。回傳是否被虛擬手把消化。
func (u *UI) handleTouch(event Pointer) bool {
	if u.surface.Profile != ProfileTouch || u.Visible() {
		return false
	}
	u.ensureTouchLayout()
	scale := u.surface.Scale
	point := image.Pt(event.X/scale, event.Y/scale)

	switch event.Phase {
	case PhaseUp, PhaseCancel:
		delete(u.touch.pointers, event.ID)
	default:
		mask, menu := u.touch.layout.maskAt(point, u.config.Input.Touch.DPadDeadzone)
		if menu {
			delete(u.touch.pointers, event.ID)
			if event.Phase == PhaseDown {
				u.Open()
			}
			u.recomputeTouchMask()
			return true
		}
		if mask == 0 {
			delete(u.touch.pointers, event.ID)
		} else {
			u.touch.pointers[event.ID] = mask
		}
	}
	u.recomputeTouchMask()
	return true
}

func (u *UI) recomputeTouchMask() {
	var mask PadMask
	for _, pointer := range u.touch.pointers {
		mask |= pointer
	}
	u.touch.mask = mask
}

// maskAt 回報一個座標按到哪些按鈕，以及是不是按到選單鍵。
func (l TouchLayout) maskAt(point image.Point, deadzonePercent int) (PadMask, bool) {
	var mask PadMask
	for _, item := range l.Controls {
		if !point.In(item.hit) {
			continue
		}
		if item.name == "menu" {
			return 0, true
		}
		if item.dpad {
			mask |= dpadMask(item.hit, point, deadzonePercent)
			continue
		}
		mask |= padBit(item.name)
	}
	return mask, false
}

// dpadMask 把九宮格的觸點轉成方向。中央死區以半徑百分比表示：死區太小會讓
// 「按上」變成「上＋左」。
func dpadMask(area image.Rectangle, point image.Point, deadzonePercent int) PadMask {
	if deadzonePercent <= 0 {
		deadzonePercent = 15
	}
	centreX := (area.Min.X + area.Max.X) / 2
	centreY := (area.Min.Y + area.Max.Y) / 2
	radius := area.Dx() / 2
	dead := radius * deadzonePercent / 100

	dx, dy := point.X-centreX, point.Y-centreY
	if dx*dx+dy*dy <= dead*dead {
		return 0
	}
	// 九宮格：以死區為界，各軸獨立判斷，對角線因此自然是兩個位元。
	var mask PadMask
	if dx < -dead {
		mask |= padBit("left")
	}
	if dx > dead {
		mask |= padBit("right")
	}
	if dy < -dead {
		mask |= padBit("up")
	}
	if dy > dead {
		mask |= padBit("down")
	}
	return mask
}

// drawVirtualPad 畫虛擬手把。覆蓋層開著時不畫：兩套控制同時存在會互相搶觸點。
func (u *UI) drawVirtualPad(c *canvas) {
	if u.surface.Profile != ProfileTouch || u.Visible() {
		return
	}
	u.ensureTouchLayout()
	// 畫面之外的區域塗成不透明底：直式版面的控制區佔了下半，留著透明會讓
	// 前端有沒有清畫面變成看得見的差異。
	solid := u.theme.Panel
	solid.A = 0xff
	screen := u.touch.layout.Screen
	c.rect(0, 0, c.width(), screen.Min.Y, solid)
	c.rect(0, screen.Max.Y, c.width(), c.height()-screen.Max.Y, solid)
	c.rect(0, screen.Min.Y, screen.Min.X, screen.Dy(), solid)
	c.rect(screen.Max.X, screen.Min.Y, c.width()-screen.Max.X, screen.Dy(), solid)

	opacity := u.config.Input.Touch.Opacity
	if opacity <= 0 || opacity > 100 {
		opacity = 60
	}
	fill := u.theme.PanelAlt
	fill.A = uint8(opacity * 255 / 100)
	border := u.theme.Border
	border.A = fill.A
	text := u.theme.Text
	text.A = fill.A

	for _, item := range u.touch.layout.Controls {
		if item.dpad {
			u.drawDPad(c, item, fill, border)
			continue
		}
		background := fill
		if u.touch.mask&padBit(item.name) != 0 {
			background = u.theme.Focus
			background.A = fill.A
		}
		c.rect(item.draw.Min.X, item.draw.Min.Y, item.draw.Dx(), item.draw.Dy(), background)
		c.border(item.draw.Min.X, item.draw.Min.Y, item.draw.Dx(), item.draw.Dy(), border)
		label := touchLabel(item.name)
		c.textCenter(item.draw.Min.X, item.draw.Min.Y+(item.draw.Dy()-c.font.Height(u.metrics.BodySize))/2,
			item.draw.Dx(), u.metrics.BodySize, text, label)
	}
}

// drawDPad 把方向鍵畫成十字而不是一個方塊：方塊看不出四個方向在哪，
// 而按下時要能一眼看出自己按到的是哪一格。
func (u *UI) drawDPad(c *canvas, item touchControl, fill, border rgba) {
	third := item.draw.Dx() / 3
	cells := []struct {
		name string
		x, y int
	}{
		{"up", 1, 0}, {"left", 0, 1}, {"", 1, 1}, {"right", 2, 1}, {"down", 1, 2},
	}
	for _, cell := range cells {
		background := fill
		if cell.name != "" && u.touch.mask&padBit(cell.name) != 0 {
			background = u.theme.Focus
			background.A = fill.A
		}
		x := item.draw.Min.X + cell.x*third
		y := item.draw.Min.Y + cell.y*third
		c.rect(x, y, third, third, background)
		c.border(x, y, third, third, border)
	}
}

func touchLabel(name string) string {
	switch name {
	case "up":
		return "＋"
	case "menu":
		return "☰"
	case "select":
		return "SEL"
	case "start":
		return "START"
	default:
		return padButtonLabels[name]
	}
}

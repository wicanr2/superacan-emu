package ui

// confirm 是 D1 確認對話。只有三種情況用 modal：覆寫既有存檔、刪除存檔、
// 在有未寫回電池記憶體時離開。其餘一律用可撤銷的操作加 toast——確認對話是打斷，
// 濫用會讓使用者養成不讀就按的習慣，真正重要的那次也一起被略過。
type confirm struct {
	title   string
	body    string
	accept  string
	onYes   func(u *UI)
	focused int // 0 取消，1 接受
}

func (d *confirm) handle(u *UI, ev Event) bool {
	switch e := ev.(type) {
	case Nav:
		if e.Dir == DirLeft || e.Dir == DirRight {
			d.focused ^= 1
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			d.activate(u)
			return true
		case ActCancel, ActMenu:
			u.modal = nil
			return true
		}
	case Life:
		if e.Kind == LifeBack {
			u.modal = nil
			return true
		}
	}
	// modal 吃掉所有事件，底下的畫面不會收到；這就是 modal 的定義。
	return true
}

// activate 是「確認目前這一個按鈕」的唯一實作，鍵盤與指標共用。
func (d *confirm) activate(u *UI) {
	yes := d.focused == 1
	u.modal = nil
	if yes && d.onYes != nil {
		d.onYes(u)
	}
}

func (d *confirm) draw(u *UI, c *canvas) {
	m := u.metrics
	theme := u.theme

	maxWidth := c.width() - m.PanelPad*4
	lines := wrapText(c.font, d.body, m.BodySize, maxWidth-m.PanelPad*2)
	width := c.font.Measure(d.title, m.BodySize)
	for _, line := range lines {
		width = max(width, c.font.Measure(line, m.BodySize))
	}
	width = min(width+m.PanelPad*2, maxWidth)

	titleH := m.RowHeight
	bodyH := m.RowHeight*len(lines) + m.Grid
	buttonH := m.RowHeight + m.Grid*2
	height := titleH + 1 + bodyH + 1 + buttonH
	x := (c.width() - width) / 2
	y := (c.height() - height) / 2

	c.rect(0, 0, c.width(), c.height(), theme.Scrim)
	c.rect(x, y, width, height, theme.PanelAlt)
	c.border(x, y, width, height, theme.Border)

	cursor := y
	c.rowText(x+m.PanelPad, cursor, titleH, m.BodySize, theme.Text, d.title)
	cursor += titleH
	c.rect(x, cursor, width, 1, theme.Border)
	cursor++
	for i, line := range lines {
		c.rowText(x+m.PanelPad, cursor+m.Grid/2+i*m.RowHeight, m.RowHeight, m.BodySize, theme.TextDim, line)
	}
	cursor += bodyH
	c.rect(x, cursor, width, 1, theme.Border)
	cursor++

	// 按鈕靠右，接受鍵在最右邊：確認對話的預設落點是「取消」，
	// 要往右移動才會碰到破壞性的那一個。
	buttonY := cursor + m.Grid
	buttonW := max(
		c.font.Measure(u.s.Cancel, m.BodySize),
		c.font.Measure(d.accept, m.BodySize),
	) + m.RowPadX*2
	right := x + width - m.PanelPad
	u.drawButton(c, right-buttonW, buttonY, buttonW, m.RowHeight, d.accept, d.focused == 1)
	u.drawButton(c, right-buttonW*2-m.Grid, buttonY, buttonW, m.RowHeight, u.s.Cancel, d.focused == 0)

	// modal 的兩個按鈕最後登記，才會蓋過底下畫面的可點區域。
	u.addHit(right-buttonW, buttonY, buttonW, m.RowHeight,
		func(*UI) { d.focused = 1 },
		func(u *UI) { d.focused = 1; d.activate(u) })
	u.addHit(right-buttonW*2-m.Grid, buttonY, buttonW, m.RowHeight,
		func(*UI) { d.focused = 0 },
		func(u *UI) { d.focused = 0; d.activate(u) })
}

func (u *UI) drawButton(c *canvas, x, y, w, h int, label string, focused bool) {
	text := u.theme.Text
	if focused {
		c.rect(x, y, w, h, u.theme.Focus)
		c.rect(x, y, 4, h, u.theme.FocusText)
		text = u.theme.FocusText
	} else {
		c.border(x, y, w, h, u.theme.Border)
	}
	c.textCenter(x, y+(h-c.font.Height(u.metrics.BodySize))/2, w, u.metrics.BodySize, text, label)
}

// wrapText 以字為單位換行。點陣字沒有連字，半寬與全寬混排時逐字量寬度最單純，
// 也不會因為語言不同而換出不一樣的結果。
func wrapText(font *Font, text string, size, limit int) []string {
	if limit <= 0 {
		return []string{text}
	}
	var lines []string
	var line []rune
	width := 0
	for _, r := range text {
		advance := font.Advance(r) * size
		if width+advance > limit && len(line) > 0 {
			lines = append(lines, string(line))
			line, width = line[:0], 0
		}
		line = append(line, r)
		width += advance
	}
	if len(line) > 0 || len(lines) == 0 {
		lines = append(lines, string(line))
	}
	return lines
}

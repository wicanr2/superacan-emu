package ui

// languageScreen 是 S5.5。選單一律以各語言自己的寫法列出，使用者才找得到自己的
// 語言——用目前語言去翻譯其他語言的名稱，看不懂的人還是看不懂。
type languageScreen struct{ focus int }

func (l *languageScreen) id() string { return "S5.5" }

func (l *languageScreen) handle(u *UI, ev Event) bool {
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			l.focus = moveFocus(l.focus, len(Languages), -1)
		case DirDown:
			l.focus = moveFocus(l.focus, len(Languages), +1)
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			// 切換立即生效：畫面每一幀都從 u.s 取字，沒有任何一份翻譯被快取。
			u.SetLanguage(Languages[l.focus])
			u.emit(ApplyConfig{Config: u.config})
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

func (l *languageScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	top, _ := page{title: u.s.SettingsLanguage, back: true}.draw(u, c)
	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	y := top
	for index, language := range Languages {
		colour := u.focusRow(c, x, y, width, index == l.focus)
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, colour, LanguageNames[language])
		mark := ""
		if language == u.language {
			mark = "●"
		}
		c.rowText(x+width/3, y, m.RowHeight, m.BodySize, colour, mark)
		c.rowText(x+width/2, y, m.RowHeight, m.SmallSize, colour, string(language))
		y += m.RowHeight
	}
}

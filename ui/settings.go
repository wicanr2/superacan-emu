package ui

import "fmt"

// settingsScreen 是 S5 設定總表。影像、音訊與語言在後面的階段才做，
// 這裡先以停用項出現並說明原因——把它們藏起來會讓使用者以為功能不存在。
type settingsScreen struct{ focus int }

func (s *settingsScreen) id() string { return "S5" }

func (s *settingsScreen) rows(u *UI) []menuRow {
	return []menuRow{
		{label: u.s.SettingsInput, chevron: true, action: func(u *UI) {
			u.push(&bindingScreen{})
		}},
		{label: u.s.SettingsHotkeys, chevron: true, action: func(u *UI) {
			u.push(&hotkeyScreen{})
		}},
		{label: u.s.SettingsVideo, chevron: true, action: func(u *UI) { u.push(&videoScreen{}) }},
		{label: u.s.SettingsAudio, chevron: true, action: func(u *UI) { u.push(&audioScreen{}) }},
		{label: u.s.SettingsLanguage, chevron: true, action: func(u *UI) { u.push(&languageScreen{}) }},
		{label: u.s.TouchTitle, chevron: true, action: func(u *UI) { u.push(&touchScreen{}) }},
	}
}

func (s *settingsScreen) handle(u *UI, ev Event) bool {
	if handleMenu(u, ev, &s.focus, s.rows(u)) {
		return true
	}
	if e, ok := ev.(Action); ok && e.Kind == ActCancel {
		u.pop()
		return true
	}
	return false
}

func (s *settingsScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	top, _ := page{title: u.s.SettingsTitle, back: true}.draw(u, c)
	drawMenuRows(u, c, m.PanelPad, top, c.width()-m.PanelPad*2, s.rows(u), &s.focus)
}

// bindingRow 是綁定畫面上的一列。
type bindingRow struct {
	key      string
	label    string
	keyboard Binding
	gamepad  Binding
	conflict string
}

// conflictsIn 找出同一欄裡指向同一個實體輸入的列。同一個鍵綁兩個動作不是錯誤，
// 但使用者一定要看得到，否則會以為某個按鈕壞了。
//
// 標記用「※」而不是「⚠」：嵌入的字型沒有 U+26A0，會畫成替換字元。
func conflictsIn(strings Strings, rows []bindingRow, pick func(bindingRow) Binding) map[int]string {
	seen := make(map[[2]interface{}][]int)
	for index, row := range rows {
		binding := pick(row)
		if binding.Empty() {
			continue
		}
		key := [2]interface{}{binding.Frontend, binding.Code}
		seen[key] = append(seen[key], index)
	}
	out := make(map[int]string)
	for _, indexes := range seen {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			for _, other := range indexes {
				if other == index {
					continue
				}
				out[index] = fmt.Sprintf(strings.ConflictWith, rows[other].label)
				break
			}
		}
	}
	return out
}

// bindingScreen 是 S5.1 輸入綁定。
type bindingScreen struct {
	player  int
	gamepad bool
	focus   int
	top     int
	waiting bool
}

func (b *bindingScreen) id() string { return "S5.1" }

func (b *bindingScreen) capturing() bool { return b.waiting }

func (b *bindingScreen) rows(u *UI) []bindingRow {
	player := u.config.Input.Players[b.player]
	rows := make([]bindingRow, 0, len(PadButtons))
	for _, key := range PadButtons {
		rows = append(rows, bindingRow{
			key: key, label: padButtonLabels[key],
			keyboard: player.Keyboard[key], gamepad: player.Gamepad[key],
		})
	}
	return rows
}

func (b *bindingScreen) assign(u *UI, binding Binding) {
	rows := b.rows(u)
	if b.focus >= len(rows) {
		return
	}
	key := rows[b.focus].key
	target := u.config.Input.Players[b.player].Keyboard
	if b.gamepad {
		target = u.config.Input.Players[b.player].Gamepad
	}
	if target == nil {
		target = map[string]Binding{}
		if b.gamepad {
			u.config.Input.Players[b.player].Gamepad = target
		} else {
			u.config.Input.Players[b.player].Keyboard = target
		}
	}
	target[key] = binding
	b.waiting = false
	u.emit(ApplyConfig{Config: u.config})
}

func (b *bindingScreen) handle(u *UI, ev Event) bool {
	rows := b.rows(u)
	if b.waiting {
		switch e := ev.(type) {
		case RawKey:
			if !b.gamepad {
				b.assign(u, Binding{Frontend: e.Frontend, Code: e.Code, Label: e.Label})
			}
			return true
		case RawPad:
			if b.gamepad {
				b.assign(u, Binding{Frontend: "pad", Code: uint32(e.Button), Label: e.Label})
			}
			return true
		case Action:
			if e.Kind == ActCancel {
				b.waiting = false
			}
			return true
		case Life:
			if e.Kind == LifeBack {
				b.waiting = false
			}
			return true
		}
		return true
	}
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			b.focus = moveFocus(b.focus, len(rows), -1)
		case DirDown:
			b.focus = moveFocus(b.focus, len(rows), +1)
		case DirLeft, DirRight:
			b.gamepad = !b.gamepad
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			b.waiting = true
			return true
		case ActDelete:
			if b.focus < len(rows) {
				key := rows[b.focus].key
				delete(u.config.Input.Players[b.player].Keyboard, key)
				delete(u.config.Input.Players[b.player].Gamepad, key)
				u.emit(ApplyConfig{Config: u.config})
			}
			return true
		case ActTabPrev, ActTabNext:
			b.player = 1 - b.player
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

func (b *bindingScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	rows := b.rows(u)
	keyboardConflicts := conflictsIn(u.s, rows, func(r bindingRow) Binding { return r.keyboard })
	gamepadConflicts := conflictsIn(u.s, rows, func(r bindingRow) Binding { return r.gamepad })

	top, _ := page{
		title:  u.s.InputTitle,
		back:   true,
		right:  fmt.Sprintf("P%d · %s", b.player+1, deviceLabel(u.s, b.gamepad)),
		status: u.s.InputHelp,
	}.draw(u, c)

	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	columnKeyboard := x + width/4
	columnGamepad := x + width/2
	columnNote := x + width*3/4

	c.rowText(x+m.RowPadX, top, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.ColumnButton)
	c.rowText(columnKeyboard, top, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.ColumnKeyboard)
	c.rowText(columnGamepad, top, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.ColumnGamepad)
	y := top + m.RowHeight
	listTop := y
	first, last := listWindow(&b.top, b.focus, len(rows), c.height()-m.FooterBar-y, m.RowHeight)

	for index := first; index < last; index++ {
		row := rows[index]
		colour := u.focusRow(c, x, y, width, index == b.focus)
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, colour, row.label)

		keyboardText := bindingText(u.s, row.keyboard)
		gamepadText := bindingText(u.s, row.gamepad)
		if index == b.focus && b.waiting {
			if b.gamepad {
				gamepadText = u.s.PressInput
			} else {
				keyboardText = u.s.PressInput
			}
		}
		c.rowText(columnKeyboard, y, m.RowHeight, m.SmallSize, colour, keyboardText)
		c.rowText(columnGamepad, y, m.RowHeight, m.SmallSize, colour, gamepadText)

		note := keyboardConflicts[index]
		if note == "" {
			note = gamepadConflicts[index]
		}
		if note != "" {
			c.rowText(columnNote, y, m.RowHeight, m.SmallSize, u.theme.Warn, "※ "+note)
		}
		if !b.waiting {
			target := index
			u.addHit(x, y, width, m.RowHeight,
				func(*UI) { b.focus = target },
				func(*UI) { b.focus, b.waiting = target, true })
		}
		y += m.RowHeight
	}
	u.listScrollHint(c, x, listTop, width, y-listTop, first, last, len(rows))
}

// hotkeyScreen 是 S5.2 熱鍵設定。
type hotkeyScreen struct {
	focus   int
	top     int
	waiting bool
}

func (h *hotkeyScreen) id() string { return "S5.2" }

func (h *hotkeyScreen) capturing() bool { return h.waiting }

func (h *hotkeyScreen) rows(u *UI) []bindingRow {
	rows := make([]bindingRow, 0, len(Hotkeys))
	for _, key := range Hotkeys {
		rows = append(rows, bindingRow{
			key: key, label: hotkeyLabels[key], keyboard: u.hotkeyBinding(key),
		})
	}
	return rows
}

func (h *hotkeyScreen) handle(u *UI, ev Event) bool {
	rows := h.rows(u)
	if h.waiting {
		switch e := ev.(type) {
		case RawKey:
			if u.config.Input.Hotkeys == nil {
				u.config.Input.Hotkeys = map[string]Binding{}
			}
			u.config.Input.Hotkeys[rows[h.focus].key] = Binding{
				Frontend: e.Frontend, Code: e.Code, Label: e.Label,
			}
			h.waiting = false
			u.emit(ApplyConfig{Config: u.config})
			return true
		case Action:
			if e.Kind == ActCancel {
				h.waiting = false
			}
			return true
		}
		return true
	}
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			h.focus = moveFocus(h.focus, len(rows), -1)
		case DirDown:
			h.focus = moveFocus(h.focus, len(rows), +1)
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			h.waiting = true
			return true
		case ActDelete:
			delete(u.config.Input.Hotkeys, rows[h.focus].key)
			u.emit(ApplyConfig{Config: u.config})
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

func (h *hotkeyScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	rows := h.rows(u)
	conflicts := conflictsIn(u.s, rows, func(r bindingRow) Binding { return r.keyboard })
	status := u.s.HotkeyHelp
	if len(conflicts) > 0 {
		status = u.s.HotkeyConflict
	}
	top, _ := page{title: u.s.HotkeyTitle, back: true, status: status}.draw(u, c)

	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	columnKeyboard := x + width/2
	columnNote := x + width*3/4

	c.rowText(x+m.RowPadX, top, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.ColumnAction)
	c.rowText(columnKeyboard, top, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.ColumnKeyboard)
	y := top + m.RowHeight
	listTop := y
	first, last := listWindow(&h.top, h.focus, len(rows), c.height()-m.FooterBar-y, m.RowHeight)

	for index := first; index < last; index++ {
		row := rows[index]
		colour := u.focusRow(c, x, y, width, index == h.focus)
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, colour, row.label)
		text := bindingText(u.s, row.keyboard)
		if index == h.focus && h.waiting {
			text = u.s.PressInput
		}
		c.rowText(columnKeyboard, y, m.RowHeight, m.SmallSize, colour, text)
		if note := conflicts[index]; note != "" {
			c.rowText(columnNote, y, m.RowHeight, m.SmallSize, u.theme.Warn, "※ "+note)
		}
		// 等待指定綁定時不登記：那個狀態在等鍵盤，這時點下去只會多送一次進入等待。
		if !h.waiting {
			target := index
			u.addHit(x, y, width, m.RowHeight,
				func(*UI) { h.focus = target },
				func(*UI) { h.focus, h.waiting = target, true })
		}
		y += m.RowHeight
	}
	u.listScrollHint(c, x, listTop, width, y-listTop, first, last, len(rows))
}

func bindingText(strings Strings, binding Binding) string {
	if binding.Empty() {
		return strings.None
	}
	if binding.Label != "" {
		return binding.Label
	}
	return fmt.Sprintf("#%d", binding.Code)
}

func deviceLabel(strings Strings, gamepad bool) string {
	if gamepad {
		return strings.ColumnGamepad
	}
	return strings.ColumnKeyboard
}

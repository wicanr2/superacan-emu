package ui

import "fmt"

// startScreen 是 S0 無卡帶啟動畫面。韌體四項缺任一項時，選擇卡帶與最近清單全部
// 停用——停用項仍可聚焦，按下確認會說明原因，這樣使用者知道條件是什麼。
type startScreen struct{ focus int }

func (s *startScreen) id() string { return "S0" }

func (s *startScreen) rows(u *UI) []menuRow {
	ready := u.firmwareReady()
	reason := u.s.FirmwareIncompl
	rows := []menuRow{
		{label: u.s.FirmwareSetUp, action: func(u *UI) { u.push(&firmwareScreen{}) }},
	}
	for _, entry := range u.recentEntries() {
		entry := entry
		row := menuRow{label: entry.Name, value: shortHash(entry.SHA256)}
		switch {
		case entry.Missing:
			row.disabled, row.reason, row.value = true, u.s.MissingFile, u.s.MissingFile
		case !ready:
			row.disabled, row.reason = true, reason
		default:
			row.action = func(u *UI) { u.emit(LoadCartridge{Path: entry.Path}) }
		}
		rows = append(rows, row)
	}
	rows = append(rows,
		menuRow{label: u.s.ChooseCartridge, gapBefore: true, disabled: !ready, reason: reason,
			action: func(u *UI) { u.push(&browserScreen{}) }},
		menuRow{label: u.s.About, action: func(u *UI) { u.push(&aboutScreen{}) }},
		menuRow{label: u.s.Quit, action: func(u *UI) { u.emit(Quit{}) }},
	)
	return rows
}

func (s *startScreen) handle(u *UI, ev Event) bool {
	return handleMenu(u, ev, &s.focus, s.rows(u))
}

func (s *startScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	status := u.s.StatusNoCartridge
	if u.firmwareReady() {
		status = u.s.StatusReady
	}
	top, _ := page{title: u.s.AppTitle, status: status}.draw(u, c)

	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	y := u.sectionTitle(c, x, top, u.s.SectionFirmware)

	entries := u.firmwareEntries()
	panelTop := y
	y = u.framedRows(c, x, y, width, len(entries)) - 1
	rowY := panelTop + 1
	for _, entry := range entries {
		mark, state, colour := "○", u.s.FirmwareMissing, u.theme.Error
		if entry.Loaded {
			mark, state, colour = "●", u.s.FirmwareLoaded, u.theme.OK
		}
		c.rowText(x+m.RowPadX, rowY, m.RowHeight, m.BodySize, colour, mark)
		c.rowText(x+m.RowPadX+m.SectionGap, rowY, m.RowHeight, m.BodySize, u.theme.Text, u.firmwareLabel(entry.Kind))
		detail := u.s.FirmwareUnset
		if entry.Loaded {
			detail = shortHash(entry.SHA256)
		}
		c.rowText(x+width/2, rowY, m.RowHeight, m.SmallSize, u.theme.TextDim, detail)
		c.rowText(x+width-m.RowPadX-c.font.Measure(state, m.BodySize), rowY, m.RowHeight, m.BodySize, colour, state)
		rowY += m.RowHeight
	}

	// 版面分成三段，但焦點是同一條線性順序：韌體設定、最近清單、底部動作。
	rows := s.rows(u)
	recent := u.recentEntries()
	y += m.Grid
	y = drawMenuRows(u, c, x, y, width, rows[:1], s.focus)

	y += m.SectionGap
	y = u.sectionTitle(c, x, y, fmt.Sprintf("%s（%d）", u.s.SectionRecent, len(recent)))
	y = drawMenuRows(u, c, x, y, width, rows[1:1+len(recent)], s.focus-1)

	y += m.SectionGap
	drawMenuRows(u, c, x, y, width, rows[1+len(recent):], s.focus-1-len(recent))
}

// firmwareScreen 是 S0.1。
type firmwareScreen struct{ focus int }

func (f *firmwareScreen) id() string { return "S0.1" }

func (f *firmwareScreen) handle(u *UI, ev Event) bool {
	entries := u.firmwareEntries()
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			f.focus = moveFocus(f.focus, len(entries), -1)
		case DirDown:
			f.focus = moveFocus(f.focus, len(entries), +1)
		}
		return true
	case Action:
		switch e.Kind {
		case ActCancel:
			u.pop()
			return true
		case ActConfirm:
			// 選檔案要平台的檔案對話框，目前入口以命令列旗標提供路徑；
			// 在那條路徑接上之前不假裝這個按鈕有作用。
			u.toast(fmt.Sprintf(u.s.NotYet, u.s.FirmwareTitle), SeverityWarn)
			return true
		}
	}
	return false
}

func (f *firmwareScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	status := ""
	if !u.firmwareReady() {
		status = u.s.FirmwareIncompl
	}
	top, _ := page{title: u.s.FirmwareTitle, back: true, status: status}.draw(u, c)

	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	y := u.sectionTitle(c, x, top, u.s.FirmwareNotice)
	y += m.Grid

	for index, entry := range u.firmwareEntries() {
		colour := u.focusRow(c, x, y, width, index == f.focus)
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, colour, u.firmwareLabel(entry.Kind))
		path := entry.Path
		if path == "" {
			path = u.s.FirmwareUnset
		}
		c.rowText(x+width/3, y, m.RowHeight, m.SmallSize, colour, path)
		y += m.RowHeight

		detail := u.s.None
		if entry.Loaded {
			detail = fmt.Sprintf("%s %s · SHA-256 %s", u.s.FieldSize, groupInt(entry.Size), shortHash(entry.SHA256))
		}
		c.rowText(x+m.RowPadX+m.SectionGap, y, m.RowHeight, m.SmallSize, u.theme.TextDim, detail)
		known, knownColour := u.s.FirmwareUnlisted, u.theme.Warn
		switch {
		case !entry.Loaded:
			known, knownColour = u.s.FirmwareMissing, u.theme.Error
		case entry.Known:
			known, knownColour = u.s.FirmwareKnown, u.theme.OK
		}
		c.rowText(x+width-m.RowPadX-c.font.Measure(known, m.SmallSize), y, m.RowHeight, m.SmallSize, knownColour, known)
		y += m.RowHeight + m.Grid
	}
}

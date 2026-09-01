package ui

import "fmt"

// browserScreen 是 S1 卡帶瀏覽器。右側面板刻意沒有遊戲畫面預覽：
// 本程式不散布遊戲畫面，瀏覽器也不顯示。
type browserScreen struct {
	focus int
	top   int
}

func (b *browserScreen) id() string { return "S1" }

func (b *browserScreen) entries(u *UI) []CartridgeEntry {
	if u.library == nil {
		return nil
	}
	return u.library.Cartridges()
}

func (b *browserScreen) handle(u *UI, ev Event) bool {
	entries := b.entries(u)
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			b.focus = moveFocus(b.focus, len(entries), -1)
		case DirDown:
			b.focus = moveFocus(b.focus, len(entries), +1)
		}
		return true
	case Edge:
		if e.To == EdgeHome {
			b.focus = 0
		} else if len(entries) > 0 {
			b.focus = len(entries) - 1
		}
		return true
	case Action:
		switch e.Kind {
		case ActCancel:
			u.pop()
			return true
		case ActConfirm:
			if b.focus >= len(entries) {
				return true
			}
			entry := entries[b.focus]
			if entry.Missing {
				u.fail(u.s.MissingFile)
				return true
			}
			if !u.firmwareReady() {
				u.toast(fmt.Sprintf(u.s.NotYet, u.s.FirmwareIncompl), SeverityWarn)
				return true
			}
			u.emit(LoadCartridge{Path: entry.Path})
			return true
		}
	}
	return false
}

func (b *browserScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	directory := ""
	if u.library != nil {
		directory = u.library.Directory()
	}
	entries := b.entries(u)
	top, height := page{
		title: u.s.BrowserTitle, back: true, right: directory,
		status: u.s.BrowserKeys,
	}.draw(u, c)

	listWidth := c.width()/2 - m.PanelPad
	x := m.PanelPad
	if len(entries) == 0 {
		c.rowText(x, top, m.RowHeight, m.BodySize, u.theme.TextDim, u.s.BrowserEmpty)
		return
	}

	// 清單只畫得下的部分，焦點永遠在可視範圍內。
	visible := height / m.RowHeight
	if visible < 1 {
		visible = 1
	}
	if b.focus < b.top {
		b.top = b.focus
	}
	if b.focus >= b.top+visible {
		b.top = b.focus - visible + 1
	}
	y := top
	for index := b.top; index < len(entries) && index < b.top+visible; index++ {
		entry := entries[index]
		colour := u.focusRow(c, x, y, listWidth, index == b.focus)
		if entry.Missing && index != b.focus {
			colour = u.theme.TextOff
		}
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, colour, entry.Name)
		y += m.RowHeight
		for _, part := range entry.Parts {
			c.rowText(x+m.RowPadX+m.SectionGap, y, m.RowHeight, m.SmallSize, u.theme.TextDim,
				fmt.Sprintf("%s  %s", part.Name, groupInt(part.Size)))
			y += m.RowHeight
		}
	}

	// 右側細節面板。
	detailX := c.width()/2 + m.Grid
	detailW := c.width() - detailX - m.PanelPad
	entry := entries[min(b.focus, len(entries)-1)]
	dy := top
	c.rowText(detailX, dy, m.RowHeight, m.BodySize, u.theme.Text, entry.Name)
	dy += m.RowHeight + m.Grid

	previewH := m.RowHeight * 4
	c.rect(detailX, dy, detailW, previewH, u.theme.PanelAlt)
	c.border(detailX, dy, detailW, previewH, u.theme.Border)
	c.textCenter(detailX, dy+(previewH-c.font.Height(m.SmallSize))/2, detailW, m.SmallSize,
		u.theme.TextOff, u.s.BrowserNoPreview)
	dy += previewH + m.Grid

	compat := u.s.CompatUnverified
	compatColour := u.theme.Warn
	if entry.Verified {
		compat, compatColour = u.s.CompatVerified, u.theme.OK
	}
	battery := u.s.None
	if entry.Battery > 0 {
		battery = groupInt(entry.Battery)
	}
	saves := u.s.None
	if len(entry.SaveSlots) > 0 {
		saves = ""
		for i, slot := range entry.SaveSlots {
			if i > 0 {
				saves += "、"
			}
			saves += fmt.Sprintf("%s%d", u.s.SlotPrefix, slot)
		}
	}
	for _, field := range []struct {
		label  string
		value  string
		colour rgba
	}{
		{u.s.FieldSize, groupInt(entry.Size), u.theme.Text},
		{u.s.FieldKind, entry.Kind, u.theme.Text},
		{u.s.FieldSHA, shortHash(entry.SHA256), u.theme.TextDim},
		{u.s.FieldSaves, saves, u.theme.TextDim},
		{u.s.FieldBattery, battery, u.theme.TextDim},
		{u.s.FieldCompat, compat, compatColour},
	} {
		c.rowText(detailX, dy, m.RowHeight, m.SmallSize, u.theme.TextDim, field.label)
		c.rowText(detailX+m.SectionGap*3, dy, m.RowHeight, m.SmallSize, field.colour, field.value)
		dy += m.RowHeight
	}
}

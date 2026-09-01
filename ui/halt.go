package ui

import "fmt"

// haltScreen 是 S9 停機畫面。未知或不支援的硬體操作一律 fail-closed：
// 執行不繼續，機器狀態保留供檢視，而且這個畫面不能用返回鍵略過。
type haltScreen struct{ focus int }

func (h *haltScreen) id() string { return "S9" }

func (h *haltScreen) rows(u *UI) []menuRow {
	return []menuRow{
		{label: u.s.SaveState, action: func(u *UI) {
			u.push(&slotsScreen{mode: slotModeSave, focus: u.config.Interface.SaveSlot})
		}},
		{label: u.s.EjectToShell, action: func(u *UI) { u.emit(UnloadCartridge{}) }},
		{label: u.s.Quit, action: func(u *UI) { u.emit(Quit{}) }},
	}
}

func (h *haltScreen) handle(u *UI, ev Event) bool {
	if handleMenu(u, ev, &h.focus, h.rows(u)) {
		return true
	}
	// 取消與返回都不關閉停機畫面：停機不能被略過。
	return true
}

func (h *haltScreen) draw(u *UI, c *canvas, snap Snapshot) {
	m := u.metrics
	theme := u.theme
	rows := h.rows(u)
	width := c.width() * 2 / 3
	titleH := m.RowHeight
	facts := u.haltFacts(snap)
	body := wrapText(c.font, u.haltNote, m.BodySize, width-m.PanelPad*2)
	height := titleH + 1 + m.RowHeight*(len(body)+len(facts)+1) + m.Grid*2 + 1 + m.RowHeight*len(rows)
	x := (c.width() - width) / 2
	y := (c.height() - height) / 2

	c.rect(0, 0, c.width(), c.height(), theme.ScrimHalt)
	c.rect(x, y, width, height, theme.Panel)
	c.border(x, y, width, height, theme.Error)

	c.rowText(x+m.PanelPad, y, titleH, m.BodySize, theme.Error, u.s.HaltTitle)
	cursor := y + titleH
	c.rect(x, cursor, width, 1, theme.Border)
	cursor++
	for _, line := range body {
		c.rowText(x+m.PanelPad, cursor, m.RowHeight, m.BodySize, theme.Text, line)
		cursor += m.RowHeight
	}
	c.rowText(x+m.PanelPad, cursor, m.RowHeight, m.BodySize, theme.TextDim, u.s.HaltBody)
	cursor += m.RowHeight + m.Grid
	for _, fact := range facts {
		c.rowText(x+m.PanelPad, cursor, m.RowHeight, m.SmallSize, theme.TextDim, fact.label)
		c.rowText(x+m.PanelPad+m.SectionGap*4, cursor, m.RowHeight, m.SmallSize, theme.Text, fact.value)
		cursor += m.RowHeight
	}
	cursor += m.Grid
	c.rect(x, cursor, width, 1, theme.Border)
	drawMenuRows(u, c, x, cursor+1, width, rows, h.focus)
}

type haltFact struct{ label, value string }

func (u *UI) haltFacts(snap Snapshot) []haltFact {
	if snap == nil {
		return nil
	}
	instructions, _ := snap.Instructions()
	name, sum, _ := snap.Cartridge()
	firmware := snap.Firmware()
	return []haltFact{
		{u.s.HaltFrame, group(snap.FrameIndex())},
		{u.s.HaltInstructions, group(instructions)},
		{u.s.HaltCartridge, fmt.Sprintf("%s  %s", name, shortHash(sum))},
		{u.s.HaltIPL, shortHash(firmware.IPL)},
	}
}

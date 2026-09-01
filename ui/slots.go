package ui

import "fmt"

type slotMode uint8

const (
	slotModeSave slotMode = iota
	slotModeLoad
)

// SlotCount 是存檔槽數量，與 Bcan 相同。
const SlotCount = 10

const slotColumns = 5

// slotsScreen 是 S4 存檔槽。拒絕標示在畫面上就顯示而不是等按下讀檔才報錯：
// ACANGOS1 的標頭只讀前 128 位元組就能驗完，payload 雜湊留到實際讀取時驗，
// 所以使用者在選之前就知道哪些槽不能用，交易式載入的保證不變。
type slotsScreen struct {
	mode  slotMode
	focus int
}

func (s *slotsScreen) id() string {
	if s.mode == slotModeSave {
		return "S4.save"
	}
	return "S4.load"
}

func (s *slotsScreen) handle(u *UI, ev Event) bool {
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirLeft:
			s.focus = rowWrap(s.focus, -1)
		case DirRight:
			s.focus = rowWrap(s.focus, +1)
		case DirUp, DirDown:
			s.focus = (s.focus + SlotCount/2) % SlotCount
		}
		return true
	case Edge:
		if e.To == EdgeHome {
			s.focus = 0
		} else {
			s.focus = SlotCount - 1
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			s.activate(u)
			return true
		case ActDelete:
			s.remove(u)
			return true
		case ActTabPrev, ActTabNext:
			s.mode = slotMode(1 - s.mode)
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

// rowWrap 讓左右在同一列的五格內環繞；跨列要用上下鍵，這樣使用者不會在
// 移動中失去「現在在哪一列」的感覺。
func rowWrap(focus, delta int) int {
	row := focus / slotColumns
	col := (focus%slotColumns + delta + slotColumns) % slotColumns
	return row*slotColumns + col
}

func (s *slotsScreen) activate(u *UI) {
	slot := s.focus
	info := u.slotInfo(slot)
	u.config.Interface.SaveSlot = slot
	if s.mode == slotModeSave {
		if info.Present {
			u.modal = &confirm{
				title:  fmt.Sprintf(textOverwriteAsk, slot),
				body:   fmt.Sprintf(textOverwriteWhy, slot, info.Stamp, info.Frame),
				accept: textOverwrite,
				onYes: func(u *UI) {
					u.emit(SaveState{Slot: slot})
					u.toast(fmt.Sprintf(textSaved, slot), SeverityInfo)
					u.pop()
				},
			}
			return
		}
		u.emit(SaveState{Slot: slot})
		u.toast(fmt.Sprintf(textSaved, slot), SeverityInfo)
		u.pop()
		return
	}
	switch {
	case info.Rejected:
		u.fail(fmt.Sprintf(textSlotRejected, slot, info.Reason))
	case !info.Present:
		u.toast(fmt.Sprintf(textSlotEmpty, slot), SeverityWarn)
	default:
		u.emit(LoadState{Slot: slot})
		u.toast(fmt.Sprintf(textLoaded, slot), SeverityInfo)
		u.Close()
	}
}

func (s *slotsScreen) remove(u *UI) {
	slot := s.focus
	info := u.slotInfo(slot)
	if !info.Present && !info.Rejected {
		u.toast(fmt.Sprintf(textSlotEmpty, slot), SeverityWarn)
		return
	}
	u.modal = &confirm{
		title:  fmt.Sprintf(textDeleteAsk, slot),
		body:   fmt.Sprintf(textDeleteWhy, slot, info.Stamp),
		accept: textDelete,
		onYes: func(u *UI) {
			u.emit(DeleteState{Slot: slot})
			u.toast(fmt.Sprintf(textDeleted, slot), SeverityInfo)
		},
	}
}

func (s *slotsScreen) draw(u *UI, c *canvas, snap Snapshot) {
	m := u.metrics
	theme := u.theme
	u.fillPage(c)

	// 標題列
	c.rowText(m.PanelPad, 0, m.TitleBar, m.BodySize, theme.TextDim, textBack)
	title := textSlotsTitle
	if snap != nil {
		if name, _, _ := snap.Cartridge(); name != "" {
			title = textSlotsTitle + " — " + name
		}
	}
	c.textCenter(0, (m.TitleBar-c.font.Height(m.TitleSize))/2, c.width(), m.TitleSize, theme.Text, title)
	u.drawModeTabs(c, m.TitleBar, s.mode)
	c.rect(0, m.TitleBar, c.width(), 1, theme.Border)

	// 縮圖格線：payload 內的 framebuffer 是 320×240，等比縮到 160×120 是整數 1/2。
	thumbW, thumbH := 160, 120
	gap := m.SectionGap
	gridW := slotColumns*thumbW + (slotColumns-1)*gap
	originX := (c.width() - gridW) / 2
	originY := m.TitleBar + 1 + m.PanelPad
	cellH := thumbH + m.RowHeight*2 + m.Grid

	for slot := 0; slot < SlotCount; slot++ {
		info := u.slotInfo(slot)
		col, row := slot%slotColumns, slot/slotColumns
		x := originX + col*(thumbW+gap)
		y := originY + row*(cellH+gap)

		if slot == s.focus {
			c.rect(x-4, y-4, thumbW+8, thumbH+8, theme.Focus)
		}
		c.rect(x, y, thumbW, thumbH, theme.PanelAlt)
		switch {
		case info.Rejected:
			c.textCenter(x, y+thumbH/2, thumbW, m.BodySize, theme.Error, "✖ "+info.Reason)
		case !info.Present:
			c.textCenter(x, y+thumbH/2, thumbW, m.BodySize, theme.TextOff, textEmptySlot)
		default:
			c.blitScaled(x, y, thumbW, thumbH, info.Thumb)
		}
		c.border(x, y, thumbW, thumbH, theme.Border)

		label := fmt.Sprintf("%s%d", textSlotPrefix, slot)
		mark, markColor := "", theme.TextDim
		switch {
		case info.Rejected:
			mark, markColor = " ✖", theme.Error
		case info.Present:
			mark, markColor = " ✔", theme.OK
		}
		c.rowText(x, y+thumbH, m.RowHeight, m.BodySize, theme.Text, label)
		if mark != "" {
			c.rowText(x+c.font.Measure(label, m.BodySize), y+thumbH, m.RowHeight, m.BodySize, markColor, mark)
		}
		detail := "—"
		if info.Present {
			detail = info.Stamp
		} else if info.Rejected {
			detail = info.Reason
		}
		c.rowText(x, y+thumbH+m.RowHeight, m.RowHeight, m.SmallSize, theme.TextDim, detail)
	}

	// 底部說明
	footerY := c.height() - m.FooterBar*2 - m.Grid
	if u.errorText != "" {
		footerY -= m.FooterBar
	}
	c.rect(0, footerY, c.width(), 1, theme.Border)
	c.rowText(m.PanelPad, footerY, m.FooterBar, m.BodySize, theme.TextDim, textSlotLegend)
	c.rowText(m.PanelPad, footerY+m.FooterBar, m.FooterBar, m.BodySize, theme.TextDim, textSlotKeys)
}

func (u *UI) drawModeTabs(c *canvas, barHeight int, mode slotMode) {
	m := u.metrics
	labels := [2]string{textModeSave, textModeLoad}
	width := 0
	for _, label := range labels {
		width = max(width, c.font.Measure(label, m.BodySize)+m.RowPadX*2)
	}
	x := c.width() - m.PanelPad - width*2
	y := (barHeight - m.RowHeight) / 2
	for i, label := range labels {
		selected := slotMode(i) == mode
		text := u.theme.TextDim
		if selected {
			c.rect(x+i*width, y, width, m.RowHeight, u.theme.Focus)
			text = u.theme.FocusText
		}
		c.textCenter(x+i*width, y+(m.RowHeight-c.font.Height(m.BodySize))/2, width, m.BodySize, text, label)
	}
}

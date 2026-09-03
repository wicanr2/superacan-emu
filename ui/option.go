package ui

import "fmt"

// optionKind 決定一列設定怎麼呈現與怎麼調整。
type optionKind uint8

const (
	// optionChoice 是在固定選項之間循環，左右鍵切換。
	optionChoice optionKind = iota
	// optionToggle 是開關。
	optionToggle
	// optionRange 是數值範圍，左右鍵加減 Step。
	optionRange
	// optionReadOnly 是只讀事實。
	optionReadOnly
)

// optionRow 是設定畫面上的一列。
//
// 停用項仍然出現而且可以聚焦：把做不到的功能藏起來，使用者會以為是自己找不到；
// 顯示出來並說明原因，才知道那是本前端的限制還是尚未實作。
type optionRow struct {
	kind     optionKind
	label    string
	note     string
	disabled bool
	reason   string

	// optionChoice
	choices []string
	index   *int
	// optionToggle
	flag *bool
	// optionRange
	value    *int
	min, max int
	step     int
	unit     string
	bar      bool
	// optionReadOnly
	text string
}

// adjust 依方向調整這一列的值，回傳是否真的改變了。
func (o optionRow) adjust(delta int) bool {
	if o.disabled {
		return false
	}
	switch o.kind {
	case optionChoice:
		if len(o.choices) == 0 || o.index == nil {
			return false
		}
		*o.index = ((*o.index+delta)%len(o.choices) + len(o.choices)) % len(o.choices)
		return true
	case optionToggle:
		if o.flag == nil {
			return false
		}
		*o.flag = !*o.flag
		return true
	case optionRange:
		if o.value == nil {
			return false
		}
		step := o.step
		if step == 0 {
			step = 1
		}
		next := *o.value + delta*step
		if next < o.min {
			next = o.min
		}
		if next > o.max {
			next = o.max
		}
		if next == *o.value {
			return false
		}
		*o.value = next
		return true
	}
	return false
}

// display 是右欄要顯示的字串。
func (o optionRow) display(s Strings) string {
	switch o.kind {
	case optionChoice:
		if o.index == nil || len(o.choices) == 0 {
			return s.None
		}
		return "‹ " + o.choices[*o.index] + " ›"
	case optionToggle:
		if o.flag != nil && *o.flag {
			return "[■] " + s.On
		}
		return "[ ] " + s.Off
	case optionRange:
		if o.value == nil {
			return s.None
		}
		if o.bar {
			return fmt.Sprintf("‹ %s %d%s ›", meterBar(*o.value, o.min, o.max, 10), *o.value, o.unit)
		}
		return fmt.Sprintf("‹ %d%s ›", *o.value, o.unit)
	default:
		return o.text
	}
}

// meterBar 畫一條長度固定的量表。用方塊字元而不是像素長條，
// 這樣它在任何字級下都對得齊。
func meterBar(value, low, high, cells int) string {
	if high <= low || cells <= 0 {
		return ""
	}
	filled := (value - low) * cells / (high - low)
	if filled < 0 {
		filled = 0
	}
	if filled > cells {
		filled = cells
	}
	out := make([]rune, 0, cells)
	for i := 0; i < cells; i++ {
		if i < filled {
			out = append(out, '█')
		} else {
			out = append(out, '░')
		}
	}
	return string(out)
}

// handleOptions 是設定畫面的共用行為。
func handleOptions(u *UI, ev Event, focus *int, rows []optionRow, changed func()) bool {
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			*focus = moveFocus(*focus, len(rows), -1)
			return true
		case DirDown:
			*focus = moveFocus(*focus, len(rows), +1)
			return true
		case DirLeft, DirRight:
			delta := 1
			if e.Dir == DirLeft {
				delta = -1
			}
			if *focus < len(rows) {
				row := rows[*focus]
				if row.disabled {
					u.toast(fmt.Sprintf(u.s.NotYet, row.reason), SeverityWarn)
					return true
				}
				if row.adjust(delta) && changed != nil {
					changed()
				}
			}
			return true
		}
	case Action:
		switch e.Kind {
		case ActConfirm:
			activateOption(u, rows, *focus, changed)
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

// activateOption 是「確認一列設定」的唯一實作，鍵盤與指標共用。
// 確認等同往右一格：選項循環到下一個、開關切換、數值加一階。
func activateOption(u *UI, rows []optionRow, index int, changed func()) {
	if index < 0 || index >= len(rows) {
		return
	}
	row := rows[index]
	if row.disabled {
		u.toast(fmt.Sprintf(u.s.NotYet, row.reason), SeverityWarn)
		return
	}
	if row.adjust(1) && changed != nil {
		changed()
	}
}

// drawOptionRows 畫一欄設定。focus 收指標並帶上 changed，是為了讓指標也能改值；
// 點一下等同按右鍵，與鍵盤的確認鍵一致。
func drawOptionRows(u *UI, c *canvas, x, y, width int, rows []optionRow, focus *int, changed func()) int {
	return drawOptionRowsWith(u, c, x, y, width, rows, focus,
		func(u *UI, index int) { activateOption(u, rows, index, changed) })
}

// drawOptionRowsWith 讓確認行為由呼叫端決定。金手指搜尋畫面的某些列按確認不是
// 改值而是開啟數值輸入，那個畫面因此不能用預設行為。
func drawOptionRowsWith(u *UI, c *canvas, x, y, width int, rows []optionRow, focus *int, onActivate func(*UI, int)) int {
	m := u.metrics
	for index, row := range rows {
		colour := u.focusRow(c, x, y, width, index == *focus)
		if row.disabled && index != *focus {
			colour = u.theme.TextOff
		}
		c.rowTextFit(x+m.RowPadX, y, m.RowHeight, m.BodySize, width/3-m.RowPadX*2, colour, row.label)
		c.rowTextFit(x+width/3, y, m.RowHeight, m.BodySize, width/3-m.Grid, colour, row.display(u.s))
		if row.note != "" {
			note := u.theme.TextDim
			if index == *focus {
				note = u.theme.FocusText
			}
			c.rowTextFit(x+width*2/3, y, m.RowHeight, m.SmallSize, width/3-m.RowPadX, note, row.note)
		}
		target := index
		u.addHit(x, y, width, m.RowHeight,
			func(*UI) { *focus = target },
			func(u *UI) { *focus = target; onActivate(u, target) })
		y += m.RowHeight
	}
	return y
}

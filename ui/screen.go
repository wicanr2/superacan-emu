package ui

import "fmt"

// screen 是畫面堆疊上的一層。返回鍵先關 modal、再退堆疊，堆疊只剩一層時
// 才是「關閉覆蓋層」，見 docs/ui-design.md §9.3。
type screen interface {
	id() string
	handle(u *UI, ev Event) bool
	draw(u *UI, c *canvas, snap Snapshot)
}

// menuRow 是一列選單項。停用項仍可聚焦：跳過停用項會讓功能看起來不存在，
// 使用者無從得知條件；聚焦後按確認會以 toast 說明原因。
type menuRow struct {
	label     string
	value     string
	chevron   bool
	hotkey    string
	disabled  bool
	reason    string
	gapBefore bool
	action    func(u *UI)
}

// moveFocus 在群組內環繞。跨群組不環繞，那由各畫面自己決定。
func moveFocus(focus, count, delta int) int {
	if count == 0 {
		return 0
	}
	return ((focus+delta)%count + count) % count
}

// handleMenu 是單欄選單的共用行為。
func handleMenu(u *UI, ev Event, focus *int, rows []menuRow) bool {
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			*focus = moveFocus(*focus, len(rows), -1)
			return true
		case DirDown:
			*focus = moveFocus(*focus, len(rows), +1)
			return true
		}
	case Edge:
		if e.To == EdgeHome {
			*focus = 0
		} else {
			*focus = len(rows) - 1
		}
		return true
	case Action:
		if e.Kind == ActConfirm && *focus < len(rows) {
			row := rows[*focus]
			if row.disabled {
				u.toast(fmt.Sprintf(textNotYet, row.reason), SeverityWarn)
				return true
			}
			if row.action != nil {
				row.action(u)
			}
			return true
		}
	}
	return false
}

// menuHeight 是一欄選單佔的高度，含分隔線。面板高度要用它算，
// 少算分隔線會讓最後一列被外框切掉。
func menuHeight(m Metrics, rows []menuRow) int {
	total := m.RowHeight * len(rows)
	for _, row := range rows {
		if row.gapBefore {
			total++
		}
	}
	return total
}

// drawMenuRows 畫一欄選單，回傳畫完後的 y。
func drawMenuRows(u *UI, c *canvas, x, y, width int, rows []menuRow, focus int) int {
	m := u.metrics
	for i, row := range rows {
		if row.gapBefore {
			c.rect(x, y, width, 1, u.theme.Border)
			y++
		}
		label, value := u.theme.Text, u.theme.TextDim
		if row.disabled {
			label, value = u.theme.TextOff, u.theme.TextOff
		}
		if i == focus {
			c.rect(x, y, width, m.RowHeight, u.theme.Focus)
			c.rect(x, y, 4, m.RowHeight, u.theme.FocusText)
			label, value = u.theme.FocusText, u.theme.FocusText
		}
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, label, row.label)
		right := x + width - m.RowPadX
		if row.chevron {
			c.rowText(right-c.font.Measure("›", m.BodySize), y, m.RowHeight, m.BodySize, value, "›")
			right -= c.font.Measure("›", m.BodySize) + m.IconGap
		}
		switch {
		case row.value != "":
			c.rowText(right-c.font.Measure(row.value, m.BodySize), y, m.RowHeight, m.BodySize, value, row.value)
		case row.hotkey != "":
			c.rowText(right-c.font.Measure(row.hotkey, m.SmallSize), y, m.RowHeight, m.SmallSize, value, row.hotkey)
		}
		y += m.RowHeight
	}
	return y
}

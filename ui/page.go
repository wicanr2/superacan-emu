package ui

// page 是整頁畫面的共用外框：不透明底、標題列、可選的返回提示與底部狀態列。
// 覆蓋選單不用它，那是浮在遊戲畫面上的面板。
type page struct {
	title  string
	back   bool
	right  string
	status string
}

// draw 畫出外框，回傳內容區的 y 起點與可用高度。
func (p page) draw(u *UI, c *canvas) (int, int) {
	m := u.metrics
	u.fillPage(c)
	if p.back {
		c.rowText(m.PanelPad, 0, m.TitleBar, m.BodySize, u.theme.TextDim, textBack)
	}
	c.textCenter(0, (m.TitleBar-c.font.Height(m.TitleSize))/2, c.width(), m.TitleSize, u.theme.Text, p.title)
	if p.right != "" {
		c.rowText(c.width()-m.PanelPad-c.font.Measure(p.right, m.BodySize), 0, m.TitleBar,
			m.BodySize, u.theme.TextDim, p.right)
	}
	c.rect(0, m.TitleBar, c.width(), 1, u.theme.Border)

	bottom := c.height()
	if p.status != "" {
		bottom -= m.FooterBar + m.Grid
		c.rect(0, bottom, c.width(), 1, u.theme.Border)
		c.rowText(m.PanelPad, bottom+1, m.FooterBar, m.BodySize, u.theme.TextDim, p.status)
	}
	top := m.TitleBar + 1 + m.PanelPad
	return top, bottom - top
}

// sectionTitle 畫區塊標題，回傳下一個 y。
func (u *UI) sectionTitle(c *canvas, x, y int, text string) int {
	c.rowText(x, y, u.metrics.RowHeight, u.metrics.BodySize, u.theme.TextDim, text)
	return y + u.metrics.RowHeight
}

// framedRows 畫一個有外框的列表區塊，回傳下一個 y。
func (u *UI) framedRows(c *canvas, x, y, width, rows int) int {
	height := rows*u.metrics.RowHeight + 2
	c.rect(x, y, width, height, u.theme.PanelAlt)
	c.border(x, y, width, height, u.theme.Border)
	return y + height
}

// focusRow 在需要時把整列反白，回傳這一列的文字色。
func (u *UI) focusRow(c *canvas, x, y, width int, focused bool) rgba {
	if !focused {
		return u.theme.Text
	}
	c.rect(x, y, width, u.metrics.RowHeight, u.theme.Focus)
	c.rect(x, y, 4, u.metrics.RowHeight, u.theme.FocusText)
	return u.theme.FocusText
}

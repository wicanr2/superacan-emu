package ui

import "fmt"

// aboutScreen 是 S8。第三方清單直接來自建置資訊，不是人工維護的副本，
// 所以不會漏掉相依。
type aboutScreen struct{ top int }

func (a *aboutScreen) id() string { return "S8" }

func (a *aboutScreen) handle(u *UI, ev Event) bool {
	switch e := ev.(type) {
	case Nav:
		if e.Dir == DirUp && a.top > 0 {
			a.top--
		}
		if e.Dir == DirDown {
			a.top++
		}
		return true
	case Action:
		if e.Kind == ActCancel || e.Kind == ActConfirm {
			u.pop()
			return true
		}
	}
	return false
}

func (a *aboutScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	top, height := page{title: u.s.AboutTitle, back: true}.draw(u, c)
	x := m.PanelPad
	width := c.width() - m.PanelPad*2

	info := u.about
	cgo := u.s.CGODisabled
	if info.CGOEnabled {
		cgo = u.s.CGOEnabled
	}
	y := top
	c.rowText(x, y, m.RowHeight, m.BodySize, u.theme.Text, u.s.AboutName)
	y += m.RowHeight
	c.rowText(x, y, m.RowHeight, m.SmallSize, u.theme.TextDim, fmt.Sprintf("%s · %s · %s · %s · %s",
		info.Version, info.BuildDate, info.GoVersion, info.Platform, cgo))
	y += m.RowHeight + m.Grid

	for _, line := range wrapText(c.font, u.s.AboutDisclaimer, m.BodySize, width) {
		c.rowText(x, y, m.RowHeight, m.BodySize, u.theme.Text, line)
		y += m.RowHeight
	}
	y += m.Grid
	y = u.sectionTitle(c, x, y, u.s.AboutThirdParty)

	remaining := (top + height - y) / m.RowHeight
	if remaining < 1 {
		remaining = 1
	}
	deps := info.Dependencies
	if a.top > len(deps)-remaining {
		a.top = max(0, len(deps)-remaining)
	}
	for index := a.top; index < len(deps) && index < a.top+remaining-1; index++ {
		dep := deps[index]
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.SmallSize, u.theme.Text, dep.Path)
		c.rowText(x+width*2/3, y, m.RowHeight, m.SmallSize, u.theme.TextDim, dep.Version)
		c.rowText(x+width-m.RowPadX-c.font.Measure(dep.License, m.SmallSize), y, m.RowHeight,
			m.SmallSize, u.theme.TextDim, dep.License)
		y += m.RowHeight
	}
	c.rowText(x, y, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.AboutMAME)
}

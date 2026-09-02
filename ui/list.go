package ui

// listWindow 算出長清單這一次要畫哪一段，並把捲動位置就地更新。
//
// 清單比畫面長是常態：熱鍵有十七個動作，金手指清單上限一千多筆。沒有這一層的
// 話多出來的列會畫到畫面外，而且焦點移過去之後就再也看不見——雜湊看不出這件事，
// 因為畫出去的部分不在畫布上。
//
// top 是捲動位置，由呼叫端保存在畫面狀態裡；焦點永遠留在可視範圍內。
func listWindow(top *int, focus, count, height, rowHeight int) (first, last int) {
	visible := 0
	if rowHeight > 0 {
		visible = height / rowHeight
	}
	if visible < 1 {
		visible = 1
	}
	if visible > count {
		visible = count
	}
	if *top > count-visible {
		*top = count - visible
	}
	if *top < 0 {
		*top = 0
	}
	if focus < *top {
		*top = focus
	}
	if focus >= *top+visible {
		*top = focus - visible + 1
	}
	return *top, *top + visible
}

// listScrollHint 是清單上下還有東西時畫在右邊的指示。沒有捲軸的畫面要讓使用者
// 知道清單沒有到底，否則看起來就像功能不存在。
func (u *UI) listScrollHint(c *canvas, x, y, width, height, first, last, count int) {
	if count <= last-first {
		return
	}
	m := u.metrics
	if first > 0 {
		c.textRight(x+width, y, m.SmallSize, u.theme.TextDim, "▲")
	}
	if last < count {
		c.textRight(x+width, y+height-c.font.Height(m.SmallSize), m.SmallSize, u.theme.TextDim, "▼")
	}
}

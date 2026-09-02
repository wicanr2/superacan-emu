package ui

import (
	"fmt"
	"time"
)

// Severity 決定訊息的存活時間與可否被抑制。
type Severity uint8

const (
	// SeverityInfo 是操作訊息，2.5 秒後消失，可由設定抑制。
	SeverityInfo Severity = iota
	// SeverityWarn 是警告，4 秒後消失，不可抑制。
	SeverityWarn
)

const (
	infoToastLife = 2500 * time.Millisecond
	warnToastLife = 4000 * time.Millisecond
	maxToasts     = 3
)

type toastItem struct {
	text     string
	severity Severity
	expires  time.Duration
}

// toast 排入一則訊息。同時最多三則，超過時最舊的立即消失。
func (u *UI) toast(text string, severity Severity) {
	if severity == SeverityInfo && u.config.Interface.SuppressInfoToasts {
		return
	}
	life := infoToastLife
	if severity == SeverityWarn {
		life = warnToastLife
	}
	u.toasts = append(u.toasts, toastItem{text: text, severity: severity, expires: u.now + life})
	if len(u.toasts) > maxToasts {
		u.toasts = u.toasts[len(u.toasts)-maxToasts:]
	}
}

// Fail 顯示錯誤列。錯誤不用 toast：會自己消失的錯誤等於沒說。
// 入口執行 Intent 失敗時走這條路，使用者才知道剛才那個動作沒有成功。
func (u *UI) Fail(text string) { u.errorText = text }

func (u *UI) fail(text string) { u.Fail(text) }

// ErrorText 是目前顯示在錯誤列上的文字，空字串代表沒有錯誤。入口用它把介面上
// 看得到的失敗同時寫進記錄；沒有這個出口的話，兩邊會各自維護一份「出了什麼事」。
func (u *UI) ErrorText() string { return u.errorText }

func (u *UI) expireToasts() {
	kept := u.toasts[:0]
	for _, t := range u.toasts {
		if t.expires > u.now {
			kept = append(kept, t)
		}
	}
	u.toasts = kept
}

func (u *UI) drawToasts(c *canvas) {
	if len(u.toasts) == 0 {
		return
	}
	m := u.metrics
	height := m.RowHeight
	bottom := c.height() - m.ToastInset
	if u.errorText != "" {
		bottom -= m.FooterBar + m.Grid
	}
	for i := len(u.toasts) - 1; i >= 0; i-- {
		t := u.toasts[i]
		width := c.font.Measure(t.text, m.BodySize) + m.RowPadX*2
		x := (c.width() - width) / 2
		y := bottom - height
		back := u.theme.PanelAlt
		if t.severity == SeverityWarn {
			back = u.theme.Warn
		}
		c.rect(x, y, width, height, back)
		c.border(x, y, width, height, u.theme.Border)
		c.rowText(x+m.RowPadX, y, height, m.BodySize, u.theme.Text, t.text)
		bottom = y - m.Grid
	}
}

func (u *UI) drawErrorBar(c *canvas) {
	if u.errorText == "" {
		return
	}
	m := u.metrics
	height := m.FooterBar
	y := c.height() - height
	c.rect(0, y, c.width(), height, u.theme.Error)
	c.rowText(m.PanelPad, y, height, m.BodySize, u.theme.FocusText, u.errorText)
	label := u.s.DismissError
	c.rowText(c.width()-m.PanelPad-c.font.Measure(label, m.SmallSize), y, height, m.SmallSize, u.theme.FocusText, label)
}

// drawFPS 畫主機端的 frame 率。它是常駐指示不是 toast：要看的是持續的數字，
// 會自己消失的東西沒有用。位置讓開金手指標記，兩者都在右上角。
func (u *UI) drawFPS(c *canvas, snap Snapshot) {
	if !u.config.Video.ShowFPS {
		return
	}
	m := u.metrics
	label := fmt.Sprintf("%.1f FPS", u.diagnostics(snap).HostFPS)
	width := c.font.Measure(label, m.SmallSize) + m.RowPadX*2
	y := m.Grid
	if state := u.cheats(); state.Enabled || state.Wrote {
		y += m.RowHeight + m.Grid
	}
	x := c.width() - width - m.Grid
	c.rect(x, y, width, m.RowHeight, u.theme.PanelAlt)
	c.border(x, y, width, m.RowHeight, u.theme.Border)
	c.textCenter(x, y+(m.RowHeight-c.font.Height(m.SmallSize))/2, width, m.SmallSize,
		u.theme.Text, label)
}

package ui

import "time"

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
	if severity == SeverityInfo && u.config.SuppressInfoToasts {
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
	label := textDismissError
	c.rowText(c.width()-m.PanelPad-c.font.Measure(label, m.SmallSize), y, height, m.SmallSize, u.theme.FocusText, label)
}

package ui

import "fmt"

// DiagnosticsFacts 是診斷畫面顯示的數值。全部由入口提供，ui 不查詢執行環境，
// 也不從畫面反推任何硬體狀態。
type DiagnosticsFacts struct {
	Frame      uint64
	M68K       uint64
	M65C02     uint64
	IRQ7       uint64
	IRQ4       uint64
	IRQ5       uint64
	SoundClash int
	HostFPS    float64
	Pacing     bool
	Frontend   string
	Platform   string
	CGOEnabled bool
	IPL        [32]byte
	Cartridge  [32]byte
	// Recording 與 CaptureFrames 讓覆蓋選單顯示擷取狀態。
	Recording     bool
	CaptureFrames int
}

// DiagnosticsSource 由入口提供。
type DiagnosticsSource interface{ Diagnostics() DiagnosticsFacts }

// 圖層遮罩的位元，與 chip/umc6618 相同。
const (
	LayerTilemap0 uint32 = 1 << 0
	LayerTilemap1 uint32 = 1 << 1
	LayerTilemap2 uint32 = 1 << 2
	LayerSprite   uint32 = 1 << 3
	LayerROZ      uint32 = 1 << 4
	LayerWindow   uint32 = 1 << 5
	// AllLayers 是全部開啟。
	AllLayers = LayerTilemap0 | LayerTilemap1 | LayerTilemap2 | LayerSprite | LayerROZ | LayerWindow
)

// layerBits 的名稱是硬體用語，五種語言相同，所以直接寫在這裡而不進語言表。
var layerBits = []struct {
	bit   uint32
	label string
}{
	{LayerTilemap0, "tilemap0"},
	{LayerTilemap1, "tilemap1"},
	{LayerTilemap2, "tilemap2"},
	{LayerSprite, "sprite"},
	{LayerROZ, "ROZ"},
	{LayerWindow, "window"},
}

// diagnosticsScreen 是 S7。只讀，唯一會動到 machine 之外的東西是圖層遮罩，
// 而遮罩只影響 framebuffer 合成，不影響指令數與硬體時序。
type diagnosticsScreen struct{ focus int }

func (d *diagnosticsScreen) id() string { return "S7" }

func (d *diagnosticsScreen) handle(u *UI, ev Event) bool {
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirLeft:
			d.focus = moveFocus(d.focus, len(layerBits), -1)
		case DirRight:
			d.focus = moveFocus(d.focus, len(layerBits), +1)
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			mask := u.layerMask() ^ layerBits[d.focus].bit
			u.emit(SetLayerMask{Mask: mask})
			u.toast(u.s.DiagMaskWarning, SeverityWarn)
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

func (d *diagnosticsScreen) draw(u *UI, c *canvas, snap Snapshot) {
	m := u.metrics
	top, _ := page{title: u.s.DiagTitle, back: true, status: u.s.DiagLayerNote}.draw(u, c)
	facts := u.diagnostics(snap)
	x := m.PanelPad
	width := c.width() - m.PanelPad*2

	left := []struct{ label, value string }{
		{u.s.DiagFrame, group(facts.Frame)},
		{u.s.DiagM68K, group(facts.M68K)},
		{u.s.DiagM65C02, group(facts.M65C02)},
		{u.s.DiagIRQ7, group(facts.IRQ7)},
		{u.s.DiagIRQ45, fmt.Sprintf("%s / %s", group(facts.IRQ4), group(facts.IRQ5))},
		{u.s.DiagClash, fmt.Sprintf("%d", facts.SoundClash)},
	}
	right := []struct{ label, value string }{
		{u.s.HaltIPL, shortHash(facts.IPL)},
		{u.s.HaltCartridge, shortHash(facts.Cartridge)},
		{u.s.DiagFrontend, fmt.Sprintf("%s / %s", facts.Frontend, facts.Platform)},
		{u.s.DiagCGO, u.cgoLabel(facts.CGOEnabled)},
	}
	y := top
	for index, fact := range left {
		c.rowTextFit(x, y, m.RowHeight, m.SmallSize, width/4-m.Grid, u.theme.TextDim, fact.label)
		c.rowTextFit(x+width/4, y, m.RowHeight, m.SmallSize, width/4-m.Grid, u.theme.Text, fact.value)
		if index < len(right) {
			c.rowTextFit(x+width/2, y, m.RowHeight, m.SmallSize, width/4-m.Grid, u.theme.TextDim, right[index].label)
			c.rowTextFit(x+width*3/4, y, m.RowHeight, m.SmallSize, width/4-m.Grid, u.theme.Text, right[index].value)
		}
		y += m.RowHeight
	}

	y += m.SectionGap
	c.rect(x, y, width, 1, u.theme.Border)
	y += 1 + m.Grid
	c.rowText(x, y, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.DiagLayerMask)
	mask := u.layerMask()
	cursor := x + width/4
	for index, layer := range layerBits {
		box := "[ ]"
		if mask&layer.bit != 0 {
			box = "[■]"
		}
		text := box + layer.label
		colour := u.theme.Text
		if index == d.focus {
			w := c.font.Measure(text, m.SmallSize) + m.Grid
			c.rect(cursor-m.Grid/2, y, w, m.RowHeight, u.theme.Focus)
			colour = u.theme.FocusText
		}
		if cursor+c.font.Measure(text, m.SmallSize) > x+width {
			// 一列排不下就換行，不要畫出面板之外。
			cursor = x + width/4
			y += m.RowHeight
		}
		c.rowText(cursor, y, m.RowHeight, m.SmallSize, colour, text)
		cursor += c.font.Measure(text, m.SmallSize) + m.SectionGap
	}
}

func (u *UI) cgoLabel(enabled bool) string {
	if enabled {
		return u.s.CGOEnabled
	}
	return u.s.CGODisabled
}

func (u *UI) diagnostics(snap Snapshot) DiagnosticsFacts {
	if u.diag != nil {
		return u.diag.Diagnostics()
	}
	facts := DiagnosticsFacts{}
	if snap != nil {
		facts.Frame = snap.FrameIndex()
		facts.M68K, facts.M65C02 = snap.Instructions()
		_, facts.Cartridge, _ = snap.Cartridge()
		facts.IPL = snap.Firmware().IPL
	}
	return facts
}

func (u *UI) layerMask() uint32 {
	if u.mask != 0 {
		return u.mask
	}
	return AllLayers
}

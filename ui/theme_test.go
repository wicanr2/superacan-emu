package ui

import "testing"

// 文字對背景的對比比值必須達到 WCAG AA。停用項本來就要弱，門檻放寬到 3:1。
// 半透明的面板底先與遊戲畫面最亮的情況合成再量，因為對比是對看得到的顏色而言。
func TestThemeContrast(t *testing.T) {
	theme := DefaultTheme()
	white := rgba{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	panelOnBright := flatten(theme.Panel, white)
	panelAlt := flatten(theme.PanelAlt, white)

	for _, c := range []struct {
		name string
		fg   rgba
		bg   rgba
		min  float64
	}{
		{"text on panel", theme.Text, panelOnBright, 4.5},
		{"text on panel-alt", theme.Text, panelAlt, 4.5},
		{"text-dim on panel", theme.TextDim, panelOnBright, 4.5},
		{"text-off on panel", theme.TextOff, panelOnBright, 3.0},
		{"focus-text on focus", theme.FocusText, theme.Focus, 4.5},
		{"focus-text on error", theme.FocusText, theme.Error, 4.5},
	} {
		if got := contrastRatio(c.fg, c.bg); got < c.min {
			t.Errorf("%s 對比 %.2f:1，至少要 %.1f:1", c.name, got, c.min)
		}
	}
}

// ok／warn／error 的明度刻意分開，色覺差異者靠明度也能區分三者。
func TestStatusColoursDifferInLuminance(t *testing.T) {
	theme := DefaultTheme()
	values := []struct {
		name string
		lum  float64
	}{
		{"ok", relativeLuminance(theme.OK)},
		{"warn", relativeLuminance(theme.Warn)},
		{"error", relativeLuminance(theme.Error)},
	}
	for i := range values {
		for j := i + 1; j < len(values); j++ {
			ratio := (values[i].lum + 0.05) / (values[j].lum + 0.05)
			if ratio < 1 {
				ratio = 1 / ratio
			}
			if ratio < 1.2 {
				t.Errorf("%s 與 %s 的明度只差 %.2f 倍", values[i].name, values[j].name, ratio)
			}
		}
	}
}

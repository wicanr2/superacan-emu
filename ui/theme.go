package ui

import (
	"image/color"
	"math"
)

// Theme 是十二個語意色。改色時 TestThemeContrast 會擋住對比不足的組合。
type Theme struct {
	Scrim     color.NRGBA
	ScrimHalt color.NRGBA
	Panel     color.NRGBA
	PanelAlt  color.NRGBA
	Border    color.NRGBA
	Text      color.NRGBA
	TextDim   color.NRGBA
	TextOff   color.NRGBA
	Focus     color.NRGBA
	FocusText color.NRGBA
	OK        color.NRGBA
	Warn      color.NRGBA
	Error     color.NRGBA
}

// DefaultTheme 見 docs/ui-design.md §10.1。面板不透明度 0.92 而不是常見的 0.6：
// 遊戲畫面是 256 色高飽和點陣圖，半透明面板上的文字對比會被背景亮塊吃掉。
func DefaultTheme() Theme {
	return Theme{
		Scrim:     color.NRGBA{0x00, 0x00, 0x00, 0x8c},
		ScrimHalt: color.NRGBA{0x00, 0x00, 0x00, 0xbf},
		Panel:     color.NRGBA{0x10, 0x14, 0x18, 0xeb},
		PanelAlt:  color.NRGBA{0x17, 0x1c, 0x22, 0xff},
		Border:    color.NRGBA{0x3a, 0x46, 0x52, 0xff},
		Text:      color.NRGBA{0xe6, 0xea, 0xee, 0xff},
		TextDim:   color.NRGBA{0x94, 0xa3, 0xae, 0xff},
		TextOff:   color.NRGBA{0x68, 0x72, 0x7c, 0xff},
		Focus:     color.NRGBA{0x2b, 0x6c, 0xb0, 0xff},
		FocusText: color.NRGBA{0xff, 0xff, 0xff, 0xff},
		OK:        color.NRGBA{0x3f, 0x8f, 0x5b, 0xff},
		Warn:      color.NRGBA{0xb8, 0x86, 0x2b, 0xff},
		Error:     color.NRGBA{0xb4, 0x42, 0x3c, 0xff},
	}
}

// relativeLuminance 是 WCAG 2.x 的相對亮度，供 TestThemeContrast 使用。
func relativeLuminance(c color.NRGBA) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

// contrastRatio 是兩個不透明色的對比比值。半透明色要先與其底色合成再傳進來，
// 因為對比是對「看得到的顏色」而言，不是對色票本身。
func contrastRatio(a, b color.NRGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// flatten 把帶 alpha 的前景合成到不透明背景上，回傳實際看得到的顏色。
func flatten(fg, bg color.NRGBA) color.NRGBA {
	a := float64(fg.A) / 255
	mix := func(f, b uint8) uint8 {
		return uint8(float64(f)*a + float64(b)*(1-a) + 0.5)
	}
	return color.NRGBA{mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B), 0xff}
}

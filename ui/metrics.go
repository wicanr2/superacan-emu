package ui

// Metrics 是一套版面度量，單位全部是設計單位。基準網格 8 單位，所有間距取其倍數。
//
// 字身格是 bitmapfont/v4 的 12 px 字型：半寬字前進 6 單位、全寬字 12 單位、
// 行高 16 單位。字級只有 1×／2×／3× 三種整數倍，因為點陣字非整數倍縮放會產生
// 不均勻筆畫。compact 的正文用 1×（16 單位高）才放得進 24 單位的列高。
type Metrics struct {
	Profile    Profile
	Grid       int
	RowHeight  int
	MinTarget  int
	PanelPad   int
	SectionGap int
	RowPadX    int
	IconGap    int
	TitleBar   int
	FooterBar  int
	BodySize   int
	TitleSize  int
	SmallSize  int
	ToastInset int
}

// 字身格常數。GlyphHeight 是行高，兩個 Advance 分別是半寬與全寬字的前進量。
const (
	GlyphHeight      = 16
	AdvanceHalfWidth = 6
	AdvanceFullWidth = 12
)

// MetricsFor 回傳指定設定檔的度量。
func MetricsFor(profile Profile) Metrics {
	switch profile {
	case ProfileTouch:
		return Metrics{
			Profile: ProfileTouch, Grid: 8,
			RowHeight: 44, MinTarget: 44, PanelPad: 24, SectionGap: 24,
			RowPadX: 16, IconGap: 8, TitleBar: 64, FooterBar: 44,
			BodySize: 2, TitleSize: 3, SmallSize: 1, ToastInset: 24,
		}
	default:
		return Metrics{
			Profile: ProfileCompact, Grid: 8,
			RowHeight: 24, MinTarget: 24, PanelPad: 16, SectionGap: 24,
			RowPadX: 12, IconGap: 8, TitleBar: 48, FooterBar: 24,
			BodySize: 1, TitleSize: 2, SmallSize: 1, ToastInset: 24,
		}
	}
}

package ui

import (
	"image"
	"sync"

	bitmapfont "github.com/hajimehoshi/bitmapfont/v4"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Font 把一份 font.Face 轉成可以整數倍放大的點陣字。放大用像素複製而不是取樣，
// 因為點陣字的非整數倍縮放會產生不均勻筆畫。
type Font struct {
	face   font.Face
	ascent int

	mu     sync.Mutex
	glyphs map[rune]*glyph
}

type glyph struct {
	w, h     int
	offX     int
	offY     int
	advance  int
	coverage []bool
}

var (
	defaultFontOnce sync.Once
	defaultFont     *Font
)

// DefaultFont 是嵌入的 bitmapfont/v4（繁體中文字形優先）。它涵蓋 ASCII、
// Latin-1 與 CJK，所以介面的五種語言共用同一份字型；散布時要一併附上其六份
// 來源授權，見 docs/ui-font.md。
func DefaultFont() *Font {
	defaultFontOnce.Do(func() { defaultFont = NewFont(bitmapfont.FaceTC) })
	return defaultFont
}

// NewFont 包裝任一 font.Face。
func NewFont(face font.Face) *Font {
	metrics := face.Metrics()
	return &Font{
		face:   face,
		ascent: metrics.Ascent.Ceil(),
		glyphs: make(map[rune]*glyph),
	}
}

// Advance 回傳一個字的前進量，單位是設計單位（未乘字級）。
func (f *Font) Advance(r rune) int {
	if g := f.glyph(r); g != nil {
		return g.advance
	}
	return AdvanceHalfWidth
}

// Measure 回傳一段文字在指定字級下的寬度，單位是設計單位。
func (f *Font) Measure(s string, size int) int {
	total := 0
	for _, r := range s {
		total += f.Advance(r)
	}
	return total * size
}

// Height 是行高乘字級。
func (f *Font) Height(size int) int { return GlyphHeight * size }

func (f *Font) glyph(r rune) *glyph {
	f.mu.Lock()
	defer f.mu.Unlock()
	if g, ok := f.glyphs[r]; ok {
		return g
	}
	g := f.rasterize(r)
	f.glyphs[r] = g
	return g
}

func (f *Font) rasterize(r rune) *glyph {
	dot := fixed.P(0, f.ascent)
	bounds, mask, maskPoint, advance, ok := f.face.Glyph(dot, r)
	if !ok {
		// 缺字回退到替換字元；它自己也缺就當成空白，寬度仍前進，
		// 這樣缺字不會讓整行的欄位對不齊。
		if r != '�' {
			return f.rasterize('�')
		}
		return &glyph{advance: AdvanceHalfWidth}
	}
	width, height := bounds.Dx(), bounds.Dy()
	g := &glyph{
		w: width, h: height,
		offX: bounds.Min.X, offY: bounds.Min.Y,
		advance:  advance.Ceil(),
		coverage: make([]bool, width*height),
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			_, _, _, alpha := mask.At(maskPoint.X+x, maskPoint.Y+y).RGBA()
			g.coverage[y*width+x] = alpha >= 0x8000
		}
	}
	return g
}

// blit 把一個字畫到 dst，(penX, penY) 是這一行的左上角，單位是像素；
// pixel 是一個字型像素在畫面上的邊長。
func (f *Font) blit(dst *image.RGBA, penX, penY int, r rune, pixel int, col rgba) int {
	g := f.glyph(r)
	if g == nil {
		return AdvanceHalfWidth * pixel
	}
	// face.Glyph 的 dr 已經是以 dot 為基準的目的地座標，rasterize 又固定把 dot
	// 放在 (0, ascent)，所以 offY 就是字身格頂端相對於行頂的位移；再加一次
	// ascent 會讓整行往下掉一個 ascent。
	baseX := penX + g.offX*pixel
	baseY := penY + g.offY*pixel
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if !g.coverage[y*g.w+x] {
				continue
			}
			fillPixels(dst, baseX+x*pixel, baseY+y*pixel, pixel, pixel, col)
		}
	}
	return g.advance * pixel
}

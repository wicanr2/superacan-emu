package ui

import (
	"image"
	"image/color"
)

type rgba = color.NRGBA

// fillPixels 以 src-over 把一塊實心矩形合成到 dst，座標與大小都是像素。
// 超出 dst 的部分裁掉，不做環繞。
func fillPixels(dst *image.RGBA, x, y, w, h int, col rgba) {
	if col.A == 0 || w <= 0 || h <= 0 {
		return
	}
	area := image.Rect(x, y, x+w, y+h).Intersect(dst.Bounds())
	if area.Empty() {
		return
	}
	alpha := uint32(col.A)
	srcR := uint32(col.R) * alpha / 255
	srcG := uint32(col.G) * alpha / 255
	srcB := uint32(col.B) * alpha / 255
	inv := 255 - alpha
	for py := area.Min.Y; py < area.Max.Y; py++ {
		row := dst.PixOffset(area.Min.X, py)
		for px := area.Min.X; px < area.Max.X; px++ {
			if alpha == 255 {
				dst.Pix[row+0] = col.R
				dst.Pix[row+1] = col.G
				dst.Pix[row+2] = col.B
				dst.Pix[row+3] = 0xff
			} else {
				dst.Pix[row+0] = uint8(srcR + uint32(dst.Pix[row+0])*inv/255)
				dst.Pix[row+1] = uint8(srcG + uint32(dst.Pix[row+1])*inv/255)
				dst.Pix[row+2] = uint8(srcB + uint32(dst.Pix[row+2])*inv/255)
				dst.Pix[row+3] = uint8(alpha + uint32(dst.Pix[row+3])*inv/255)
			}
			row += 4
		}
	}
}

// canvas 是以設計單位描述版面的畫布。所有座標乘上 scale 才落到像素，
// ui 本身不查詢 DPI。
type canvas struct {
	dst     *image.RGBA
	scale   int
	metrics Metrics
	font    *Font
	theme   Theme
}

func (c *canvas) width() int  { return c.dst.Bounds().Dx() / c.scale }
func (c *canvas) height() int { return c.dst.Bounds().Dy() / c.scale }

func (c *canvas) rect(x, y, w, h int, col rgba) {
	fillPixels(c.dst, x*c.scale, y*c.scale, w*c.scale, h*c.scale, col)
}

// border 畫 1 設計單位寬的外框。
func (c *canvas) border(x, y, w, h int, col rgba) {
	c.rect(x, y, w, 1, col)
	c.rect(x, y+h-1, w, 1, col)
	c.rect(x, y+1, 1, h-2, col)
	c.rect(x+w-1, y+1, 1, h-2, col)
}

// text 從 (x, y) 這個左上角畫一行字，回傳畫出的寬度（設計單位）。
func (c *canvas) text(x, y, size int, col rgba, s string) int {
	pen := x * c.scale
	top := y * c.scale
	pixel := size * c.scale
	for _, r := range s {
		pen += c.font.blit(c.dst, pen, top, r, pixel, col)
	}
	return (pen - x*c.scale) / c.scale
}

// textRight 讓一行字的右緣落在 x。
func (c *canvas) textRight(x, y, size int, col rgba, s string) {
	c.text(x-c.font.Measure(s, size), y, size, col, s)
}

// textCenter 讓一行字水平置中於 [x, x+w)。
func (c *canvas) textCenter(x, y, w, size int, col rgba, s string) {
	c.text(x+(w-c.font.Measure(s, size))/2, y, size, col, s)
}

// rowText 是「在 h 高的列裡垂直置中」的常用寫法。
func (c *canvas) rowText(x, y, h, size int, col rgba, s string) {
	c.text(x, y+(h-c.font.Height(size))/2, size, col, s)
}

// blitScaled 以最近鄰把 src 畫進 (x, y, w, h) 這塊設計單位矩形。
// 存檔縮圖走這條路：320×240 縮到 160×120 是整數 1/2，不會有取樣雜訊。
func (c *canvas) blitScaled(x, y, w, h int, src *image.RGBA) {
	if src == nil {
		return
	}
	dstW, dstH := w*c.scale, h*c.scale
	srcB := src.Bounds()
	if dstW <= 0 || dstH <= 0 || srcB.Empty() {
		return
	}
	originX, originY := x*c.scale, y*c.scale
	for py := 0; py < dstH; py++ {
		sy := srcB.Min.Y + py*srcB.Dy()/dstH
		for px := 0; px < dstW; px++ {
			sx := srcB.Min.X + px*srcB.Dx()/dstW
			offset := src.PixOffset(sx, sy)
			fillPixels(c.dst, originX+px, originY+py, 1, 1, rgba{
				R: src.Pix[offset], G: src.Pix[offset+1], B: src.Pix[offset+2], A: 0xff,
			})
		}
	}
}

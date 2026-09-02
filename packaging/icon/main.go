// Command icon 產生發行包用的圖示：副檔名是 .icns 就寫 macOS 的圖示容器，
// 其餘寫 256×256 的 PNG（AppImage 用）。圖示是建置產物，所以它必須能從原始碼重現，
// 而不是某次手工畫好之後就沒人知道怎麼來的檔案。
//
// 圖案是主機本身：一塊 4:3 的畫面，裡面是三層 tilemap 疊出來的色帶與一個 sprite。
// 不放字：字在 16×16 的工作列圖示上看不清楚，而且會變成需要跟著介面語言改的東西。
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const size = 256

var (
	background = color.RGBA{0x0e, 0x11, 0x16, 0xff}
	bezel      = color.RGBA{0x1c, 0x22, 0x2b, 0xff}
	border     = color.RGBA{0x3d, 0x7e, 0xbf, 0xff}
	sky        = color.RGBA{0x2b, 0x5f, 0x9e, 0xff}
	hills      = color.RGBA{0x2f, 0x7d, 0x4f, 0xff}
	ground     = color.RGBA{0x8a, 0x5a, 0x2b, 0xff}
	sprite     = color.RGBA{0xe8, 0xc1, 0x4a, 0xff}
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "用法：icon <輸出.png>")
		os.Exit(2)
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, img.Bounds(), background)

	// 外框做成圓角：方角在深色底上會被系統的圖示遮罩切掉。
	roundRect(img, image.Rect(16, 16, size-16, size-16), 28, bezel)

	// 4:3 的畫面。240×180 是 320×240 的四分之三，剛好放進圓角框裡。
	screen := image.Rect(size/2-120, size/2-90, size/2+120, size/2+90)
	fill(img, screen, sky)

	// 三層色帶：由遠到近，對應 tilemap 的優先度順序。
	fill(img, image.Rect(screen.Min.X, screen.Min.Y+96, screen.Max.X, screen.Min.Y+124), hills)
	fill(img, image.Rect(screen.Min.X, screen.Min.Y+124, screen.Max.X, screen.Max.Y), ground)

	// sprite：故意畫成整數倍的像素塊，方塊大小就是「一個 tile」的感覺。
	block := 14
	for _, cell := range [][2]int{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {2, 1}, {0, 2}, {1, 2}, {2, 2}} {
		x := screen.Min.X + 99 + cell[0]*block
		y := screen.Min.Y + 46 + cell[1]*block
		fill(img, image.Rect(x, y, x+block, y+block), sprite)
	}

	// 掃描線：讓它一眼看得出來是主機畫面而不是一般的圖示。
	for y := screen.Min.Y; y < screen.Max.Y; y += 6 {
		shade(img, image.Rect(screen.Min.X, y, screen.Max.X, y+1), 0x14)
	}
	outline(img, screen, 2, border)

	var err error
	if filepath.Ext(os.Args[1]) == ".icns" {
		err = writeICNS(os.Args[1], img)
	} else {
		err = writePNG(os.Args[1], img)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "icon:", err)
		os.Exit(1)
	}
}

func writePNG(path string, img *image.RGBA) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

// icnsEntries 是要寫進 .icns 的尺寸與它們的型別碼。全部由 256×256 的原圖以整數
// 倍最近鄰縮放而來——圖案本來就是像素塊，插值只會把邊緣糊掉。
var icnsEntries = []struct {
	kind string
	size int
}{
	{"ic07", 128},
	{"ic08", 256},
	{"ic09", 512},
	{"ic10", 1024},
}

// writeICNS 寫出 macOS 的圖示容器。格式是「'icns' + 總長度」之後接一串
// 「型別碼 + 長度 + 資料」，而 ic07 之後的型別碼直接吃 PNG，所以不需要
// Apple 的工具就寫得出來。
func writeICNS(path string, img *image.RGBA) error {
	var body bytes.Buffer
	for _, entry := range icnsEntries {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, scaleNearest(img, entry.size)); err != nil {
			return err
		}
		body.WriteString(entry.kind)
		if err := binary.Write(&body, binary.BigEndian, uint32(encoded.Len()+8)); err != nil {
			return err
		}
		body.Write(encoded.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	if err := binary.Write(&out, binary.BigEndian, uint32(body.Len()+8)); err != nil {
		return err
	}
	out.Write(body.Bytes())
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// scaleNearest 以最近鄰縮放到指定邊長。
func scaleNearest(src *image.RGBA, side int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	width := src.Bounds().Dx()
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(x*width/side, y*width/side))
		}
	}
	return dst
}

func fill(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// shade 把一段區域壓暗，用來畫掃描線。
func shade(img *image.RGBA, rect image.Rectangle, amount uint8) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{sub(c.R, amount), sub(c.G, amount), sub(c.B, amount), c.A})
		}
	}
}

func sub(value, amount uint8) uint8 {
	if value < amount {
		return 0
	}
	return value - amount
}

func roundRect(img *image.RGBA, rect image.Rectangle, radius int, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if insideRounded(x, y, rect, radius) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func insideRounded(x, y int, rect image.Rectangle, radius int) bool {
	corners := [][2]int{
		{rect.Min.X + radius, rect.Min.Y + radius},
		{rect.Max.X - radius - 1, rect.Min.Y + radius},
		{rect.Min.X + radius, rect.Max.Y - radius - 1},
		{rect.Max.X - radius - 1, rect.Max.Y - radius - 1},
	}
	for index, corner := range corners {
		beyondX := (index%2 == 0 && x < corner[0]) || (index%2 == 1 && x > corner[0])
		beyondY := (index < 2 && y < corner[1]) || (index >= 2 && y > corner[1])
		if beyondX && beyondY {
			dx, dy := x-corner[0], y-corner[1]
			return dx*dx+dy*dy <= radius*radius
		}
	}
	return true
}

func outline(img *image.RGBA, rect image.Rectangle, width int, c color.RGBA) {
	for i := 0; i < width; i++ {
		r := image.Rect(rect.Min.X-i-1, rect.Min.Y-i-1, rect.Max.X+i+1, rect.Max.Y+i+1)
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, r.Min.Y, c)
			img.SetRGBA(x, r.Max.Y-1, c)
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			img.SetRGBA(r.Min.X, y, c)
			img.SetRGBA(r.Max.X-1, y, c)
		}
	}
}

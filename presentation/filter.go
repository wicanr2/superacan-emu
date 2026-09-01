package presentation

import "image"

// ApplyScanlines 把每隔一列的亮度降低 percent。這是顯示端的濾鏡：
// 它作用在放大後的畫面上，不改 framebuffer，所以截圖與畫面雜湊不受影響。
//
// 用整數運算而不是浮點，任何平台上的結果都逐位元相同。
func ApplyScanlines(image *image.RGBA, percent int) {
	if percent <= 0 {
		return
	}
	if percent > 100 {
		percent = 100
	}
	keep := uint32(100 - percent)
	bounds := image.Bounds()
	for y := bounds.Min.Y + 1; y < bounds.Max.Y; y += 2 {
		row := image.PixOffset(bounds.Min.X, y)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			image.Pix[row+0] = uint8(uint32(image.Pix[row+0]) * keep / 100)
			image.Pix[row+1] = uint8(uint32(image.Pix[row+1]) * keep / 100)
			image.Pix[row+2] = uint8(uint32(image.Pix[row+2]) * keep / 100)
			row += 4
		}
	}
}

// ScanlinePercent 把設定檔的濾鏡名稱轉成百分比。未知的名稱視為不套濾鏡，
// 而不是猜一個——設定檔可能來自比這個版本新的模擬器。
func ScanlinePercent(filter string) int {
	switch filter {
	case "scanline25":
		return 25
	case "scanline50":
		return 50
	case "scanline75":
		return 75
	default:
		return 0
	}
}

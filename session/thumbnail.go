package session

import (
	"image"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/presentation"
)

// thumbnail 把存檔 payload 內的 framebuffer 轉成縮圖來源。存檔槽畫面會再縮到
// 160×120（整數 1/2），所以這裡只做格式轉換不做縮放，避免縮兩次。
func thumbnail(framebuffer []uint32) *image.RGBA {
	if len(framebuffer) != umc6618.Width*umc6618.Height {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, umc6618.Width, umc6618.Height))
	presentation.ARGBToRGBA(img.Pix, framebuffer)
	return img
}

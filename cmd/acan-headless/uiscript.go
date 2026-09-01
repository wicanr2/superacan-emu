package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wicanr2/superacan-emu/session"
	"github.com/wicanr2/superacan-emu/ui"
)

// uiSurface 解析 WxH。
func uiSurface(spec string, profile ui.Profile, scale int) (ui.Surface, error) {
	widthText, heightText, found := strings.Cut(strings.ToLower(spec), "x")
	if !found {
		return ui.Surface{}, fmt.Errorf("ui surface %q is not WxH", spec)
	}
	width, err := strconv.Atoi(widthText)
	if err != nil {
		return ui.Surface{}, err
	}
	height, err := strconv.Atoi(heightText)
	if err != nil {
		return ui.Surface{}, err
	}
	if width <= 0 || height <= 0 {
		return ui.Surface{}, fmt.Errorf("ui surface %q must be positive", spec)
	}
	return ui.Surface{W: width, H: height, Scale: scale, Profile: profile}, nil
}

// composeFrame 產生「遊戲畫面加覆蓋層」的合成圖，供雜湊與 PNG 輸出。
func composeFrame(s *session.Session, surface ui.Surface) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, surface.W, surface.H))
	s.Compose(dst)
	return dst
}

func writeComposedPNG(name string, frame *image.RGBA) error {
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, frame)
}

// frameClock 讓 UI 的時間跟著模擬 frame 走，而不是掛鐘。toast 的存活時間因此
// 在 headless 是可重現的。
func frameClock(frame uint64) time.Duration {
	return time.Duration(frame) * 16667 * time.Microsecond
}

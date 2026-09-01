package main

import (
	"fmt"
	"image"
	"time"

	"github.com/wicanr2/superacan-emu/frontend/x11"
	"github.com/wicanr2/superacan-emu/ui"
)

// 覆蓋層用到的 keysym。目前用 F1 叫出選單而不是 Esc：Esc 改成「開啟選單」還沒
// 拍板（WORKLIST A1），在那之前不動現行前端「Esc 離開」的行為。
const (
	keysymF1        = 0xffbe
	keysymBackspace = 0xff08
	keysymDelete    = 0xffff
	keysymTab       = 0xff09
	keysymHome      = 0xff50
	keysymEnd       = 0xff57
)

// overlayKey 是一個會產生介面事件的按鍵。事件在按下的那一瞬間送出一次，
// 按著不放不重複——選單導覽的重複行為要由介面決定，不是由 X11 的自動重複決定。
type overlayKey struct {
	keysym uint32
	event  ui.Event
}

var overlayKeys = []overlayKey{
	{keysymUp, ui.Nav{Dir: ui.DirUp}},
	{keysymDown, ui.Nav{Dir: ui.DirDown}},
	{keysymLeft, ui.Nav{Dir: ui.DirLeft}},
	{keysymRight, ui.Nav{Dir: ui.DirRight}},
	{keysymReturn, ui.Action{Kind: ui.ActConfirm}},
	{keysymBackspace, ui.Action{Kind: ui.ActCancel}},
	{keysymDelete, ui.Action{Kind: ui.ActDelete}},
	{keysymTab, ui.Action{Kind: ui.ActTabNext}},
	{keysymHome, ui.Edge{To: ui.EdgeHome}},
	{keysymEnd, ui.Edge{To: ui.EdgeEnd}},
}

// overlayInput 把持續的按鍵狀態轉成一次性的介面事件。
type overlayInput struct {
	previous map[uint32]bool
	surface  *image.RGBA
}

func newOverlayInput() *overlayInput {
	return &overlayInput{previous: make(map[uint32]bool, len(overlayKeys)+2)}
}

// edge 回報這個 keysym 是否剛剛被按下。
func (o *overlayInput) edge(window *x11.Window, keysym uint32) bool {
	now := window.KeysymPressed(keysym)
	was := o.previous[keysym]
	o.previous[keysym] = now
	return now && !was
}

// canvas 回傳與視窗同尺寸的合成畫布，重複使用同一張圖。
func (o *overlayInput) canvas(width, height int) *image.RGBA {
	if o.surface == nil || o.surface.Bounds().Dx() != width || o.surface.Bounds().Dy() != height {
		o.surface = image.NewRGBA(image.Rect(0, 0, width, height))
	}
	return o.surface
}

// screenshotName 以本地時間命名截圖。截圖直接取自 UM6618 的顯示孔徑，
// 不含覆蓋層也不套濾鏡，所以它可以當畫面證據用。
func screenshotName() string {
	return fmt.Sprintf("acan-%s.png", time.Now().Format("20060102-150405"))
}

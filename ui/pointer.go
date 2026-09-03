package ui

import "image"

// hitRegion 是一塊可以用指標點的區域。座標是設計單位，與 canvas 一致。
//
// 命中區在每一次 Draw 重建：畫面是即時模式繪製的，版面只在畫的當下才算得出來，
// 另外維護一份可點區域的資料結構會有兩份真相，而且一定會有一份先過期。
type hitRegion struct {
	rect image.Rectangle
	// hover 是指標移到這一塊上面時做的事，通常是把焦點移過來。
	hover func(*UI)
	// action 是在同一塊上按下再放開時做的事，等同鍵盤的「確認」。
	action func(*UI)
}

// addHit 由各畫面在自己的 draw 裡呼叫，登記一塊可點區域。
func (u *UI) addHit(x, y, w, h int, hover, action func(*UI)) {
	if w <= 0 || h <= 0 {
		return
	}
	u.hits = append(u.hits, hitRegion{rect: image.Rect(x, y, x+w, y+h), hover: hover, action: action})
}

// hitAt 找出座標落在哪一塊。由後往前找：後畫的在上面，modal 疊在畫面上時
// 要先命中 modal 的按鈕，而不是被它遮住的那一列。
func (u *UI) hitAt(point image.Point) *hitRegion {
	for i := len(u.hits) - 1; i >= 0; i-- {
		if point.In(u.hits[i].rect) {
			return &u.hits[i]
		}
	}
	return nil
}

// handlePointer 把滑鼠或觸控翻成焦點移動與確認。
//
// 覆蓋層沒開時一律不吃：遊戲本身沒有滑鼠輸入，吃掉只會讓前端以為事件被處理了。
//
// 「按下的位置」與「放開的位置」要落在同一塊才算數，這是視窗系統的通則——
// 按下去之後滑開再放，使用者的意思是取消，不是按下那一個。
func (u *UI) handlePointer(event Pointer) bool {
	scale := u.surface.Scale
	if scale <= 0 || !u.Visible() {
		u.pointerDown = false
		return false
	}
	point := image.Pt(event.X/scale, event.Y/scale)
	switch event.Phase {
	case PhaseDown:
		u.pointerDown, u.pointerPress = true, point
		if region := u.hitAt(point); region != nil && region.hover != nil {
			region.hover(u)
		}
		return true
	case PhaseMove:
		if region := u.hitAt(point); region != nil && region.hover != nil {
			region.hover(u)
		}
		return u.pointerDown
	case PhaseUp:
		down := u.pointerDown
		u.pointerDown = false
		if !down {
			return false
		}
		region := u.hitAt(point)
		if region == nil || !u.pointerPress.In(region.rect) {
			return true
		}
		if region.action != nil {
			region.action(u)
		}
		return true
	default: // PhaseCancel
		u.pointerDown = false
		return true
	}
}

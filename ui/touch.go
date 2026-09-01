package ui

import "image"

// PadMask 是 ui 自己的按鈕位元，位元順序即 PadButtons。入口把它翻成 machine 的
// 位元——ui 不 import machine，所以這裡不能直接用硬體的位元定義。
type PadMask uint16

// Pressed 回報某個按鈕名稱是否被按住。
func (m PadMask) Pressed(name string) bool {
	for index, candidate := range PadButtons {
		if candidate == name {
			return m&(1<<index) != 0
		}
	}
	return false
}

func padBit(name string) PadMask {
	for index, candidate := range PadButtons {
		if candidate == name {
			return 1 << index
		}
	}
	return 0
}

// 虛擬手把的尺寸，單位是設計單位。最小命中區 44 見方是硬性下限：低於這個值
// 誤觸率明顯上升。
const (
	TouchMinTarget  = 44
	touchDPadSize   = 176
	touchFaceRadius = 96
	touchFaceSize   = 72
	touchShoulderW  = 96
	touchShoulderH  = 56
	touchSystemW    = 128
	touchSystemH    = 48
	// touchHitGrow 是命中區比繪製區每邊大出來的量。手指會遮住按鍵，
	// 使用者看不到自己按在哪，命中區必須比看得見的圖形寬鬆。
	touchHitGrow = 8
)

// touchControl 是虛擬手把上的一個元件。
type touchControl struct {
	// name 是 PadButtons 裡的名稱，或 "menu"。
	name string
	// draw 是看得見的圖形，hit 是實際的命中區，後者一定比前者大。
	draw image.Rectangle
	hit  image.Rectangle
	// dpad 為真時，這個元件是九宮格方向鍵，命中要再細分。
	dpad bool
}

// TouchLayout 是一個方向下的完整版面。
type TouchLayout struct {
	Portrait bool
	Screen   image.Rectangle
	Controls []touchControl
}

func grow(r image.Rectangle, by int) image.Rectangle {
	return image.Rect(r.Min.X-by, r.Min.Y-by, r.Max.X+by, r.Max.Y+by)
}

func control(name string, x, y, w, h int) touchControl {
	draw := image.Rect(x, y, x+w, y+h)
	hit := grow(draw, touchHitGrow)
	// 命中區不得小於最小值，即使圖形本身畫得比較小。
	if hit.Dx() < TouchMinTarget {
		pad := (TouchMinTarget - hit.Dx() + 1) / 2
		hit = image.Rect(hit.Min.X-pad, hit.Min.Y, hit.Max.X+pad, hit.Max.Y)
	}
	if hit.Dy() < TouchMinTarget {
		pad := (TouchMinTarget - hit.Dy() + 1) / 2
		hit = image.Rect(hit.Min.X, hit.Min.Y-pad, hit.Max.X, hit.Max.Y+pad)
	}
	return touchControl{name: name, draw: draw, hit: hit}
}

// touchLayoutFor 依表面大小算出版面。橫式把按鍵放在 4:3 畫面左右的黑邊上，
// 直式把畫面貼齊上方、控制區獨占下半而不與畫面重疊。
func touchLayoutFor(width, height int, swapHands bool) TouchLayout {
	layout := TouchLayout{Portrait: height > width}
	if layout.Portrait {
		screenW := width
		screenH := screenW * 3 / 4
		layout.Screen = image.Rect(0, TouchMinTarget, screenW, TouchMinTarget+screenH)
		layout.Controls = portraitControls(width, height, layout.Screen.Max.Y)
	} else {
		screenH := height
		screenW := screenH * 4 / 3
		left := (width - screenW) / 2
		layout.Screen = image.Rect(left, 0, left+screenW, screenH)
		layout.Controls = landscapeControls(width, height)
	}
	if swapHands {
		layout.Controls = mirrorControls(layout.Controls, width)
	}
	layout.Controls = append(layout.Controls, control("menu", 8, 8, TouchMinTarget, TouchMinTarget))
	for index := range layout.Controls {
		if layout.Controls[index].name == "up" {
			layout.Controls[index].dpad = true
		}
	}
	return layout
}

// dpadControl 是九宮格方向鍵。它以 "up" 這一個元件承載整塊區域，命中時再依
// 觸點落在哪一格決定方向；中央死區半徑 15%，避免「按上」變成「上＋左」。
func dpadControl(x, y int) touchControl {
	pad := control("up", x, y, touchDPadSize, touchDPadSize)
	pad.dpad = true
	return pad
}

func faceControls(centreX, centreY int) []touchControl {
	half := touchFaceSize / 2
	offset := touchFaceRadius / 2
	return []touchControl{
		control("x", centreX-half, centreY-offset-half, touchFaceSize, touchFaceSize),
		control("b", centreX-half, centreY+offset-half, touchFaceSize, touchFaceSize),
		control("y", centreX-offset-half, centreY-half, touchFaceSize, touchFaceSize),
		control("a", centreX+offset-half, centreY-half, touchFaceSize, touchFaceSize),
	}
}

func landscapeControls(width, height int) []touchControl {
	margin := 24
	dpadY := height/2 - touchDPadSize/2
	controls := []touchControl{dpadControl(margin, dpadY)}
	controls = append(controls, faceControls(width-margin-touchFaceRadius, height/2)...)
	shoulderY := height - touchShoulderH - margin
	controls = append(controls,
		control("l", margin, shoulderY, touchShoulderW, touchShoulderH),
		control("r", width-margin-touchShoulderW, shoulderY, touchShoulderW, touchShoulderH),
		control("select", width/2-touchSystemW-margin/2, height-touchSystemH-margin, touchSystemW, touchSystemH),
		control("start", width/2+margin/2, height-touchSystemH-margin, touchSystemW, touchSystemH),
	)
	return controls
}

func portraitControls(width, height, screenBottom int) []touchControl {
	margin := 24
	area := height - screenBottom
	dpadY := screenBottom + area/4 - touchDPadSize/2
	controls := []touchControl{dpadControl(margin, dpadY)}
	controls = append(controls, faceControls(width-margin-touchFaceRadius, dpadY+touchDPadSize/2)...)
	shoulderY := screenBottom + area*3/5
	controls = append(controls,
		control("l", margin, shoulderY, touchShoulderW, touchShoulderH),
		control("r", width-margin-touchShoulderW, shoulderY, touchShoulderW, touchShoulderH),
		control("select", width/2-touchSystemW-margin/2, height-touchSystemH-margin, touchSystemW, touchSystemH),
		control("start", width/2+margin/2, height-touchSystemH-margin, touchSystemW, touchSystemH),
	)
	return controls
}

// mirrorControls 左右手互換。手掌大小與慣用手的差異無法用單一預設涵蓋。
func mirrorControls(controls []touchControl, width int) []touchControl {
	out := make([]touchControl, len(controls))
	for index, item := range controls {
		flip := func(r image.Rectangle) image.Rectangle {
			return image.Rect(width-r.Max.X, r.Min.Y, width-r.Min.X, r.Max.Y)
		}
		item.draw, item.hit = flip(item.draw), flip(item.hit)
		out[index] = item
	}
	return out
}

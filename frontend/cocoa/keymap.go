// Package cocoa 是 macOS 的視窗與輸入層，透過 purego 直接呼叫 Objective-C runtime，
// 因此整個 binary 在 CGO_ENABLED=0 下可以建置。
//
// 只有 window_darwin.go 會碰到 Objective-C；鍵碼對照表與版面無關的邏輯放在沒有
// build tag 的檔案裡，這樣它們在任何平台上都測得到。
package cocoa

// macOS 的虛擬鍵碼（kVK_*）。這些值出自 Carbon 的 HIToolbox/Events.h，
// 與鍵盤配置無關：它們描述鍵的實體位置，不是鍵面上印的字。
//
// 因此同一個鍵碼在 QWERTY 與 AZERTY 上是不同的字母。設定畫面顯示的名稱以
// ANSI 配置為準並標明，使用者換配置時重新指定即可。
const (
	KeyA          = 0x00
	KeyS          = 0x01
	KeyD          = 0x02
	KeyF          = 0x03
	KeyH          = 0x04
	KeyG          = 0x05
	KeyZ          = 0x06
	KeyX          = 0x07
	KeyC          = 0x08
	KeyV          = 0x09
	KeyB          = 0x0b
	KeyQ          = 0x0c
	KeyW          = 0x0d
	KeyE          = 0x0e
	KeyR          = 0x0f
	KeyY          = 0x10
	KeyT          = 0x11
	Key1          = 0x12
	Key2          = 0x13
	Key3          = 0x14
	Key4          = 0x15
	Key6          = 0x16
	Key5          = 0x17
	Key9          = 0x19
	Key7          = 0x1a
	Key8          = 0x1c
	Key0          = 0x1d
	KeyO          = 0x1f
	KeyU          = 0x20
	KeyI          = 0x22
	KeyP          = 0x23
	KeyReturn     = 0x24
	KeyL          = 0x25
	KeyJ          = 0x26
	KeyK          = 0x28
	KeyN          = 0x2d
	KeyM          = 0x2e
	KeyTab        = 0x30
	KeySpace      = 0x31
	KeyBackspace  = 0x33
	KeyEscape     = 0x35
	KeyRightShift = 0x3c
	KeyF5         = 0x60
	KeyF6         = 0x61
	KeyF7         = 0x62
	KeyF3         = 0x63
	KeyF8         = 0x64
	KeyF9         = 0x65
	KeyF11        = 0x67
	KeyF10        = 0x6d
	KeyF12        = 0x6f
	KeyF4         = 0x76
	KeyF2         = 0x78
	KeyF1         = 0x7a
	KeyHome       = 0x73
	KeyForwardDel = 0x75
	KeyEnd        = 0x77
	KeyLeft       = 0x7b
	KeyRight      = 0x7c
	KeyDown       = 0x7d
	KeyUp         = 0x7e
)

// KeyNames 是設定畫面上顯示的鍵名。只列本前端用得到的鍵；其餘以鍵碼呈現，
// 猜一個好看的名字不如誠實顯示使用者按到的東西。
var KeyNames = map[uint16]string{
	KeyA: "A", KeyB: "B", KeyC: "C", KeyD: "D", KeyE: "E", KeyF: "F",
	KeyG: "G", KeyH: "H", KeyI: "I", KeyJ: "J", KeyK: "K", KeyL: "L",
	KeyM: "M", KeyN: "N", KeyO: "O", KeyP: "P", KeyQ: "Q", KeyR: "R",
	KeyS: "S", KeyT: "T", KeyU: "U", KeyV: "V", KeyW: "W", KeyX: "X",
	KeyY: "Y", KeyZ: "Z",
	Key0: "0", Key1: "1", Key2: "2", Key3: "3", Key4: "4",
	Key5: "5", Key6: "6", Key7: "7", Key8: "8", Key9: "9",
	KeyReturn: "Return", KeyTab: "Tab", KeySpace: "Space",
	KeyBackspace: "Delete", KeyEscape: "Escape", KeyRightShift: "RightShift",
	KeyForwardDel: "ForwardDelete", KeyHome: "Home", KeyEnd: "End",
	KeyLeft: "Left", KeyRight: "Right", KeyDown: "Down", KeyUp: "Up",
	KeyF1: "F1", KeyF2: "F2", KeyF3: "F3", KeyF4: "F4", KeyF5: "F5", KeyF6: "F6",
	KeyF7: "F7", KeyF8: "F8", KeyF9: "F9", KeyF10: "F10", KeyF11: "F11", KeyF12: "F12",
}

// KeyLabel 回傳顯示用的鍵名。
func KeyLabel(code uint16) string {
	if name, ok := KeyNames[code]; ok {
		return name
	}
	return ""
}

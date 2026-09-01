package main

import (
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

// frontendName 是寫進設定檔的前端識別字串。X11 keysym 與 ebiten.Key 是不同的
// 數值空間，設定檔必須記得這組綁定是誰寫的。
const frontendName = "x11"

// padButtons 把介面用的按鈕名稱對到 machine 的位元。
var padButtons = map[string]uint16{
	"up": machine.ButtonUp, "down": machine.ButtonDown,
	"left": machine.ButtonLeft, "right": machine.ButtonRight,
	"a": machine.ButtonA, "b": machine.ButtonB,
	"x": machine.ButtonX, "y": machine.ButtonY,
	"l": machine.ButtonL, "r": machine.ButtonR,
	"start": machine.ButtonStart, "select": machine.ButtonSelect,
}

// defaultKeysyms 是兩位玩家的預設鍵位，沿用 Bcan.ini 的配置。
var defaultKeysyms = [2]map[string]uint32{
	{
		"up": keysymUp, "down": keysymDown, "left": keysymLeft, "right": keysymRight,
		"a": keysymLowerZ, "b": keysymLowerX, "x": keysymLowerA, "y": keysymLowerS,
		"l": keysymLowerQ, "r": keysymLowerW,
		"start": keysymReturn, "select": keysymShiftR,
	},
	{
		"up": keysymLowerI, "down": keysymLowerK, "left": keysymLowerD, "right": keysymLowerG,
		"a": keysymLowerU, "b": keysymLowerR, "x": keysymLowerF, "y": keysymLowerY,
		"l": keysymLowerO, "r": keysymLowerP,
		"start": keysymDigit6, "select": keysymDigit2,
	},
}

// bindingsFor 依設定檔組出一位玩家的鍵位。設定檔裡屬於別的前端的綁定不套用，
// 那組鍵碼在這裡沒有意義；該按鈕回到預設值。
func bindingsFor(config ui.Config, player int) []keyBinding {
	out := make([]keyBinding, 0, len(ui.PadButtons))
	for _, name := range ui.PadButtons {
		keysym := defaultKeysyms[player][name]
		if binding, ok := config.Input.Players[player].Keyboard[name]; ok && binding.Usable(frontendName) {
			keysym = binding.Code
		}
		if keysym == 0 {
			continue
		}
		out = append(out, keyBinding{keysym: keysym, button: padButtons[name]})
	}
	return out
}

// hotkeyKeysym 取一個熱鍵的 keysym，設定檔沒有可用的就回傳預設值。
func hotkeyKeysym(config ui.Config, name string, fallback uint32) uint32 {
	if binding, ok := config.Input.Hotkeys[name]; ok && binding.Usable(frontendName) {
		return binding.Code
	}
	return fallback
}

// keysymNames 是設定畫面上顯示的按鍵名稱。只列常用鍵；其餘以 keysym 數值呈現，
// 猜一個好看的名字不如誠實顯示使用者按到的東西。
var keysymNames = map[uint32]string{
	keysymLeft: "Left", keysymUp: "Up", keysymRight: "Right", keysymDown: "Down",
	keysymReturn: "Enter", keysymShiftR: "RightShift", keysymEscape: "Escape",
	keysymF1: "F1", keysymF5: "F5", keysymF7: "F7",
	keysymTab: "Tab", keysymBackspace: "Backspace", keysymDelete: "Delete",
	keysymHome: "Home", keysymEnd: "End",
}

// keysymLabel 回傳顯示用的按鍵名稱。可列印 ASCII 直接顯示自己。
func keysymLabel(keysym uint32) string {
	if name, ok := keysymNames[keysym]; ok {
		return name
	}
	if keysym >= 0x20 && keysym < 0x7f {
		return string(rune(keysym))
	}
	return ""
}

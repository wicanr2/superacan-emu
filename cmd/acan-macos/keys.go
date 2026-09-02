//go:build darwin

package main

import (
	"github.com/wicanr2/superacan-emu/frontend/cocoa"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

// frontendName 是寫進設定檔的前端識別字串。macOS 的虛擬鍵碼與 X11 keysym 是
// 不同的數值空間，設定檔必須記得這組綁定是誰寫的，否則跨前端讀入會得到錯誤的
// 按鍵——所以這個字串不能與 x11 相同。
const frontendName = "cocoa"

type keyBinding struct {
	code   uint32
	button uint16
}

var padButtons = map[string]uint16{
	"up": machine.ButtonUp, "down": machine.ButtonDown,
	"left": machine.ButtonLeft, "right": machine.ButtonRight,
	"a": machine.ButtonA, "b": machine.ButtonB,
	"x": machine.ButtonX, "y": machine.ButtonY,
	"l": machine.ButtonL, "r": machine.ButtonR,
	"start": machine.ButtonStart, "select": machine.ButtonSelect,
}

// defaultKeys 是兩位玩家的預設鍵位，鍵的實體位置與 X11 前端相同
// （ANSI 配置下的 Z/X/A/S/Q/W 與 I/K/D/G/U/R/F/Y/O/P）。
var defaultKeys = [2]map[string]uint32{
	{
		"up": cocoa.KeyUp, "down": cocoa.KeyDown, "left": cocoa.KeyLeft, "right": cocoa.KeyRight,
		"a": cocoa.KeyZ, "b": cocoa.KeyX, "x": cocoa.KeyA, "y": cocoa.KeyS,
		"l": cocoa.KeyQ, "r": cocoa.KeyW,
		"start": cocoa.KeyReturn, "select": cocoa.KeyRightShift,
	},
	{
		"up": cocoa.KeyI, "down": cocoa.KeyK, "left": cocoa.KeyD, "right": cocoa.KeyG,
		"a": cocoa.KeyU, "b": cocoa.KeyR, "x": cocoa.KeyF, "y": cocoa.KeyY,
		"l": cocoa.KeyO, "r": cocoa.KeyP,
		"start": cocoa.Key6, "select": cocoa.Key2,
	},
}

// bindingsFor 依設定檔組出一位玩家的鍵位。設定檔裡屬於別的前端的綁定不套用，
// 那組鍵碼在這裡沒有意義；該按鈕回到預設值。
func bindingsFor(config ui.Config, player int) []keyBinding {
	out := make([]keyBinding, 0, len(ui.PadButtons))
	for _, name := range ui.PadButtons {
		code := defaultKeys[player][name]
		if binding, ok := config.Input.Players[player].Keyboard[name]; ok && binding.Usable(frontendName) {
			code = binding.Code
		}
		out = append(out, keyBinding{code: code, button: padButtons[name]})
	}
	return out
}

func hotkeyCode(config ui.Config, name string, fallback uint32) uint32 {
	if binding, ok := config.Input.Hotkeys[name]; ok && binding.Usable(frontendName) {
		return binding.Code
	}
	return fallback
}

// defaultHotkeyCodes 是出廠的熱鍵鍵位，與 X11 前端同一組功能鍵。沒有列在這裡的
// 動作預設不綁鍵：它們要嘛只在診斷時用，要嘛需要修飾鍵才不會誤觸，而這一層還
// 沒有修飾鍵的概念。使用者可以在 S5.2 自己指定。
//
// 這些是虛擬鍵碼，描述的是鍵盤上的實體位置。功能鍵在各種版面上位置一致，
// 所以這一組不像字母鍵那樣受版面影響。
var defaultHotkeyCodes = map[string]uint32{
	"menu":           cocoa.KeyF1,
	"pause":          cocoa.KeyF2,
	"soft_reset":     cocoa.KeyF3,
	"mute":           cocoa.KeyF4,
	"save_state":     cocoa.KeyF5,
	"next_slot":      cocoa.KeyF6,
	"load_state":     cocoa.KeyF7,
	"screenshot":     cocoa.KeyF8,
	"capture":        cocoa.KeyF9,
	"show_fps":       cocoa.KeyF10,
	"fullscreen":     cocoa.KeyF11,
	"load_cartridge": cocoa.KeyF12,
	"fast_forward":   cocoa.KeyTab,
}

// defaultHotkeyBindings 是交給介面顯示的出廠鍵位。設定畫面顯示的必須是實際
// 生效的鍵，而不是「設定檔裡有沒有寫」。
func defaultHotkeyBindings() map[string]ui.Binding {
	out := make(map[string]ui.Binding, len(defaultHotkeyCodes))
	for action, code := range defaultHotkeyCodes {
		out[action] = ui.Binding{Frontend: frontendName, Code: code, Label: cocoa.KeyLabel(uint16(code))}
	}
	return out
}

// hotkeyBinding 是一個已解析到鍵碼的熱鍵動作。
type hotkeyBinding struct {
	action string
	code   uint32
}

// hotkeyBindings 依 ui.Hotkeys 的順序解析全部熱鍵，順序固定所以遍歷可重現。
func hotkeyBindings(config ui.Config, skip ...string) []hotkeyBinding {
	skipped := make(map[string]bool, len(skip))
	for _, name := range skip {
		skipped[name] = true
	}
	out := make([]hotkeyBinding, 0, len(ui.Hotkeys))
	for _, action := range ui.Hotkeys {
		if skipped[action] {
			continue
		}
		code := hotkeyCode(config, action, defaultHotkeyCodes[action])
		if code == 0 {
			continue
		}
		out = append(out, hotkeyBinding{action: action, code: code})
	}
	return out
}

// overlayKeys 把實體鍵翻成介面事件。與 X11 前端相同的一組操作方式。
var overlayKeys = []struct {
	code  uint32
	event ui.Event
}{
	{cocoa.KeyUp, ui.Nav{Dir: ui.DirUp}},
	{cocoa.KeyDown, ui.Nav{Dir: ui.DirDown}},
	{cocoa.KeyLeft, ui.Nav{Dir: ui.DirLeft}},
	{cocoa.KeyRight, ui.Nav{Dir: ui.DirRight}},
	{cocoa.KeyReturn, ui.Action{Kind: ui.ActConfirm}},
	{cocoa.KeyBackspace, ui.Action{Kind: ui.ActCancel}},
	{cocoa.KeyForwardDel, ui.Action{Kind: ui.ActDelete}},
	{cocoa.KeyTab, ui.Action{Kind: ui.ActTabNext}},
	{cocoa.KeyHome, ui.Edge{To: ui.EdgeHome}},
	{cocoa.KeyEnd, ui.Edge{To: ui.EdgeEnd}},
}

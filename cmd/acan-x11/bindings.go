package main

import (
	"os"
	"path/filepath"

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

// defaultHotkeyKeysyms 是出廠的熱鍵鍵位。沒有列在這裡的動作預設不綁鍵：
// 它們要嘛只在診斷時用（圖層遮罩），要嘛需要一個修飾鍵才不會誤觸（上一個槽），
// 而這一層還沒有修飾鍵的概念。使用者可以在 S5.2 自己指定。
var defaultHotkeyKeysyms = map[string]uint32{
	"menu":           keysymF1,
	"pause":          keysymF2,
	"soft_reset":     keysymF3,
	"mute":           keysymF4,
	"save_state":     keysymF5,
	"next_slot":      keysymF6,
	"load_state":     keysymF7,
	"screenshot":     keysymF8,
	"capture":        keysymF9,
	"show_fps":       keysymF10,
	"fullscreen":     keysymF11,
	"load_cartridge": keysymF12,
	"fast_forward":   keysymTab,
}

// defaultHotkeyBindings 是交給介面顯示的出廠鍵位。設定畫面顯示的必須是實際
// 生效的鍵，而不是「設定檔裡有沒有寫」。
func defaultHotkeyBindings() map[string]ui.Binding {
	out := make(map[string]ui.Binding, len(defaultHotkeyKeysyms))
	for action, keysym := range defaultHotkeyKeysyms {
		out[action] = ui.Binding{Frontend: frontendName, Code: keysym, Label: keysymLabel(keysym)}
	}
	return out
}

// hotkeyBinding 是一個已解析到鍵碼的熱鍵動作。
type hotkeyBinding struct {
	action string
	keysym uint32
}

// hotkeyBindings 依 ui.Hotkeys 的順序解析全部熱鍵。順序固定，遍歷才可重現；
// 設定檔裡屬於別的前端的綁定不套用，那組鍵碼在 X11 底下沒有意義。
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
		keysym := hotkeyKeysym(config, action, defaultHotkeyKeysyms[action])
		if keysym == 0 {
			continue
		}
		out = append(out, hotkeyBinding{action: action, keysym: keysym})
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
	keysymF1: "F1", keysymF2: "F2", keysymF3: "F3", keysymF4: "F4",
	keysymF5: "F5", keysymF6: "F6", keysymF7: "F7", keysymF8: "F8",
	keysymF9: "F9", keysymF10: "F10", keysymF11: "F11", keysymF12: "F12",
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

// paths 是沒有給命令列旗標時的預設位置。發行包（AppImage）要能直接點兩下就開，
// 所以每一個路徑都要有一個可預期而且寫得下來的預設值。
type paths struct {
	ipl, key, soundA, soundB string
	cartridges               string
	root                     string
}

// defaultPaths 依 XDG 慣例組出預設位置。受版權保護的韌體與卡帶由使用者自己放進去；
// 程式不隨附也不代為下載。
func defaultPaths() paths {
	root := os.Getenv("XDG_DATA_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		root = filepath.Join(home, ".local", "share")
	}
	root = filepath.Join(root, "superacan-emu")
	firmware := filepath.Join(root, "firmware")
	return paths{
		ipl:        filepath.Join(firmware, "internal_68k.bin"),
		key:        filepath.Join(firmware, "umc6650.bin"),
		soundA:     filepath.Join(firmware, "internal_6502_1.bin"),
		soundB:     filepath.Join(firmware, "internal_6502_2.bin"),
		cartridges: filepath.Join(root, "cartridges"),
		root:       root,
	}
}

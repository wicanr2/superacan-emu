package main

import (
	"testing"

	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

func keysymFor(bindings []keyBinding, button uint16) uint32 {
	for _, binding := range bindings {
		if binding.button == button {
			return binding.keysym
		}
	}
	return 0
}

// 沒有設定檔時用預設鍵位。
func TestDefaultBindings(t *testing.T) {
	bindings := bindingsFor(ui.DefaultConfig(), 0)
	if got := keysymFor(bindings, machine.ButtonA); got != keysymLowerZ {
		t.Fatalf("A=$%04X，want $%04X", got, keysymLowerZ)
	}
	if got := keysymFor(bindings, machine.ButtonStart); got != keysymReturn {
		t.Fatalf("Start=$%04X，want $%04X", got, keysymReturn)
	}
}

// 設定檔裡屬於這個前端的綁定要生效——這就是「重新綁定後重啟仍生效」。
func TestConfiguredBindingWins(t *testing.T) {
	config := ui.DefaultConfig()
	config.Input.Players[0].Keyboard["a"] = ui.Binding{Frontend: frontendName, Code: 0x71, Label: "q"}
	if got := keysymFor(bindingsFor(config, 0), machine.ButtonA); got != 0x71 {
		t.Fatalf("A=$%04X，want $0071", got)
	}
}

// 別的前端寫的鍵碼不得被套用：X11 keysym 與 ebiten.Key 是不同的數值空間，
// 硬套會得到一個看起來有設定、實際按不動的按鈕。
func TestForeignFrontendBindingFallsBackToDefault(t *testing.T) {
	config := ui.DefaultConfig()
	config.Input.Players[0].Keyboard["a"] = ui.Binding{Frontend: "ebiten", Code: 12, Label: "Z"}
	if got := keysymFor(bindingsFor(config, 0), machine.ButtonA); got != keysymLowerZ {
		t.Fatalf("A=$%04X，want 預設的 $%04X", got, keysymLowerZ)
	}
}

// 熱鍵同樣依前端過濾。
func TestHotkeyFallsBackWhenFrontendDiffers(t *testing.T) {
	config := ui.DefaultConfig()
	config.Input.Hotkeys["menu"] = ui.Binding{Frontend: "ebiten", Code: 99}
	if got := hotkeyKeysym(config, "menu", keysymF1); got != keysymF1 {
		t.Fatalf("menu=$%04X，want $%04X", got, keysymF1)
	}
	config.Input.Hotkeys["menu"] = ui.Binding{Frontend: frontendName, Code: keysymEscape}
	if got := hotkeyKeysym(config, "menu", keysymF1); got != keysymEscape {
		t.Fatalf("menu=$%04X，want $%04X", got, keysymEscape)
	}
}

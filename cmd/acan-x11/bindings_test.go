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

// ui.Hotkeys 多一個動作時，這裡會提醒要不要給它一個出廠鍵位。沒有預設鍵位是
// 合法的選擇（診斷用、容易誤觸的動作），但必須是選擇過的。
func TestDefaultHotkeysCoverTheExpectedActions(t *testing.T) {
	unbound := map[string]bool{
		"prev_slot": true, "fast_forward_lock": true,
		"lock_cheats": true, "cycle_layer_mask": true,
	}
	for _, action := range ui.Hotkeys {
		_, hasDefault := defaultHotkeyKeysyms[action]
		if hasDefault == unbound[action] {
			t.Errorf("動作 %q 的出廠鍵位狀態不符預期（有預設=%v）", action, hasDefault)
		}
	}
	seen := map[uint32]string{}
	for action, keysym := range defaultHotkeyKeysyms {
		if other, clash := seen[keysym]; clash {
			t.Errorf("出廠鍵位 %#x 同時給了 %q 與 %q", keysym, other, action)
		}
		seen[keysym] = action
	}
}

func TestHotkeyBindingsFollowTheConfigAndSkipList(t *testing.T) {
	config := ui.DefaultConfig()
	config.Input.Hotkeys["screenshot"] = ui.Binding{Frontend: frontendName, Code: 0x61, Label: "a"}
	// 別的前端寫的綁定不套用：那組鍵碼在 X11 底下沒有意義。
	config.Input.Hotkeys["capture"] = ui.Binding{Frontend: "cocoa", Code: 0x65, Label: "F9"}

	bindings := hotkeyBindings(config, "save_state")
	byAction := map[string]uint32{}
	for _, binding := range bindings {
		byAction[binding.action] = binding.keysym
	}
	if got := byAction["screenshot"]; got != 0x61 {
		t.Errorf("screenshot 的鍵碼是 %#x，預期設定檔的 0x61", got)
	}
	if got := byAction["capture"]; got != keysymF9 {
		t.Errorf("capture 的鍵碼是 %#x，預期回到出廠的 F9", got)
	}
	if _, present := byAction["save_state"]; present {
		t.Error("save_state 在略過清單裡，不該出現")
	}
	if _, present := byAction["cycle_layer_mask"]; present {
		t.Error("沒有出廠鍵位也沒有設定的動作不該出現")
	}
	// 順序固定才可重現。
	for index := 1; index < len(bindings); index++ {
		if indexOfAction(bindings[index-1].action) > indexOfAction(bindings[index].action) {
			t.Fatalf("順序與 ui.Hotkeys 不同：%v", bindings)
		}
	}
}

func indexOfAction(action string) int {
	for index, name := range ui.Hotkeys {
		if name == action {
			return index
		}
	}
	return -1
}

// 手把狀態是 active-low：按下是把位元清掉。這條擋的是「從 PadReleased 開始再 OR
// 上按鍵位元」——那樣寫編得過、跑得動、看起來合理，但因為那些位元本來就全是 1，
// 結果永遠是「全部放開」，鍵盤等於沒有接上。
func TestPadStateClearsBitsForPressedKeys(t *testing.T) {
	bindings := bindingsFor(ui.DefaultConfig(), 0)
	if len(bindings) == 0 {
		t.Fatal("預設鍵位是空的")
	}

	if got := padStateFor(bindings, func(uint32) bool { return false }); got != machine.PadReleased {
		t.Fatalf("沒有按鍵時是 %#x，預期 %#x", got, machine.PadReleased)
	}

	// 只按住第一個綁定的鍵：它的位元要被清掉，其餘維持 1。
	first := bindings[0]
	state := padStateFor(bindings, func(keysym uint32) bool { return keysym == first.keysym })
	if state&first.button != 0 {
		t.Fatalf("按住之後 %#x 的位元沒有被清掉：%#x", first.button, state)
	}
	if state|first.button != machine.PadReleased {
		t.Fatalf("除了被按的鍵之外還動到別的位元：%#x", state)
	}

	// 全部按住：十二個按鈕的位元都要清掉。
	all := padStateFor(bindings, func(uint32) bool { return true })
	var everything uint16
	for _, binding := range bindings {
		everything |= binding.button
	}
	if all != machine.PadState(everything) {
		t.Fatalf("全部按住時是 %#x，預期 %#x", all, machine.PadState(everything))
	}
}

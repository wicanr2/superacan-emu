package ui

import (
	"reflect"
	"testing"
)

var hotkeySurface = Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}

// recordingDiag 讓擷取熱鍵有一個可控的「正在錄影嗎」。
type recordingDiag struct{ recording bool }

func (d *recordingDiag) Diagnostics() DiagnosticsFacts {
	return DiagnosticsFacts{Recording: d.recording}
}

func hotkeyUI(t *testing.T) *UI {
	t.Helper()
	u := New(Options{Surface: hotkeySurface, Config: DefaultConfig(), Slots: fixedSlots{}})
	u.Update(0)
	return u
}

// TestEveryHotkeyActionIsImplemented 守著「設定畫面列得出來、按下去卻沒作用」
// 這個缺口：Hotkeys 多一個名稱就必須多一個動作。
func TestEveryHotkeyActionIsImplemented(t *testing.T) {
	for _, action := range Hotkeys {
		u := hotkeyUI(t)
		if !u.Hotkey(action) {
			t.Errorf("熱鍵 %q 沒有對應動作", action)
		}
	}
}

func TestHotkeyMenuOpensAndClosesTheOverlay(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("menu")
	if !u.Visible() {
		t.Fatal("menu 沒有叫出覆蓋層")
	}
	u.Hotkey("menu")
	if u.Visible() {
		t.Fatal("menu 沒有關掉覆蓋層")
	}
}

// TestHotkeyPauseSurvivesTheMenu 是 u.paused 與 userPaused 分開記的理由：
// 用熱鍵暫停之後開一次選單再關掉，暫停不能被吃掉。
func TestHotkeyPauseSurvivesTheMenu(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("pause")
	if got := lastPaused(t, u.TakeIntents()); !got {
		t.Fatal("pause 沒有送出 SetPaused{true}")
	}
	u.Hotkey("menu")
	u.TakeIntents()
	u.Handle(Action{Kind: ActMenu}) // 關掉選單
	if u.Visible() {
		t.Fatal("選單沒有關掉")
	}
	if got := lastPaused(t, u.TakeIntents()); !got {
		t.Fatal("關掉選單把使用者的暫停一起解除了")
	}
	u.Hotkey("pause")
	if got := lastPaused(t, u.TakeIntents()); got {
		t.Fatal("再按一次 pause 沒有恢復執行")
	}
}

func lastPaused(t *testing.T, intents []Intent) bool {
	t.Helper()
	for i := len(intents) - 1; i >= 0; i-- {
		if paused, ok := intents[i].(SetPaused); ok {
			return paused.Paused
		}
	}
	t.Fatal("沒有 SetPaused intent")
	return false
}

func TestHotkeyStateUsesTheCurrentSlot(t *testing.T) {
	u := hotkeyUI(t)
	u.config.Interface.SaveSlot = 3
	u.Hotkey("save_state")
	u.Hotkey("load_state")
	intents := u.TakeIntents()
	want := []Intent{SaveState{Slot: 3}, LoadState{Slot: 3}}
	if !reflect.DeepEqual(intents, want) {
		t.Fatalf("intents = %#v，預期 %#v", intents, want)
	}
}

func TestHotkeySlotCycleWraps(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("prev_slot")
	if got := u.config.Interface.SaveSlot; got != SlotCount-1 {
		t.Fatalf("從槽 0 往前是槽 %d，預期 %d", got, SlotCount-1)
	}
	u.Hotkey("next_slot")
	if got := u.config.Interface.SaveSlot; got != 0 {
		t.Fatalf("繞回來是槽 %d，預期 0", got)
	}
	// 槽號存在設定檔裡，換槽必須寫回去，否則下次啟動悄悄回到舊值。
	if !hasApplyConfig(u.TakeIntents()) {
		t.Fatal("換槽沒有送出 ApplyConfig")
	}
}

func hasApplyConfig(intents []Intent) bool {
	for _, intent := range intents {
		if _, ok := intent.(ApplyConfig); ok {
			return true
		}
	}
	return false
}

// TestHotkeyFastForwardRespectsTheLock 是按住型與鎖定型並存的理由：
// 按住全速鍵放開之後要回到鎖定狀態，不是無條件回到實時速度。
func TestHotkeyFastForwardRespectsTheLock(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("fast_forward")
	u.HotkeyRelease("fast_forward")
	intents := u.TakeIntents()
	want := []Intent{SetPacing{Paced: false}, SetPacing{Paced: true}}
	if !reflect.DeepEqual(intents, want) {
		t.Fatalf("未鎖定時 intents = %#v，預期 %#v", intents, want)
	}

	u.Hotkey("fast_forward_lock")
	u.TakeIntents()
	u.Hotkey("fast_forward")
	u.HotkeyRelease("fast_forward")
	for _, intent := range u.TakeIntents() {
		if pacing, ok := intent.(SetPacing); ok && pacing.Paced {
			t.Fatal("鎖定中放開全速鍵把鎖定解掉了")
		}
	}
}

func TestHotkeyMuteRestoresThePreviousVolume(t *testing.T) {
	u := hotkeyUI(t)
	u.config.Audio.MasterVolume = 35
	u.Hotkey("mute")
	if got := u.config.Audio.MasterVolume; got != 0 {
		t.Fatalf("靜音後音量 %d，預期 0", got)
	}
	u.Hotkey("mute")
	if got := u.config.Audio.MasterVolume; got != 35 {
		t.Fatalf("解除靜音後音量 %d，預期 35", got)
	}
}

// TestHotkeyMuteWithoutMemoryFallsBackToDefault 是「靜音狀態下重開」的情況：
// 設定檔存的是實際生效的音量，這時沒有原音量可以回去。
func TestHotkeyMuteWithoutMemoryFallsBackToDefault(t *testing.T) {
	u := hotkeyUI(t)
	u.config.Audio.MasterVolume = 0
	u.Hotkey("mute")
	if got, want := u.config.Audio.MasterVolume, DefaultConfig().Audio.MasterVolume; got != want {
		t.Fatalf("音量 %d，預期回到預設值 %d", got, want)
	}
}

func TestHotkeyCaptureFollowsTheRecordingState(t *testing.T) {
	diag := &recordingDiag{}
	u := New(Options{Surface: hotkeySurface, Config: DefaultConfig(), Diagnostics: diag})
	u.Update(0)

	u.Hotkey("capture")
	if intents := u.TakeIntents(); !reflect.DeepEqual(intents, []Intent{Capture{Kind: CaptureClipStart}}) {
		t.Fatalf("沒在錄影時 intents = %#v", intents)
	}
	diag.recording = true
	u.Hotkey("capture")
	if intents := u.TakeIntents(); !reflect.DeepEqual(intents, []Intent{Capture{Kind: CaptureClipStop}}) {
		t.Fatalf("錄影中 intents = %#v", intents)
	}
}

func TestHotkeyLayerMaskCyclesThroughEveryLayer(t *testing.T) {
	u := hotkeyUI(t)
	seen := map[uint32]bool{}
	for range layerMaskCycle {
		u.Hotkey("cycle_layer_mask")
		intents := u.TakeIntents()
		mask, ok := intents[len(intents)-1].(SetLayerMask)
		if !ok {
			t.Fatalf("最後一個 intent 不是 SetLayerMask：%#v", intents)
		}
		seen[mask.Mask] = true
		// 入口套用成功之後才回報，這裡直接模擬那一步。
		u.SetLayerMask(mask.Mask)
	}
	if len(seen) != len(layerMaskCycle) {
		t.Fatalf("循環只走過 %d 種遮罩，預期 %d", len(seen), len(layerMaskCycle))
	}
}

// TestHotkeysAreInertWhileTheOverlayIsOpen 守著雙重語意：選單開著時方向鍵與
// Enter 是導覽，熱鍵不能同時把它們當成命令。
func TestHotkeysAreInertWhileTheOverlayIsOpen(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("menu")
	u.TakeIntents()
	for _, action := range Hotkeys {
		if action == "menu" {
			continue
		}
		if u.Hotkey(action) {
			t.Errorf("選單開著時熱鍵 %q 仍然生效", action)
		}
	}
	if intents := u.TakeIntents(); len(intents) != 0 {
		t.Fatalf("選單開著時熱鍵送出了 %#v", intents)
	}
}

// TestHotkeysAreInertWhileRebinding 是同一件事的另一面：正在指定綁定時，
// 每一個鍵都必須能被指定，包含已經被別的動作佔用的鍵。
func TestHotkeysAreInertWhileRebinding(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("menu")
	u.push(&hotkeyScreen{waiting: true})
	if !u.WantsRawInput() {
		t.Fatal("熱鍵設定畫面沒有進入等待指定狀態")
	}
	for _, action := range Hotkeys {
		if u.Hotkey(action) {
			t.Errorf("等待指定綁定時熱鍵 %q 仍然生效", action)
		}
	}
}

func TestHotkeyLoadCartridgeOpensTheBrowser(t *testing.T) {
	u := hotkeyUI(t)
	u.Hotkey("load_cartridge")
	if !u.Visible() {
		t.Fatal("沒有叫出覆蓋層")
	}
	if got := u.stack[len(u.stack)-1].id(); got != "S1" {
		t.Fatalf("最上層是 %s，預期 S1 卡帶瀏覽器", got)
	}
}

func TestHotkeyUnknownActionIsRejected(t *testing.T) {
	u := hotkeyUI(t)
	if u.Hotkey("teleport") {
		t.Fatal("不認得的動作被當成有效")
	}
}

// 設定畫面要顯示實際生效的鍵：出廠鍵位一樣會生效，設定檔沒寫不代表沒有鍵。
func TestHotkeyScreenShowsFactoryBindings(t *testing.T) {
	u := hotkeyUI(t)
	u.SetDefaultHotkeys(map[string]Binding{
		"save_state": {Frontend: "x11", Code: 0xffc2, Label: "F5"},
	})
	u.config.Input.Hotkeys["screenshot"] = Binding{Frontend: "x11", Code: 0x61, Label: "a"}

	rows := (&hotkeyScreen{}).rows(u)
	byAction := map[string]Binding{}
	for _, row := range rows {
		byAction[row.key] = row.keyboard
	}
	if got := byAction["save_state"].Label; got != "F5" {
		t.Errorf("save_state 顯示 %q，預期出廠鍵位 F5", got)
	}
	if got := byAction["screenshot"].Label; got != "a" {
		t.Errorf("screenshot 顯示 %q，預期設定檔的 a", got)
	}
	if got := byAction["cycle_layer_mask"]; !got.Empty() {
		t.Errorf("沒有鍵位的動作顯示了 %+v", got)
	}
}

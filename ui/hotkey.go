package ui

import "fmt"

// SetDefaultHotkeys 讓入口把出廠鍵位交給介面。設定畫面上顯示的必須是實際生效的
// 鍵：出廠鍵位一樣會生效，「設定檔裡沒寫」不等於「沒有鍵」。傳進來的綁定要帶
// 自己的前端識別字串，換前端時才不會顯示別人的鍵碼。
func (u *UI) SetDefaultHotkeys(defaults map[string]Binding) { u.defaultHotkeys = defaults }

// hotkeyBinding 取一個動作實際生效的綁定：設定檔優先，其次出廠鍵位。
func (u *UI) hotkeyBinding(action string) Binding {
	if binding, ok := u.config.Input.Hotkeys[action]; ok && !binding.Empty() {
		return binding
	}
	return u.defaultHotkeys[action]
}

// Hotkey 執行一個熱鍵動作，名稱與 Hotkeys 相同，回報這一次有沒有真的做事。
//
// 熱鍵走的是覆蓋選單同一條路：改自己的設定、送出 Intent、留下提示。ui 一樣
// 不碰模擬核心，所以熱鍵不會變成一條繞過 Intent 邊界的捷徑；入口只負責把
// 「這個鍵剛按下」翻譯成動作名稱。
//
// 覆蓋層開著時只有 menu 有作用。其餘動作在選單裡有自己的入口，讓熱鍵同時生效
// 會出現「按鍵同時被當成導覽與命令」的雙重語意；等待指定綁定時全部不生效，
// 否則使用者永遠指定不到已經被佔用的鍵。
func (u *UI) Hotkey(action string) bool {
	if u.WantsRawInput() {
		return false
	}
	if u.Visible() && action != "menu" {
		return false
	}
	switch action {
	case "menu":
		u.Handle(Action{Kind: ActMenu})
	case "pause":
		u.userPaused = !u.userPaused
		u.paused = u.userPaused
		u.emit(SetPaused{Paused: u.userPaused})
		if u.userPaused {
			u.toast(u.s.Paused, SeverityInfo)
		} else {
			u.toast(u.s.HotkeyResumed, SeverityInfo)
		}
	case "save_state":
		slot := u.config.Interface.SaveSlot
		u.emit(SaveState{Slot: slot})
		u.toast(fmt.Sprintf(u.s.Saved, slot), SeverityInfo)
	case "load_state":
		slot := u.config.Interface.SaveSlot
		// 空槽與壞檔由入口拒絕並寫進錯誤列，這裡不先判斷：判斷兩次會出現
		// 「畫面說可以、載入卻失敗」的兩套真相。
		u.emit(LoadState{Slot: slot})
		u.toast(fmt.Sprintf(u.s.Loaded, slot), SeverityInfo)
	case "next_slot":
		u.cycleSlot(1)
	case "prev_slot":
		u.cycleSlot(-1)
	case "screenshot":
		u.emit(Capture{Kind: CaptureScreenshot})
		u.toast(u.s.ScreenshotSaved, SeverityInfo)
	case "capture":
		if u.diagnostics(nil).Recording {
			u.emit(Capture{Kind: CaptureClipStop})
			u.toast(u.s.CaptureStopped, SeverityInfo)
			break
		}
		u.emit(Capture{Kind: CaptureClipStart})
		u.toast(u.s.CaptureStarted, SeverityInfo)
	case "show_fps":
		u.config.Video.ShowFPS = !u.config.Video.ShowFPS
		u.applyConfig()
		u.toastSetting(u.s.VideoShowFPS, u.config.Video.ShowFPS)
	case "fast_forward":
		// 按住型：放開時由 HotkeyRelease 還原。鎖定中再按住不改變任何事。
		u.emit(SetPacing{Paced: false})
	case "fast_forward_lock":
		u.fastForwardLock = !u.fastForwardLock
		u.emit(SetPacing{Paced: !u.fastForwardLock})
		u.toastSetting(u.s.HotkeyFastForward, u.fastForwardLock)
	case "mute":
		u.toggleMute()
	case "fullscreen":
		u.config.Video.Fullscreen = !u.config.Video.Fullscreen
		u.applyConfig()
		u.toastSetting(u.s.VideoFullscreen, u.config.Video.Fullscreen)
	case "soft_reset":
		u.emit(Reset{Kind: ResetSoft})
		u.toast(u.s.SoftReset, SeverityInfo)
	case "load_cartridge":
		// 卡帶瀏覽器是一個畫面，不是一個 Intent：選了哪一個檔案才會有 Intent。
		u.Open()
		u.push(&browserScreen{})
	case "lock_cheats":
		u.config.Cheats.LockAll = !u.config.Cheats.LockAll
		u.applyConfig()
		u.toastSetting(u.s.CheatColumnLock, u.config.Cheats.LockAll)
	case "cycle_layer_mask":
		mask := nextLayerMask(u.layerMask())
		u.emit(SetLayerMask{Mask: mask})
		u.toast(fmt.Sprintf(u.s.HotkeyToast, u.s.DiagLayerMask, fmt.Sprintf("0x%02X", mask)), SeverityInfo)
	default:
		return false
	}
	return true
}

// HotkeyRelease 處理按住型熱鍵放開。只有 fast_forward 是按住型；放開之後回到
// 鎖定狀態決定的速度，而不是無條件回到實時速度——否則按一下全速鍵會把鎖定解掉。
func (u *UI) HotkeyRelease(action string) bool {
	if action != "fast_forward" {
		return false
	}
	u.emit(SetPacing{Paced: !u.fastForwardLock})
	return true
}

// cycleSlot 換目前的存檔槽。槽號在 0 與 SlotCount-1 之間繞回，
// 因為熱鍵沒有「到底了」可以顯示的地方。
func (u *UI) cycleSlot(delta int) {
	slot := (u.config.Interface.SaveSlot + delta + SlotCount) % SlotCount
	u.config.Interface.SaveSlot = slot
	u.applyConfig()
	u.toast(fmt.Sprintf(u.s.HotkeySlot, slot), SeverityInfo)
}

// toggleMute 在靜音與原音量之間切換。原音量記在 UI 裡：設定檔存的是實際生效的
// 音量，靜音之後重開仍然是靜音，這時解除靜音回到預設值。
func (u *UI) toggleMute() {
	if u.config.Audio.MasterVolume > 0 {
		u.mutedVolume = u.config.Audio.MasterVolume
		u.config.Audio.MasterVolume = 0
		u.applyConfig()
		u.toastSetting(u.s.HotkeyMute, true)
		return
	}
	restored := u.mutedVolume
	if restored <= 0 {
		restored = DefaultConfig().Audio.MasterVolume
	}
	u.config.Audio.MasterVolume = restored
	u.applyConfig()
	u.toastSetting(u.s.HotkeyMute, false)
}

// applyConfig 把整份設定交給入口寫檔。熱鍵改的每一項都是設定檔裡的欄位，
// 只改記憶體裡的那份會讓下次啟動悄悄回到舊值。
func (u *UI) applyConfig() { u.emit(ApplyConfig{Config: u.config}) }

// toastSetting 顯示「某項設定：開／關」。
func (u *UI) toastSetting(label string, on bool) {
	state := u.s.Off
	if on {
		state = u.s.On
	}
	u.toast(fmt.Sprintf(u.s.HotkeyToast, label, state), SeverityInfo)
}

// layerMaskCycle 是循環順序：全部，然後逐層單獨顯示。逐層看是這個診斷唯一
// 的用途，所以不做任意組合。
var layerMaskCycle = []uint32{
	AllLayers, LayerTilemap0, LayerTilemap1, LayerTilemap2,
	LayerSprite, LayerROZ, LayerWindow,
}

func nextLayerMask(current uint32) uint32 {
	for index, mask := range layerMaskCycle {
		if mask == current {
			return layerMaskCycle[(index+1)%len(layerMaskCycle)]
		}
	}
	return layerMaskCycle[0]
}

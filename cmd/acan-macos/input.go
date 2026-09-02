//go:build darwin

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/wicanr2/superacan-emu/frontend/cocoa"
	"github.com/wicanr2/superacan-emu/machine"
)

// 版本資訊。發行時由 -ldflags 覆蓋。
var (
	buildVersion = "dev"
	buildDate    = "unknown"
)

// inputState 把持續的按鍵狀態轉成一次性的介面事件。導覽的重複行為要由介面決定，
// 不是由系統的按鍵自動重複決定。
type inputState struct{ previous map[uint32]bool }

func newInputState() *inputState { return &inputState{previous: map[uint32]bool{}} }

func (i *inputState) edge(window *cocoa.Window, code uint32) bool {
	now := window.KeysymPressed(code)
	was := i.previous[code]
	i.previous[code] = now
	return now && !was
}

// transition 回報這個鍵的按下與放開瞬間。按住型熱鍵兩邊都要知道，
// 而 edge 只回報按下，兩者共用同一份 previous 才不會互相吃掉狀態。
func (i *inputState) transition(window *cocoa.Window, code uint32) (down, up bool) {
	now := window.KeysymPressed(code)
	was := i.previous[code]
	i.previous[code] = now
	return now && !was, was && !now
}

// sync 在不產生事件的情況下更新邊緣狀態，否則關掉選單之後第一次按鍵會被吃掉。
func (i *inputState) sync(window *cocoa.Window, hotkeys []hotkeyBinding) {
	for _, key := range overlayKeys {
		i.previous[key.code] = window.KeysymPressed(key.code)
	}
	i.previous[cocoa.KeyEscape] = window.KeysymPressed(cocoa.KeyEscape)
	for _, hotkey := range hotkeys {
		i.previous[hotkey.code] = window.KeysymPressed(hotkey.code)
	}
}

// padState 把「哪些鍵正被按著」組成手把狀態。手把狀態是 active-low：按下是把
// 位元清掉，所以要先用「按下的位元」組出正常邏輯的值，最後交給
// machine.PadState 反相。
//
// 從 PadReleased 開始再 OR 上按鍵位元是不會有作用的——那些位元本來就全是 1。
func padState(window *cocoa.Window, bindings []keyBinding) uint16 {
	var pressed uint16
	for _, binding := range bindings {
		if binding.code != 0 && window.KeysymPressed(binding.code) {
			pressed |= binding.button
		}
	}
	return machine.PadState(pressed)
}

func splitList(spec string) []string {
	var out []string
	for _, item := range strings.Split(spec, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func recentFor(romPath string) []string {
	if romPath == "" {
		return nil
	}
	return []string{romPath}
}

// screenshotName 以本地時間命名截圖。
func screenshotName() string {
	return fmt.Sprintf("acan-%s.png", time.Now().Format("20060102-150405"))
}

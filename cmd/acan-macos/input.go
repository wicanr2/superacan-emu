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

// sync 在不產生事件的情況下更新邊緣狀態，否則關掉選單之後第一次按鍵會被吃掉。
func (i *inputState) sync(window *cocoa.Window) {
	for _, key := range overlayKeys {
		i.previous[key.code] = window.KeysymPressed(key.code)
	}
	i.previous[cocoa.KeyEscape] = window.KeysymPressed(cocoa.KeyEscape)
	i.previous[cocoa.KeyF1] = window.KeysymPressed(cocoa.KeyF1)
}

func padState(window *cocoa.Window, bindings []keyBinding) uint16 {
	state := machine.PadReleased
	for _, binding := range bindings {
		if binding.code != 0 && window.KeysymPressed(binding.code) {
			state |= binding.button
		}
	}
	return state
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

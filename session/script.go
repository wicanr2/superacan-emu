package session

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/superacan-emu/ui"
)

// scriptEvents 是腳本認得的事件名稱。名稱是介面層的抽象動作而不是按鍵，
// 所以同一份腳本在 headless、X11 或任何前端上意義相同。
var scriptEvents = map[string]ui.Event{
	"menu":      ui.Action{Kind: ui.ActMenu},
	"confirm":   ui.Action{Kind: ui.ActConfirm},
	"cancel":    ui.Action{Kind: ui.ActCancel},
	"delete":    ui.Action{Kind: ui.ActDelete},
	"secondary": ui.Action{Kind: ui.ActSecondary},
	"tabprev":   ui.Action{Kind: ui.ActTabPrev},
	"tabnext":   ui.Action{Kind: ui.ActTabNext},
	"up":        ui.Nav{Dir: ui.DirUp},
	"down":      ui.Nav{Dir: ui.DirDown},
	"left":      ui.Nav{Dir: ui.DirLeft},
	"right":     ui.Nav{Dir: ui.DirRight},
	"home":      ui.Edge{To: ui.EdgeHome},
	"end":       ui.Edge{To: ui.EdgeEnd},
	"back":      ui.Life{Kind: ui.LifeBack},
}

// Script 把 frame 對應到該 frame 要送出的事件。
type Script map[uint64][]ui.Event

// ParseScript 讀 "frame:event,frame:event" 形式的腳本，格式與 --press 一致。
func ParseScript(spec string) (Script, error) {
	script := make(Script)
	if strings.TrimSpace(spec) == "" {
		return script, nil
	}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		frameText, name, found := strings.Cut(entry, ":")
		if !found {
			return nil, fmt.Errorf("session: script entry %q is not frame:event", entry)
		}
		frame, err := strconv.ParseUint(strings.TrimSpace(frameText), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("session: script entry %q: %w", entry, err)
		}
		name = strings.ToLower(strings.TrimSpace(name))
		// raw<code> 送出一個原始按鍵，讓腳本也能走完「指定綁定」這條路；
		// 前端識別字串由 Session.ScriptFrontend 決定，因為那才是綁定的歸屬。
		// hk:<動作> 送出一個熱鍵，hkup:<動作> 是按住型放開。熱鍵的名稱表在
		// ui.Hotkeys，這裡不另外複製一份——複製過的清單會各自過期。
		if action, isHotkey := strings.CutPrefix(name, "hkup"); isHotkey {
			if !knownHotkey(action) {
				return nil, fmt.Errorf("session: script event %q: hkup 之後要接熱鍵動作名稱", name)
			}
			script[frame] = append(script[frame], ui.HotkeyEvent{Action: action, Released: true})
			continue
		}
		if action, isHotkey := strings.CutPrefix(name, "hk"); isHotkey {
			if !knownHotkey(action) {
				return nil, fmt.Errorf("session: script event %q: hk 之後要接熱鍵動作名稱", name)
			}
			script[frame] = append(script[frame], ui.HotkeyEvent{Action: action})
			continue
		}
		if code, isRaw := strings.CutPrefix(name, "raw"); isRaw {
			value, err := strconv.ParseUint(strings.TrimPrefix(code, "0x"), 16, 32)
			if err != nil {
				return nil, fmt.Errorf("session: script event %q: raw 之後要接十六進位鍵碼", name)
			}
			script[frame] = append(script[frame], ui.RawKey{Code: uint32(value)})
			continue
		}
		event, ok := scriptEvents[name]
		if !ok {
			return nil, fmt.Errorf("session: script event %q is not one of %s（或 raw<十六進位鍵碼>）", name, ScriptEventNames())
		}
		script[frame] = append(script[frame], event)
	}
	return script, nil
}

// knownHotkey 回報這是不是 ui.Hotkeys 裡的動作。
func knownHotkey(action string) bool {
	for _, name := range ui.Hotkeys {
		if name == action {
			return true
		}
	}
	return false
}

// ScriptEventNames 列出所有事件名稱，供說明文字與錯誤訊息使用。
func ScriptEventNames() string {
	names := make([]string, 0, len(scriptEvents))
	for name := range scriptEvents {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ") + "，或 hk<動作>／hkup<動作>／raw<十六進位鍵碼>"
}

// Play 送出這個 frame 的事件。腳本裡的原始按鍵在送出前補上前端識別字串：
// 綁定要記得是誰寫的，腳本本身不是一個前端。
func (s *Session) Play(script Script, frame uint64) {
	for _, event := range script[frame] {
		if raw, ok := event.(ui.RawKey); ok && raw.Frontend == "" {
			raw.Frontend = s.ScriptFrontend
			if raw.Frontend == "" {
				raw.Frontend = "script"
			}
			event = raw
		}
		s.Handle(event)
	}
}

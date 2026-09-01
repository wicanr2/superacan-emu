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
		event, ok := scriptEvents[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, fmt.Errorf("session: script event %q is not one of %s", name, ScriptEventNames())
		}
		script[frame] = append(script[frame], event)
	}
	return script, nil
}

// ScriptEventNames 列出所有事件名稱，供說明文字與錯誤訊息使用。
func ScriptEventNames() string {
	names := make([]string, 0, len(scriptEvents))
	for name := range scriptEvents {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Play 送出這個 frame 的事件。
func (s *Session) Play(script Script, frame uint64) {
	for _, event := range script[frame] {
		s.Handle(event)
	}
}

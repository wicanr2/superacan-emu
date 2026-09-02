package session

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/superacan-emu/machine"
)

// DefaultPressFrames 是沒有指定長度時按住的幀數。十幀約六分之一秒，
// 足夠讓遊戲的輸入掃描看到，又不會長到變成連續操作。
const DefaultPressFrames = 10

// PressEvent 是一次按鍵注入：在 Frame 按下 Bits，Frames 幀之後放開。
type PressEvent struct {
	Frame  uint64
	Frames uint64
	Bits   uint16
}

var buttonBits = map[string]uint16{
	"A": machine.ButtonA, "B": machine.ButtonB,
	"START": machine.ButtonStart, "SELECT": machine.ButtonSelect,
	"UP": machine.ButtonUp, "DOWN": machine.ButtonDown,
	"LEFT": machine.ButtonLeft, "RIGHT": machine.ButtonRight,
	"X": machine.ButtonX, "Y": machine.ButtonY,
	"L": machine.ButtonL, "R": machine.ButtonR,
}

// ParsePresses 讀 "frame:BUTTON+BUTTON" 形式的注入時間表，可加 "*幀數" 指定按住
// 多久（例如 "600:right*45"）。沒有指定就是 DefaultPressFrames。
//
// 這份解析由 headless 與桌面前端共用：同一份時間表在兩邊要有同樣的意思，
// 否則「在 headless 驗過」對前端就不成立。
func ParsePresses(spec string) ([]PressEvent, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	var events []PressEvent
	for _, entry := range strings.Split(spec, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("session: 按鍵注入 %q 不是 frame:BUTTON+BUTTON 形式", entry)
		}
		frame, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("session: 按鍵注入的 frame %q：%w", parts[0], err)
		}
		buttons, hold := parts[1], uint64(DefaultPressFrames)
		if name, length, found := strings.Cut(parts[1], "*"); found {
			buttons = name
			hold, err = strconv.ParseUint(strings.TrimSpace(length), 10, 64)
			if err != nil || hold == 0 {
				return nil, fmt.Errorf("session: 按鍵注入 %q 的長度要是正整數幀數", entry)
			}
		}
		var bits uint16
		for _, name := range strings.Split(buttons, "+") {
			bit, ok := buttonBits[strings.ToUpper(strings.TrimSpace(name))]
			if !ok {
				return nil, fmt.Errorf("session: 沒有這個按鍵 %q", name)
			}
			bits |= bit
		}
		events = append(events, PressEvent{Frame: frame, Frames: hold, Bits: bits})
	}
	return events, nil
}

// ApplyPresses 依時間表更新 active-low 的手把狀態。
func ApplyPresses(frame uint64, state uint16, events []PressEvent) uint16 {
	for _, event := range events {
		if frame == event.Frame {
			state &^= event.Bits
		}
		if frame == event.Frame+event.Frames {
			state |= event.Bits
		}
	}
	return state
}

package session

import (
	"testing"

	"github.com/wicanr2/superacan-emu/machine"
)

func TestParseAndApplyPresses(t *testing.T) {
	events, err := ParsePresses("12:start+a,30:LEFT")
	if err != nil {
		t.Fatalf("ParsePresses：%v", err)
	}
	state := ApplyPresses(12, machine.PadReleased, events)
	if state&machine.ButtonStart != 0 || state&machine.ButtonA != 0 {
		t.Fatalf("frame 12 應該按著 Start 與 A，得到 %#x", state)
	}
	state = ApplyPresses(22, state, events)
	if state != machine.PadReleased {
		t.Fatalf("十幀之後應該全部放開，得到 %#x", state)
	}
}

// 按住長度可以指定：展示影片需要「按住右邊走一段」這種輸入，
// 而預設的十幀在遊戲裡只是輕點一下。
func TestParsePressesAcceptsAHoldLength(t *testing.T) {
	events, err := ParsePresses("600:right*45")
	if err != nil {
		t.Fatalf("ParsePresses：%v", err)
	}
	if events[0].Frames != 45 {
		t.Fatalf("按住 %d 幀，預期 45", events[0].Frames)
	}
	state := ApplyPresses(600, machine.PadReleased, events)
	if state&machine.ButtonRight != 0 {
		t.Fatal("frame 600 應該按著右")
	}
	if state = ApplyPresses(610, state, events); state&machine.ButtonRight != 0 {
		t.Fatal("指定了 45 幀，第十幀不該放開")
	}
	if state = ApplyPresses(645, state, events); state != machine.PadReleased {
		t.Fatalf("第 45 幀應該放開，得到 %#x", state)
	}
}

func TestParsePressesRejectsBadInput(t *testing.T) {
	for _, spec := range []string{"1:TURBO", "abc:a", "1:", "1:a*0", "1:a*x"} {
		if _, err := ParsePresses(spec); err == nil {
			t.Errorf("%q 應該被拒絕", spec)
		}
	}
}

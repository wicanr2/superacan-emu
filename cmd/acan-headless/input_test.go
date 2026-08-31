package main

import (
	"testing"

	"github.com/wicanr2/superacan-emu/machine"
)

func TestParseAndApplyPresses(t *testing.T) {
	events, err := parsePresses("12:start+a,30:LEFT")
	if err != nil {
		t.Fatal(err)
	}
	state := applyPresses(12, machine.PadReleased, events)
	if state&machine.ButtonStart != 0 || state&machine.ButtonA != 0 || state&machine.ButtonB == 0 {
		t.Fatalf("pressed state=$%04X", state)
	}
	state = applyPresses(22, state, events)
	if state != machine.PadReleased {
		t.Fatalf("released state=$%04X", state)
	}
	if _, err := parsePresses("1:TURBO"); err == nil {
		t.Fatal("unknown button accepted")
	}
}

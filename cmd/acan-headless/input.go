package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/superacan-emu/machine"
)

type pressEvent struct {
	frame uint64
	bits  uint16
}

var buttonBits = map[string]uint16{
	"A": machine.ButtonA, "B": machine.ButtonB,
	"START": machine.ButtonStart, "SELECT": machine.ButtonSelect,
	"UP": machine.ButtonUp, "DOWN": machine.ButtonDown,
	"LEFT": machine.ButtonLeft, "RIGHT": machine.ButtonRight,
	"X": machine.ButtonX, "Y": machine.ButtonY,
	"L": machine.ButtonL, "R": machine.ButtonR,
}

func parsePresses(spec string) ([]pressEvent, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	var events []pressEvent
	for _, entry := range strings.Split(spec, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid press %q, want frame:BUTTON+BUTTON", entry)
		}
		frame, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid press frame %q: %w", parts[0], err)
		}
		var bits uint16
		for _, name := range strings.Split(parts[1], "+") {
			bit, ok := buttonBits[strings.ToUpper(strings.TrimSpace(name))]
			if !ok {
				return nil, fmt.Errorf("unknown controller button %q", name)
			}
			bits |= bit
		}
		events = append(events, pressEvent{frame: frame, bits: bits})
	}
	return events, nil
}

func applyPresses(frame uint64, state uint16, events []pressEvent) uint16 {
	for _, event := range events {
		if frame == event.frame {
			state &^= event.bits
		}
		if frame == event.frame+10 {
			state |= event.bits
		}
	}
	return state
}

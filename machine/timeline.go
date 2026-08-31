package machine

import "github.com/wicanr2/superacan-emu/cpu/m68k"

// Timeline is the first shared scheduler consumer. Other chips and DMA will
// attach work to these phase boundaries rather than advancing per video frame.
type Timeline struct {
	Cycles    uint64
	Counts    [5]uint64
	Last      m68k.Phase
	OnAdvance func(cycles uint8) error
}

func (t *Timeline) Advance(phase m68k.Phase) error {
	t.Cycles += uint64(phase.Cycles)
	if int(phase.Kind) < len(t.Counts) {
		t.Counts[phase.Kind]++
	}
	t.Last = phase
	if t.OnAdvance != nil {
		return t.OnAdvance(phase.Cycles)
	}
	return nil
}

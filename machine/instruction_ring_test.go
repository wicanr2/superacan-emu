package machine

import (
	"testing"

	"github.com/wicanr2/superacan-emu/cpu/m68k"
)

func TestInstructionRingKeepsOnlyTheMostRecentRecords(t *testing.T) {
	ring := NewInstructionRing(3)
	for index := uint64(0); index < 5; index++ {
		ring.observe(index, m68k.StepResult{PCBefore: uint32(0x400 + index*2), Opcode: uint16(index)})
	}
	records := ring.Records()
	if len(records) != 3 {
		t.Fatalf("records=%d, want 3", len(records))
	}
	for offset, record := range records {
		wantIndex := uint64(2 + offset)
		if record.Index != wantIndex || record.Opcode != uint16(wantIndex) {
			t.Fatalf("record %d = %+v, want index %d", offset, record, wantIndex)
		}
	}
}

func TestNilInstructionRingIsInert(t *testing.T) {
	var ring *InstructionRing
	ring.observe(0, m68k.StepResult{})
	if records := ring.Records(); records != nil {
		t.Fatalf("records=%v, want nil", records)
	}
	if NewInstructionRing(0) != nil {
		t.Fatal("size 0 must disable the ring")
	}
}

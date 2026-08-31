package machine

import "testing"

func TestSystemResetUsesSharedTimeline(t *testing.T) {
	ipl := make([]byte, IPLSize)
	rom := make([]byte, 2)
	key := make([]byte, 16)
	// SSP=$00001000, PC=$00000400, then NOP/NOP/NOP.
	ipl[2], ipl[3] = 0x10, 0x00
	ipl[6], ipl[7] = 0x04, 0x00
	ipl[0x400], ipl[0x401] = 0x4e, 0x71
	ipl[0x402], ipl[0x403] = 0x4e, 0x71
	ipl[0x404], ipl[0x405] = 0x4e, 0x71
	system, err := NewSystem(ipl, rom, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := system.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := system.RunInstructions(1); err != nil {
		t.Fatal(err)
	}
	if state := system.M68K.State(); state.Cycles != 44 || system.Timeline.Cycles != state.Cycles {
		t.Fatalf("CPU/timeline cycles: cpu=%d timeline=%d", state.Cycles, system.Timeline.Cycles)
	}
	if system.Instructions != 1 {
		t.Fatalf("instructions=%d", system.Instructions)
	}
}

func TestHighOverlayWriteKeepsPrefetchedJump(t *testing.T) {
	ipl := make([]byte, IPLSize)
	rom := make([]byte, 0x1000)
	key := make([]byte, 16)
	putWord := func(offset int, value uint16) {
		ipl[offset], ipl[offset+1] = byte(value>>8), byte(value)
	}
	// Reset SSP=$1000, PC=$F80604.
	putWord(2, 0x1000)
	putWord(4, 0x00f8)
	putWord(6, 0x0604)
	// A1=$E9001C; A0=$400; D0=$A; MOVE.W D0,(A1); JMP (A0).
	for offset, value := range map[int]uint16{
		0x604: 0x227c, 0x606: 0x00e9, 0x608: 0x001c,
		0x60a: 0x207c, 0x60c: 0x0000, 0x60e: 0x0400,
		0x610: 0x303c, 0x612: 0x000a,
		0x614: 0x3280, 0x616: 0x4ed0,
	} {
		putWord(offset, value)
	}
	rom[0x400], rom[0x401], rom[0x402], rom[0x403] = 0x4e, 0x71, 0x70, 0x00
	system, err := NewSystem(ipl, rom, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := system.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := system.RunInstructions(5); err != nil {
		t.Fatal(err)
	}
	state := system.M68K.State()
	if state.PC != 0x400 || state.IRD != 0x4e71 || system.Bus.LowOverlayEnabled() || system.Bus.HighOverlayEnabled() {
		t.Fatalf("transfer: PC=$%06X IRD=$%04X low=%v high=%v", state.PC, state.IRD, system.Bus.LowOverlayEnabled(), system.Bus.HighOverlayEnabled())
	}
}

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

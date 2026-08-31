package m68k

import "testing"

func newExecutionCPU(words map[uint32]uint16) (*CPU, *testBus) {
	log := &eventLog{}
	bus := &testBus{log: log, words: words, bytes: map[uint32]uint8{}}
	return New(bus, log), bus
}

func TestPEAPCDisplacementPushesEffectiveAddress(t *testing.T) {
	cpu, bus := newExecutionCPU(map[uint32]uint16{0x404: 0x4e71, 0x406: 0x4e71})
	cpu.state = State{PC: 0x400, IRD: 0x487a, IRC: 0x0010, A: [8]uint32{7: 0x1000}}

	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.A[7] != 0x0ffc || bus.words[0x0ffc] != 0 || bus.words[0x0ffe] != 0x0412 {
		t.Fatalf("PEA stack: A7=$%06X words=$%04X,$%04X", state.A[7], bus.words[0x0ffc], bus.words[0x0ffe])
	}
	if result.Cycles != 16 || state.PC != 0x404 {
		t.Fatalf("PEA result: %+v", result)
	}
}

func TestJSRAddressIndexedPushesReturnAndRefillsTarget(t *testing.T) {
	cpu, bus := newExecutionCPU(map[uint32]uint16{
		0x404:  0x4e71,
		0x1006: 0x4e71, 0x1008: 0xffff,
	})
	cpu.state = State{
		D: [8]uint32{0: 2}, A: [8]uint32{3: 0x1000, 7: 0x2000},
		PC: 0x400, IRD: 0x4eb3, IRC: 0x0004,
	}

	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.PC != 0x1006 || state.IRD != 0x4e71 || state.IRC != 0xffff {
		t.Fatalf("JSR target/prefetch: %+v", state)
	}
	if state.A[7] != 0x1ffc || bus.words[0x1ffc] != 0 || bus.words[0x1ffe] != 0x0404 {
		t.Fatalf("JSR return stack: A7=$%06X words=$%04X,$%04X", state.A[7], bus.words[0x1ffc], bus.words[0x1ffe])
	}
	if result.Cycles != 22 {
		t.Fatalf("JSR cycles=%d, want 22", result.Cycles)
	}
}

func TestMOVEMLongPredecrementAndPostincrementRoundTrip(t *testing.T) {
	cpu, _ := newExecutionCPU(map[uint32]uint16{
		0x404: 0x4cdf, 0x406: 0x7fff, 0x408: 0x4e71, 0x40a: 0xffff,
	})
	cpu.state.PC, cpu.state.IRD, cpu.state.IRC = 0x400, 0x48e7, 0xfffe
	for i := range 8 {
		cpu.state.D[i] = 0xd0000000 + uint32(i)
		cpu.state.A[i] = 0xa0000000 + uint32(i)
	}
	cpu.state.A[7] = 0x2000
	wantD, wantA := cpu.state.D, cpu.state.A

	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[7] != 0x1fc4 {
		t.Fatalf("MOVEM push A7=$%06X, want $001FC4", cpu.state.A[7])
	}
	for i := range 8 {
		cpu.state.D[i] = 0
	}
	for i := 0; i < 7; i++ {
		cpu.state.A[i] = 0
	}
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.state.D != wantD || cpu.state.A != wantA {
		t.Fatalf("MOVEM round trip:\nD=%08X want=%08X\nA=%08X want=%08X", cpu.state.D, wantD, cpu.state.A, wantA)
	}
}

func TestROLByteImmediatePreservesExtend(t *testing.T) {
	cpu, _ := newExecutionCPU(map[uint32]uint16{0x404: 0xffff})
	cpu.state = State{D: [8]uint32{1: 0x12340081}, PC: 0x400, IRD: 0xe319, IRC: 0x4e71, SR: flagExtend}

	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[1] != 0x12340003 || cpu.state.SR&0x1f != flagExtend|flagCarry {
		t.Fatalf("ROL state: D1=$%08X CCR=$%02X", cpu.state.D[1], cpu.state.SR&0x1f)
	}
	if result.Cycles != 8 {
		t.Fatalf("ROL cycles=%d, want 8", result.Cycles)
	}
}

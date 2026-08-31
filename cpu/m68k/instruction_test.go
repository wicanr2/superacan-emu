package m68k

import (
	"reflect"
	"testing"
)

func newInstructionCPU(words map[uint32]uint16) (*CPU, *eventLog, *testBus) {
	log := &eventLog{}
	base := map[uint32]uint16{0: 0, 2: 0x1000, 4: 0, 6: 0x0400}
	for address, value := range words {
		base[address] = value
	}
	bus := &testBus{log: log, words: base}
	return New(bus, log), log, bus
}

func TestMOVEQDataAndFlags(t *testing.T) {
	tests := []struct {
		name       string
		opcode     uint16
		want       uint32
		wantNZ     uint16
		initialCCR uint16
	}{
		{"zero", 0x7400, 0, flagZero, flagExtend | flagCarry | flagOverflow | flagNegative},
		{"positive", 0x747f, 127, 0, flagExtend | flagCarry | flagOverflow | flagZero},
		{"negative", 0x74ff, 0xffff_ffff, flagNegative, flagExtend | flagCarry | flagOverflow | flagZero},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpu, _, _ := newInstructionCPU(map[uint32]uint16{
				0x0400: test.opcode, 0x0402: 0x4e71, 0x0404: 0x4e71,
			})
			if err := cpu.Reset(); err != nil {
				t.Fatal(err)
			}
			cpu.state.SR |= test.initialCCR
			result, err := cpu.Step()
			if err != nil {
				t.Fatal(err)
			}
			state := cpu.State()
			if state.D[2] != test.want {
				t.Fatalf("D2=$%08X, want $%08X", state.D[2], test.want)
			}
			if got := state.SR & 0x1f; got != flagExtend|test.wantNZ {
				t.Fatalf("CCR=$%02X, want $%02X", got, flagExtend|test.wantNZ)
			}
			if result.Cycles != 4 || len(result.Phases) != 1 || result.Phases[0].Kind != PhaseInstructionFetch {
				t.Fatalf("MOVEQ phase result: %+v", result)
			}
		})
	}
}

func TestBRAByteAndWordRefillPrefetch(t *testing.T) {
	tests := []struct {
		name   string
		opcode uint16
		ext    uint16
		target uint32
	}{
		{"byte forward", 0x6006, 0, 0x0408},
		{"byte backward", 0x60fa, 0, 0x03fc},
		{"word forward", 0x6000, 0x0010, 0x0412},
		{"word backward", 0x6000, 0xfffa, 0x03fc},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpu, _, _ := newInstructionCPU(map[uint32]uint16{
				0x0400: test.opcode, 0x0402: test.ext,
				test.target: 0x4e71, test.target + 2: 0x7000,
			})
			if err := cpu.Reset(); err != nil {
				t.Fatal(err)
			}
			result, err := cpu.Step()
			if err != nil {
				t.Fatal(err)
			}
			state := cpu.State()
			if state.PC != test.target || state.IRD != 0x4e71 || state.IRC != 0x7000 {
				t.Fatalf("branch state: PC=$%06X IRD=$%04X IRC=$%04X", state.PC, state.IRD, state.IRC)
			}
			if result.Cycles != 10 || len(result.Phases) != 3 {
				t.Fatalf("branch phases: %+v", result)
			}
			if result.Phases[0].Kind != PhaseInternal || result.Phases[0].Cycles != 2 ||
				result.Phases[1].Address != test.target || result.Phases[2].Address != test.target+2 ||
				result.Phases[1].FC != FCSupervisorProgram || result.Phases[2].FC != FCSupervisorProgram {
				t.Fatalf("branch phase details: %+v", result.Phases)
			}
		})
	}
}

func TestBccTakenAndNotTakenTiming(t *testing.T) {
	tests := []struct {
		name       string
		opcode     uint16
		ext        uint16
		sr         uint16
		wantPC     uint32
		wantCycles uint64
		wantPhases int
	}{
		{"BNE byte taken", 0x6606, 0, 0, 0x0408, 10, 3},
		{"BNE byte not taken", 0x6606, 0, flagZero, 0x0402, 8, 2},
		{"BEQ word taken", 0x6700, 0x0010, flagZero, 0x0412, 10, 3},
		{"BEQ word not taken", 0x6700, 0x0010, 0, 0x0404, 12, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpu, _, _ := newInstructionCPU(map[uint32]uint16{
				0x0400: test.opcode, 0x0402: test.ext,
				0x0404: 0x4e71, 0x0406: 0x7000,
				0x0408: 0x4e71, 0x040a: 0x7000,
				0x0412: 0x4e71, 0x0414: 0x7000,
			})
			if err := cpu.Reset(); err != nil {
				t.Fatal(err)
			}
			cpu.state.SR = 0x2700 | test.sr
			result, err := cpu.Step()
			if err != nil {
				t.Fatal(err)
			}
			if state := cpu.State(); state.PC != test.wantPC {
				t.Fatalf("PC=$%06X, want $%06X", state.PC, test.wantPC)
			}
			if result.Cycles != test.wantCycles || len(result.Phases) != test.wantPhases {
				t.Fatalf("result: %+v", result)
			}
			if result.Phases[0].Kind != PhaseInternal {
				t.Fatalf("first phase is not internal: %+v", result.Phases)
			}
			if test.wantPC != 0x0402 && test.wantPC != 0x0404 {
				if result.Phases[1].Address != test.wantPC || result.Phases[2].Address != test.wantPC+2 {
					t.Fatalf("taken branch phase addresses: %+v", result.Phases)
				}
			} else if test.wantPC == 0x0402 {
				if result.Phases[1].Address != 0x0404 {
					t.Fatalf("byte fall-through phase address: %+v", result.Phases)
				}
			} else if result.Phases[1].Address != 0x0404 || result.Phases[2].Address != 0x0406 {
				t.Fatalf("word fall-through phase addresses: %+v", result.Phases)
			}
		})
	}
}

func TestMOVEAImmediateLongConsumesExtensionsAndPreservesCCR(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x247c, 0x0402: 0x00fc, 0x0404: 0x1234,
		0x0406: 0x4e71, 0x0408: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR = 0x271f
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.A[2] != 0x00fc_1234 || state.SR != 0x271f {
		t.Fatalf("MOVEA state: A2=$%08X SR=$%04X", state.A[2], state.SR)
	}
	if state.PC != 0x0406 || state.IRD != 0x4e71 || state.IRC != 0x7000 {
		t.Fatalf("MOVEA queue: PC=$%06X IRD=$%04X IRC=$%04X", state.PC, state.IRD, state.IRC)
	}
	if result.Cycles != 12 || len(result.Phases) != 3 {
		t.Fatalf("MOVEA phases: %+v", result)
	}
	for i, address := range []uint32{0x0404, 0x0406, 0x0408} {
		if phase := result.Phases[i]; phase.Kind != PhaseInstructionFetch || phase.Address != address {
			t.Fatalf("MOVEA phase %d: %+v", i, phase)
		}
	}
}

func TestJSRAbsoluteWordPushesReturnAddressAndRefillsQueue(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4eb8, 0x0402: 0x040a,
		0x040a: 0x207c, 0x040c: 0x00eb,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.A[7] != 0x0ffc || state.PC != 0x040a || state.IRD != 0x207c || state.IRC != 0x00eb {
		t.Fatalf("JSR state: A7=$%08X PC=$%06X IRD=$%04X IRC=$%04X", state.A[7], state.PC, state.IRD, state.IRC)
	}
	wantWrites := []wordWrite{{address: 0x0ffc, value: 0}, {address: 0x0ffe, value: 0x0404}}
	if !reflect.DeepEqual(bus.writes, wantWrites) {
		t.Fatalf("JSR stack writes: got %+v want %+v", bus.writes, wantWrites)
	}
	if result.Cycles != 18 || len(result.Phases) != 5 {
		t.Fatalf("JSR phases: %+v", result)
	}
	wantKinds := []PhaseKind{PhaseInternal, PhaseDataWrite, PhaseDataWrite, PhaseInstructionFetch, PhaseInstructionFetch}
	for i, kind := range wantKinds {
		if result.Phases[i].Kind != kind {
			t.Fatalf("JSR phase %d: %+v", i, result.Phases[i])
		}
	}
}

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

func TestMOVEImmediateToSRIsPrivilegedAndTimed(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x46fc, 0x0402: 0x2000, 0x0404: 0x4e71, 0x0406: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.SR != 0x2000 || result.Cycles != 16 || len(result.Phases) != 3 || result.Phases[1].Kind != PhaseInternal || result.Phases[1].Cycles != 8 {
		t.Fatalf("state=%+v result=%+v", cpu.state, result)
	}
	cpu.state.SR = 0
	cpu.state.PC, cpu.state.IRD, cpu.state.IRC = 0x0400, 0x46fc, 0x2000
	if _, err := cpu.Step(); err == nil {
		t.Fatal("user-mode MOVE to SR did not fail closed")
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

func TestMOVEWordAbsoluteLongToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x3439, 0x0402: 0x00e9, 0x0404: 0x0b3c,
		0x0406: 0x4e71, 0x0408: 0x7000,
		0xe90b3c: 0x8001,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[2] = 0x1234_5678
	cpu.state.SR |= flagExtend | flagZero | flagCarry | flagOverflow
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.D[2] != 0x1234_8001 {
		t.Fatalf("D2=$%08X, want $12348001", state.D[2])
	}
	if got := state.SR & 0x1f; got != flagExtend|flagNegative {
		t.Fatalf("CCR=$%02X, want X|N", got)
	}
	if result.Cycles != 16 || len(result.Phases) != 4 ||
		result.Phases[2].Kind != PhaseDataRead || result.Phases[2].Address != 0xe90b3c {
		t.Fatalf("MOVE.W read phases: %+v", result)
	}
}

func TestMOVEWordDataToAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x33c3, 0x0402: 0x00e9, 0x0404: 0x0b3c,
		0x0406: 0x4e71, 0x0408: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[3] = 0xabcd_1357
	cpu.state.SR = 0x271f
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); state.SR != 0x271f {
		t.Fatalf("MOVE.W changed SR: $%04X", state.SR)
	}
	want := []wordWrite{{address: 0xe90b3c, value: 0x1357}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("MOVE.W writes: got %+v want %+v", bus.writes, want)
	}
	if result.Cycles != 16 || len(result.Phases) != 4 || result.Phases[2].Kind != PhaseDataWrite {
		t.Fatalf("MOVE.W write phases: %+v", result)
	}
}

func TestMOVELongAddressToAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x23ca, 0x0402: 0x00fc, 0x0404: 0x0200,
		0x0406: 0x4e71, 0x0408: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2] = 0x9234_8001
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if len(bus.writes) != 2 || bus.writes[0] != (wordWrite{address: 0xfc0200, value: 0x9234}) || bus.writes[1] != (wordWrite{address: 0xfc0202, value: 0x8001}) {
		t.Fatalf("writes=%+v", bus.writes)
	}
	if result.Cycles != 20 || cpu.state.SR&flagNegative == 0 {
		t.Fatalf("result=%+v SR=$%04X", result, cpu.state.SR)
	}
}

func TestMOVELongImmediateToAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x23fc, 0x0402: 0x89ab, 0x0404: 0xcdef,
		0x0406: 0x00fc, 0x0408: 0x0200, 0x040a: 0x4e71, 0x040c: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if len(bus.writes) != 2 || bus.writes[0] != (wordWrite{address: 0xfc0200, value: 0x89ab}) || bus.writes[1] != (wordWrite{address: 0xfc0202, value: 0xcdef}) {
		t.Fatalf("writes=%+v", bus.writes)
	}
	if result.Cycles != 28 || cpu.state.SR&flagNegative == 0 {
		t.Fatalf("result=%+v SR=$%04X", result, cpu.state.SR)
	}
}

func TestADDALongImmediateDoesNotChangeFlags(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xd5fc, 0x0402: 0x0001, 0x0404: 0xfffe,
		0x0406: 0x4e71, 0x0408: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2], cpu.state.SR = 3, 0x271f
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[2] != 0x0002_0001 || cpu.state.SR != 0x271f || result.Cycles != 16 {
		t.Fatalf("state=%+v result=%+v", cpu.state, result)
	}
}

func TestMOVEWordPostincrementToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x341a, 0x0402: 0x4e71, 0x0404: 0x7000,
		0x0800: 0x8001,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2] = 0x0800
	cpu.state.D[2] = 0x1234_5678
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.D[2] != 0x1234_8001 || state.A[2] != 0x0802 {
		t.Fatalf("D2=$%08X A2=$%08X", state.D[2], state.A[2])
	}
	if got := state.SR & 0x1f; got != flagExtend|flagNegative {
		t.Fatalf("CCR=$%02X", got)
	}
	if result.Cycles != 8 || len(result.Phases) != 2 || result.Phases[0].Kind != PhaseDataRead {
		t.Fatalf("phases=%+v cycles=%d", result.Phases, result.Cycles)
	}
}

func TestANDIWordDataPreservesUpperWordAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0245, 0x0402: 0x000f, 0x0404: 0x4e71, 0x0406: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5] = 0xbeef_fff0
	cpu.state.SR |= flagExtend | flagNegative | flagCarry | flagOverflow
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.D[5] != 0xbeef_0000 || state.SR&0x1f != flagExtend|flagZero {
		t.Fatalf("ANDI state: D5=$%08X CCR=$%02X", state.D[5], state.SR&0x1f)
	}
	if result.Cycles != 8 || len(result.Phases) != 2 {
		t.Fatalf("ANDI phases: %+v", result)
	}
}

func TestSyntheticIPLReachesFirstPollBranch(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4e71,
		0x0402: 0x4eb8, 0x0404: 0x040a,
		0x0406: 0x4e97, 0x0408: 0x4e75,
		0x040a: 0x207c, 0x040c: 0x00eb, 0x040e: 0x0d03,
		0x0410: 0x227c, 0x0412: 0x00eb, 0x0414: 0x0d01,
		0x0416: 0x247c, 0x0418: 0x00fc, 0x041a: 0x0000,
		0x041c: 0x3039, 0x041e: 0x00e9, 0x0420: 0x0b3c,
		0x0422: 0x0240, 0x0424: 0x0001,
		0x0426: 0x6700, 0x0428: 0x0008,
		0x0430: 0x4e71, 0x0432: 0x4e71,
		0xe90b3c: 0,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 8; step++ {
		if _, err := cpu.Step(); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
	state := cpu.State()
	if state.PC != 0x0430 || state.IRD != 0x4e71 {
		t.Fatalf("poll branch queue: PC=$%06X IRD=$%04X", state.PC, state.IRD)
	}
	if state.A[0] != 0x00eb0d03 || state.A[1] != 0x00eb0d01 || state.A[2] != 0x00fc0000 {
		t.Fatalf("IPL base registers: A0=$%08X A1=$%08X A2=$%08X", state.A[0], state.A[1], state.A[2])
	}
	if state.Cycles != 132 {
		t.Fatalf("cycles=%d, want 132 including reset", state.Cycles)
	}
}

func TestCMPIWordFlags(t *testing.T) {
	tests := []struct {
		name string
		dst  uint16
		src  uint16
		want uint16
	}{
		{"equal", 0x0040, 0x0040, flagZero},
		{"positive", 0x005f, 0x0040, 0},
		{"borrow negative", 0x003f, 0x0040, flagNegative | flagCarry},
		{"signed overflow", 0x8000, 0x0001, flagOverflow},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpu, _, _ := newInstructionCPU(map[uint32]uint16{
				0x0400: 0x0c43, 0x0402: test.src, 0x0404: 0x4e71, 0x0406: 0x7000,
			})
			if err := cpu.Reset(); err != nil {
				t.Fatal(err)
			}
			cpu.state.D[3] = 0xabcd_0000 | uint32(test.dst)
			cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
			result, err := cpu.Step()
			if err != nil {
				t.Fatal(err)
			}
			if state := cpu.State(); state.D[3] != 0xabcd_0000|uint32(test.dst) || state.SR&0x1f != flagExtend|test.want {
				t.Fatalf("CMPI state: D3=$%08X CCR=$%02X", state.D[3], state.SR&0x1f)
			}
			if result.Cycles != 8 {
				t.Fatalf("CMPI cycles=%d", result.Cycles)
			}
		})
	}
}

func TestCMPIWordAbsoluteLongReadsOperandAndPreservesExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0c79, 0x0402: 0x0040, 0x0404: 0x0001, 0x0406: 0x2000,
		0x0408: 0x4e71, 0x040a: 0x4e71, 0x012000: 0x003f,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 20 || state.PC != 0x0408 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
}

func TestORIWordAbsoluteLongReadModifyWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0079, 0x0402: 0x00f0, 0x0404: 0x0001, 0x0406: 0x2000,
		0x0408: 0x4e71, 0x040a: 0x4e71, 0x012000: 0x0f01,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 28 || state.PC != 0x0408 || state.SR&0x1f != flagExtend {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
	want := []wordWrite{{address: 0x012000, value: 0x0ff1}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestMOVEByteDataToAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x13c5, 0x0402: 0x0001, 0x0404: 0x2001,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5] = 0x1234_5680
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 16 || state.PC != 0x0406 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
	want := []byteWrite{{address: 0x012001, value: 0x80}}
	if !reflect.DeepEqual(bus.byteWrites, want) {
		t.Fatalf("writes=%+v, want %+v", bus.byteWrites, want)
	}
}

func TestADDALongDataPreservesFlags(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xd9c2, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[4] = 0xffff_fffe
	cpu.state.D[2] = 5
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 8 || state.A[4] != 3 || state.SR&0x1f != 0x1f {
		t.Fatalf("cycles=%d A4=$%06X SR=$%04X", result.Cycles, state.A[4], state.SR)
	}
}

func TestTSTByteAddressIndirect(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4a13, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x012001
	bus.bytes = map[uint32]uint8{0x012001: 0x80}
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d SR=$%04X", result.Cycles, state.SR)
	}
}

func TestSUBALongAddressPreservesFlags(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x95ce, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2], cpu.state.A[6] = 2, 5
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.A[2] != 0xffff_fffd || state.SR&0x1f != 0x1f {
		t.Fatalf("cycles=%d A2=$%06X SR=$%04X", result.Cycles, state.A[2], state.SR)
	}
}

func TestTSTWordDataPreservesOperandAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4a41, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[1] = 0x1234_8000
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 4 || state.D[1] != 0x1234_8000 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D1=$%08X SR=$%04X", result.Cycles, state.D[1], state.SR)
	}
}

func TestTSTByteAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4a39, 0x0402: 0x0001, 0x0404: 0x2001,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	bus.bytes = map[uint32]uint8{0x012001: 0}
	cpu.state.SR |= flagExtend | flagNegative | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.PC != 0x0406 || state.SR&0x1f != flagExtend|flagZero {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
}

func TestEORWordDataToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xbf46, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[6], cpu.state.D[7] = 0xaaaa_0f0f, 0xbbbb_ffff
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 4 || state.D[6] != 0xaaaa_f0f0 || state.D[7] != 0xbbbb_ffff || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D6=$%08X D7=$%08X SR=$%04X", result.Cycles, state.D[6], state.D[7], state.SR)
	}
}

func TestNOTWordDataPreservesUpperWordAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4647, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[7] = 0x1234_ffff
	cpu.state.SR |= flagExtend | flagNegative | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 4 || state.D[7] != 0x1234_0000 || state.SR&0x1f != flagExtend|flagZero {
		t.Fatalf("cycles=%d D7=$%08X SR=$%04X", result.Cycles, state.D[7], state.SR)
	}
}

func TestANDWordDataToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xcc47, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[6], cpu.state.D[7] = 0xaaaa_f0f0, 0xbbbb_0f0f
	cpu.state.SR |= flagExtend | flagNegative | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 4 || state.D[6] != 0xaaaa_0000 || state.D[7] != 0xbbbb_0f0f || state.SR&0x1f != flagExtend|flagZero {
		t.Fatalf("cycles=%d D6=$%08X D7=$%08X SR=$%04X", result.Cycles, state.D[6], state.D[7], state.SR)
	}
}

func TestMOVEByteImmediateToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x163c, 0x0402: 0x0080, 0x0404: 0x4e71, 0x0406: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[3] = 0x1234_5678
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.D[3] != 0x1234_5680 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D3=$%08X SR=$%04X", result.Cycles, state.D[3], state.SR)
	}
}

func TestANDILongData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0285, 0x0402: 0x0f0f, 0x0404: 0xffff,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5] = 0xf0ff_00ff
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.D[5] != 0x000f_00ff || state.SR&0x1f != flagExtend {
		t.Fatalf("cycles=%d D5=$%08X SR=$%04X", result.Cycles, state.D[5], state.SR)
	}
}

func TestMOVELongImmediateToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x263c, 0x0402: 0x8000, 0x0404: 0x0001,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 12 || state.D[3] != 0x8000_0001 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D3=$%08X SR=$%04X", result.Cycles, state.D[3], state.SR)
	}
}

func TestORLongDataToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x8280, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[0], cpu.state.D[1] = 0x8000_0000, 1
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.D[0] != 0x8000_0000 || state.D[1] != 0x8000_0001 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D0=$%08X D1=$%08X SR=$%04X", result.Cycles, state.D[0], state.D[1], state.SR)
	}
}

func TestMULUWordImmediateDataDependentTiming(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xc6fc, 0x0402: 0x0003, 0x0404: 0x4e71, 0x0406: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[3] = 0xabcd_0004
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	// Immediate EA adds one read to the 38+2n register-source timing.
	if state := cpu.State(); result.Cycles != 46 || state.D[3] != 12 || state.SR&0x1f != flagExtend {
		t.Fatalf("cycles=%d D3=$%08X SR=$%04X", result.Cycles, state.D[3], state.SR)
	}
}

func TestMOVEWordPostincrementToAddressIndirect(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x369d, 0x0402: 0x4e71, 0x0404: 0x4e71,
		0x012000: 0x8001,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[5], cpu.state.A[3] = 0x012000, 0x013000
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 12 || state.A[5] != 0x012002 || state.A[3] != 0x013000 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d A5=$%06X A3=$%06X SR=$%04X", result.Cycles, state.A[5], state.A[3], state.SR)
	}
	want := []wordWrite{{address: 0x013000, value: 0x8001}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestMOVEWordPostincrementToDisplacement(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x375d, 0x0402: 0xfffe, 0x0404: 0x4e71, 0x0406: 0x4e71,
		0x012000: 0x7fff,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[5], cpu.state.A[3] = 0x012000, 0x013002
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 16 || state.A[5] != 0x012002 || state.A[3] != 0x013002 || state.SR&0x1f != flagExtend {
		t.Fatalf("cycles=%d A5=$%06X A3=$%06X SR=$%04X", result.Cycles, state.A[5], state.A[3], state.SR)
	}
	want := []wordWrite{{address: 0x013000, value: 0x7fff}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestMOVEWordImmediateToAddressIndirect(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x36bc, 0x0402: 0x8000, 0x0404: 0x4e71, 0x0406: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x013000
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 12 || state.A[3] != 0x013000 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d A3=$%06X SR=$%04X", result.Cycles, state.A[3], state.SR)
	}
	want := []wordWrite{{address: 0x013000, value: 0x8000}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestMOVEWordImmediateToDisplacement(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x377c, 0x0402: 0x8000, 0x0404: 0xfffe,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x013002
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.A[3] != 0x013002 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d A3=$%06X SR=$%04X", result.Cycles, state.A[3], state.SR)
	}
	want := []wordWrite{{address: 0x013000, value: 0x8000}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestMOVELongImmediateToDisplacement(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x277c, 0x0402: 0x8000, 0x0404: 0x0001, 0x0406: 0xfffc,
		0x0408: 0x4e71, 0x040a: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x013004
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 24 || state.A[3] != 0x013004 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d A3=$%06X SR=$%04X", result.Cycles, state.A[3], state.SR)
	}
	want := []wordWrite{{address: 0x013000, value: 0x8000}, {address: 0x013002, value: 0x0001}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestSUBQByteAbsoluteLongReadModifyWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x5339, 0x0402: 0x0001, 0x0404: 0x2001,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	bus.bytes = map[uint32]uint8{0x012001: 0}
	cpu.state.SR |= flagZero
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 20 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d SR=$%04X", result.Cycles, state.SR)
	}
	want := []byteWrite{{address: 0x012001, value: 0xff}}
	if !reflect.DeepEqual(bus.byteWrites, want) {
		t.Fatalf("writes=%+v, want %+v", bus.byteWrites, want)
	}
}

func TestMOVEALongAbsoluteLongPreservesFlags(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x2679, 0x0402: 0x0001, 0x0404: 0x2000,
		0x0406: 0x4e71, 0x0408: 0x4e71,
		0x012000: 0x89ab, 0x012002: 0xcdef,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 20 || state.A[3] != 0x89ab_cdef || state.SR&0x1f != 0x1f {
		t.Fatalf("cycles=%d A3=$%08X SR=$%04X", result.Cycles, state.A[3], state.SR)
	}
}

func TestMOVEByteDisplacementToAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x13eb, 0x0402: 0xfffe, 0x0404: 0x0001, 0x0406: 0x3001,
		0x0408: 0x4e71, 0x040a: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x012002
	bus.bytes = map[uint32]uint8{0x012000: 0x80}
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 24 || state.A[3] != 0x012002 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d A3=$%06X SR=$%04X", result.Cycles, state.A[3], state.SR)
	}
	want := []byteWrite{{address: 0x013001, value: 0x80}}
	if !reflect.DeepEqual(bus.byteWrites, want) {
		t.Fatalf("writes=%+v, want %+v", bus.byteWrites, want)
	}
}

func TestCMPILongAddressIndirectPreservesOperandAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0c93, 0x0402: 0x0000, 0x0404: 0x0002,
		0x0406: 0x4e71, 0x0408: 0x4e71,
		0x012000: 0x0000, 0x012002: 0x0001,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x012000
	cpu.state.SR |= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 20 || state.A[3] != 0x012000 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d A3=$%06X SR=$%04X", result.Cycles, state.A[3], state.SR)
	}
}

func TestMOVEWordDisplacementToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x322b, 0x0402: 0xfffe, 0x0404: 0x4e71, 0x0406: 0x4e71,
		0x012000: 0x8001,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x012002
	cpu.state.D[1] = 0xabcd_1234
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 12 || state.D[1] != 0xabcd_8001 || state.A[3] != 0x012002 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D1=$%08X A3=$%06X SR=$%04X", result.Cycles, state.D[1], state.A[3], state.SR)
	}
}

func TestMOVEByteDisplacementToData(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x142b, 0x0402: 0xfffe, 0x0404: 0x4e71, 0x0406: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x012002
	cpu.state.D[2] = 0xabcd_1234
	bus.bytes = map[uint32]uint8{0x012000: 0x80}
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 12 || state.D[2] != 0xabcd_1280 || state.A[3] != 0x012002 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D2=$%08X A3=$%06X SR=$%04X", result.Cycles, state.D[2], state.A[3], state.SR)
	}
}

func TestADDWordDataToAbsoluteLongReadModifyWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xd579, 0x0402: 0x0001, 0x0404: 0x2000,
		0x0406: 0x4e71, 0x0408: 0x4e71, 0x012000: 0x7fff,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[2] = 1
	cpu.state.SR |= flagZero | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 20 || state.SR&0x1f != flagNegative|flagOverflow {
		t.Fatalf("cycles=%d SR=$%04X", result.Cycles, state.SR)
	}
	want := []wordWrite{{address: 0x012000, value: 0x8000}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestADDQLongAbsoluteLongReadModifyWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x5cb9, 0x0402: 0x0001, 0x0404: 0x2000,
		0x0406: 0x4e71, 0x0408: 0x4e71,
		0x012000: 0x7fff, 0x012002: 0xfffa,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagZero | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 28 || state.SR&0x1f != flagNegative|flagOverflow {
		t.Fatalf("cycles=%d SR=$%04X", result.Cycles, state.SR)
	}
	want := []wordWrite{{address: 0x012000, value: 0x8000}, {address: 0x012002, value: 0x0000}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestBTSTImmediateDataUsesModulo32AndOnlyChangesZero(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0805, 0x0402: 0x0021, 0x0404: 0x4e71, 0x0406: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5] = 2
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 10 || state.D[5] != 2 || state.SR&0x1f != flagExtend|flagNegative|flagOverflow|flagCarry {
		t.Fatalf("cycles=%d D5=$%08X SR=$%04X", result.Cycles, state.D[5], state.SR)
	}
}

func TestMOVEByteAbsoluteLongToData(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x1e39, 0x0402: 0x0001, 0x0404: 0x2001,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	bus.bytes = map[uint32]uint8{0x012001: 0x80}
	cpu.state.D[7] = 0x1234_5678
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.D[7] != 0x1234_5680 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D7=$%08X SR=$%04X", result.Cycles, state.D[7], state.SR)
	}
}

func TestCMPByteAbsoluteLongToDataPreservesOperandAndExtend(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xbe39, 0x0402: 0x0001, 0x0404: 0x2001,
		0x0406: 0x4e71, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	bus.bytes = map[uint32]uint8{0x012001: 2}
	cpu.state.D[7] = 1
	cpu.state.SR |= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.D[7] != 1 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d D7=$%08X SR=$%04X", result.Cycles, state.D[7], state.SR)
	}
}

func TestCMPWordAbsoluteLongToDataPreservesOperandAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xb279, 0x0402: 0x0001, 0x0404: 0x2000,
		0x0406: 0x4e71, 0x0408: 0x4e71, 0x012000: 2,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[1] = 1
	cpu.state.SR |= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.D[1] != 1 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d D1=$%08X SR=$%04X", result.Cycles, state.D[1], state.SR)
	}
}

func TestCMPWordDataToDataPreservesOperandsAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xbc45, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5], cpu.state.D[6] = 2, 1
	cpu.state.SR |= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 4 || state.D[5] != 2 || state.D[6] != 1 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d D5=$%08X D6=$%08X SR=$%04X", result.Cycles, state.D[5], state.D[6], state.SR)
	}
}

func TestADDWordDataToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xdc43, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[3], cpu.state.D[6] = 1, 0xabcd_7fff
	cpu.state.SR |= flagZero | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 4 || state.D[3] != 1 || state.D[6] != 0xabcd_8000 || state.SR&0x1f != flagNegative|flagOverflow {
		t.Fatalf("cycles=%d D3=$%08X D6=$%08X SR=$%04X", result.Cycles, state.D[3], state.D[6], state.SR)
	}
}

func TestCMPLongDataToDataPreservesOperandsAndExtend(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xbc85, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5], cpu.state.D[6] = 2, 1
	cpu.state.SR |= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.Cycles != 6 || state.D[5] != 2 || state.D[6] != 1 || state.SR&0x1f != flagExtend|flagNegative|flagCarry {
		t.Fatalf("cycles=%d D5=$%08X D6=$%08X SR=$%04X", result.Cycles, state.D[5], state.D[6], state.SR)
	}
}

func TestMOVEALongDisplacementPreservesFlags(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x2a68, 0x0402: 0xfffe, 0x0404: 0x4e71, 0x0406: 0x4e71,
		0x012000: 0x89ab, 0x012002: 0xcdef,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[0] = 0x012002
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 16 || state.A[5] != 0x89ab_cdef || state.A[0] != 0x012002 || state.SR&0x1f != 0x1f {
		t.Fatalf("cycles=%d A5=$%08X A0=$%06X SR=$%04X", result.Cycles, state.A[5], state.A[0], state.SR)
	}
}

func TestMOVEByteAddressIndirectToData(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x1415, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[5] = 0x012001
	cpu.state.D[2] = 0x1234_5678
	bus.bytes = map[uint32]uint8{0x012001: 0x80}
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.D[2] != 0x1234_5680 || state.A[5] != 0x012001 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D2=$%08X A5=$%06X SR=$%04X", result.Cycles, state.D[2], state.A[5], state.SR)
	}
}

func TestLSLWordRegisterCountAndFlags(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xef6d, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5], cpu.state.D[7] = 0xabcd_8001, 1
	cpu.state.SR |= flagExtend | flagZero | flagOverflow
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.D[5] != 0xabcd_0002 || state.D[7] != 1 || state.SR&0x1f != flagExtend|flagCarry {
		t.Fatalf("cycles=%d D5=$%08X D7=$%08X SR=$%04X", result.Cycles, state.D[5], state.D[7], state.SR)
	}
}

func TestLSLWordRegisterZeroCountPreservesExtendAndClearsCarry(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xef6d, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5], cpu.state.D[7] = 0x8000, 64
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 6 || state.D[5] != 0x8000 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D5=$%08X SR=$%04X", result.Cycles, state.D[5], state.SR)
	}
}

func TestEORByteDataToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xbb06, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5], cpu.state.D[6] = 0xaaaa_00ff, 0xbbbb_007f
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 4 || state.D[5] != 0xaaaa_00ff || state.D[6] != 0xbbbb_0080 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D5=$%08X D6=$%08X SR=$%04X", result.Cycles, state.D[5], state.D[6], state.SR)
	}
}

func TestLSRByteImmediate(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xe20d, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5] = 0x1234_5601
	cpu.state.SR |= flagZero | flagOverflow
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 8 || state.D[5] != 0x1234_5600 || state.SR&0x1f != flagZero|flagCarry|flagExtend {
		t.Fatalf("cycles=%d D5=$%08X SR=$%04X", result.Cycles, state.D[5], state.SR)
	}
}

func TestORByteDataToData(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x8c05, 0x0402: 0x4e71, 0x0404: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5], cpu.state.D[6] = 0xaaaa_0080, 0xbbbb_0001
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 4 || state.D[5] != 0xaaaa_0080 || state.D[6] != 0xbbbb_0081 || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d D5=$%08X D6=$%08X SR=$%04X", result.Cycles, state.D[5], state.D[6], state.SR)
	}
}

func TestADDILongAbsoluteLongReadModifyWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x06b9, 0x0402: 0x0000, 0x0404: 0x0006,
		0x0406: 0x0001, 0x0408: 0x2000, 0x040a: 0x4e71, 0x040c: 0x4e71,
		0x012000: 0x7fff, 0x012002: 0xfffa,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagZero | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 36 || state.PC != 0x040a || state.SR&0x1f != flagNegative|flagOverflow {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
	want := []wordWrite{{address: 0x012000, value: 0x8000}, {address: 0x012002, value: 0}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestMOVELongAbsoluteLongToAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x23f9, 0x0402: 0x0001, 0x0404: 0x2000,
		0x0406: 0x0001, 0x0408: 0x3000, 0x040a: 0x4e71, 0x040c: 0x4e71,
		0x012000: 0x8000, 0x012002: 0x0001,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend | flagZero | flagOverflow | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 36 || state.PC != 0x040a || state.SR&0x1f != flagExtend|flagNegative {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
	want := []wordWrite{{address: 0x013000, value: 0x8000}, {address: 0x013002, value: 1}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestSUBILongAbsoluteLongReadModifyWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x04b9, 0x0402: 0x0000, 0x0404: 0x0001,
		0x0406: 0x0001, 0x0408: 0x2000, 0x040a: 0x4e71, 0x040c: 0x4e71,
		0x012000: 0x8000, 0x012002: 0x0000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend | flagNegative | flagZero | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); result.Cycles != 36 || state.PC != 0x040a || state.SR&0x1f != flagOverflow {
		t.Fatalf("cycles=%d PC=$%06X SR=$%04X", result.Cycles, state.PC, state.SR)
	}
	want := []wordWrite{{address: 0x012000, value: 0x7fff}, {address: 0x012002, value: 0xffff}}
	if !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes=%+v, want %+v", bus.writes, want)
	}
}

func TestCMPByteAddressIndirectToData(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{
		log: log,
		words: map[uint32]uint16{
			0: 0, 2: 0x1000, 4: 0, 6: 0x0400,
			0x0400: 0xb411, 0x0402: 0x4e71, 0x0404: 0x7000,
		},
		bytes: map[uint32]uint8{0x2000: 0x80},
	}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[1] = 0x2000
	cpu.state.D[2] = 0x1234_0080
	cpu.state.SR |= flagExtend | flagNegative | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.D[2] != 0x1234_0080 || state.A[1] != 0x2000 || state.SR&0x1f != flagExtend|flagZero {
		t.Fatalf("CMP.B state: D2=$%08X A1=$%08X CCR=$%02X", state.D[2], state.A[1], state.SR&0x1f)
	}
	if result.Cycles != 8 || len(result.Phases) != 2 || result.Phases[0].Kind != PhaseDataRead {
		t.Fatalf("CMP.B phases: %+v", result)
	}
}

func TestMOVEBytePredecrementToAddressIndirect(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{
		log: log,
		words: map[uint32]uint16{
			0: 0, 2: 0x1000, 4: 0, 6: 0x0400,
			0x0400: 0x12a2, 0x0402: 0x4e71, 0x0404: 0x7000,
		},
		bytes: map[uint32]uint8{0x201f: 0xa5},
	}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[1] = 0x3000
	cpu.state.A[2] = 0x2020
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); state.A[2] != 0x201f {
		t.Fatalf("A2=$%08X", state.A[2])
	}
	if want := []byteWrite{{address: 0x3000, value: 0xa5}}; !reflect.DeepEqual(bus.byteWrites, want) {
		t.Fatalf("writes: got %+v want %+v", bus.byteWrites, want)
	}
	if result.Cycles != 14 || len(result.Phases) != 4 || result.Phases[0].Kind != PhaseInternal {
		t.Fatalf("phases: %+v", result)
	}
}

func TestMOVEBytePostincrementUsesTwoBytesForA7(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{
		log: log,
		words: map[uint32]uint16{
			0: 0, 2: 0x1000, 4: 0, 6: 0x0400,
			0x0400: 0x1ed1, 0x0402: 0x4e71, 0x0404: 0x7000,
		},
		bytes: map[uint32]uint8{0x2000: 0x7f},
	}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[1] = 0x2000
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); state.A[7] != 0x1002 {
		t.Fatalf("A7=$%08X, want $00001002", state.A[7])
	}
	if want := []byteWrite{{address: 0x1000, value: 0x7f}}; !reflect.DeepEqual(bus.byteWrites, want) {
		t.Fatalf("stack byte writes: got %+v want %+v", bus.byteWrites, want)
	}
}

func TestDBccPaths(t *testing.T) {
	tests := []struct {
		name       string
		sr         uint16
		count      uint16
		wantCount  uint16
		wantPC     uint32
		wantCycles uint64
	}{
		{"condition true", flagZero, 5, 5, 0x0404, 12},
		{"branch", 0, 5, 4, 0x03fc, 10},
		{"counter expired", 0, 0, 0xffff, 0x0404, 14},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpu, _, _ := newInstructionCPU(map[uint32]uint16{
				0x0400: 0x57c8, 0x0402: 0xfffa,
				0x0404: 0x4e71, 0x0406: 0x7000,
				0x03fc: 0x4e71, 0x03fe: 0x7000,
			})
			if err := cpu.Reset(); err != nil {
				t.Fatal(err)
			}
			cpu.state.SR = 0x2700 | test.sr
			cpu.state.D[0] = 0xbeef_0000 | uint32(test.count)
			result, err := cpu.Step()
			if err != nil {
				t.Fatal(err)
			}
			state := cpu.State()
			if uint16(state.D[0]) != test.wantCount || state.PC != test.wantPC {
				t.Fatalf("DBEQ state: D0=$%08X PC=$%06X", state.D[0], state.PC)
			}
			if result.Cycles != test.wantCycles {
				t.Fatalf("DBEQ result: %+v", result)
			}
		})
	}
}

func TestSyntheticIPLCompletesFirstUMC6650BackupLoop(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{
		log: log,
		words: map[uint32]uint16{
			0: 0, 2: 0x1000, 4: 0, 6: 0x0400,
			0x0400: 0x303c, 0x0402: 0x005f,
			0x0404: 0x3200,
			0x0406: 0x1080,
			0x0408: 0x14d1,
			0x040a: 0x33fc, 0x040c: 0xc170, 0x040e: 0x00e9, 0x0410: 0x0b3c,
			0x0412: 0x0c40, 0x0414: 0x0040,
			0x0416: 0x5fc8, 0x0418: 0xffee,
			0x041a: 0x4e71, 0x041c: 0x7000,
		},
		bytes: map[uint32]uint8{0xeb0d01: 0xa5},
	}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[0] = 0xeb0d03
	cpu.state.A[1] = 0xeb0d01
	cpu.state.A[2] = 0xfc0000
	// Two setup instructions followed by 32 loop iterations of five
	// instructions each ($5F down to and including $40).
	for step := 0; step < 162; step++ {
		if _, err := cpu.Step(); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
	state := cpu.State()
	if state.PC != 0x041a || uint16(state.D[0]) != 0x0040 || uint16(state.D[1]) != 0x005f {
		t.Fatalf("loop state: PC=$%06X D0=$%08X D1=$%08X", state.PC, state.D[0], state.D[1])
	}
	if state.A[2] != 0xfc0020 {
		t.Fatalf("A2=$%08X, want $00FC0020", state.A[2])
	}
	if len(bus.byteWrites) != 64 {
		t.Fatalf("byte write count=%d, want 64", len(bus.byteWrites))
	}
	for i := 0; i < 32; i++ {
		wantAddressWrite := byteWrite{address: 0xeb0d03, value: uint8(0x5f - i)}
		wantBackupWrite := byteWrite{address: 0xfc0000 + uint32(i), value: 0xa5}
		if bus.byteWrites[2*i] != wantAddressWrite || bus.byteWrites[2*i+1] != wantBackupWrite {
			t.Fatalf("iteration %d writes: got %+v %+v", i, bus.byteWrites[2*i], bus.byteWrites[2*i+1])
		}
	}
	if len(bus.writes) != 32 {
		t.Fatalf("word write count=%d, want 32", len(bus.writes))
	}
	for i, write := range bus.writes {
		if write != (wordWrite{address: 0xe90b3c, value: 0xc170}) {
			t.Fatalf("noise write %d: %+v", i, write)
		}
	}
	if state.Cycles != 1910 {
		t.Fatalf("cycles=%d, want 1910 including reset", state.Cycles)
	}
}

package m68k

import (
	"reflect"
	"testing"
)

func TestChecksumArithmeticAddressModes(t *testing.T) {
	tests := []struct {
		name       string
		opcode     uint16
		value      uint8
		data       uint32
		wantData   uint32
		startA2    uint32
		wantA2     uint32
		wantCycles uint64
	}{
		{"ADD.B predecrement", 0xd822, 2, 0x1234_007f, 0x1234_0081, 0x2001, 0x2000, 10},
		{"SUB.B postincrement", 0x9a1a, 2, 0x1234_0080, 0x1234_007e, 0x2000, 0x2001, 8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &eventLog{}
			bus := &testBus{log: log, words: map[uint32]uint16{
				0: 0, 2: 0x1000, 4: 0, 6: 0x0400,
				0x0400: test.opcode, 0x0402: 0x4e71, 0x0404: 0x7000,
			}, bytes: map[uint32]uint8{0x2000: test.value}}
			cpu := New(bus, log)
			if err := cpu.Reset(); err != nil {
				t.Fatal(err)
			}
			register := uint8(4)
			if test.opcode == 0x9a1a {
				register = 5
			}
			cpu.state.D[register] = test.data
			cpu.state.A[2] = test.startA2
			result, err := cpu.Step()
			if err != nil {
				t.Fatal(err)
			}
			state := cpu.State()
			if state.D[register] != test.wantData || state.A[2] != test.wantA2 {
				t.Fatalf("state: D%d=$%08X A2=$%08X", register, state.D[register], state.A[2])
			}
			if result.Cycles != test.wantCycles {
				t.Fatalf("cycles=%d", result.Cycles)
			}
		})
	}
}

func TestClearAbsoluteLongReadsBeforeWrite(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4279, 0x0402: 0x00e9, 0x0404: 0x0b3c,
		0x0406: 0x4e71, 0x0408: 0x7000, 0xe90b3c: 0xffff,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR |= flagExtend | flagNegative | flagCarry
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if want := []wordWrite{{address: 0xe90b3c, value: 0}}; !reflect.DeepEqual(bus.writes, want) {
		t.Fatalf("writes: got %+v want %+v", bus.writes, want)
	}
	if cpu.State().SR&0x1f != flagExtend|flagZero {
		t.Fatalf("CCR=$%02X", cpu.State().SR&0x1f)
	}
	if result.Cycles != 20 || len(result.Phases) != 5 || result.Phases[2].Kind != PhaseDataRead || result.Phases[3].Kind != PhaseDataWrite {
		t.Fatalf("phases: %+v", result)
	}
}

func TestCMPIWordPredecrementAndSUBQAddress(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x0c62, 0x0402: 0x4d55, 0x0404: 0x5d8a,
		0x0406: 0x4e71, 0x0408: 0x7000, 0x2000: 0x4d55,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2] = 0x2002
	first, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); state.A[2] != 0x2000 || state.SR&flagZero == 0 {
		t.Fatalf("CMPI state: A2=$%08X SR=$%04X", state.A[2], state.SR)
	}
	if first.Cycles != 14 {
		t.Fatalf("CMPI cycles=%d", first.Cycles)
	}
	beforeSR := cpu.State().SR
	second, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); state.A[2] != 0x1ffa || state.SR != beforeSR {
		t.Fatalf("SUBQ state: A2=$%08X SR=$%04X", state.A[2], state.SR)
	}
	if second.Cycles != 8 {
		t.Fatalf("SUBQ cycles=%d", second.Cycles)
	}
}

func TestMOVEByteImmediateAbsoluteLong(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x13fc, 0x0402: 0x00ff, 0x0404: 0x00eb, 0x0406: 0x0d01,
		0x0408: 0x4e71, 0x040a: 0x7000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byteWrite{{address: 0xeb0d01, value: 0xff}}; !reflect.DeepEqual(bus.byteWrites, want) {
		t.Fatalf("writes: got %+v want %+v", bus.byteWrites, want)
	}
	if result.Cycles != 20 {
		t.Fatalf("cycles=%d", result.Cycles)
	}
}

func TestCMPMWordReadsAndIncrementsBothOperands(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xb94b, 0x0402: 0x4e71, 0x0404: 0x7000,
		0x2000: 0x1234, 0x3000: 0x1234,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3], cpu.state.A[4] = 0x2000, 0x3000
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.A[3] != 0x2002 || state.A[4] != 0x3002 || state.SR&flagZero == 0 {
		t.Fatalf("state: A3=$%08X A4=$%08X SR=$%04X", state.A[3], state.A[4], state.SR)
	}
	if result.Cycles != 12 || len(result.Phases) != 3 || result.Phases[0].Kind != PhaseDataRead || result.Phases[1].Kind != PhaseDataRead {
		t.Fatalf("phases: %+v", result)
	}
}

func TestMULSWordPostincrementDataDependentTiming(t *testing.T) {
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xc5db, 0x0402: 0x4e71, 0x0404: 0x7000,
		0x2000: 0x0002,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x2000
	cpu.state.D[2] = 0xffff_fffd // low word -3
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if state.D[2] != 0xffff_fffa || state.A[3] != 0x2002 || state.SR&flagNegative == 0 {
		t.Fatalf("state: D2=$%08X A3=$%08X SR=$%04X", state.D[2], state.A[3], state.SR)
	}
	// operand 2 has two Booth transitions: 42 + 2*2 = 46 cycles for (An)+.
	if result.Cycles != 46 {
		t.Fatalf("cycles=%d", result.Cycles)
	}
}

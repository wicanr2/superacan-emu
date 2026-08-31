package m68k

import "testing"

func TestSub32Flags(t *testing.T) {
	cpu, _ := newExecutionCPU(map[uint32]uint16{})
	cpu.state.SR = flagExtend
	if result := cpu.sub32(0, 1); result != 0xffff_ffff || cpu.state.SR&0x1f != flagNegative|flagCarry|flagExtend {
		t.Fatalf("borrow result=$%08X CCR=$%02X", result, cpu.state.SR&0x1f)
	}
	cpu.state.SR = flagExtend
	if result := cpu.sub32(0x8000_0000, 1); result != 0x7fff_ffff || cpu.state.SR&0x1f != flagOverflow {
		t.Fatalf("overflow result=$%08X CCR=$%02X", result, cpu.state.SR&0x1f)
	}
}

func TestMULUAndDIVURegisterSemantics(t *testing.T) {
	cpu, _ := newExecutionCPU(map[uint32]uint16{0x404: 0x4e71})
	cpu.state = State{D: [8]uint32{1: 3, 2: 0x0001_0002}, PC: 0x400, IRC: 0x4e71}
	if err := cpu.muluWordData(1, 2); err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[2] != 6 || cpu.state.Cycles != 42 {
		t.Fatalf("MULU D2=$%08X cycles=%d", cpu.state.D[2], cpu.state.Cycles)
	}
	cpu.state.PC, cpu.state.IRC = 0x400, 0x4e71
	cpu.state.D[1], cpu.state.D[2], cpu.state.Cycles = 7, 100, 0
	if err := cpu.divuWordData(1, 2); err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[2] != 2<<16|14 || cpu.state.Cycles != 140 {
		t.Fatalf("DIVU D2=$%08X cycles=%d", cpu.state.D[2], cpu.state.Cycles)
	}
}

func TestDIVUZeroFailsBeforeMutation(t *testing.T) {
	cpu, _ := newExecutionCPU(map[uint32]uint16{})
	cpu.state.D[2] = 0x1234_5678
	if err := cpu.divuWordData(1, 2); err == nil || cpu.state.D[2] != 0x1234_5678 || cpu.state.Cycles != 0 {
		t.Fatalf("DIVU zero state=%+v err=%v", cpu.state, err)
	}
}

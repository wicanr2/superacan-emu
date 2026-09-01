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

func TestAdd32Flags(t *testing.T) {
	cpu, _ := newExecutionCPU(map[uint32]uint16{})
	cpu.state.SR = flagExtend
	if result := cpu.add32(0xffff_ffff, 1); result != 0 || cpu.state.SR&0x1f != flagZero|flagCarry|flagExtend {
		t.Fatalf("carry result=$%08X CCR=$%02X", result, cpu.state.SR&0x1f)
	}
	cpu.state.SR = flagExtend
	if result := cpu.add32(0x7fff_ffff, 1); result != 0x8000_0000 || cpu.state.SR&0x1f != flagNegative|flagOverflow {
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

func TestDIVUZeroTakesVectorFiveWithoutMutatingQuotient(t *testing.T) {
	// 除以零是 68000 的向量 5 例外，不是模擬器的錯誤：目的暫存器不得被改動。
	cpu, _ := newExecutionCPU(map[uint32]uint16{
		0x0014: 0x0000, 0x0016: 0x0900,
		0x0900: 0x4e71, 0x0902: 0x4e71,
	})
	cpu.state.SR = 0x2000
	cpu.state.A[7] = 0x2000
	cpu.state.D[2] = 0x1234_5678
	if err := cpu.divuWordData(1, 2); err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[2] != 0x1234_5678 {
		t.Fatalf("D2=$%08X，除以零不得改動目的暫存器", cpu.state.D[2])
	}
	if cpu.state.PC != 0x0900 {
		t.Fatalf("PC=$%06X, want $000900", cpu.state.PC)
	}
}

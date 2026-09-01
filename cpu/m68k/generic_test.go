package m68k

import "testing"

// 這些測試針對一般化 effective-address 執行層，全部用合成程式，不依賴商業 ROM。
// 週期數取自 M68000 Programmer's Reference Manual 的指令時間表。

func TestGenericAddWordDisplacementToData(t *testing.T) {
	// ADD.W (8,A2),D0：Boom Zoo 在第 1,695 個 frame 停住的編碼。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xd06a, 0x0402: 0x0008, 0x0404: 0x4e71,
		0x2008: 0x1111,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2] = 0x2000
	cpu.state.D[0] = 0x9999_2222
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[0] != 0x9999_3333 {
		t.Fatalf("D0=$%08X, want $99993333", cpu.state.D[0])
	}
	if result.Cycles != 12 {
		t.Fatalf("cycles=%d, want 12", result.Cycles)
	}
}

func TestGenericMoveLongPostincrementToAbsoluteLong(t *testing.T) {
	// MOVE.L (A0)+,(xxx).L：Speedy Dragon 的停止編碼。
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x23d8, 0x0402: 0x0000, 0x0404: 0x3000, 0x0406: 0x4e71,
		0x2000: 0xdead, 0x2002: 0xbeef,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[0] = 0x2000
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[0] != 0x2004 {
		t.Fatalf("A0=$%08X, want $00002004", cpu.state.A[0])
	}
	if bus.words[0x3000] != 0xdead || bus.words[0x3002] != 0xbeef {
		t.Fatalf("destination=$%04X%04X", bus.words[0x3000], bus.words[0x3002])
	}
	if result.Cycles != 28 {
		t.Fatalf("cycles=%d, want 28", result.Cycles)
	}
}

func TestGenericAddQuickToAbsoluteWord(t *testing.T) {
	// ADDQ.W #1,(xxx).W：Journey to the Laugh 的停止編碼。
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x5278, 0x0402: 0x2000, 0x0404: 0x4e71,
		0x2000: 0x0041,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if bus.words[0x2000] != 0x0042 {
		t.Fatalf("value=$%04X, want $0042", bus.words[0x2000])
	}
	if result.Cycles != 16 {
		t.Fatalf("cycles=%d, want 16", result.Cycles)
	}
}

func TestGenericMoveFromStatusToPredecrement(t *testing.T) {
	// MOVE SR,-(A7)：Monopoly 與 Sango Fighter 的停止編碼。
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x40e7, 0x0402: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[7] = 0x2000
	cpu.state.SR = 0x2704
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[7] != 0x1ffe {
		t.Fatalf("A7=$%08X, want $00001FFE", cpu.state.A[7])
	}
	if bus.words[0x1ffe] != 0x2704 {
		t.Fatalf("pushed=$%04X, want $2704", bus.words[0x1ffe])
	}
}

func TestGenericClearWordPredecrement(t *testing.T) {
	// CLR.W -(A4)：The Son of Evil 的停止編碼。CLR 在 68000 上仍會先讀目的。
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4264, 0x0402: 0x4e71,
		0x1ffe: 0xffff,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[4] = 0x2000
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[4] != 0x1ffe || bus.words[0x1ffe] != 0 {
		t.Fatalf("A4=$%08X value=$%04X", cpu.state.A[4], bus.words[0x1ffe])
	}
	if cpu.state.SR&flagZero == 0 {
		t.Fatalf("CLR must set Z, SR=$%04X", cpu.state.SR)
	}
	if result.Cycles != 14 {
		t.Fatalf("cycles=%d, want 14", result.Cycles)
	}
}

func TestGenericLinkAndUnlink(t *testing.T) {
	// LINK A6,#-4：Super Taiwanese Baseball League 的停止編碼。
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4e56, 0x0402: 0xfffc, 0x0404: 0x4e5e, 0x0406: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[7] = 0x3000
	cpu.state.A[6] = 0x1234
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[6] != 0x2ffc || cpu.state.A[7] != 0x2ff8 {
		t.Fatalf("A6=$%08X A7=$%08X", cpu.state.A[6], cpu.state.A[7])
	}
	if bus.words[0x2ffc] != 0x0000 || bus.words[0x2ffe] != 0x1234 {
		t.Fatalf("saved frame pointer=$%04X%04X", bus.words[0x2ffc], bus.words[0x2ffe])
	}
	if result.Cycles != 16 {
		t.Fatalf("LINK cycles=%d, want 16", result.Cycles)
	}

	if _, err := cpu.Step(); err != nil { // UNLK A6
		t.Fatal(err)
	}
	if cpu.state.A[6] != 0x1234 || cpu.state.A[7] != 0x3000 {
		t.Fatalf("after UNLK A6=$%08X A7=$%08X", cpu.state.A[6], cpu.state.A[7])
	}
}

func TestGenericMoveLongIndexedToData(t *testing.T) {
	// MOVE.L (2,A3,D0.W),D1：Formosa Duel 的停止編碼。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x2233, 0x0402: 0x0002, 0x0404: 0x4e71,
		0x2006: 0x0123, 0x2008: 0x4567,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[3] = 0x2000
	cpu.state.D[0] = 4
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[1] != 0x0123_4567 {
		t.Fatalf("D1=$%08X, want $01234567", cpu.state.D[1])
	}
	// PRM 的 MOVE Long 表：來源 (d8,An,Xn)、目的 Dn 為 18(4/0)。
	if result.Cycles != 18 {
		t.Fatalf("cycles=%d, want 18", result.Cycles)
	}
}

func TestGenericProgramCounterRelativeSource(t *testing.T) {
	// MOVE.W (6,PC),D3：基底是延伸字本身的位址，不是指令字。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x363a, 0x0402: 0x0006, 0x0404: 0x4e71,
		0x0408: 0xa5a5,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if uint16(cpu.state.D[3]) != 0xa5a5 {
		t.Fatalf("D3=$%08X, want low word $A5A5", cpu.state.D[3])
	}
}

func TestGenericShiftUpdatesCarryAndExtend(t *testing.T) {
	// LSR.L #3,D5。PRM 的位移表：長字為 8 + 2n，位元組與字為 6 + 2n；
	// 扣掉自動計費的 prefetch 之後，內部時間分別是 4 + 2n 與 2 + 2n。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xe68d, 0x0402: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[5] = 0x0000_0014
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[5] != 2 {
		t.Fatalf("D5=$%08X, want $00000002", cpu.state.D[5])
	}
	if cpu.state.SR&flagCarry == 0 || cpu.state.SR&flagExtend == 0 {
		t.Fatalf("SR=$%04X, want C and X set", cpu.state.SR)
	}
	if result.Cycles != 14 {
		t.Fatalf("cycles=%d, want 14", result.Cycles)
	}
}

func TestGenericJumpToSubroutineIndirect(t *testing.T) {
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4e92, 0x0402: 0x4e71,
		0x1000: 0x4e75,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[2] = 0x1000
	cpu.state.A[7] = 0x3000
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.PC != 0x1000 {
		t.Fatalf("PC=$%06X, want $001000", cpu.state.PC)
	}
	if bus.words[0x2ffc] != 0 || bus.words[0x2ffe] != 0x0402 {
		t.Fatalf("return address=$%04X%04X", bus.words[0x2ffc], bus.words[0x2ffe])
	}
	if result.Cycles != 16 {
		t.Fatalf("cycles=%d, want 16", result.Cycles)
	}
}

func TestGenericMoveMultipleWordSignExtendsIntoRegisters(t *testing.T) {
	// MOVEM.W (A0)+,D0/A1：字大小進暫存器一律符號延伸成 32 位元。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x4c98, 0x0402: 0x0201, 0x0404: 0x4e71,
		0x2000: 0xffff, 0x2002: 0x8000,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[0] = 0x2000
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.state.D[0] != 0xffff_ffff {
		t.Fatalf("D0=$%08X, want $FFFFFFFF", cpu.state.D[0])
	}
	if cpu.state.A[1] != 0xffff_8000 {
		t.Fatalf("A1=$%08X, want $FFFF8000", cpu.state.A[1])
	}
	if cpu.state.A[0] != 0x2004 {
		t.Fatalf("A0=$%08X, want $00002004", cpu.state.A[0])
	}
}

func TestGenericDecimalAddAndSubtractRegisters(t *testing.T) {
	// ABCD D0,D1：$25 + $37 = $62，且 X 參與運算。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xc300, 0x0402: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[0] = 0x25
	cpu.state.D[1] = 0x37
	cpu.state.SR |= flagZero
	cpu.state.SR &^= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if uint8(cpu.state.D[1]) != 0x62 {
		t.Fatalf("D1=$%02X, want $62", uint8(cpu.state.D[1]))
	}
	if cpu.state.SR&flagZero != 0 {
		t.Fatalf("非零結果必須清除 Z，SR=$%04X", cpu.state.SR)
	}
	if result.Cycles != 6 {
		t.Fatalf("cycles=%d, want 6", result.Cycles)
	}
}

func TestGenericDecimalSubtractPredecrement(t *testing.T) {
	// SBCD -(A4),-(A3)：Speedy Dragon 的停止編碼。$42 - $17 = $25。
	cpu, _, bus := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x870c, 0x0402: 0x4e71,
	})
	bus.bytes = map[uint32]uint8{0x2000: 0x17, 0x3000: 0x42}
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[4] = 0x2001
	cpu.state.A[3] = 0x3001
	cpu.state.SR |= flagZero
	cpu.state.SR &^= flagExtend
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[4] != 0x2000 || cpu.state.A[3] != 0x3000 {
		t.Fatalf("A4=$%08X A3=$%08X", cpu.state.A[4], cpu.state.A[3])
	}
	if len(bus.byteWrites) != 1 || bus.byteWrites[0].address != 0x3000 || bus.byteWrites[0].value != 0x25 {
		t.Fatalf("byte writes=%+v, want one $25 至 $3000", bus.byteWrites)
	}
	// PRM 的 ABCD／SBCD 表：-(Ay),-(Ax) 形式為 18(3/1)。
	if result.Cycles != 18 {
		t.Fatalf("cycles=%d, want 18", result.Cycles)
	}
}

func TestGenericExtendedAddAccumulatesCarryAndZero(t *testing.T) {
	// ADDX.W D0,D1 帶進位：$FFFF + $0001 + X(1) = $0001，並保持 C 與 X。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0xd340, 0x0402: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.D[0] = 0x0001
	cpu.state.D[1] = 0xffff
	cpu.state.SR |= flagExtend | flagZero
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if uint16(cpu.state.D[1]) != 0x0001 {
		t.Fatalf("D1=$%08X, want low word $0001", cpu.state.D[1])
	}
	if cpu.state.SR&flagCarry == 0 || cpu.state.SR&flagExtend == 0 {
		t.Fatalf("SR=$%04X, want C and X set", cpu.state.SR)
	}
	if cpu.state.SR&flagZero != 0 {
		t.Fatalf("非零結果必須清除 Z，SR=$%04X", cpu.state.SR)
	}
}

func TestGenericAddressRegistersKeepAllThirtyTwoBits(t *testing.T) {
	// MOVE.W (A4)+,(A5)+：68000 的 An 是 32 位元暫存器，24 位元遮罩只發生在
	// 位址匯流排。若在暫存器裡就截斷，`CMPA.L #$FFFFA122,A5` 這種與 Work RAM
	// 高位址比較的複製迴圈永遠不會結束。
	cpu, _, _ := newInstructionCPU(map[uint32]uint16{
		0x0400: 0x3adc, 0x0402: 0xbbfc, 0x0404: 0xffff, 0x0406: 0xa122, 0x0408: 0x4e71,
	})
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.A[4] = 0x0000_2000
	cpu.state.A[5] = 0xffff_a120
	if _, err := cpu.Step(); err != nil {
		t.Fatal(err)
	}
	if cpu.state.A[5] != 0xffff_a122 {
		t.Fatalf("A5=$%08X, want $FFFFA122", cpu.state.A[5])
	}
	if _, err := cpu.Step(); err != nil { // CMPA.L #$FFFFA122,A5
		t.Fatal(err)
	}
	if cpu.state.SR&flagZero == 0 {
		t.Fatalf("CMPA.L 必須相等，SR=$%04X", cpu.state.SR)
	}
}

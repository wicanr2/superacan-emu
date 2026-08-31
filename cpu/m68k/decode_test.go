package m68k

import "testing"

func TestDecodeFirstVerticalSlice(t *testing.T) {
	tests := []struct {
		opcode uint16
		want   Decoded
	}{
		{0x4e71, Decoded{Instruction: InstructionNOP}},
		{0x4eb8, Decoded{Instruction: InstructionJSRAbsoluteWord}},
		{0x207c, Decoded{Instruction: InstructionMOVEAImmediateLong, Register: 0}},
		{0x2e7c, Decoded{Instruction: InstructionMOVEAImmediateLong, Register: 7}},
		{0x7000, Decoded{Instruction: InstructionMOVEQ, Register: 0, Immediate8: 0}},
		{0x76ff, Decoded{Instruction: InstructionMOVEQ, Register: 3, Immediate8: 0xff}},
		{0x6002, Decoded{Instruction: InstructionBRA, Immediate8: 2}},
		{0x6100, Decoded{Instruction: InstructionBSR, Condition: 1}},
		{0x66fe, Decoded{Instruction: InstructionBcc, Condition: 6, Immediate8: 0xfe}},
		{0xffff, Decoded{Instruction: InstructionIllegal}},
	}
	for _, test := range tests {
		if got := Decode(test.opcode); got != test.want {
			t.Errorf("Decode($%04X) = %+v, want %+v", test.opcode, got, test.want)
		}
	}
}

func TestConditionsExhaustiveAgainstBooleanDefinitions(t *testing.T) {
	for flags := uint16(0); flags < 16; flags++ {
		c := flags&flagCarry != 0
		v := flags&flagOverflow != 0
		z := flags&flagZero != 0
		n := flags&flagNegative != 0
		want := [16]bool{
			true, false, !c && !z, c || z,
			!c, c, !z, z,
			!v, v, !n, n,
			n == v, n != v, !z && n == v, z || n != v,
		}
		for condition := uint8(0); condition < 16; condition++ {
			if got := conditionTrue(condition, flags); got != want[condition] {
				t.Errorf("condition %d flags $%X = %v, want %v", condition, flags, got, want[condition])
			}
		}
	}
}

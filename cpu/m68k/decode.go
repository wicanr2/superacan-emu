package m68k

// Instruction identifies the semantic operation selected by the first opcode
// word. It deliberately does not encode an execution function pointer.
type Instruction uint8

const (
	InstructionIllegal Instruction = iota
	InstructionNOP
	InstructionMOVEQ
	InstructionBRA
	InstructionBSR
	InstructionBcc
	InstructionJSRAbsoluteWord
	InstructionMOVEAImmediateLong
	InstructionMOVEWordAbsoluteLongToData
	InstructionMOVEWordDataToAbsoluteLong
	InstructionANDIWordData
)

// Decoded is the auditable result of decoding one opcode word.
type Decoded struct {
	Instruction Instruction
	Register    uint8
	Condition   uint8
	Immediate8  uint8
}

// Decode currently covers the first vertical slice only. Every unmatched
// encoding is illegal to this implementation and will fail closed in Step.
func Decode(opcode uint16) Decoded {
	switch {
	case opcode == 0x4e71:
		return Decoded{Instruction: InstructionNOP}
	case opcode == 0x4eb8:
		return Decoded{Instruction: InstructionJSRAbsoluteWord}
	case opcode&0xf1ff == 0x207c:
		return Decoded{
			Instruction: InstructionMOVEAImmediateLong,
			Register:    uint8(opcode >> 9 & 7),
		}
	case opcode&0xf1ff == 0x3039:
		return Decoded{
			Instruction: InstructionMOVEWordAbsoluteLongToData,
			Register:    uint8(opcode >> 9 & 7),
		}
	case opcode&0xfff8 == 0x33c0:
		return Decoded{
			Instruction: InstructionMOVEWordDataToAbsoluteLong,
			Register:    uint8(opcode & 7),
		}
	case opcode&0xfff8 == 0x0240:
		return Decoded{
			Instruction: InstructionANDIWordData,
			Register:    uint8(opcode & 7),
		}
	case opcode&0xf100 == 0x7000:
		return Decoded{
			Instruction: InstructionMOVEQ,
			Register:    uint8(opcode >> 9 & 7),
			Immediate8:  uint8(opcode),
		}
	case opcode&0xf000 == 0x6000:
		condition := uint8(opcode >> 8 & 0x0f)
		instruction := InstructionBcc
		if condition == 0 {
			instruction = InstructionBRA
		} else if condition == 1 {
			instruction = InstructionBSR
		}
		return Decoded{
			Instruction: instruction,
			Condition:   condition,
			Immediate8:  uint8(opcode),
		}
	default:
		return Decoded{Instruction: InstructionIllegal}
	}
}

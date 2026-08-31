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
	InstructionMOVEWordImmediateToData
	InstructionMOVEWordDataToData
	InstructionMOVEByteDataToAddressIndirect
	InstructionMOVEByteAddressIndirectToPostincrement
	InstructionMOVEWordImmediateToAbsoluteLong
	InstructionCMPIWordData
	InstructionDBcc
	InstructionCMPByteAddressIndirectToData
	InstructionMOVEBytePredecrementToAddressIndirect
	InstructionCLRWordData
	InstructionCLRWordAbsoluteLong
	InstructionCLRByteAbsoluteLong
	InstructionADDBytePredecrementToData
	InstructionSUBBytePostincrementFromData
	InstructionADDWordPredecrementToData
	InstructionSUBWordPostincrementFromData
	InstructionCMPIWordPredecrement
	InstructionSUBQLongAddress
	InstructionCMPBytePredecrementToData
	InstructionMOVEByteImmediateToAbsoluteLong
	InstructionCMPMWord
	InstructionMOVEAAddressToAddress
	InstructionADDAWordImmediate
	InstructionSWAP
	InstructionCLRLongData
	InstructionCLRByteData
	InstructionADDQByteData
	InstructionADDQByteAbsoluteLong
	InstructionBTSTDataData
	InstructionNEGWordData
	InstructionMULSWordPostincrement
	InstructionSUBQByteData
	InstructionADDLongDataToData
	InstructionADDXByteData
	InstructionANDIByteData
	InstructionCMPBytePostincrementToData
	InstructionCMPLongPostincrementToData
	InstructionMOVEWordImmediateToPredecrement
	InstructionJMPAbsoluteLong
	InstructionORIWordData
	InstructionMOVEWordAddressIndirectToData
	InstructionMOVEWordDataToAddressIndirect
	InstructionMOVEAAbsoluteWord
	InstructionJMPAddressIndirect
	InstructionJSRAbsoluteLong
)

// Decoded is the auditable result of decoding one opcode word.
type Decoded struct {
	Instruction    Instruction
	Register       uint8
	SourceRegister uint8
	Condition      uint8
	Immediate8     uint8
	Quick          uint8
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
	case opcode&0xf1ff == 0x303c:
		return Decoded{
			Instruction: InstructionMOVEWordImmediateToData,
			Register:    uint8(opcode >> 9 & 7),
		}
	case opcode&0xf1f8 == 0x3000:
		return Decoded{
			Instruction:    InstructionMOVEWordDataToData,
			Register:       uint8(opcode >> 9 & 7),
			SourceRegister: uint8(opcode & 7),
		}
	case opcode&0xf1f8 == 0x1080:
		return Decoded{
			Instruction:    InstructionMOVEByteDataToAddressIndirect,
			Register:       uint8(opcode >> 9 & 7),
			SourceRegister: uint8(opcode & 7),
		}
	case opcode&0xf1f8 == 0x10d0:
		return Decoded{
			Instruction:    InstructionMOVEByteAddressIndirectToPostincrement,
			Register:       uint8(opcode >> 9 & 7),
			SourceRegister: uint8(opcode & 7),
		}
	case opcode == 0x33fc:
		return Decoded{Instruction: InstructionMOVEWordImmediateToAbsoluteLong}
	case opcode&0xfff8 == 0x0c40:
		return Decoded{
			Instruction: InstructionCMPIWordData,
			Register:    uint8(opcode & 7),
		}
	case opcode&0xf0f8 == 0x50c8:
		return Decoded{
			Instruction: InstructionDBcc,
			Register:    uint8(opcode & 7),
			Condition:   uint8(opcode >> 8 & 0x0f),
		}
	case opcode&0xf1f8 == 0xb010:
		return Decoded{
			Instruction:    InstructionCMPByteAddressIndirectToData,
			Register:       uint8(opcode >> 9 & 7),
			SourceRegister: uint8(opcode & 7),
		}
	case opcode&0xf1f8 == 0x10a0:
		return Decoded{
			Instruction:    InstructionMOVEBytePredecrementToAddressIndirect,
			Register:       uint8(opcode >> 9 & 7),
			SourceRegister: uint8(opcode & 7),
		}
	case opcode&0xfff8 == 0x4240:
		return Decoded{Instruction: InstructionCLRWordData, Register: uint8(opcode & 7)}
	case opcode == 0x4279:
		return Decoded{Instruction: InstructionCLRWordAbsoluteLong}
	case opcode == 0x4239:
		return Decoded{Instruction: InstructionCLRByteAbsoluteLong}
	case opcode&0xf1f8 == 0xd020:
		return Decoded{Instruction: InstructionADDBytePredecrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x9018:
		return Decoded{Instruction: InstructionSUBBytePostincrementFromData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xd060:
		return Decoded{Instruction: InstructionADDWordPredecrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x9058:
		return Decoded{Instruction: InstructionSUBWordPostincrementFromData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x0c60:
		return Decoded{Instruction: InstructionCMPIWordPredecrement, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x5188:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionSUBQLongAddress, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0xb020:
		return Decoded{Instruction: InstructionCMPBytePredecrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode == 0x13fc:
		return Decoded{Instruction: InstructionMOVEByteImmediateToAbsoluteLong}
	case opcode&0xf1f8 == 0xb148:
		return Decoded{
			Instruction:    InstructionCMPMWord,
			Register:       uint8(opcode >> 9 & 7),
			SourceRegister: uint8(opcode & 7),
		}
	case opcode&0xf1f8 == 0x2048:
		return Decoded{Instruction: InstructionMOVEAAddressToAddress, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0xd0fc:
		return Decoded{Instruction: InstructionADDAWordImmediate, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xfff8 == 0x4840:
		return Decoded{Instruction: InstructionSWAP, Register: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x4280:
		return Decoded{Instruction: InstructionCLRLongData, Register: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x4200:
		return Decoded{Instruction: InstructionCLRByteData, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x5000:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionADDQByteData, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1ff == 0x5039:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionADDQByteAbsoluteLong, Quick: quick}
	case opcode&0xf1f8 == 0x0100:
		return Decoded{Instruction: InstructionBTSTDataData, Register: uint8(opcode & 7), SourceRegister: uint8(opcode >> 9 & 7)}
	case opcode&0xfff8 == 0x4440:
		return Decoded{Instruction: InstructionNEGWordData, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xc1d8:
		return Decoded{Instruction: InstructionMULSWordPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x5100:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionSUBQByteData, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0xd080:
		return Decoded{Instruction: InstructionADDLongDataToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xd100:
		return Decoded{Instruction: InstructionADDXByteData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x0200:
		return Decoded{Instruction: InstructionANDIByteData, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xb018:
		return Decoded{Instruction: InstructionCMPBytePostincrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xb098:
		return Decoded{Instruction: InstructionCMPLongPostincrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x313c:
		return Decoded{Instruction: InstructionMOVEWordImmediateToPredecrement, Register: uint8(opcode >> 9 & 7)}
	case opcode == 0x4ef9:
		return Decoded{Instruction: InstructionJMPAbsoluteLong}
	case opcode == 0x4eb9:
		return Decoded{Instruction: InstructionJSRAbsoluteLong}
	case opcode&0xfff8 == 0x0040:
		return Decoded{Instruction: InstructionORIWordData, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x3010:
		return Decoded{Instruction: InstructionMOVEWordAddressIndirectToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x3080:
		return Decoded{Instruction: InstructionMOVEWordDataToAddressIndirect, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x2078:
		return Decoded{Instruction: InstructionMOVEAAbsoluteWord, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xfff8 == 0x4ed0:
		return Decoded{Instruction: InstructionJMPAddressIndirect, Register: uint8(opcode & 7)}
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

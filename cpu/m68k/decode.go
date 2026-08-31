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
	InstructionCMPIWordAbsoluteLong
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
	InstructionMOVEMLongRegistersToPredecrement
	InstructionMOVEMLongPostincrementToRegisters
	InstructionRTS
	InstructionMOVEBytePostincrementToData
	InstructionADDQAddress
	InstructionLSLWordImmediate
	InstructionCMPIByteData
	InstructionMOVEWordImmediateToPostincrement
	InstructionMOVEBytePredecrementToData
	InstructionMOVEByteDataToData
	InstructionMOVEWordIndexedToData
	InstructionMOVEWordDataToIndexed
	InstructionSUBQWordData
	InstructionADDQWordData
	InstructionADDQWordAbsoluteLong
	InstructionRTE
	InstructionLSRWordImmediate
	InstructionADDIWordData
	InstructionMOVEWordDataToPostincrement
	InstructionLEAAbsoluteLong
	InstructionLSLByteImmediate
	InstructionROXLWordImmediate
	InstructionMOVEByteDataToPostincrement
	InstructionMOVEALongPostincrement
	InstructionMOVEBytePCIndexedToData
	InstructionADDAWordData
	InstructionCMPALongImmediate
	InstructionMOVELongPostincrementToAddressIndirect
	InstructionSUBAWordImmediate
	InstructionMOVEByteIndexedToData
	InstructionBSETDataData
	InstructionORWordDataToData
	InstructionSUBAWordData
	InstructionSUBIWordData
	InstructionJMPPCIndexed
	InstructionMOVEBytePostincrementToPostincrement
	InstructionMOVEALongPCIndexed
	InstructionMOVEByteIndexedToPostincrement
	InstructionMOVEWordPostincrementToPostincrement
	InstructionMOVEAWordImmediate
	InstructionPEAPCDisplacement
	InstructionMOVELongImmediateToPredecrement
	InstructionMOVELongAddressToPredecrement
	InstructionLEAPCDisplacement
	InstructionMOVELongPostincrementToPostincrement
	InstructionROLByteImmediate
	InstructionMOVEWordPCIndexedToData
	InstructionJSRAddressIndexed
	InstructionCMPIByteAbsoluteLong
	InstructionCLRLongPostincrement
	InstructionCLRBytePostincrement
	InstructionMOVELongImmediateToPostincrement
	InstructionCLRWordPostincrement
	InstructionSUBILongData
	InstructionTSTWordAbsoluteLong
	InstructionMOVELongDataToAbsoluteLong
	InstructionMOVEWordDataToDisplacement
	InstructionMOVELongDataToDisplacement
	InstructionEXTLongData
	InstructionSUBWordDataFromData
	InstructionMULUWordData
	InstructionDIVUWordData
	InstructionMOVEWordDataToPredecrement
	InstructionLSLLongImmediate
	InstructionMOVELongDataToPredecrement
	InstructionCLRWordAddressIndirect
	InstructionCLRLongPredecrement
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
	case opcode == 0x0c79:
		return Decoded{Instruction: InstructionCMPIWordAbsoluteLong}
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
	case opcode&0xfff8 == 0x48e0:
		return Decoded{Instruction: InstructionMOVEMLongRegistersToPredecrement, Register: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x4cd8:
		return Decoded{Instruction: InstructionMOVEMLongPostincrementToRegisters, Register: uint8(opcode & 7)}
	case opcode == 0x4e75:
		return Decoded{Instruction: InstructionRTS}
	case opcode&0xf1f8 == 0x1018:
		return Decoded{Instruction: InstructionMOVEBytePostincrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x5048:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionADDQAddress, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0xe148:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionLSLWordImmediate, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xfff8 == 0x0c00:
		return Decoded{Instruction: InstructionCMPIByteData, Register: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x30fc:
		return Decoded{Instruction: InstructionMOVEWordImmediateToPostincrement, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x1020:
		return Decoded{Instruction: InstructionMOVEBytePredecrementToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x1000:
		return Decoded{Instruction: InstructionMOVEByteDataToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x3030:
		return Decoded{Instruction: InstructionMOVEWordIndexedToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x3180:
		return Decoded{Instruction: InstructionMOVEWordDataToIndexed, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x5140:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionSUBQWordData, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0x5040:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionADDQWordData, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1ff == 0x5079:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionADDQWordAbsoluteLong, Quick: quick}
	case opcode == 0x4e73:
		return Decoded{Instruction: InstructionRTE}
	case opcode&0xf1f8 == 0xe048:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionLSRWordImmediate, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xfff8 == 0x0640:
		return Decoded{Instruction: InstructionADDIWordData, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x30c0:
		return Decoded{Instruction: InstructionMOVEWordDataToPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x41f9:
		return Decoded{Instruction: InstructionLEAAbsoluteLong, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0xe108:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionLSLByteImmediate, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0xe150:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionROXLWordImmediate, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0x10c0:
		return Decoded{Instruction: InstructionMOVEByteDataToPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x2058:
		return Decoded{Instruction: InstructionMOVEALongPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x103b:
		return Decoded{Instruction: InstructionMOVEBytePCIndexedToData, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0xd0c0:
		return Decoded{Instruction: InstructionADDAWordData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0xb1fc:
		return Decoded{Instruction: InstructionCMPALongImmediate, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x2098:
		return Decoded{Instruction: InstructionMOVELongPostincrementToAddressIndirect, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x90fc:
		return Decoded{Instruction: InstructionSUBAWordImmediate, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x1030:
		return Decoded{Instruction: InstructionMOVEByteIndexedToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x01c0:
		return Decoded{Instruction: InstructionBSETDataData, Register: uint8(opcode & 7), SourceRegister: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x8040:
		return Decoded{Instruction: InstructionORWordDataToData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x90c0:
		return Decoded{Instruction: InstructionSUBAWordData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x0440:
		return Decoded{Instruction: InstructionSUBIWordData, Register: uint8(opcode & 7)}
	case opcode == 0x4efb:
		return Decoded{Instruction: InstructionJMPPCIndexed}
	case opcode&0xf1f8 == 0x10d8:
		return Decoded{Instruction: InstructionMOVEBytePostincrementToPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x207b:
		return Decoded{Instruction: InstructionMOVEALongPCIndexed, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x10f0:
		return Decoded{Instruction: InstructionMOVEByteIndexedToPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x30d8:
		return Decoded{Instruction: InstructionMOVEWordPostincrementToPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x307c:
		return Decoded{Instruction: InstructionMOVEAWordImmediate, Register: uint8(opcode >> 9 & 7)}
	case opcode == 0x487a:
		return Decoded{Instruction: InstructionPEAPCDisplacement}
	case opcode&0xf1ff == 0x213c:
		return Decoded{Instruction: InstructionMOVELongImmediateToPredecrement, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x2108:
		return Decoded{Instruction: InstructionMOVELongAddressToPredecrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x41fa:
		return Decoded{Instruction: InstructionLEAPCDisplacement, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xf1f8 == 0x20d8:
		return Decoded{Instruction: InstructionMOVELongPostincrementToPostincrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xe118:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionROLByteImmediate, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1ff == 0x303b:
		return Decoded{Instruction: InstructionMOVEWordPCIndexedToData, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xfff8 == 0x4eb0:
		return Decoded{Instruction: InstructionJSRAddressIndexed, Register: uint8(opcode & 7)}
	case opcode == 0x0c39:
		return Decoded{Instruction: InstructionCMPIByteAbsoluteLong}
	case opcode&0xfff8 == 0x4298:
		return Decoded{Instruction: InstructionCLRLongPostincrement, Register: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x4218:
		return Decoded{Instruction: InstructionCLRBytePostincrement, Register: uint8(opcode & 7)}
	case opcode&0xf1ff == 0x20fc:
		return Decoded{Instruction: InstructionMOVELongImmediateToPostincrement, Register: uint8(opcode >> 9 & 7)}
	case opcode&0xfff8 == 0x4258:
		return Decoded{Instruction: InstructionCLRWordPostincrement, Register: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x0480:
		return Decoded{Instruction: InstructionSUBILongData, Register: uint8(opcode & 7)}
	case opcode == 0x4a79:
		return Decoded{Instruction: InstructionTSTWordAbsoluteLong}
	case opcode&0xfff8 == 0x23c0:
		return Decoded{Instruction: InstructionMOVELongDataToAbsoluteLong, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x3140:
		return Decoded{Instruction: InstructionMOVEWordDataToDisplacement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x2140:
		return Decoded{Instruction: InstructionMOVELongDataToDisplacement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x48c0:
		return Decoded{Instruction: InstructionEXTLongData, Register: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x9040:
		return Decoded{Instruction: InstructionSUBWordDataFromData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xc0c0:
		return Decoded{Instruction: InstructionMULUWordData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x80c0:
		return Decoded{Instruction: InstructionDIVUWordData, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0x3100:
		return Decoded{Instruction: InstructionMOVEWordDataToPredecrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xf1f8 == 0xe188:
		quick := uint8(opcode >> 9 & 7)
		if quick == 0 {
			quick = 8
		}
		return Decoded{Instruction: InstructionLSLLongImmediate, Register: uint8(opcode & 7), Quick: quick}
	case opcode&0xf1f8 == 0x2100:
		return Decoded{Instruction: InstructionMOVELongDataToPredecrement, Register: uint8(opcode >> 9 & 7), SourceRegister: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x4250:
		return Decoded{Instruction: InstructionCLRWordAddressIndirect, Register: uint8(opcode & 7)}
	case opcode&0xfff8 == 0x42a0:
		return Decoded{Instruction: InstructionCLRLongPredecrement, Register: uint8(opcode & 7)}
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

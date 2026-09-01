package m68k

import "fmt"

const addressMask uint32 = 0x00ff_ffff

// State contains all currently modelled architectural and prefetch state.
// More exception and pipeline state will be added with the corresponding
// evidence-backed vertical slices.
type State struct {
	D [8]uint32
	A [8]uint32

	PC uint32
	SR uint16

	// InactiveSP 是目前沒有映射到 A7 的那個堆疊指標：監督者模式時是 USP，
	// 使用者模式時是 SSP。68000 依 SR 的 S 位元切換兩者。
	InactiveSP uint32

	IRD uint16 // current instruction word
	IRC uint16 // next prefetched word

	Cycles uint64
}

// StepResult describes one complete instruction without hiding its timing.
type StepResult struct {
	PCBefore       uint32
	PCAfter        uint32
	Opcode         uint16
	Cycles         uint64
	Phases         []Phase
	InterruptLevel uint8
}

// CPU is an independent Motorola 68000 implementation.
type CPU struct {
	bus                  Bus
	scheduler            Scheduler
	state                State
	stepTrace            []Phase
	interruptLevel       uint8
	level7Pending        bool
	interruptAcknowledge func(uint8)
}

// SetInterruptLevel drives the external IPL2-IPL0 pins. Level 7 is latched on
// its rising edge, matching the 68000 non-maskable interrupt behavior.
func (c *CPU) SetInterruptLevel(level uint8) {
	level &= 7
	if level == 7 && c.interruptLevel != 7 {
		c.level7Pending = true
	}
	c.interruptLevel = level
}

func (c *CPU) SetInterruptAcknowledge(callback func(uint8)) {
	c.interruptAcknowledge = callback
}

func New(bus Bus, scheduler Scheduler) *CPU {
	if bus == nil {
		panic("m68k: nil bus")
	}
	if scheduler == nil {
		panic("m68k: nil scheduler")
	}
	return &CPU{bus: bus, scheduler: scheduler}
}

func (c *CPU) State() State { return c.state }

// Reset performs the first evidence-backed vertical slice: supervisor state,
// initial SSP/PC vector reads and two-word prefetch. The 40-cycle total is a
// sample-derived starting contract and remains subject to Motorola-spec review.
func (c *CPU) Reset() error {
	c.state = State{SR: 0x2700}

	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 16}); err != nil {
		return err
	}

	sspHi, err := c.readWord(0, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset SSP high: %w", err)
	}
	sspLo, err := c.readWord(2, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset SSP low: %w", err)
	}
	c.state.A[7] = uint32(sspHi)<<16 | uint32(sspLo)

	pcHi, err := c.readWord(4, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset PC high: %w", err)
	}
	pcLo, err := c.readWord(6, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset PC low: %w", err)
	}
	c.state.PC = (uint32(pcHi)<<16 | uint32(pcLo)) & addressMask

	c.state.IRD, err = c.readWord(c.state.PC, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return fmt.Errorf("m68k reset first prefetch: %w", err)
	}
	c.state.IRC, err = c.readWord(c.state.PC+2, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return fmt.Errorf("m68k reset second prefetch: %w", err)
	}
	return nil
}

// Step executes one complete instruction. Unknown opcodes fail closed and do
// not silently behave as NOP. NOP is the first opcode vertical slice.
func (c *CPU) Step() (StepResult, error) {
	result := StepResult{PCBefore: c.state.PC, Opcode: c.state.IRD}
	start := c.state.Cycles
	c.stepTrace = make([]Phase, 0, 3)
	defer func() { c.stepTrace = nil }()
	if level := c.acceptedInterrupt(); level != 0 {
		if err := c.serviceInterrupt(level); err != nil {
			return result, fmt.Errorf("m68k interrupt level %d: %w", level, err)
		}
		result.PCAfter = c.state.PC
		result.Cycles = c.state.Cycles - start
		result.Phases = append(result.Phases, c.stepTrace...)
		result.InterruptLevel = level
		return result, nil
	}

	decoded := Decode(c.state.IRD)
	switch decoded.Instruction {
	case InstructionNOP:
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k NOP prefetch: %w", err)
		}
	case InstructionMOVEImmediateToSR:
		if err := c.moveImmediateToSR(); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,SR: %w", err)
		}
	case InstructionMOVELongImmediateToAbsoluteLong:
		if err := c.moveLongImmediateToAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k MOVE.L #imm,(xxx).L: %w", err)
		}
	case InstructionMOVEQ:
		c.state.D[decoded.Register] = uint32(int32(int8(decoded.Immediate8)))
		c.setNZ32(c.state.D[decoded.Register])
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k MOVEQ prefetch: %w", err)
		}
	case InstructionBRA:
		if err := c.branch(decoded.Immediate8); err != nil {
			return result, fmt.Errorf("m68k BRA: %w", err)
		}
	case InstructionBcc:
		if err := c.branchConditional(decoded); err != nil {
			return result, fmt.Errorf("m68k Bcc: %w", err)
		}
	case InstructionJSRAbsoluteWord:
		if err := c.jsrAbsoluteWord(); err != nil {
			return result, fmt.Errorf("m68k JSR (xxx).W: %w", err)
		}
	case InstructionMOVEAImmediateLong:
		if err := c.moveAImmediateLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L #imm,A%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordAbsoluteLongToData:
		if err := c.moveWordAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordDataToAbsoluteLong:
		if err := c.moveWordDataToAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,(xxx).L: %w", decoded.Register, err)
		}
	case InstructionANDIWordData:
		if err := c.andiWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ANDI.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordImmediateToData:
		if err := c.moveWordImmediateToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordDataToData:
		if err := c.moveWordDataToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteDataToAddressIndirect:
		if err := c.moveByteDataToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B D%d,(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteAddressIndirectToPostincrement:
		if err := c.moveByteAddressIndirectToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (A%d),(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordImmediateToAbsoluteLong:
		if err := c.moveWordImmediateToAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,(xxx).L: %w", err)
		}
	case InstructionCMPIWordData:
		if err := c.cmpiWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPI.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionCMPIWordAbsoluteLong:
		if err := c.cmpiWordAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k CMPI.W #imm,(xxx).L: %w", err)
		}
	case InstructionORIWordAbsoluteLong:
		if err := c.oriWordAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k ORI.W #imm,(xxx).L: %w", err)
		}
	case InstructionMOVEByteDataToAbsoluteLong:
		if err := c.moveByteDataToAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B D%d,(xxx).L: %w", decoded.Register, err)
		}
	case InstructionTSTByteAddressIndirect:
		if err := c.tstByteAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k TST.B (A%d): %w", decoded.Register, err)
		}
	case InstructionTSTLongAddressIndirect:
		if err := c.tstLongAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k TST.L (A%d): %w", decoded.Register, err)
		}
	case InstructionMOVELongAbsoluteLongToData:
		if err := c.moveLongAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVELongDataToData:
		if err := c.moveLongRegisterToData(c.state.D[decoded.SourceRegister], decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVELongAddressToData:
		if err := c.moveLongRegisterToData(c.state.A[decoded.SourceRegister], decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L A%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVELongAddressToAddressIndirect:
		if err := c.moveLongAddressToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L A%d,(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDQLongData:
		if err := c.addqLongData(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.L #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionMOVELongAddressToDisplacement:
		if err := c.moveLongAddressToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L A%d,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionDIVUWordImmediate:
		if err := c.divuWordImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k DIVU.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionCLRLongDisplacement:
		if err := c.clearLongDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.L (d16,A%d): %w", decoded.Register, err)
		}
	case InstructionJSRAddressIndirect:
		if err := c.jsrAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k JSR (A%d): %w", decoded.Register, err)
		}
	case InstructionADDWordDataToPostincrement:
		if err := c.addWordDataToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADD.W D%d,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEAWordAbsoluteLong:
		if err := c.moveAWordAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.W (xxx).L,A%d: %w", decoded.Register, err)
		}
	case InstructionMOVELongDataToPostincrement:
		if err := c.moveLongDataToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L D%d,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordAddressToData:
		if err := c.moveWordAddressToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W A%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEALongAddressIndirect:
		if err := c.moveALongAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (A%d),A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEAWordPostincrement:
		if err := c.moveAWordPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.W (A%d)+,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDQWordDisplacement:
		if err := c.addqWordDisplacement(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.W #%d,(d16,A%d): %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionMOVEAWordAddressIndirect:
		if err := c.moveAWordAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.W (A%d),A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCMPIWordDisplacement:
		if err := c.cmpiWordDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPI.W #imm,(d16,A%d): %w", decoded.Register, err)
		}
	case InstructionADDWordAbsoluteLongToData:
		if err := c.addWordAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADD.W (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionORLongAddressIndirectToData:
		if err := c.orLongAddressIndirectToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k OR.L (A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionDIVUWordAbsoluteLong:
		if err := c.divuWordAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k DIVU.W (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionORLongDisplacementToData:
		if err := c.orLongDisplacementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k OR.L (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordAddressIndirectToDisplacement:
		if err := c.moveWordAddressIndirectToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (A%d),(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBWordDisplacementFromData:
		if err := c.subWordDisplacementFromData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUB.W (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEMWordRegistersToPredecrement:
		if err := c.movemWordRegistersToPredecrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEM.W <list>,-(A%d): %w", decoded.Register, err)
		}
	case InstructionCMPWordDisplacementToData:
		if err := c.cmpWordDisplacementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.W (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEAWordData:
		if err := c.moveAWordData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.W D%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionDIVUWordDisplacement:
		if err := c.divuWordDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k DIVU.W (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDWordAddressToData:
		if err := c.addWordAddressToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADD.W A%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordAbsoluteLongToAbsoluteLong:
		if err := c.moveWordAbsoluteLongToAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (xxx).L,(xxx).L: %w", err)
		}
	case InstructionMOVEMWordPostincrementToRegisters:
		if err := c.movemWordPostincrementToRegisters(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEM.W (A%d)+,<list>: %w", decoded.Register, err)
		}
	case InstructionORWordAbsoluteLongToData:
		if err := c.orWordAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k OR.W (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEALongData:
		if err := c.moveALongData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L D%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCMPWordAddressIndirectToData:
		if err := c.cmpWordAddressIndirectToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.W (A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionTSTByteDisplacement:
		if err := c.tstByteDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k TST.B (d16,A%d): %w", decoded.Register, err)
		}
	case InstructionCMPByteDisplacementToData:
		if err := c.cmpByteDisplacementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.B (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionTSTWordDisplacement:
		if err := c.tstWordDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k TST.W (d16,A%d): %w", decoded.Register, err)
		}
	case InstructionMOVEByteDataToDisplacement:
		if err := c.moveByteDataToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B D%d,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteImmediateToAddressIndirect:
		if err := c.moveByteImmediateToAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B #imm,(A%d): %w", decoded.Register, err)
		}
	case InstructionBTSTImmediateDisplacement:
		if err := c.btstImmediateDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k BTST #imm,(d16,A%d): %w", decoded.Register, err)
		}
	case InstructionMOVELongPostincrementToDisplacement:
		if err := c.moveLongPostincrementToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (A%d)+,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDWordDataToDisplacement:
		if err := c.addWordDataToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADD.W D%d,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionORWordDataToAddressIndirect:
		if err := c.orWordDataToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k OR.W D%d,(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCLRByteDisplacement:
		if err := c.clearByteDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.B (d16,A%d): %w", decoded.Register, err)
		}
	case InstructionORWordDataToDisplacement:
		if err := c.orWordDataToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k OR.W D%d,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBQWordDisplacement:
		if err := c.subqWordDisplacement(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k SUBQ.W #%d,(d16,A%d): %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionMOVEWordIndexedToDisplacement:
		if err := c.moveWordIndexedToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (d8,A%d,Xn),(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCLRWordDisplacement:
		if err := c.clearWordDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.W (d16,A%d): %w", decoded.Register, err)
		}
	case InstructionEXTWordData:
		if err := c.extWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k EXT.W D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEByteImmediateToDisplacement:
		if err := c.moveByteImmediateToDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B #imm,(d16,A%d): %w", decoded.Register, err)
		}
	case InstructionMOVELongImmediateToAddressIndirect:
		if err := c.moveLongImmediateToAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L #imm,(A%d): %w", decoded.Register, err)
		}
	case InstructionSUBQWordAbsoluteLong:
		if err := c.subqWordAbsoluteLong(decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k SUBQ.W #%d,(xxx).L: %w", decoded.Quick, err)
		}
	case InstructionMOVELongDisplacementToDisplacement:
		if err := c.moveLongDisplacementToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (d16,A%d),(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVELongAddressIndirectToDisplacement:
		if err := c.moveLongAddressIndirectToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (A%d),(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBALongAddress:
		if err := c.subaLongAddress(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUBA.L A%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionTSTWordData:
		c.setNZ16(uint16(c.state.D[decoded.Register]))
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k TST.W D%d: %w", decoded.Register, err)
		}
	case InstructionTSTByteAbsoluteLong:
		if err := c.tstByteAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k TST.B (xxx).L: %w", err)
		}
	case InstructionEORWordDataToData:
		value := uint16(c.state.D[decoded.Register]) ^ uint16(c.state.D[decoded.SourceRegister])
		c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffff0000 | uint32(value)
		c.setNZ16(value)
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k EOR.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionNOTWordData:
		value := ^uint16(c.state.D[decoded.Register])
		c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffff0000 | uint32(value)
		c.setNZ16(value)
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k NOT.W D%d: %w", decoded.Register, err)
		}
	case InstructionANDWordDataToData:
		value := uint16(c.state.D[decoded.Register]) & uint16(c.state.D[decoded.SourceRegister])
		c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffff0000 | uint32(value)
		c.setNZ16(value)
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k AND.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteImmediateToData:
		if err := c.moveByteImmediateToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionANDILongData:
		if err := c.andiLongData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ANDI.L #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVELongImmediateToData:
		if err := c.moveLongImmediateToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionORLongDataToData:
		value := c.state.D[decoded.Register] | c.state.D[decoded.SourceRegister]
		c.state.D[decoded.Register] = value
		c.setNZ32(value)
		if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
			return result, fmt.Errorf("m68k OR.L D%d,D%d internal: %w", decoded.SourceRegister, decoded.Register, err)
		}
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k OR.L D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMULUWordImmediate:
		if err := c.muluWordImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MULU.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordPostincrementToAddressIndirect:
		if err := c.moveWordPostincrementToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (A%d)+,(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordPostincrementToDisplacement:
		if err := c.moveWordPostincrementToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (A%d)+,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordImmediateToAddressIndirect:
		if err := c.moveWordImmediateToAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,(A%d): %w", decoded.Register, err)
		}
	case InstructionMOVEWordImmediateToDisplacement:
		if err := c.moveWordImmediateToDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,(d16,A%d): %w", decoded.Register, err)
		}
	case InstructionMOVELongImmediateToDisplacement:
		if err := c.moveLongImmediateToDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L #imm,(d16,A%d): %w", decoded.Register, err)
		}
	case InstructionSUBQByteAbsoluteLong:
		if err := c.subqByteAbsoluteLong(decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k SUBQ.B #%d,(xxx).L: %w", decoded.Quick, err)
		}
	case InstructionMOVEALongAbsoluteLong:
		if err := c.moveALongAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (xxx).L,A%d: %w", decoded.Register, err)
		}
	case InstructionMOVEByteDisplacementToAbsoluteLong:
		if err := c.moveByteDisplacementToAbsoluteLong(decoded.SourceRegister); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (d16,A%d),(xxx).L: %w", decoded.SourceRegister, err)
		}
	case InstructionCMPILongAddressIndirect:
		if err := c.cmpiLongAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPI.L #imm,(A%d): %w", decoded.Register, err)
		}
	case InstructionMOVEWordDisplacementToData:
		if err := c.moveWordDisplacementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteDisplacementToData:
		if err := c.moveByteDisplacementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (d16,A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDWordDataToAbsoluteLong:
		if err := c.addWordDataToAbsoluteLong(decoded.SourceRegister); err != nil {
			return result, fmt.Errorf("m68k ADD.W D%d,(xxx).L: %w", decoded.SourceRegister, err)
		}
	case InstructionADDQLongAbsoluteLong:
		if err := c.addqLongAbsoluteLong(decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.L #%d,(xxx).L: %w", decoded.Quick, err)
		}
	case InstructionBTSTImmediateData:
		if err := c.btstImmediateData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k BTST #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEByteAbsoluteLongToData:
		if err := c.moveByteAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionCMPByteAbsoluteLongToData:
		if err := c.cmpByteAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.B (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionCMPWordAbsoluteLongToData:
		if err := c.cmpWordAbsoluteLongToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.W (xxx).L,D%d: %w", decoded.Register, err)
		}
	case InstructionCMPWordDataToData:
		c.setCompare16(uint16(c.state.D[decoded.Register]), uint16(c.state.D[decoded.SourceRegister]))
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k CMP.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDWordDataToData:
		value := c.add16(uint16(c.state.D[decoded.Register]), uint16(c.state.D[decoded.SourceRegister]))
		c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffff0000 | uint32(value)
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k ADD.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCMPLongDataToData:
		c.setCompare32(c.state.D[decoded.Register], c.state.D[decoded.SourceRegister])
		if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
			return result, fmt.Errorf("m68k CMP.L D%d,D%d internal: %w", decoded.SourceRegister, decoded.Register, err)
		}
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k CMP.L D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEALongDisplacement:
		if err := c.moveALongDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (d16,A%d),A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteAddressIndirectToData:
		if err := c.moveByteAddressIndirectToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionLSLWordRegister:
		if err := c.lslWordRegister(decoded.Register, decoded.SourceRegister); err != nil {
			return result, fmt.Errorf("m68k LSL.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionEORByteDataToData:
		value := uint8(c.state.D[decoded.Register]) ^ uint8(c.state.D[decoded.SourceRegister])
		c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffffff00 | uint32(value)
		c.setNZ8(value)
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k EOR.B D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionLSRByteImmediate:
		if err := c.lsrByteImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k LSR.B #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionORByteDataToData:
		value := uint8(c.state.D[decoded.Register]) | uint8(c.state.D[decoded.SourceRegister])
		c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffffff00 | uint32(value)
		c.setNZ8(value)
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k OR.B D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDILongAbsoluteLong:
		if err := c.addiLongAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k ADDI.L #imm,(xxx).L: %w", err)
		}
	case InstructionMOVELongAbsoluteLongToAbsoluteLong:
		if err := c.moveLongAbsoluteLongToAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (xxx).L,(xxx).L: %w", err)
		}
	case InstructionSUBILongAbsoluteLong:
		if err := c.subiLongAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k SUBI.L #imm,(xxx).L: %w", err)
		}
	case InstructionCLRLongAbsoluteLong:
		if err := c.clearAbsoluteLong(WidthLong); err != nil {
			return result, fmt.Errorf("m68k CLR.L (xxx).L: %w", err)
		}
	case InstructionLSRLongImmediate:
		if err := c.lsrLongImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k LSR.L #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionANDIWordAbsoluteLong:
		if err := c.andiWordAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k ANDI.W #imm,(xxx).L: %w", err)
		}
	case InstructionDBcc:
		if err := c.dbcc(decoded); err != nil {
			return result, fmt.Errorf("m68k DBcc condition %d,D%d: %w", decoded.Condition, decoded.Register, err)
		}
	case InstructionCMPByteAddressIndirectToData:
		if err := c.cmpByteAddressIndirectToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.B (A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEBytePredecrementToAddressIndirect:
		if err := c.moveBytePredecrementToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B -(A%d),(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCLRWordData:
		if err := c.clearWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.W D%d: %w", decoded.Register, err)
		}
	case InstructionCLRWordAbsoluteLong:
		if err := c.clearAbsoluteLong(WidthWord); err != nil {
			return result, fmt.Errorf("m68k CLR.W (xxx).L: %w", err)
		}
	case InstructionCLRByteAbsoluteLong:
		if err := c.clearAbsoluteLong(WidthByte); err != nil {
			return result, fmt.Errorf("m68k CLR.B (xxx).L: %w", err)
		}
	case InstructionADDBytePredecrementToData:
		if err := c.addPredecrementToData(decoded.SourceRegister, decoded.Register, WidthByte); err != nil {
			return result, fmt.Errorf("m68k ADD.B -(A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBBytePostincrementFromData:
		if err := c.subPostincrementFromData(decoded.SourceRegister, decoded.Register, WidthByte); err != nil {
			return result, fmt.Errorf("m68k SUB.B (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDWordPredecrementToData:
		if err := c.addPredecrementToData(decoded.SourceRegister, decoded.Register, WidthWord); err != nil {
			return result, fmt.Errorf("m68k ADD.W -(A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBWordPostincrementFromData:
		if err := c.subPostincrementFromData(decoded.SourceRegister, decoded.Register, WidthWord); err != nil {
			return result, fmt.Errorf("m68k SUB.W (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCMPIWordPredecrement:
		if err := c.cmpiWordPredecrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPI.W #imm,-(A%d): %w", decoded.Register, err)
		}
	case InstructionSUBQLongAddress:
		if err := c.subqLongAddress(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k SUBQ.L #%d,A%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionCMPBytePredecrementToData:
		if err := c.cmpBytePredecrementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.B -(A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteImmediateToAbsoluteLong:
		if err := c.moveByteImmediateToAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k MOVE.B #imm,(xxx).L: %w", err)
		}
	case InstructionCMPMWord:
		if err := c.cmpmWord(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPM.W (A%d)+,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEAAddressToAddress:
		if err := c.moveAAddressToAddress(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L A%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDAWordImmediate:
		if err := c.addaWordImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADDA.W #imm,A%d: %w", decoded.Register, err)
		}
	case InstructionADDALongImmediate:
		if err := c.addaLongImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADDA.L #imm,A%d: %w", decoded.Register, err)
		}
	case InstructionSWAP:
		if err := c.swap(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SWAP D%d: %w", decoded.Register, err)
		}
	case InstructionCLRLongData:
		if err := c.clearLongData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.L D%d: %w", decoded.Register, err)
		}
	case InstructionCLRByteData:
		if err := c.clearByteData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.B D%d: %w", decoded.Register, err)
		}
	case InstructionADDQByteData:
		if err := c.addqByteData(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.B #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionADDQByteAbsoluteLong:
		if err := c.addqByteAbsoluteLong(decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.B #%d,(xxx).L: %w", decoded.Quick, err)
		}
	case InstructionBTSTDataData:
		if err := c.btstDataData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k BTST D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionNEGWordData:
		if err := c.negWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k NEG.W D%d: %w", decoded.Register, err)
		}
	case InstructionMULSWordPostincrement:
		if err := c.mulsWordPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MULS.W (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBQByteData:
		if err := c.subqByteData(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k SUBQ.B #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionADDLongDataToData:
		if err := c.addLongDataToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADD.L D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDXByteData:
		if err := c.addxByteData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADDX.B D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionANDIByteData:
		if err := c.andiByteData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ANDI.B #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionCMPBytePostincrementToData:
		if err := c.cmpBytePostincrementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.B (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCMPLongPostincrementToData:
		if err := c.cmpLongPostincrementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMP.L (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordImmediateToPredecrement:
		if err := c.moveWordImmediateToPredecrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,-(A%d): %w", decoded.Register, err)
		}
	case InstructionJMPAbsoluteLong:
		if err := c.jmpAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k JMP (xxx).L: %w", err)
		}
	case InstructionORIWordData:
		if err := c.oriWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ORI.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordAddressIndirectToData:
		if err := c.moveWordAddressIndirectToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordPostincrementToData:
		if err := c.moveWordPostincrementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordDataToAddressIndirect:
		if err := c.moveWordDataToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEAAbsoluteWord:
		if err := c.moveAAbsoluteWord(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (xxx).W,A%d: %w", decoded.Register, err)
		}
	case InstructionJMPAddressIndirect:
		if err := c.jmpAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k JMP (A%d): %w", decoded.Register, err)
		}
	case InstructionJSRAbsoluteLong:
		if err := c.jsrAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k JSR (xxx).L: %w", err)
		}
	case InstructionMOVEMLongRegistersToPredecrement:
		if err := c.movemLongRegistersToPredecrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEM.L regs,-(A%d): %w", decoded.Register, err)
		}
	case InstructionMOVEMLongPostincrementToRegisters:
		if err := c.movemLongPostincrementToRegisters(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEM.L (A%d)+,regs: %w", decoded.Register, err)
		}
	case InstructionRTS:
		if err := c.rts(); err != nil {
			return result, fmt.Errorf("m68k RTS: %w", err)
		}
	case InstructionMOVEBytePostincrementToData:
		if err := c.moveBytePostincrementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (A%d)+,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDQAddress:
		if err := c.addqAddress(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ #%d,A%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionLSLWordImmediate:
		if err := c.lslWordImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k LSL.W #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionCMPIByteData:
		if err := c.cmpiByteData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPI.B #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordImmediateToPostincrement:
		if err := c.moveWordImmediateToPostincrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W #imm,(A%d)+: %w", decoded.Register, err)
		}
	case InstructionMOVEBytePredecrementToData:
		if err := c.moveBytePredecrementToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B -(A%d),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEByteDataToData:
		if err := c.moveByteDataToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordIndexedToData:
		if err := c.moveWordIndexedToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (d8,A%d,Xn),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEALongIndexed:
		if err := c.moveALongIndexed(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (d8,A%d,Xn),A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordDataToIndexed:
		if err := c.moveWordDataToIndexed(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,(d8,A%d,Xn): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBQWordData:
		if err := c.subqWordData(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k SUBQ.W #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionADDQWordData:
		if err := c.addqWordData(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.W #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionADDQWordAbsoluteLong:
		if err := c.addqWordAbsoluteLong(decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ADDQ.W #%d,(xxx).L: %w", decoded.Quick, err)
		}
	case InstructionRTE:
		if err := c.rte(); err != nil {
			return result, fmt.Errorf("m68k RTE: %w", err)
		}
	case InstructionLSRWordImmediate:
		if err := c.lsrWordImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k LSR.W #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionADDIWordData:
		if err := c.addiWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADDI.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionMOVEWordDataToPostincrement:
		if err := c.moveWordDataToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionLEAAbsoluteLong:
		if err := c.leaAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k LEA (xxx).L,A%d: %w", decoded.Register, err)
		}
	case InstructionLSLByteImmediate:
		if err := c.lslByteImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k LSL.B #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionROXLWordImmediate:
		if err := c.roxlWordImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ROXL.W #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionMOVEByteDataToPostincrement:
		if err := c.moveByteDataToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B D%d,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEALongPostincrement:
		if err := c.moveALongPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (A%d)+,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEBytePCIndexedToData:
		if err := c.moveBytePCIndexedToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (d8,PC,Xn),D%d: %w", decoded.Register, err)
		}
	case InstructionADDAWordData:
		if err := c.addaWordData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADDA.W D%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionADDALongData:
		if err := c.addaLongData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k ADDA.L D%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCMPALongImmediate:
		if err := c.cmpaLongImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CMPA.L #imm,A%d: %w", decoded.Register, err)
		}
	case InstructionMOVELongPostincrementToAddressIndirect:
		if err := c.moveLongPostincrementToAddressIndirect(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (A%d)+,(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBAWordImmediate:
		if err := c.subaWordImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUBA.W #imm,A%d: %w", decoded.Register, err)
		}
	case InstructionMOVEByteIndexedToData:
		if err := c.moveByteIndexedToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (d8,A%d,Xn),D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionBSETDataData:
		if err := c.bsetDataData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k BSET D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionORWordDataToData:
		if err := c.orWordDataToData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k OR.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBAWordData:
		if err := c.subaWordData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUBA.W D%d,A%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionSUBIWordData:
		if err := c.subiWordData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUBI.W #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionJMPPCIndexed:
		if err := c.jmpPCIndexed(); err != nil {
			return result, fmt.Errorf("m68k JMP (d8,PC,Xn): %w", err)
		}
	case InstructionMOVEBytePostincrementToPostincrement:
		if err := c.moveBytePostincrementToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (A%d)+,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEALongPCIndexed:
		if err := c.moveALongPCIndexed(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.L (d8,PC,Xn),A%d: %w", decoded.Register, err)
		}
	case InstructionMOVEByteIndexedToPostincrement:
		if err := c.moveByteIndexedToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.B (d8,A%d,Xn),(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordPostincrementToPostincrement:
		if err := c.moveWordPostincrementToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (A%d)+,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEAWordImmediate:
		if err := c.moveAWordImmediate(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVEA.W #imm,A%d: %w", decoded.Register, err)
		}
	case InstructionPEAPCDisplacement:
		if err := c.peaPCDisplacement(); err != nil {
			return result, fmt.Errorf("m68k PEA (d16,PC): %w", err)
		}
	case InstructionMOVELongImmediateToPredecrement:
		if err := c.moveLongImmediateToPredecrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L #imm,-(A%d): %w", decoded.Register, err)
		}
	case InstructionMOVELongAddressToPredecrement:
		if err := c.moveLongAddressToPredecrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L A%d,-(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionLEAPCDisplacement:
		if err := c.leaPCDisplacement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k LEA (d16,PC),A%d: %w", decoded.Register, err)
		}
	case InstructionMOVELongPostincrementToPostincrement:
		if err := c.moveLongPostincrementToPostincrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L (A%d)+,(A%d)+: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionROLByteImmediate:
		if err := c.rolByteImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k ROL.B #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionMOVEWordPCIndexedToData:
		if err := c.moveWordPCIndexedToData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W (d8,PC,Xn),D%d: %w", decoded.Register, err)
		}
	case InstructionJSRAddressIndexed:
		if err := c.jsrAddressIndexed(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k JSR (d8,A%d,Xn): %w", decoded.Register, err)
		}
	case InstructionCMPIByteAbsoluteLong:
		if err := c.cmpiByteAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k CMPI.B #imm,(xxx).L: %w", err)
		}
	case InstructionCLRLongPostincrement:
		if err := c.clearLongPostincrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.L (A%d)+: %w", decoded.Register, err)
		}
	case InstructionCLRBytePostincrement:
		if err := c.clearBytePostincrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.B (A%d)+: %w", decoded.Register, err)
		}
	case InstructionMOVELongImmediateToPostincrement:
		if err := c.moveLongImmediateToPostincrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L #imm,(A%d)+: %w", decoded.Register, err)
		}
	case InstructionCLRWordPostincrement:
		if err := c.clearWordPostincrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.W (A%d)+: %w", decoded.Register, err)
		}
	case InstructionSUBILongData:
		if err := c.subiLongData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUBI.L #imm,D%d: %w", decoded.Register, err)
		}
	case InstructionTSTWordAbsoluteLong:
		if err := c.tstWordAbsoluteLong(); err != nil {
			return result, fmt.Errorf("m68k TST.W (xxx).L: %w", err)
		}
	case InstructionMOVELongDataToAbsoluteLong:
		if err := c.moveLongDataToAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L D%d,(xxx).L: %w", decoded.Register, err)
		}
	case InstructionMOVELongAddressToAbsoluteLong:
		if err := c.moveLongAddressToAbsoluteLong(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L A%d,(xxx).L: %w", decoded.Register, err)
		}
	case InstructionMOVEWordDataToDisplacement:
		if err := c.moveWordDataToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVELongDataToDisplacement:
		if err := c.moveLongDataToDisplacement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L D%d,(d16,A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionEXTLongData:
		if err := c.extLongData(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k EXT.L D%d: %w", decoded.Register, err)
		}
	case InstructionSUBWordDataFromData:
		if err := c.subWordDataFromData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k SUB.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMULUWordData:
		if err := c.muluWordData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MULU.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionDIVUWordData:
		if err := c.divuWordData(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k DIVU.W D%d,D%d: %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionMOVEWordDataToPredecrement:
		if err := c.moveWordDataToPredecrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.W D%d,-(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionLSLLongImmediate:
		if err := c.lslLongImmediate(decoded.Register, decoded.Quick); err != nil {
			return result, fmt.Errorf("m68k LSL.L #%d,D%d: %w", decoded.Quick, decoded.Register, err)
		}
	case InstructionMOVELongDataToPredecrement:
		if err := c.moveLongDataToPredecrement(decoded.SourceRegister, decoded.Register); err != nil {
			return result, fmt.Errorf("m68k MOVE.L D%d,-(A%d): %w", decoded.SourceRegister, decoded.Register, err)
		}
	case InstructionCLRWordAddressIndirect:
		if err := c.clearWordAddressIndirect(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.W (A%d): %w", decoded.Register, err)
		}
	case InstructionCLRLongPredecrement:
		if err := c.clearLongPredecrement(decoded.Register); err != nil {
			return result, fmt.Errorf("m68k CLR.L -(A%d): %w", decoded.Register, err)
		}
	case InstructionBSR:
		if err := c.bsr(decoded.Immediate8); err != nil {
			return result, fmt.Errorf("m68k BSR: %w", err)
		}
	default:
		// 逐一列舉的 decoder 沒有這個編碼時，交給一般化 effective-address 執行層；
		// 兩邊都不認識才 fail-closed，不得靜默當成 NOP。
		handled, err := c.executeGeneric(c.state.IRD)
		if !handled && err == nil {
			// 68000 明確定義為非法的編碼要產生例外；我們還沒實作的編碼維持
			// fail-closed，兩者不可混為一談。
			handled, err = c.illegalInstruction(c.state.IRD)
		}
		if err != nil {
			return result, fmt.Errorf("m68k opcode $%04X at $%06X: %w", c.state.IRD, c.state.PC, err)
		}
		if !handled {
			return result, fmt.Errorf("m68k: unimplemented opcode $%04X at $%06X", c.state.IRD, c.state.PC)
		}
	}

	result.PCAfter = c.state.PC
	result.Cycles = c.state.Cycles - start
	result.Phases = append(result.Phases, c.stepTrace...)
	return result, nil
}

func (c *CPU) acceptedInterrupt() uint8 {
	if c.level7Pending {
		return 7
	}
	if c.interruptLevel > uint8(c.state.SR>>8&7) {
		return c.interruptLevel
	}
	return 0
}

func (c *CPU) serviceInterrupt(level uint8) error {
	oldSR, oldPC := c.state.SR, c.state.PC
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 12}); err != nil {
		return err
	}
	if err := c.advance(Phase{Kind: PhaseInterruptAcknowledge, Cycles: 4, Address: uint32(level), Width: WidthByte, FC: FCCPU}); err != nil {
		return err
	}
	if c.interruptAcknowledge != nil {
		c.interruptAcknowledge(level)
	}
	if level == 7 {
		c.level7Pending = false
	}
	c.state.A[7] = c.state.A[7] - 4
	if err := c.writeLong(c.state.A[7], oldPC, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[7] = c.state.A[7] - 2
	if err := c.writeWord(c.state.A[7], oldSR, FCSupervisorData); err != nil {
		return err
	}
	c.setStatusRegister(oldSR&0xf8ff | 0x2000 | uint16(level)<<8)
	target, err := c.readLong(uint32(24+level)*4, FCSupervisorData)
	if err != nil {
		return err
	}
	return c.refillPrefetch(target&addressMask, 0)
}

func (c *CPU) moveBytePredecrementToAddressIndirect(source, destination uint8) error {
	decrement := uint32(1)
	if source == 7 {
		decrement = 2
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[source] = c.state.A[source] - decrement
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setNZ8(value)
	if err := c.writeByte(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) cmpByteAddressIndirectToData(source, destination uint8) error {
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setCompare8(uint8(c.state.D[destination]), value)
	return c.prefetch()
}

func (c *CPU) cmpByteAbsoluteLongToData(destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.setCompare8(uint8(c.state.D[destination]), value)
	return stream.finish()
}

func (c *CPU) cmpWordAbsoluteLongToData(destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.setCompare16(uint16(c.state.D[destination]), value)
	return stream.finish()
}

func (c *CPU) moveWordImmediateToData(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return stream.finish()
}

func (c *CPU) moveByteImmediateToData(register uint8) error {
	stream := c.newInstructionStream()
	word, err := stream.nextWord()
	if err != nil {
		return err
	}
	value := uint8(word)
	c.state.D[register] = c.state.D[register]&0xffffff00 | uint32(value)
	c.setNZ8(value)
	return stream.finish()
}

func (c *CPU) moveLongImmediateToData(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.state.D[register] = value
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) moveWordDataToData(source, destination uint8) error {
	value := uint16(c.state.D[source])
	c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return c.prefetch()
}

func (c *CPU) moveByteDataToAddressIndirect(source, destination uint8) error {
	value := uint8(c.state.D[source])
	c.setNZ8(value)
	if err := c.writeByte(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveByteAddressIndirectToPostincrement(source, destination uint8) error {
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setNZ8(value)
	if err := c.writeByte(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	increment := uint32(1)
	if destination == 7 {
		increment = 2
	}
	c.state.A[destination] = c.state.A[destination] + increment
	return c.prefetch()
}

func (c *CPU) moveWordPostincrementToAddressIndirect(source, destination uint8) error {
	value, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[source] += 2
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveWordImmediateToAbsoluteLong() error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.setNZ16(value)
	if err := c.writeWord(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveWordImmediateToAddressIndirect(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[register], value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveWordImmediateToDisplacement(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	displacement, err := stream.nextWord()
	if err != nil {
		return err
	}
	address := uint32(int32(c.state.A[register])+int32(int16(displacement))) & addressMask
	c.setNZ16(value)
	if err := c.writeWord(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveByteDataToAbsoluteLong(register uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value := uint8(c.state.D[register])
	c.setNZ8(value)
	if err := c.writeByte(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) tstByteAddressIndirect(register uint8) error {
	value, err := c.readByte(c.state.A[register], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setNZ8(value)
	return c.prefetch()
}

func (c *CPU) tstLongAddressIndirect(register uint8) error {
	value, err := c.readLong(c.state.A[register], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setNZ32(value)
	return c.prefetch()
}

func (c *CPU) moveLongAbsoluteLongToData(register uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readLong(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.D[register] = value
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) moveLongRegisterToData(value uint32, destination uint8) error {
	c.state.D[destination] = value
	c.setNZ32(value)
	return c.prefetch()
}

func (c *CPU) moveLongAddressToAddressIndirect(source, destination uint8) error {
	value := c.state.A[source]
	c.setNZ32(value)
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) addqLongData(register, quick uint8) error {
	c.state.D[register] = c.add32(c.state.D[register], uint32(quick))
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) tstByteAbsoluteLong() error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.setNZ8(value)
	return stream.finish()
}

func (c *CPU) cmpiWordData(register uint8) error {
	stream := c.newInstructionStream()
	source, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.setCompare16(uint16(c.state.D[register]), source)
	return stream.finish()
}

func (c *CPU) cmpiLongAddressIndirect(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readLong(c.state.A[register], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setCompare32(value, immediate)
	return stream.finish()
}

func (c *CPU) cmpiWordAbsoluteLong() error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.setCompare16(value, immediate)
	return stream.finish()
}

func (c *CPU) oriWordAbsoluteLong() error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	value |= immediate
	c.setNZ16(value)
	// Moira's 68000 path accounts for the immediate-memory read/modify/write
	// handoff before the write bus cycle. Together with absolute-long EA
	// fetches this produces Motorola's documented 28-cycle total.
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	if err := c.writeWord(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) dbcc(decoded Decoded) error {
	fallthroughPC := (c.state.PC + 4) & addressMask
	if conditionTrue(decoded.Condition, c.state.SR) {
		return c.refillPrefetch(fallthroughPC, 4)
	}

	count := uint16(c.state.D[decoded.Register]) - 1
	c.state.D[decoded.Register] = c.state.D[decoded.Register]&0xffff_0000 | uint32(count)
	if count == 0xffff {
		return c.refillPrefetch(fallthroughPC, 6)
	}
	target := uint32(int32(c.state.PC+2)+int32(int16(c.state.IRC))) & addressMask
	return c.refillPrefetch(target, 2)
}

func (c *CPU) moveWordAbsoluteLongToData(register uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return stream.finish()
}

func (c *CPU) moveByteAbsoluteLongToData(register uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.D[register] = c.state.D[register]&0xffffff00 | uint32(value)
	c.setNZ8(value)
	return stream.finish()
}

func (c *CPU) moveWordDataToAbsoluteLong(register uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	if err := c.writeWord(address, uint16(c.state.D[register]), FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) andiWordData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	value := uint16(c.state.D[register]) & immediate
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return stream.finish()
}

func (c *CPU) moveAImmediateLong(register uint8) error {
	stream := c.newInstructionStream()
	hi, err := stream.nextWord()
	if err != nil {
		return err
	}
	lo, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.state.A[register] = uint32(hi)<<16 | uint32(lo)
	return stream.finish()
}

func (c *CPU) jsrAbsoluteWord() error {
	// Absolute-short addresses are sign-extended to 32 bits, then constrained
	// by the MC68000's 24-bit physical address bus.
	target := uint32(int32(int16(c.state.IRC))) & addressMask
	returnAddress := (c.state.PC + 4) & addressMask

	// Moira's 68000 handler computes the effective address, performs its
	// absolute-word two-cycle delay, pushes the return PC, then refills the
	// destination queue. This also matches Motorola's 18-cycle (2R/2W) total.
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[7] = c.state.A[7] - 4
	if err := c.writeWord(c.state.A[7], uint16(returnAddress>>16), FCSupervisorData); err != nil {
		return err
	}
	if err := c.writeWord(c.state.A[7]+2, uint16(returnAddress), FCSupervisorData); err != nil {
		return err
	}
	return c.refillPrefetch(target, 0)
}

func (c *CPU) setNZ32(value uint32) {
	// MOVEQ affects N and Z, clears V and C, and leaves X unchanged.
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if value == 0 {
		c.state.SR |= flagZero
	}
	if value&0x8000_0000 != 0 {
		c.state.SR |= flagNegative
	}
}

func (c *CPU) setNZ16(value uint16) {
	// MOVE.W and ANDI.W affect N and Z, clear V and C, and preserve X.
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if value == 0 {
		c.state.SR |= flagZero
	}
	if value&0x8000 != 0 {
		c.state.SR |= flagNegative
	}
}

func (c *CPU) setNZ8(value uint8) {
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if value == 0 {
		c.state.SR |= flagZero
	}
	if value&0x80 != 0 {
		c.state.SR |= flagNegative
	}
}

func (c *CPU) setCompare16(destination, source uint16) {
	result := destination - source
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if result == 0 {
		c.state.SR |= flagZero
	}
	if result&0x8000 != 0 {
		c.state.SR |= flagNegative
	}
	if (destination^source)&(destination^result)&0x8000 != 0 {
		c.state.SR |= flagOverflow
	}
	if source > destination {
		c.state.SR |= flagCarry
	}
}

func (c *CPU) setCompare32(destination, source uint32) {
	result := destination - source
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if result == 0 {
		c.state.SR |= flagZero
	}
	if result&0x8000_0000 != 0 {
		c.state.SR |= flagNegative
	}
	if (destination^source)&(destination^result)&0x8000_0000 != 0 {
		c.state.SR |= flagOverflow
	}
	if source > destination {
		c.state.SR |= flagCarry
	}
}

func (c *CPU) setCompare8(destination, source uint8) {
	result := destination - source
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if result == 0 {
		c.state.SR |= flagZero
	}
	if result&0x80 != 0 {
		c.state.SR |= flagNegative
	}
	if (destination^source)&(destination^result)&0x80 != 0 {
		c.state.SR |= flagOverflow
	}
	if source > destination {
		c.state.SR |= flagCarry
	}
}

func (c *CPU) branch(displacement8 uint8) error {
	base := (c.state.PC + 2) & addressMask
	var target uint32
	if displacement8 == 0 {
		target = uint32(int32(base)+int32(int16(c.state.IRC))) & addressMask
	} else {
		target = uint32(int32(base)+int32(int8(displacement8))) & addressMask
	}
	return c.refillPrefetch(target, 2)
}

func (c *CPU) branchConditional(decoded Decoded) error {
	if conditionTrue(decoded.Condition, c.state.SR) {
		return c.branch(decoded.Immediate8)
	}

	if decoded.Immediate8 == 0 {
		// The prefetched word is the displacement. Fetch both words of the
		// fall-through queue. M68000 Bcc.w not-taken timing is 12 cycles.
		if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
			return err
		}
		return c.refillPrefetch((c.state.PC+4)&addressMask, 0)
	}

	// Byte displacement not taken: normal one-word prefetch plus four internal
	// cycles, for the documented M68000 total of eight cycles.
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) refillPrefetch(target uint32, internalCycles uint8) error {
	if internalCycles != 0 {
		if err := c.advance(Phase{Kind: PhaseInternal, Cycles: internalCycles}); err != nil {
			return err
		}
	}
	first, err := c.readWord(target, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	second, err := c.readWord(target+2, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	c.state.PC = target
	c.state.IRD = first
	c.state.IRC = second
	return nil
}

func (c *CPU) prefetch() error {
	nextAddress := (c.state.PC + 4) & addressMask
	next, err := c.readWord(nextAddress, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	c.state.PC = (c.state.PC + 2) & addressMask
	c.state.IRD = c.state.IRC
	c.state.IRC = next
	return nil
}

func (c *CPU) readWord(address uint32, fc FunctionCode, kind PhaseKind) (uint16, error) {
	address &= addressMask
	if address&1 != 0 {
		return 0, fmt.Errorf("odd word address $%06X", address)
	}
	phase := Phase{Kind: kind, Cycles: 4, Address: address, Width: WidthWord, FC: fc}
	if err := c.advance(phase); err != nil {
		return 0, err
	}
	return c.bus.Read16(address)
}

func (c *CPU) readByte(address uint32, fc FunctionCode) (uint8, error) {
	address &= addressMask
	phase := Phase{Kind: PhaseDataRead, Cycles: 4, Address: address, Width: WidthByte, FC: fc}
	if err := c.advance(phase); err != nil {
		return 0, err
	}
	return c.bus.Read8(address)
}

func (c *CPU) writeByte(address uint32, value uint8, fc FunctionCode) error {
	address &= addressMask
	phase := Phase{
		Kind: PhaseDataWrite, Cycles: 4, Address: address,
		Width: WidthByte, Write: true, Value: uint32(value), FC: fc,
	}
	if err := c.advance(phase); err != nil {
		return err
	}
	return c.bus.Write8(address, value)
}

func (c *CPU) writeWord(address uint32, value uint16, fc FunctionCode) error {
	address &= addressMask
	if address&1 != 0 {
		return fmt.Errorf("odd word address $%06X", address)
	}
	phase := Phase{
		Kind: PhaseDataWrite, Cycles: 4, Address: address,
		Width: WidthWord, Write: true, Value: uint32(value), FC: fc,
	}
	if err := c.advance(phase); err != nil {
		return err
	}
	return c.bus.Write16(address, value)
}

func (c *CPU) advance(phase Phase) error {
	if err := c.scheduler.Advance(phase); err != nil {
		return err
	}
	c.state.Cycles += uint64(phase.Cycles)
	if c.stepTrace != nil {
		c.stepTrace = append(c.stepTrace, phase)
	}
	return nil
}

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

	IRD uint16 // current instruction word
	IRC uint16 // next prefetched word

	Cycles uint64
}

// StepResult describes one complete instruction without hiding its timing.
type StepResult struct {
	PCBefore uint32
	PCAfter  uint32
	Opcode   uint16
	Cycles   uint64
	Phases   []Phase
}

// CPU is an independent Motorola 68000 implementation.
type CPU struct {
	bus       Bus
	scheduler Scheduler
	state     State
	stepTrace []Phase
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

	decoded := Decode(c.state.IRD)
	switch decoded.Instruction {
	case InstructionNOP:
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k NOP prefetch: %w", err)
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
	case InstructionBSR:
		return result, fmt.Errorf("m68k: unimplemented BSR opcode $%04X at $%06X", c.state.IRD, c.state.PC)
	default:
		return result, fmt.Errorf("m68k: unimplemented opcode $%04X at $%06X", c.state.IRD, c.state.PC)
	}

	result.PCAfter = c.state.PC
	result.Cycles = c.state.Cycles - start
	result.Phases = append(result.Phases, c.stepTrace...)
	return result, nil
}

func (c *CPU) moveBytePredecrementToAddressIndirect(source, destination uint8) error {
	decrement := uint32(1)
	if source == 7 {
		decrement = 2
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] - decrement) & addressMask
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
	c.state.A[destination] = (c.state.A[destination] + increment) & addressMask
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

func (c *CPU) cmpiWordData(register uint8) error {
	stream := c.newInstructionStream()
	source, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.setCompare16(uint16(c.state.D[register]), source)
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
	c.state.A[7] = (c.state.A[7] - 4) & addressMask
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

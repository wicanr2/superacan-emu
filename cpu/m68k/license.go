package m68k

import "math/bits"

func (c *CPU) moveAAddressToAddress(source, destination uint8) error {
	c.state.A[destination] = c.state.A[source]
	return c.prefetch()
}

func (c *CPU) addaWordImmediate(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.state.A[register] = uint32(int32(c.state.A[register]) + int32(int16(value)))
	return stream.finish()
}

func (c *CPU) swap(register uint8) error {
	value := c.state.D[register]
	value = value<<16 | value>>16
	c.state.D[register] = value
	c.setNZ32(value)
	return c.prefetch()
}

func (c *CPU) clearLongData(register uint8) error {
	c.state.D[register] = 0
	c.setNZ32(0)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) clearByteData(register uint8) error {
	c.state.D[register] &= 0xffff_ff00
	c.setNZ8(0)
	return c.prefetch()
}

func (c *CPU) addqByteData(register, quick uint8) error {
	result := c.add8(uint8(c.state.D[register]), quick)
	c.state.D[register] = c.state.D[register]&0xffff_ff00 | uint32(result)
	return c.prefetch()
}

func (c *CPU) addqByteAbsoluteLong(quick uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	value = c.add8(value, quick)
	if err := c.writeByte(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) btstDataData(bitRegister, dataRegister uint8) error {
	bit := c.state.D[bitRegister] & 31
	c.state.SR &^= flagZero
	if c.state.D[dataRegister]&(uint32(1)<<bit) == 0 {
		c.state.SR |= flagZero
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) negWordData(register uint8) error {
	value := uint16(c.state.D[register])
	result := c.sub16(0, value)
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(result)
	return c.prefetch()
}

func (c *CPU) mulsWordPostincrement(source, destination uint8) error {
	operand, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 2) & addressMask
	if err := c.prefetch(); err != nil {
		return err
	}
	result := int32(int16(operand)) * int32(int16(c.state.D[destination]))
	c.state.D[destination] = uint32(result)
	c.setNZ32(uint32(result))
	transitions := bits.OnesCount16(((operand << 1) ^ operand) & 0xffff)
	return c.advance(Phase{Kind: PhaseInternal, Cycles: uint8(34 + 2*transitions)})
}

func (c *CPU) subqByteData(register, quick uint8) error {
	result := c.sub8(uint8(c.state.D[register]), quick)
	c.state.D[register] = c.state.D[register]&0xffff_ff00 | uint32(result)
	return c.prefetch()
}

func (c *CPU) addLongDataToData(source, destination uint8) error {
	left, right := c.state.D[destination], c.state.D[source]
	result := left + right
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry | flagExtend
	if result == 0 {
		c.state.SR |= flagZero
	}
	if result&0x8000_0000 != 0 {
		c.state.SR |= flagNegative
	}
	if ^(left^right)&(left^result)&0x8000_0000 != 0 {
		c.state.SR |= flagOverflow
	}
	if uint64(left)+uint64(right) > 0xffff_ffff {
		c.state.SR |= flagCarry | flagExtend
	}
	c.state.D[destination] = result
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) addxByteData(source, destination uint8) error {
	left, right := uint8(c.state.D[destination]), uint8(c.state.D[source])
	extend := uint16(0)
	if c.state.SR&flagExtend != 0 {
		extend = 1
	}
	wide := uint16(left) + uint16(right) + extend
	result := uint8(wide)
	oldZero := c.state.SR&flagZero != 0
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry | flagExtend
	if oldZero && result == 0 {
		c.state.SR |= flagZero
	}
	if result&0x80 != 0 {
		c.state.SR |= flagNegative
	}
	if ^(left^right)&(left^result)&0x80 != 0 {
		c.state.SR |= flagOverflow
	}
	if wide > 0xff {
		c.state.SR |= flagCarry | flagExtend
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(result)
	return c.prefetch()
}

func (c *CPU) andiByteData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	result := uint8(c.state.D[register]) & uint8(immediate)
	c.state.D[register] = c.state.D[register]&0xffff_ff00 | uint32(result)
	c.setNZ8(result)
	return stream.finish()
}

func (c *CPU) cmpBytePostincrementToData(source, destination uint8) error {
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + addressStep(source, WidthByte)) & addressMask
	c.setCompare8(uint8(c.state.D[destination]), value)
	return c.prefetch()
}

func (c *CPU) cmpLongPostincrementToData(source, destination uint8) error {
	hi, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	lo, err := c.readWord(c.state.A[source]+2, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 4) & addressMask
	left, right := c.state.D[destination], uint32(hi)<<16|uint32(lo)
	result := left - right
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	if result == 0 {
		c.state.SR |= flagZero
	}
	if result&0x8000_0000 != 0 {
		c.state.SR |= flagNegative
	}
	if (left^right)&(left^result)&0x8000_0000 != 0 {
		c.state.SR |= flagOverflow
	}
	if right > left {
		c.state.SR |= flagCarry
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveWordImmediateToPredecrement(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[register] = (c.state.A[register] - 2) & addressMask
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[register], value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) jmpAbsoluteLong() error {
	hi := c.state.IRC
	lo, err := c.readWord(c.state.PC+4, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	target := (uint32(hi)<<16 | uint32(lo)) & addressMask
	return c.refillPrefetch(target, 0)
}

func (c *CPU) oriWordData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	value := uint16(c.state.D[register]) | immediate
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return stream.finish()
}

func (c *CPU) moveWordAddressIndirectToData(source, destination uint8) error {
	value, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return c.prefetch()
}

func (c *CPU) moveWordDataToAddressIndirect(source, destination uint8) error {
	value := uint16(c.state.D[source])
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveAAbsoluteWord(destination uint8) error {
	stream := c.newInstructionStream()
	extension, err := stream.nextWord()
	if err != nil {
		return err
	}
	address := uint32(int32(int16(extension))) & addressMask
	hi, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	lo, err := c.readWord(address+2, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[destination] = uint32(hi)<<16 | uint32(lo)
	return stream.finish()
}

func (c *CPU) jmpAddressIndirect(register uint8) error {
	return c.refillPrefetch(c.state.A[register]&addressMask, 0)
}

func (c *CPU) jsrAbsoluteLong() error {
	hi := c.state.IRC
	lo, err := c.readWord(c.state.PC+4, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	target := (uint32(hi)<<16 | uint32(lo)) & addressMask
	returnAddress := (c.state.PC + 6) & addressMask
	c.state.A[7] = (c.state.A[7] - 4) & addressMask
	if err := c.writeWord(c.state.A[7], uint16(returnAddress>>16), FCSupervisorData); err != nil {
		return err
	}
	if err := c.writeWord(c.state.A[7]+2, uint16(returnAddress), FCSupervisorData); err != nil {
		return err
	}
	return c.refillPrefetch(target, 0)
}

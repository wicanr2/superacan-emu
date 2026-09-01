package m68k

import "fmt"

func (c *CPU) registerValue(index uint8) uint32 {
	if index < 8 {
		return c.state.D[index]
	}
	return c.state.A[index-8]
}

func (c *CPU) setRegisterValue(index uint8, value uint32) {
	if index < 8 {
		c.state.D[index] = value
	} else {
		c.state.A[index-8] = value
	}
}

func (c *CPU) writeLong(address, value uint32, fc FunctionCode) error {
	if err := c.writeWord(address, uint16(value>>16), fc); err != nil {
		return err
	}
	return c.writeWord(address+2, uint16(value), fc)
}

func (c *CPU) readLong(address uint32, fc FunctionCode) (uint32, error) {
	hi, err := c.readWord(address, fc, PhaseDataRead)
	if err != nil {
		return 0, err
	}
	lo, err := c.readWord(address+2, fc, PhaseDataRead)
	if err != nil {
		return 0, err
	}
	return uint32(hi)<<16 | uint32(lo), nil
}

func (c *CPU) movemLongRegistersToPredecrement(addressRegister uint8) error {
	stream := c.newInstructionStream()
	mask, err := stream.nextWord()
	if err != nil {
		return err
	}
	// Predecrement masks are reversed: bit 0=A7 through bit 15=D0.
	for bit := uint8(0); bit < 16; bit++ {
		if mask&(uint16(1)<<bit) == 0 {
			continue
		}
		register := uint8(15) - bit
		c.state.A[addressRegister] = c.state.A[addressRegister] - 4
		value := c.registerValue(register)
		if register == addressRegister+8 {
			value = c.state.A[addressRegister]
		}
		if err := c.writeLong(c.state.A[addressRegister], value, FCSupervisorData); err != nil {
			return err
		}
	}
	return stream.finish()
}

func (c *CPU) movemLongPostincrementToRegisters(addressRegister uint8) error {
	stream := c.newInstructionStream()
	mask, err := stream.nextWord()
	if err != nil {
		return err
	}
	address := c.state.A[addressRegister]
	for register := uint8(0); register < 16; register++ {
		if mask&(uint16(1)<<register) == 0 {
			continue
		}
		value, err := c.readLong(address, FCSupervisorData)
		if err != nil {
			return err
		}
		address = (address + 4) & addressMask
		c.setRegisterValue(register, value)
	}
	c.state.A[addressRegister] = address
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) bsr(displacement8 uint8) error {
	base := (c.state.PC + 2) & addressMask
	returnAddress := base
	var target uint32
	if displacement8 == 0 {
		returnAddress = (c.state.PC + 4) & addressMask
		target = uint32(int32(base)+int32(int16(c.state.IRC))) & addressMask
	} else {
		target = uint32(int32(base)+int32(int8(displacement8))) & addressMask
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[7] = c.state.A[7] - 4
	if err := c.writeLong(c.state.A[7], returnAddress, FCSupervisorData); err != nil {
		return err
	}
	return c.refillPrefetch(target, 0)
}

func (c *CPU) rts() error {
	target, err := c.readLong(c.state.A[7], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[7] = c.state.A[7] + 4
	return c.refillPrefetch(target&addressMask, 0)
}

func (c *CPU) rte() error {
	if c.state.SR&0x2000 == 0 {
		return fmt.Errorf("privilege violation exception is not implemented")
	}
	restoredSR, err := c.readWord(c.state.A[7], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	target, err := c.readLong(c.state.A[7]+2, FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[7] = c.state.A[7] + 6
	c.state.SR = restoredSR
	return c.refillPrefetch(target&addressMask, 0)
}

func (c *CPU) peaPCDisplacement() error {
	stream := c.newInstructionStream()
	displacement, err := stream.nextWord()
	if err != nil {
		return err
	}
	address := uint32(int32((c.state.PC+2)&addressMask)+int32(int16(displacement))) & addressMask
	c.state.A[7] = c.state.A[7] - 4
	if err := c.writeLong(c.state.A[7], address, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveLongImmediateToPredecrement(destination uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.state.A[destination] = c.state.A[destination] - 4
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveLongAddressToPredecrement(source, destination uint8) error {
	value := c.state.A[source]
	c.state.A[destination] = c.state.A[destination] - 4
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) jsrAddressIndexed(addressRegister uint8) error {
	stream := c.newInstructionStream()
	target, err := stream.nextBriefIndexedAddress(c.state.A[addressRegister])
	if err != nil {
		return err
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[7] = c.state.A[7] - 4
	if err := c.writeLong(c.state.A[7], (c.state.PC+4)&addressMask, FCSupervisorData); err != nil {
		return err
	}
	return c.refillPrefetch(target, 0)
}

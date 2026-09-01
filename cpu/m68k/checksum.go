package m68k

func (c *CPU) clearWordData(register uint8) error {
	c.state.D[register] &= 0xffff_0000
	c.setNZ16(0)
	return c.prefetch()
}

func (c *CPU) clearAbsoluteLong(size Width) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	switch size {
	case WidthByte:
		if _, err := c.readByte(address, FCSupervisorData); err != nil {
			return err
		}
		c.setNZ8(0)
		if err := c.writeByte(address, 0, FCSupervisorData); err != nil {
			return err
		}
	case WidthWord:
		if _, err := c.readWord(address, FCSupervisorData, PhaseDataRead); err != nil {
			return err
		}
		c.setNZ16(0)
		if err := c.writeWord(address, 0, FCSupervisorData); err != nil {
			return err
		}
	case WidthLong:
		if _, err := c.readLong(address, FCSupervisorData); err != nil {
			return err
		}
		c.setNZ32(0)
		if err := c.writeLong(address, 0, FCSupervisorData); err != nil {
			return err
		}
	}
	return stream.finish()
}

func addressStep(register uint8, size Width) uint32 {
	if size == WidthByte && register == 7 {
		return 2
	}
	return uint32(size / 8)
}

func (c *CPU) addPredecrementToData(source, destination uint8, size Width) error {
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] - addressStep(source, size)) & addressMask
	if size == WidthByte {
		operand, err := c.readByte(c.state.A[source], FCSupervisorData)
		if err != nil {
			return err
		}
		result := c.add8(uint8(c.state.D[destination]), operand)
		c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(result)
	} else {
		operand, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
		if err != nil {
			return err
		}
		result := c.add16(uint16(c.state.D[destination]), operand)
		c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(result)
	}
	return c.prefetch()
}

func (c *CPU) subPostincrementFromData(source, destination uint8, size Width) error {
	address := c.state.A[source]
	if size == WidthByte {
		operand, err := c.readByte(address, FCSupervisorData)
		if err != nil {
			return err
		}
		result := c.sub8(uint8(c.state.D[destination]), operand)
		c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(result)
	} else {
		operand, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
		if err != nil {
			return err
		}
		result := c.sub16(uint16(c.state.D[destination]), operand)
		c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(result)
	}
	c.state.A[source] = (c.state.A[source] + addressStep(source, size)) & addressMask
	return c.prefetch()
}

func (c *CPU) cmpiWordPredecrement(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[register] = c.state.A[register] - 2
	value, err := c.readWord(c.state.A[register], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.setCompare16(value, immediate)
	return stream.finish()
}

func (c *CPU) subqLongAddress(register, quick uint8) error {
	c.state.A[register] = (c.state.A[register] - uint32(quick)) & addressMask
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) cmpBytePredecrementToData(source, destination uint8) error {
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] - addressStep(source, WidthByte)) & addressMask
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.setCompare8(uint8(c.state.D[destination]), value)
	return c.prefetch()
}

func (c *CPU) moveByteImmediateToAbsoluteLong() error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value := uint8(immediate)
	c.setNZ8(value)
	if err := c.writeByte(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) cmpmWord(source, destination uint8) error {
	sourceValue, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[source] = c.state.A[source] + 2
	destinationValue, err := c.readWord(c.state.A[destination], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[destination] = c.state.A[destination] + 2
	c.setCompare16(destinationValue, sourceValue)
	return c.prefetch()
}

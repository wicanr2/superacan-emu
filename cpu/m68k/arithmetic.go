package m68k

func (c *CPU) add8(destination, source uint8) uint8 {
	result := destination + source
	c.setNZ8(result)
	carry := uint16(destination)+uint16(source) > 0xff
	if ^(destination^source)&(destination^result)&0x80 != 0 {
		c.state.SR |= flagOverflow
	}
	if carry {
		c.state.SR |= flagCarry | flagExtend
	} else {
		c.state.SR &^= flagExtend
	}
	return result
}

func (c *CPU) add16(destination, source uint16) uint16 {
	result := destination + source
	c.setNZ16(result)
	carry := uint32(destination)+uint32(source) > 0xffff
	if ^(destination^source)&(destination^result)&0x8000 != 0 {
		c.state.SR |= flagOverflow
	}
	if carry {
		c.state.SR |= flagCarry | flagExtend
	} else {
		c.state.SR &^= flagExtend
	}
	return result
}

func (c *CPU) sub8(destination, source uint8) uint8 {
	result := destination - source
	c.setNZ8(result)
	if (destination^source)&(destination^result)&0x80 != 0 {
		c.state.SR |= flagOverflow
	}
	if source > destination {
		c.state.SR |= flagCarry | flagExtend
	} else {
		c.state.SR &^= flagExtend
	}
	return result
}

func (c *CPU) sub16(destination, source uint16) uint16 {
	result := destination - source
	c.setNZ16(result)
	if (destination^source)&(destination^result)&0x8000 != 0 {
		c.state.SR |= flagOverflow
	}
	if source > destination {
		c.state.SR |= flagCarry | flagExtend
	} else {
		c.state.SR &^= flagExtend
	}
	return result
}

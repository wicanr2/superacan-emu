package m68k

import (
	"fmt"
	"math/bits"
)

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

func (c *CPU) moveImmediateToSR() error {
	if c.state.SR&0x2000 == 0 {
		return fmt.Errorf("privilege violation exception is not implemented")
	}
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	if value&0x2000 == 0 {
		return fmt.Errorf("user stack pointer switch is not implemented")
	}
	c.state.SR = value
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 8}); err != nil {
		return err
	}
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

func (c *CPU) moveWordPostincrementToData(source, destination uint8) error {
	value, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 2) & addressMask
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

func (c *CPU) moveBytePostincrementToData(source, destination uint8) error {
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + addressStep(source, WidthByte)) & addressMask
	c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	return c.prefetch()
}

func (c *CPU) addqAddress(register, quick uint8) error {
	c.state.A[register] += uint32(quick)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) lslWordImmediate(register, count uint8) error {
	value := uint16(c.state.D[register])
	var carry bool
	for i := uint8(0); i < count; i++ {
		carry = value&0x8000 != 0
		value <<= 1
	}
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	c.state.SR &^= flagExtend
	if carry {
		c.state.SR |= flagCarry | flagExtend
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2 + 2*count}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) cmpiByteData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.setCompare8(uint8(c.state.D[register]), uint8(immediate))
	return stream.finish()
}

func (c *CPU) moveWordImmediateToPostincrement(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[register], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[register] = (c.state.A[register] + 2) & addressMask
	return stream.finish()
}

func (c *CPU) moveBytePredecrementToData(source, destination uint8) error {
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] - addressStep(source, WidthByte)) & addressMask
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	return c.prefetch()
}

func (c *CPU) moveByteDataToData(source, destination uint8) error {
	value := uint8(c.state.D[source])
	c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	return c.prefetch()
}

func (c *CPU) moveWordIndexedToData(base, destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress(c.state.A[base])
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveWordDataToIndexed(source, base uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress(c.state.A[base])
	if err != nil {
		return err
	}
	value := uint16(c.state.D[source])
	c.setNZ16(value)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	if err := c.writeWord(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) subqWordData(register, quick uint8) error {
	result := c.sub16(uint16(c.state.D[register]), uint16(quick))
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(result)
	return c.prefetch()
}

func (c *CPU) addqWordData(register, quick uint8) error {
	result := c.add16(uint16(c.state.D[register]), uint16(quick))
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(result)
	return c.prefetch()
}

func (c *CPU) addqWordAbsoluteLong(quick uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	value = c.add16(value, uint16(quick))
	if err := c.writeWord(address, value, FCSupervisorData); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) lsrWordImmediate(register, count uint8) error {
	value := uint16(c.state.D[register])
	var carry bool
	for i := uint8(0); i < count; i++ {
		carry = value&1 != 0
		value >>= 1
	}
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	c.state.SR &^= flagExtend
	if carry {
		c.state.SR |= flagCarry | flagExtend
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2 + 2*count}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) addiWordData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	result := c.add16(uint16(c.state.D[register]), immediate)
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(result)
	return stream.finish()
}

func (c *CPU) moveWordDataToPostincrement(source, destination uint8) error {
	value := uint16(c.state.D[source])
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + 2) & addressMask
	return c.prefetch()
}

func (c *CPU) leaAbsoluteLong(destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.state.A[destination] = address
	return stream.finish()
}

func (c *CPU) lslByteImmediate(register, count uint8) error {
	value := uint8(c.state.D[register])
	var carry bool
	for i := uint8(0); i < count; i++ {
		carry = value&0x80 != 0
		value <<= 1
	}
	c.state.D[register] = c.state.D[register]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	c.state.SR &^= flagExtend
	if carry {
		c.state.SR |= flagCarry | flagExtend
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2 + 2*count}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) roxlWordImmediate(register, count uint8) error {
	value := uint16(c.state.D[register])
	extend := c.state.SR&flagExtend != 0
	for i := uint8(0); i < count; i++ {
		newExtend := value&0x8000 != 0
		value <<= 1
		if extend {
			value |= 1
		}
		extend = newExtend
	}
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	c.state.SR &^= flagExtend
	if extend {
		c.state.SR |= flagCarry | flagExtend
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2 + 2*count}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveByteDataToPostincrement(source, destination uint8) error {
	value := uint8(c.state.D[source])
	c.setNZ8(value)
	if err := c.writeByte(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + addressStep(destination, WidthByte)) & addressMask
	return c.prefetch()
}

func (c *CPU) moveALongPostincrement(source, destination uint8) error {
	value, err := c.readLong(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 4) & addressMask
	c.state.A[destination] = value
	return c.prefetch()
}

func (c *CPU) moveBytePCIndexedToData(destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress((c.state.PC + 2) & addressMask)
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) addaWordData(source, destination uint8) error {
	c.state.A[destination] = uint32(int32(c.state.A[destination]) + int32(int16(c.state.D[source])))
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) addaLongData(source, destination uint8) error {
	c.state.A[destination] += c.state.D[source]
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) cmpaLongImmediate(destination uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.setCompare32(c.state.A[destination], value)
	return stream.finish()
}

func (c *CPU) moveLongPostincrementToAddressIndirect(source, destination uint8) error {
	value, err := c.readLong(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 4) & addressMask
	c.setNZ32(value)
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) subaWordImmediate(destination uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.state.A[destination] = uint32(int32(c.state.A[destination]) - int32(int16(value)))
	return stream.finish()
}

func (c *CPU) moveByteIndexedToData(base, destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress(c.state.A[base])
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) bsetDataData(bitRegister, dataRegister uint8) error {
	bit := c.state.D[bitRegister] & 31
	mask := uint32(1) << bit
	c.state.SR &^= flagZero
	if c.state.D[dataRegister]&mask == 0 {
		c.state.SR |= flagZero
	}
	c.state.D[dataRegister] |= mask
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) orWordDataToData(source, destination uint8) error {
	value := uint16(c.state.D[destination]) | uint16(c.state.D[source])
	c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	return c.prefetch()
}

func (c *CPU) subaWordData(source, destination uint8) error {
	c.state.A[destination] = uint32(int32(c.state.A[destination]) - int32(int16(c.state.D[source])))
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) subaLongAddress(source, destination uint8) error {
	c.state.A[destination] -= c.state.A[source]
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) subiWordData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	result := c.sub16(uint16(c.state.D[register]), immediate)
	c.state.D[register] = c.state.D[register]&0xffff_0000 | uint32(result)
	return stream.finish()
}

func (c *CPU) jmpPCIndexed() error {
	target := c.briefIndexedAddress((c.state.PC+2)&addressMask, c.state.IRC)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 6}); err != nil {
		return err
	}
	return c.refillPrefetch(target, 0)
}

func (c *CPU) moveBytePostincrementToPostincrement(source, destination uint8) error {
	value, err := c.readByte(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + addressStep(source, WidthByte)) & addressMask
	c.setNZ8(value)
	if err := c.writeByte(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + addressStep(destination, WidthByte)) & addressMask
	return c.prefetch()
}

func (c *CPU) moveALongPCIndexed(destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress((c.state.PC + 2) & addressMask)
	if err != nil {
		return err
	}
	value, err := c.readLong(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[destination] = value
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveByteIndexedToPostincrement(source, destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress(c.state.A[source])
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.setNZ8(value)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	if err := c.writeByte(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + addressStep(destination, WidthByte)) & addressMask
	return stream.finish()
}

func (c *CPU) moveWordPostincrementToPostincrement(source, destination uint8) error {
	value, err := c.readWord(c.state.A[source], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 2) & addressMask
	c.setNZ16(value)
	if err := c.writeWord(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + 2) & addressMask
	return c.prefetch()
}

func (c *CPU) moveAWordImmediate(destination uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.state.A[destination] = uint32(int32(int16(value)))
	return stream.finish()
}

func (c *CPU) leaPCDisplacement(destination uint8) error {
	stream := c.newInstructionStream()
	displacement, err := stream.nextWord()
	if err != nil {
		return err
	}
	c.state.A[destination] = uint32(int32((c.state.PC+2)&addressMask)+int32(int16(displacement))) & addressMask
	return stream.finish()
}

func (c *CPU) moveLongPostincrementToPostincrement(source, destination uint8) error {
	value, err := c.readLong(c.state.A[source], FCSupervisorData)
	if err != nil {
		return err
	}
	c.state.A[source] = (c.state.A[source] + 4) & addressMask
	c.setNZ32(value)
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + 4) & addressMask
	return c.prefetch()
}

func (c *CPU) rolByteImmediate(register, count uint8) error {
	value := uint8(c.state.D[register])
	var carry bool
	for i := uint8(0); i < count; i++ {
		carry = value&0x80 != 0
		value <<= 1
		if carry {
			value |= 1
		}
	}
	c.state.D[register] = c.state.D[register]&0xffff_ff00 | uint32(value)
	c.setNZ8(value)
	if carry {
		c.state.SR |= flagCarry
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2 + 2*count}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveWordPCIndexedToData(destination uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextBriefIndexedAddress((c.state.PC + 2) & addressMask)
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(value)
	c.setNZ16(value)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) cmpiByteAbsoluteLong() error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return err
	}
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readByte(address, FCSupervisorData)
	if err != nil {
		return err
	}
	c.setCompare8(value, uint8(immediate))
	return stream.finish()
}

func (c *CPU) clearLongPostincrement(register uint8) error {
	address := c.state.A[register]
	if _, err := c.readLong(address, FCSupervisorData); err != nil {
		return err
	}
	if err := c.writeLong(address, 0, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[register] = (address + 4) & addressMask
	c.setNZ32(0)
	return c.prefetch()
}

func (c *CPU) clearBytePostincrement(register uint8) error {
	address := c.state.A[register]
	if _, err := c.readByte(address, FCSupervisorData); err != nil {
		return err
	}
	if err := c.writeByte(address, 0, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[register] = (address + addressStep(register, WidthByte)) & addressMask
	c.setNZ8(0)
	return c.prefetch()
}

func (c *CPU) moveLongImmediateToPostincrement(destination uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextLong()
	if err != nil {
		return err
	}
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[destination] = (c.state.A[destination] + 4) & addressMask
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) clearWordPostincrement(register uint8) error {
	address := c.state.A[register]
	if _, err := c.readWord(address, FCSupervisorData, PhaseDataRead); err != nil {
		return err
	}
	if err := c.writeWord(address, 0, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[register] = (address + 2) & addressMask
	c.setNZ16(0)
	return c.prefetch()
}

func (c *CPU) subiLongData(register uint8) error {
	stream := c.newInstructionStream()
	immediate, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.state.D[register] = c.sub32(c.state.D[register], immediate)
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) tstWordAbsoluteLong() error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	if err != nil {
		return err
	}
	c.setNZ16(value)
	return stream.finish()
}

func (c *CPU) moveLongDataToAbsoluteLong(source uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value := c.state.D[source]
	if err := c.writeLong(address, value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) moveLongAddressToAbsoluteLong(source uint8) error {
	stream := c.newInstructionStream()
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	value := c.state.A[source]
	if err := c.writeLong(address, value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) moveLongImmediateToAbsoluteLong() error {
	stream := c.newInstructionStream()
	value, err := stream.nextLong()
	if err != nil {
		return err
	}
	address, err := stream.nextLong()
	if err != nil {
		return err
	}
	if err := c.writeLong(address, value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) addaLongImmediate(register uint8) error {
	stream := c.newInstructionStream()
	value, err := stream.nextLong()
	if err != nil {
		return err
	}
	c.state.A[register] += value
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 4}); err != nil {
		return err
	}
	return stream.finish()
}

func (c *CPU) moveWordDataToDisplacement(source, destination uint8) error {
	stream := c.newInstructionStream()
	displacement, err := stream.nextWord()
	if err != nil {
		return err
	}
	address := uint32(int32(c.state.A[destination])+int32(int16(displacement))) & addressMask
	value := uint16(c.state.D[source])
	if err := c.writeWord(address, value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ16(value)
	return stream.finish()
}

func (c *CPU) moveLongDataToDisplacement(source, destination uint8) error {
	stream := c.newInstructionStream()
	displacement, err := stream.nextWord()
	if err != nil {
		return err
	}
	address := uint32(int32(c.state.A[destination])+int32(int16(displacement))) & addressMask
	value := c.state.D[source]
	if err := c.writeLong(address, value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ32(value)
	return stream.finish()
}

func (c *CPU) extLongData(register uint8) error {
	c.state.D[register] = uint32(int32(int16(c.state.D[register])))
	c.setNZ32(c.state.D[register])
	return c.prefetch()
}

func (c *CPU) subWordDataFromData(source, destination uint8) error {
	result := c.sub16(uint16(c.state.D[destination]), uint16(c.state.D[source]))
	c.state.D[destination] = c.state.D[destination]&0xffff_0000 | uint32(result)
	return c.prefetch()
}

func (c *CPU) muluWordData(source, destination uint8) error {
	multiplier := uint16(c.state.D[source])
	c.state.D[destination] = uint32(uint16(c.state.D[destination])) * uint32(multiplier)
	c.setNZ32(c.state.D[destination])
	internal := uint8(34 + 2*bits.OnesCount16(multiplier))
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: internal}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) divuWordData(source, destination uint8) error {
	divisor := uint16(c.state.D[source])
	if divisor == 0 {
		return fmt.Errorf("divide-by-zero exception is not implemented")
	}
	dividend := c.state.D[destination]
	quotient := dividend / uint32(divisor)
	c.state.SR &^= flagNegative | flagZero | flagOverflow | flagCarry
	internal := uint8(136) // documented MC68000 worst-case 140 cycles minus final prefetch
	if quotient > 0xffff {
		c.state.SR |= flagOverflow
		internal = 6 // documented overflow path is 10 cycles for register source
	} else {
		remainder := dividend % uint32(divisor)
		c.state.D[destination] = remainder<<16 | quotient
		if uint16(quotient) == 0 {
			c.state.SR |= flagZero
		}
		if quotient&0x8000 != 0 {
			c.state.SR |= flagNegative
		}
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: internal}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveWordDataToPredecrement(source, destination uint8) error {
	c.state.A[destination] = (c.state.A[destination] - 2) & addressMask
	value := uint16(c.state.D[source])
	if err := c.writeWord(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ16(value)
	return c.prefetch()
}

func (c *CPU) lslLongImmediate(register, count uint8) error {
	value := c.state.D[register]
	var carry bool
	for range count {
		carry = value&0x8000_0000 != 0
		value <<= 1
	}
	c.state.D[register] = value
	c.setNZ32(value)
	c.state.SR &^= flagExtend
	if carry {
		c.state.SR |= flagCarry | flagExtend
	}
	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2 + 2*count}); err != nil {
		return err
	}
	return c.prefetch()
}

func (c *CPU) moveLongDataToPredecrement(source, destination uint8) error {
	value := c.state.D[source]
	c.state.A[destination] = (c.state.A[destination] - 4) & addressMask
	if err := c.writeLong(c.state.A[destination], value, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ32(value)
	return c.prefetch()
}

func (c *CPU) clearWordAddressIndirect(register uint8) error {
	address := c.state.A[register]
	if _, err := c.readWord(address, FCSupervisorData, PhaseDataRead); err != nil {
		return err
	}
	if err := c.writeWord(address, 0, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ16(0)
	return c.prefetch()
}

func (c *CPU) clearLongPredecrement(register uint8) error {
	c.state.A[register] = (c.state.A[register] - 4) & addressMask
	address := c.state.A[register]
	if _, err := c.readLong(address, FCSupervisorData); err != nil {
		return err
	}
	if err := c.writeLong(address, 0, FCSupervisorData); err != nil {
		return err
	}
	c.setNZ32(0)
	return c.prefetch()
}

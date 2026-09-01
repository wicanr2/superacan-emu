package m68k

// ABCD／SBCD／NBCD 與 ADDX／SUBX。這些指令的來源與目的都限定「兩個資料暫存器」
// 或「兩個預減位址暫存器」，位址遞減不走一般的 EA 解析路徑，因為 PRM 的時間表
// 只為整條指令算一次額外內部時間，不是每個運算元各算一次。

// bcdAdd 依 68000 規則做十進位加法：X 參與運算，Z 是累積的，C 與 X 同步。
func (c *CPU) bcdAdd(destination, source uint8) uint8 {
	extend := uint8(0)
	if c.state.SR&flagExtend != 0 {
		extend = 1
	}
	low := destination&0x0f + source&0x0f + extend
	high := uint16(destination>>4) + uint16(source>>4)
	if low > 9 {
		low += 6
		high++
	}
	if low > 0x0f {
		low &= 0x0f
	}
	carry := false
	if high > 9 {
		high += 6
		carry = true
	}
	result := uint8(high&0x0f)<<4 | low&0x0f
	c.setBCDFlags(result, carry)
	return result
}

func (c *CPU) bcdSubtract(destination, source uint8) uint8 {
	extend := uint8(0)
	if c.state.SR&flagExtend != 0 {
		extend = 1
	}
	low := int16(destination&0x0f) - int16(source&0x0f) - int16(extend)
	high := int16(destination>>4) - int16(source>>4)
	if low < 0 {
		low += 10
		high--
	}
	borrow := false
	if high < 0 {
		high += 10
		borrow = true
	}
	result := uint8(high&0x0f)<<4 | uint8(low&0x0f)
	c.setBCDFlags(result, borrow)
	return result
}

// setBCDFlags：Z 只在結果非零時被清除，因此多精度運算可以串接；
// N 取結果的最高位元，V 在 68000 上未定義，此處保持不變。
func (c *CPU) setBCDFlags(result uint8, carry bool) {
	if result != 0 {
		c.state.SR &^= flagZero
	}
	c.state.SR &^= flagNegative
	if result&0x80 != 0 {
		c.state.SR |= flagNegative
	}
	c.state.SR &^= flagCarry | flagExtend
	if carry {
		c.state.SR |= flagCarry | flagExtend
	}
}

// genericDecimal 實作 ABCD 與 SBCD 的兩種運算元形式。
func (c *CPU) genericDecimal(opcode uint16, isAdd bool) (bool, error) {
	destinationRegister := uint8(opcode >> 9 & 7)
	sourceRegister := uint8(opcode & 7)

	if opcode&0x0008 == 0 { // Dy,Dx
		source := uint8(c.state.D[sourceRegister])
		destination := uint8(c.state.D[destinationRegister])
		var result uint8
		if isAdd {
			result = c.bcdAdd(destination, source)
		} else {
			result = c.bcdSubtract(destination, source)
		}
		c.state.D[destinationRegister] = merge(c.state.D[destinationRegister], uint32(result), WidthByte)
		if err := c.internal(2); err != nil {
			return true, err
		}
		return true, c.prefetch()
	}

	// -(Ay),-(Ax)
	c.state.A[sourceRegister] = (c.state.A[sourceRegister] - operandStride(sourceRegister, WidthByte)) & addressMask
	source, err := c.readByte(c.state.A[sourceRegister], FCSupervisorData)
	if err != nil {
		return true, err
	}
	c.state.A[destinationRegister] = (c.state.A[destinationRegister] - operandStride(destinationRegister, WidthByte)) & addressMask
	destination, err := c.readByte(c.state.A[destinationRegister], FCSupervisorData)
	if err != nil {
		return true, err
	}
	var result uint8
	if isAdd {
		result = c.bcdAdd(destination, source)
	} else {
		result = c.bcdSubtract(destination, source)
	}
	if err := c.internal(2); err != nil {
		return true, err
	}
	if err := c.writeByte(c.state.A[destinationRegister], result, FCSupervisorData); err != nil {
		return true, err
	}
	return true, c.prefetch()
}

func (c *CPU) genericNegateDecimal(opcode uint16) (bool, error) {
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 1 {
		return false, nil
	}
	stream := c.newInstructionStream()
	destination, ok, err := c.resolveOperand(stream, mode, register, WidthByte)
	if err != nil || !ok {
		return ok, err
	}
	current, err := c.readOperand(destination, WidthByte)
	if err != nil {
		return true, err
	}
	result := c.bcdSubtract(0, uint8(current))
	if destination.kind == operandDataRegister {
		if err := c.internal(2); err != nil {
			return true, err
		}
	}
	if err := c.writeOperand(destination, WidthByte, uint32(result)); err != nil {
		return true, err
	}
	return true, stream.finish()
}

// genericExtendedArithmetic 實作 ADDX 與 SUBX。X 參與運算，Z 累積。
func (c *CPU) genericExtendedArithmetic(opcode uint16, isAdd bool) (bool, error) {
	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	destinationRegister := uint8(opcode >> 9 & 7)
	sourceRegister := uint8(opcode & 7)
	extend := uint32(0)
	if c.state.SR&flagExtend != 0 {
		extend = 1
	}

	if opcode&0x0008 == 0 { // Dy,Dx
		source := truncate(c.state.D[sourceRegister], size)
		destination := truncate(c.state.D[destinationRegister], size)
		result := c.extendedResult(destination, source, extend, size, isAdd)
		c.state.D[destinationRegister] = merge(c.state.D[destinationRegister], result, size)
		if size == WidthLong {
			if err := c.internal(4); err != nil {
				return true, err
			}
		}
		return true, c.prefetch()
	}

	stride := operandStride(sourceRegister, size)
	c.state.A[sourceRegister] = (c.state.A[sourceRegister] - stride) & addressMask
	source, err := c.readSized(c.state.A[sourceRegister], size)
	if err != nil {
		return true, err
	}
	c.state.A[destinationRegister] = (c.state.A[destinationRegister] - operandStride(destinationRegister, size)) & addressMask
	destination, err := c.readSized(c.state.A[destinationRegister], size)
	if err != nil {
		return true, err
	}
	result := c.extendedResult(destination, source, extend, size, isAdd)
	if err := c.internal(2); err != nil {
		return true, err
	}
	if err := c.writeSized(c.state.A[destinationRegister], result, size); err != nil {
		return true, err
	}
	return true, c.prefetch()
}

func (c *CPU) extendedResult(destination, source, extend uint32, size Width, isAdd bool) uint32 {
	zeroBefore := c.state.SR&flagZero != 0
	var result uint32
	if isAdd {
		result = c.addSized(destination, source, size)
		carry := c.state.SR&flagCarry != 0
		result = c.addSized(result, extend, size)
		if carry {
			c.state.SR |= flagCarry | flagExtend
		}
	} else {
		result = c.subSized(destination, source, size)
		borrow := c.state.SR&flagCarry != 0
		result = c.subSized(result, extend, size)
		if borrow {
			c.state.SR |= flagCarry | flagExtend
		}
	}
	if truncate(result, size) != 0 {
		c.state.SR &^= flagZero
	} else if zeroBefore {
		c.state.SR |= flagZero
	} else {
		c.state.SR &^= flagZero
	}
	return result
}

func (c *CPU) readSized(address uint32, size Width) (uint32, error) {
	switch size {
	case WidthByte:
		value, err := c.readByte(address, FCSupervisorData)
		return uint32(value), err
	case WidthLong:
		return c.readLong(address, FCSupervisorData)
	default:
		value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
		return uint32(value), err
	}
}

func (c *CPU) writeSized(address, value uint32, size Width) error {
	switch size {
	case WidthByte:
		return c.writeByte(address, uint8(value), FCSupervisorData)
	case WidthLong:
		return c.writeLong(address, value, FCSupervisorData)
	default:
		return c.writeWord(address, uint16(value), FCSupervisorData)
	}
}

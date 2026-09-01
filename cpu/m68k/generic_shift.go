package m68k

// 1110 群組：位移與旋轉。暫存器形式可位移 1–64 位，記憶體形式固定位移一位。

func (c *CPU) genericShift(opcode uint16) (bool, error) {
	if opcode>>6&3 == 3 {
		return c.genericMemoryShift(opcode)
	}
	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	register := uint8(opcode & 7)
	kind := uint8(opcode >> 3 & 3)
	left := opcode&0x0100 != 0

	var count uint8
	if opcode&0x0020 == 0 {
		count = uint8(opcode >> 9 & 7)
		if count == 0 {
			count = 8
		}
	} else {
		count = uint8(c.state.D[opcode>>9&7] & 63)
	}

	value := truncate(c.state.D[register], size)
	result := c.shiftValue(kind, left, value, count, size)
	c.state.D[register] = merge(c.state.D[register], result, size)

	base := uint8(2)
	if size == WidthLong {
		base = 4
	}
	if err := c.internal(base + 2*count); err != nil {
		return true, err
	}
	return true, c.prefetch()
}

func (c *CPU) genericMemoryShift(opcode uint16) (bool, error) {
	kind := uint8(opcode >> 9 & 3)
	if opcode>>9&7 > 3 {
		return false, nil
	}
	left := opcode&0x0100 != 0
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 0 || mode == 1 {
		return false, nil
	}

	stream := c.newInstructionStream()
	destination, ok, err := c.resolveOperand(stream, mode, register, WidthWord)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(destination, WidthWord)
	if err != nil {
		return true, err
	}
	result := c.shiftValue(kind, left, value, 1, WidthWord)
	if err := c.writeOperand(destination, WidthWord, result); err != nil {
		return true, err
	}
	return true, stream.finish()
}

// shiftValue 依 68000 規則更新 X／N／Z／V／C：位移量為零時 C 清除且 X 不變，
// 只有算術左移會設定 V。
func (c *CPU) shiftValue(kind uint8, left bool, value uint32, count uint8, size Width) uint32 {
	bits := uint8(8)
	switch size {
	case WidthWord:
		bits = 16
	case WidthLong:
		bits = 32
	}
	signBit := uint32(1) << (bits - 1)
	mask := uint32(1)<<bits - 1
	value &= mask

	if count == 0 {
		c.state.SR &^= flagCarry | flagOverflow
		c.setNZSized(value, size)
		return value
	}

	carry := false
	overflow := false
	extend := c.state.SR&flagExtend != 0

	for step := uint8(0); step < count; step++ {
		switch kind {
		case 0: // ASL／ASR
			if left {
				carry = value&signBit != 0
				before := value
				value = value << 1 & mask
				if (before^value)&signBit != 0 {
					overflow = true
				}
			} else {
				carry = value&1 != 0
				value = value>>1 | value&signBit
			}
			extend = carry
		case 1: // LSL／LSR
			if left {
				carry = value&signBit != 0
				value = value << 1 & mask
			} else {
				carry = value&1 != 0
				value >>= 1
			}
			extend = carry
		case 2: // ROXL／ROXR：X 參與旋轉
			previous := extend
			if left {
				carry = value&signBit != 0
				value = value << 1 & mask
				if previous {
					value |= 1
				}
			} else {
				carry = value&1 != 0
				value >>= 1
				if previous {
					value |= signBit
				}
			}
			extend = carry
		default: // ROL／ROR：X 不變
			if left {
				carry = value&signBit != 0
				value = value << 1 & mask
				if carry {
					value |= 1
				}
			} else {
				carry = value&1 != 0
				value >>= 1
				if carry {
					value |= signBit
				}
			}
		}
	}

	c.setNZSized(value, size)
	if carry {
		c.state.SR |= flagCarry
	}
	if overflow {
		c.state.SR |= flagOverflow
	}
	if kind != 3 { // ROL／ROR 不影響 X
		if extend {
			c.state.SR |= flagExtend
		} else {
			c.state.SR &^= flagExtend
		}
	}
	return value
}

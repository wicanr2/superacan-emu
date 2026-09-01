package m68k

// 0100 群組：單運算元、系統控制與跳躍指令。

func (c *CPU) genericMiscellaneous(opcode uint16) (bool, error) {
	switch {
	case opcode&0x01c0 == 0x01c0: // LEA <ea>,An
		return c.genericLoadEffectiveAddress(opcode)
	case opcode == 0x4e70, opcode == 0x4e72, opcode == 0x4e76:
		return false, nil // RESET／STOP／TRAPV 仍待例外路徑
	case opcode == 0x4e77:
		return c.genericReturnAndRestore()
	case opcode&0xfff0 == 0x4e40:
		return false, nil // TRAP 仍待例外路徑
	case opcode&0xfff8 == 0x4e50:
		return c.genericLink(uint8(opcode & 7))
	case opcode&0xfff8 == 0x4e58:
		return c.genericUnlink(uint8(opcode & 7))
	case opcode&0xfff0 == 0x4e60:
		return false, nil // MOVE USP 仍待 USP/SSP 分離
	case opcode&0xffc0 == 0x4e80:
		return c.genericJump(opcode, true)
	case opcode&0xffc0 == 0x4ec0:
		return c.genericJump(opcode, false)
	case opcode&0xffc0 == 0x4840:
		return c.genericPushEffectiveAddress(opcode)
	case opcode&0xfff8 == 0x4840:
		return c.genericSwap(uint8(opcode & 7))
	case opcode&0xffb8 == 0x4880:
		return c.genericExtend(opcode)
	case opcode&0xfb80 == 0x4880:
		return c.genericMoveMultiple(opcode)
	case opcode&0xffc0 == 0x4ac0:
		return c.genericTestAndSet(opcode)
	case opcode&0xff00 == 0x4a00:
		return c.genericTest(opcode)
	case opcode&0xffc0 == 0x40c0:
		return c.genericMoveFromStatus(opcode)
	case opcode&0xffc0 == 0x44c0:
		return c.genericMoveToCondition(opcode)
	case opcode&0xffc0 == 0x46c0:
		return c.genericMoveToStatus(opcode)
	case opcode&0xf000 == 0x4000 && opcode&0x0100 == 0:
		return c.genericSingleOperand(opcode)
	}
	return false, nil
}

// genericSingleOperand 處理 NEGX／CLR／NEG／NOT。
func (c *CPU) genericSingleOperand(opcode uint16) (bool, error) {
	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	operation := opcode >> 9 & 7
	if operation > 3 {
		return false, nil
	}
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 1 {
		return false, nil
	}

	stream := c.newInstructionStream()
	destination, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	current, err := c.readOperand(destination, size)
	if err != nil {
		return true, err
	}

	var result uint32
	switch operation {
	case 0: // NEGX
		extend := uint32(0)
		if c.state.SR&flagExtend != 0 {
			extend = 1
		}
		zeroBefore := c.state.SR&flagZero != 0
		result = c.subSized(0, current, size)
		result = c.subSized(result, extend, size)
		// NEGX 的 Z 是累積的：結果為零時保留先前的 Z。
		if truncate(result, size) != 0 {
			c.state.SR &^= flagZero
		} else if zeroBefore {
			c.state.SR |= flagZero
		} else {
			c.state.SR &^= flagZero
		}
	case 1: // CLR：68000 仍會先讀目的
		result = 0
		c.setNZSized(0, size)
	case 2: // NEG
		result = c.subSized(0, current, size)
	default: // NOT
		result = ^current
		c.setNZSized(result, size)
	}

	if size == WidthLong && destination.kind == operandDataRegister {
		if err := c.internal(2); err != nil {
			return true, err
		}
	}
	if err := c.writeOperand(destination, size, result); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericTest(opcode uint16) (bool, error) {
	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 1 {
		return false, nil
	}
	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(source, size)
	if err != nil {
		return true, err
	}
	c.setNZSized(value, size)
	return true, stream.finish()
}

// genericTestAndSet 以獨立的讀與寫建模 TAS 的不可分割存取；
// A'Can 沒有其他匯流排主控者會在兩者之間介入。
func (c *CPU) genericTestAndSet(opcode uint16) (bool, error) {
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
	value, err := c.readOperand(destination, WidthByte)
	if err != nil {
		return true, err
	}
	c.setNZ8(uint8(value))
	if destination.kind != operandDataRegister {
		if err := c.internal(2); err != nil {
			return true, err
		}
	}
	if err := c.writeOperand(destination, WidthByte, value|0x80); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericMoveFromStatus(opcode uint16) (bool, error) {
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 1 {
		return false, nil
	}
	stream := c.newInstructionStream()
	destination, ok, err := c.resolveOperand(stream, mode, register, WidthWord)
	if err != nil || !ok {
		return ok, err
	}
	if destination.kind == operandDataRegister {
		if err := c.internal(2); err != nil {
			return true, err
		}
	} else {
		if err := c.internal(4); err != nil {
			return true, err
		}
	}
	if err := c.writeOperand(destination, WidthWord, uint32(c.state.SR)); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericMoveToCondition(opcode uint16) (bool, error) {
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, WidthWord)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(source, WidthWord)
	if err != nil {
		return true, err
	}
	c.state.SR = c.state.SR&0xff00 | uint16(value)&0x001f
	if err := c.internal(8); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericMoveToStatus(opcode uint16) (bool, error) {
	if c.state.SR&0x2000 == 0 {
		return true, errPrivilege
	}
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, WidthWord)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(source, WidthWord)
	if err != nil {
		return true, err
	}
	c.state.SR = uint16(value)
	if err := c.internal(8); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericSwap(register uint8) (bool, error) {
	value := c.state.D[register]
	value = value>>16 | value<<16
	c.state.D[register] = value
	c.setNZ32(value)
	return true, c.prefetch()
}

func (c *CPU) genericExtend(opcode uint16) (bool, error) {
	register := uint8(opcode & 7)
	if opcode&0x0040 == 0 { // EXT.W：位元組延伸成字
		value := uint32(int32(int16(int8(c.state.D[register]))))
		c.state.D[register] = merge(c.state.D[register], value, WidthWord)
		c.setNZ16(uint16(value))
	} else { // EXT.L：字延伸成長字
		value := uint32(int32(int16(c.state.D[register])))
		c.state.D[register] = value
		c.setNZ32(value)
	}
	return true, c.prefetch()
}

func (c *CPU) genericLoadEffectiveAddress(opcode uint16) (bool, error) {
	destination := uint8(opcode >> 9 & 7)
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, WidthLong)
	if err != nil || !ok {
		return ok, err
	}
	if source.kind != operandMemory {
		return false, nil
	}
	c.state.A[destination] = source.address
	if isIndexedMode(mode, register) {
		if err := c.internal(2); err != nil {
			return true, err
		}
	}
	return true, stream.finish()
}

func (c *CPU) genericPushEffectiveAddress(opcode uint16) (bool, error) {
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, WidthLong)
	if err != nil || !ok {
		return ok, err
	}
	if source.kind != operandMemory {
		return false, nil
	}
	if isIndexedMode(mode, register) {
		if err := c.internal(2); err != nil {
			return true, err
		}
	}
	c.state.A[7] = (c.state.A[7] - 4) & addressMask
	if err := c.writeLong(c.state.A[7], source.address, FCSupervisorData); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func isIndexedMode(mode, register uint8) bool {
	return mode == 6 || (mode == 7 && register == 3)
}

func (c *CPU) genericLink(register uint8) (bool, error) {
	stream := c.newInstructionStream()
	displacement, err := stream.nextWord()
	if err != nil {
		return true, err
	}
	c.state.A[7] = (c.state.A[7] - 4) & addressMask
	if err := c.writeLong(c.state.A[7], c.state.A[register], FCSupervisorData); err != nil {
		return true, err
	}
	c.state.A[register] = c.state.A[7]
	c.state.A[7] = uint32(int32(c.state.A[7])+int32(int16(displacement))) & addressMask
	return true, stream.finish()
}

func (c *CPU) genericUnlink(register uint8) (bool, error) {
	c.state.A[7] = c.state.A[register]
	value, err := c.readLong(c.state.A[7], FCSupervisorData)
	if err != nil {
		return true, err
	}
	c.state.A[7] = (c.state.A[7] + 4) & addressMask
	c.state.A[register] = value
	return true, c.prefetch()
}

func (c *CPU) genericReturnAndRestore() (bool, error) {
	status, err := c.readWord(c.state.A[7], FCSupervisorData, PhaseDataRead)
	if err != nil {
		return true, err
	}
	target, err := c.readLong((c.state.A[7]+2)&addressMask, FCSupervisorData)
	if err != nil {
		return true, err
	}
	c.state.A[7] = (c.state.A[7] + 6) & addressMask
	c.state.SR = c.state.SR&0xff00 | status&0x001f
	return true, c.refillPrefetch(target&addressMask, 0)
}

// genericJump 實作 JSR 與 JMP。延伸字直接讀取，不經過 prefetch queue 的補位，
// 否則會產生真實硬體沒有的額外取指令週期。
func (c *CPU) genericJump(opcode uint16, isSubroutine bool) (bool, error) {
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	target, extensionWords, internal, ok, err := c.jumpTarget(mode, register)
	if err != nil || !ok {
		return ok, err
	}
	if isSubroutine {
		returnAddress := (c.state.PC + 2 + uint32(extensionWords)*2) & addressMask
		c.state.A[7] = (c.state.A[7] - 4) & addressMask
		if err := c.writeLong(c.state.A[7], returnAddress, FCSupervisorData); err != nil {
			return true, err
		}
	}
	return true, c.refillPrefetch(target, internal)
}

// jumpTarget 回傳跳躍目標、消耗的延伸字數，以及該定址模式在 PRM 表中
// 不對應任何 bus transaction 的內部時間。
func (c *CPU) jumpTarget(mode, register uint8) (uint32, uint8, uint8, bool, error) {
	switch mode {
	case 2:
		return c.state.A[register] & addressMask, 0, 0, true, nil
	case 5:
		displacement := c.state.IRC
		return uint32(int32(c.state.A[register])+int32(int16(displacement))) & addressMask, 1, 2, true, nil
	case 6:
		return c.briefIndexedAddress(c.state.A[register], c.state.IRC), 1, 6, true, nil
	case 7:
		switch register {
		case 0:
			return uint32(int32(int16(c.state.IRC))) & addressMask, 1, 2, true, nil
		case 1:
			low, err := c.readWord((c.state.PC+4)&addressMask, FCSupervisorProgram, PhaseInstructionFetch)
			if err != nil {
				return 0, 0, 0, true, err
			}
			return (uint32(c.state.IRC)<<16 | uint32(low)) & addressMask, 2, 0, true, nil
		case 2:
			base := (c.state.PC + 2) & addressMask
			return uint32(int32(base)+int32(int16(c.state.IRC))) & addressMask, 1, 2, true, nil
		case 3:
			base := (c.state.PC + 2) & addressMask
			return c.briefIndexedAddress(base, c.state.IRC), 1, 6, true, nil
		}
	}
	return 0, 0, 0, false, nil
}

// genericMoveMultiple 實作 MOVEM 的兩個方向與兩種大小。
func (c *CPU) genericMoveMultiple(opcode uint16) (bool, error) {
	size := WidthWord
	if opcode&0x0040 != 0 {
		size = WidthLong
	}
	toRegisters := opcode&0x0400 != 0
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 0 || mode == 1 {
		return false, nil
	}
	if toRegisters && mode == 4 {
		return false, nil
	}
	if !toRegisters && mode == 3 {
		return false, nil
	}

	stream := c.newInstructionStream()
	mask, err := stream.nextWord()
	if err != nil {
		return true, err
	}

	stride := uint32(2)
	if size == WidthLong {
		stride = 4
	}

	var address uint32
	switch mode {
	case 3, 2:
		address = c.state.A[register] & addressMask
	case 4:
		address = c.state.A[register] & addressMask
	default:
		resolved, ok, err := c.resolveOperand(stream, mode, register, size)
		if err != nil || !ok {
			return ok, err
		}
		if resolved.kind != operandMemory {
			return false, nil
		}
		address = resolved.address
	}

	if !toRegisters && mode == 4 { // 預減：遮罩位元順序相反，且位址先減再寫
		for bit := uint8(0); bit < 16; bit++ {
			if mask&(uint16(1)<<bit) == 0 {
				continue
			}
			address = (address - stride) & addressMask
			value := c.registerValue(15 - bit)
			if err := c.writeMultiple(address, value, size); err != nil {
				return true, err
			}
		}
		c.state.A[register] = address
		return true, stream.finish()
	}

	for bit := uint8(0); bit < 16; bit++ {
		if mask&(uint16(1)<<bit) == 0 {
			continue
		}
		if toRegisters {
			value, err := c.readMultiple(address, size)
			if err != nil {
				return true, err
			}
			c.setRegisterValue(bit, value)
		} else {
			if err := c.writeMultiple(address, c.registerValue(bit), size); err != nil {
				return true, err
			}
		}
		address = (address + stride) & addressMask
	}
	if toRegisters && mode == 3 {
		c.state.A[register] = address
	}
	if toRegisters {
		// 68000 在暫存器方向多做一次讀取；此處以等值的內部時間表示。
		if err := c.internal(4); err != nil {
			return true, err
		}
	}
	return true, stream.finish()
}

func (c *CPU) readMultiple(address uint32, size Width) (uint32, error) {
	if size == WidthLong {
		return c.readLong(address, FCSupervisorData)
	}
	value, err := c.readWord(address, FCSupervisorData, PhaseDataRead)
	// MOVEM.W 進暫存器一律符號延伸成 32 位元，位址暫存器與資料暫存器都是。
	return uint32(int32(int16(value))), err
}

func (c *CPU) writeMultiple(address, value uint32, size Width) error {
	if size == WidthLong {
		return c.writeLong(address, value, FCSupervisorData)
	}
	return c.writeWord(address, uint16(value), FCSupervisorData)
}

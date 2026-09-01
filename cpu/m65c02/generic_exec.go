package m65c02

// executeEntry 執行一條由 opcodeTable 解出的指令。進入時 PC 已越過 opcode。
func (c *CPU) executeEntry(entry instruction) error {
	switch entry.operation {
	case opNOP:
		return c.internal()
	case opCLC:
		c.state.P &^= flagCarry
		return c.internal()
	case opSEC:
		c.state.P |= flagCarry
		return c.internal()
	case opCLI:
		c.state.P &^= flagInterruptDisable
		return c.internal()
	case opSEI:
		c.state.P |= flagInterruptDisable
		return c.internal()
	case opCLV:
		c.state.P &^= flagOverflow
		return c.internal()
	case opCLD:
		c.state.P &^= flagDecimal
		return c.internal()
	case opSED:
		c.state.P |= flagDecimal
		return c.internal()
	case opWAI:
		c.waiting = true
		if err := c.internal(); err != nil {
			return err
		}
		return c.internal()

	case opINX:
		c.state.X++
		c.setNZ(c.state.X)
		return c.internal()
	case opINY:
		c.state.Y++
		c.setNZ(c.state.Y)
		return c.internal()
	case opDEX:
		c.state.X--
		c.setNZ(c.state.X)
		return c.internal()
	case opDEY:
		c.state.Y--
		c.setNZ(c.state.Y)
		return c.internal()
	case opTAX:
		c.state.X = c.state.A
		c.setNZ(c.state.X)
		return c.internal()
	case opTAY:
		c.state.Y = c.state.A
		c.setNZ(c.state.Y)
		return c.internal()
	case opTXA:
		c.state.A = c.state.X
		c.setNZ(c.state.A)
		return c.internal()
	case opTYA:
		c.state.A = c.state.Y
		c.setNZ(c.state.A)
		return c.internal()
	case opTSX:
		c.state.X = c.state.SP
		c.setNZ(c.state.X)
		return c.internal()
	case opTXS:
		// TXS 是唯一不影響旗標的傳送指令。
		c.state.SP = c.state.X
		return c.internal()

	case opPHA:
		return c.pushWithInternal(c.state.A)
	case opPHX:
		return c.pushWithInternal(c.state.X)
	case opPHY:
		return c.pushWithInternal(c.state.Y)
	case opPHP:
		return c.pushWithInternal(c.state.P | flagBreak | flagUnused)
	case opPLA:
		value, err := c.pullWithInternal()
		if err != nil {
			return err
		}
		c.state.A = value
		c.setNZ(value)
		return nil
	case opPLX:
		value, err := c.pullWithInternal()
		if err != nil {
			return err
		}
		c.state.X = value
		c.setNZ(value)
		return nil
	case opPLY:
		value, err := c.pullWithInternal()
		if err != nil {
			return err
		}
		c.state.Y = value
		c.setNZ(value)
		return nil
	case opPLP:
		value, err := c.pullWithInternal()
		if err != nil {
			return err
		}
		c.state.P = value | flagUnused
		return nil

	case opBCC:
		return c.branchRelative(c.state.P&flagCarry == 0)
	case opBCS:
		return c.branchRelative(c.state.P&flagCarry != 0)
	case opBNE:
		return c.branchRelative(c.state.P&flagZero == 0)
	case opBEQ:
		return c.branchRelative(c.state.P&flagZero != 0)
	case opBPL:
		return c.branchRelative(c.state.P&flagNegative == 0)
	case opBMI:
		return c.branchRelative(c.state.P&flagNegative != 0)
	case opBVC:
		return c.branchRelative(c.state.P&flagOverflow == 0)
	case opBVS:
		return c.branchRelative(c.state.P&flagOverflow != 0)
	case opBRA:
		return c.branchRelative(true)

	case opJMP:
		address, err := c.resolveAddress(entry.mode, false)
		if err != nil {
			return err
		}
		c.state.PC = address
		return nil
	case opJSR:
		address, err := c.resolveAddress(modeAbsolute, false)
		if err != nil {
			return err
		}
		if err := c.internal(); err != nil {
			return err
		}
		returnAddress := c.state.PC - 1
		if err := c.push(uint8(returnAddress >> 8)); err != nil {
			return err
		}
		if err := c.push(uint8(returnAddress)); err != nil {
			return err
		}
		c.state.PC = address
		return nil
	case opRTS:
		if err := c.internal(); err != nil {
			return err
		}
		if err := c.internal(); err != nil {
			return err
		}
		lo, err := c.pull()
		if err != nil {
			return err
		}
		hi, err := c.pull()
		if err != nil {
			return err
		}
		if err := c.internal(); err != nil {
			return err
		}
		c.state.PC = uint16(hi)<<8 | uint16(lo) + 1
		return nil
	case opRTI:
		if err := c.internal(); err != nil {
			return err
		}
		if err := c.internal(); err != nil {
			return err
		}
		status, err := c.pull()
		if err != nil {
			return err
		}
		lo, err := c.pull()
		if err != nil {
			return err
		}
		hi, err := c.pull()
		if err != nil {
			return err
		}
		c.state.P = status | flagUnused
		c.state.PC = uint16(hi)<<8 | uint16(lo)
		return nil
	case opBRK:
		c.state.PC++ // BRK 的簽章位元組
		if err := c.push(uint8(c.state.PC >> 8)); err != nil {
			return err
		}
		if err := c.push(uint8(c.state.PC)); err != nil {
			return err
		}
		if err := c.push(c.state.P | flagBreak | flagUnused); err != nil {
			return err
		}
		c.state.P |= flagInterruptDisable
		c.state.P &^= flagDecimal
		lo, err := c.read(0xfffe)
		if err != nil {
			return err
		}
		hi, err := c.read(0xffff)
		if err != nil {
			return err
		}
		c.state.PC = uint16(hi)<<8 | uint16(lo)
		return nil

	case opLDA, opLDX, opLDY, opADC, opSBC, opAND, opORA, opEOR, opCMP, opCPX, opCPY, opBIT:
		return c.executeRead(entry)
	case opSTA, opSTX, opSTY, opSTZ:
		return c.executeStore(entry)
	case opASL, opLSR, opROL, opROR, opINC, opDEC, opTSB, opTRB:
		return c.executeModify(entry)
	case opRMB, opSMB:
		return c.executeBitModify(entry)
	case opBBR, opBBS:
		return c.executeBitBranch(entry)
	}
	return nil
}

func (c *CPU) pushWithInternal(value uint8) error {
	if err := c.internal(); err != nil {
		return err
	}
	return c.push(value)
}

func (c *CPU) pullWithInternal() (uint8, error) {
	if err := c.internal(); err != nil {
		return 0, err
	}
	if err := c.internal(); err != nil {
		return 0, err
	}
	return c.pull()
}

func (c *CPU) branchRelative(taken bool) error {
	displacement, err := c.fetch()
	if err != nil || !taken {
		return err
	}
	origin := c.state.PC
	target := uint16(int32(origin) + int32(int8(displacement)))
	if err := c.internal(); err != nil {
		return err
	}
	if origin&0xff00 != target&0xff00 {
		if err := c.internal(); err != nil {
			return err
		}
	}
	c.state.PC = target
	return nil
}

func (c *CPU) executeRead(entry instruction) error {
	value, _, err := c.loadOperand(entry)
	if err != nil {
		return err
	}
	switch entry.operation {
	case opLDA:
		c.state.A = value
		c.setNZ(value)
	case opLDX:
		c.state.X = value
		c.setNZ(value)
	case opLDY:
		c.state.Y = value
		c.setNZ(value)
	case opAND:
		c.state.A &= value
		c.setNZ(c.state.A)
	case opORA:
		c.state.A |= value
		c.setNZ(c.state.A)
	case opEOR:
		c.state.A ^= value
		c.setNZ(c.state.A)
	case opCMP:
		c.compare(c.state.A, value)
	case opCPX:
		c.compare(c.state.X, value)
	case opCPY:
		c.compare(c.state.Y, value)
	case opADC:
		return c.adc(value)
	case opSBC:
		return c.sbc(value)
	case opBIT:
		// BIT #imm 只影響 Z；其他定址模式另外把來源的位元 7／6 複製到 N／V。
		c.state.P &^= flagZero
		if c.state.A&value == 0 {
			c.state.P |= flagZero
		}
		if entry.mode != modeImmediate {
			c.state.P &^= flagNegative | flagOverflow
			c.state.P |= value & (flagNegative | flagOverflow)
		}
	}
	return nil
}

func (c *CPU) executeStore(entry instruction) error {
	address, err := c.resolveAddress(entry.mode, true)
	if err != nil {
		return err
	}
	var value uint8
	switch entry.operation {
	case opSTA:
		value = c.state.A
	case opSTX:
		value = c.state.X
	case opSTY:
		value = c.state.Y
	}
	return c.write(address, value)
}

func (c *CPU) executeModify(entry instruction) error {
	if entry.mode == modeAccumulator {
		c.state.A = c.modifyValue(entry.operation, c.state.A)
		return c.internal()
	}
	address, err := c.resolveAddress(entry.mode, true)
	if err != nil {
		return err
	}
	value, err := c.read(address)
	if err != nil {
		return err
	}
	result := c.modifyValue(entry.operation, value)
	if err := c.internal(); err != nil {
		return err
	}
	return c.write(address, result)
}

func (c *CPU) modifyValue(op operation, value uint8) uint8 {
	switch op {
	case opASL:
		c.state.P &^= flagCarry
		if value&0x80 != 0 {
			c.state.P |= flagCarry
		}
		value <<= 1
	case opLSR:
		c.state.P &^= flagCarry
		if value&0x01 != 0 {
			c.state.P |= flagCarry
		}
		value >>= 1
	case opROL:
		carry := c.state.P & flagCarry
		c.state.P &^= flagCarry
		if value&0x80 != 0 {
			c.state.P |= flagCarry
		}
		value = value<<1 | carry
	case opROR:
		carry := c.state.P & flagCarry
		c.state.P &^= flagCarry
		if value&0x01 != 0 {
			c.state.P |= flagCarry
		}
		value = value>>1 | carry<<7
	case opINC:
		value++
	case opDEC:
		value--
	case opTSB, opTRB:
		// TSB／TRB 的 Z 來自 A 與記憶體的 AND，N 與 V 不變。
		c.state.P &^= flagZero
		if c.state.A&value == 0 {
			c.state.P |= flagZero
		}
		if op == opTSB {
			return value | c.state.A
		}
		return value &^ c.state.A
	}
	c.setNZ(value)
	return value
}

func (c *CPU) executeBitModify(entry instruction) error {
	address, err := c.resolveAddress(modeZeroPage, true)
	if err != nil {
		return err
	}
	value, err := c.read(address)
	if err != nil {
		return err
	}
	if entry.operation == opSMB {
		value |= 1 << entry.bit
	} else {
		value &^= 1 << entry.bit
	}
	if err := c.internal(); err != nil {
		return err
	}
	return c.write(address, value)
}

func (c *CPU) executeBitBranch(entry instruction) error {
	address, err := c.resolveAddress(modeZeroPage, false)
	if err != nil {
		return err
	}
	value, err := c.read(address)
	if err != nil {
		return err
	}
	if err := c.internal(); err != nil {
		return err
	}
	set := value&(1<<entry.bit) != 0
	taken := set
	if entry.operation == opBBR {
		taken = !set
	}
	return c.branchRelative(taken)
}

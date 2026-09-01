package m65c02

// 一般化的 65C02 指令表。既有的逐一 case 仍然優先，只有它不認識的編碼才進入本檔；
// 因此既有已釘住的行為與週期數不受影響。
//
// 週期模型沿用既有約定：每次 bus 存取一個 cycle，額外的內部週期明確呼叫 internal。

type addressMode uint8

const (
	modeNone addressMode = iota
	modeImplied
	modeAccumulator
	modeImmediate
	modeZeroPage
	modeZeroPageX
	modeZeroPageY
	modeAbsolute
	modeAbsoluteX
	modeAbsoluteY
	modeIndirect
	modeIndirectX
	modeIndirectY
	modeZeroPageIndirect
	modeAbsoluteIndirectX
	modeRelative
)

type operation uint8

const (
	opNone operation = iota
	opLDA
	opLDX
	opLDY
	opSTA
	opSTX
	opSTY
	opSTZ
	opADC
	opSBC
	opAND
	opORA
	opEOR
	opCMP
	opCPX
	opCPY
	opBIT
	opASL
	opLSR
	opROL
	opROR
	opINC
	opDEC
	opTSB
	opTRB
	opINX
	opINY
	opDEX
	opDEY
	opTAX
	opTAY
	opTXA
	opTYA
	opTSX
	opTXS
	opPHA
	opPHP
	opPHX
	opPHY
	opPLA
	opPLP
	opPLX
	opPLY
	opJMP
	opJSR
	opRTS
	opRTI
	opBRK
	opNOP
	opCLC
	opSEC
	opCLI
	opSEI
	opCLV
	opCLD
	opSED
	opBCC
	opBCS
	opBEQ
	opBNE
	opBMI
	opBPL
	opBVC
	opBVS
	opBRA
	opWAI
	opRMB
	opSMB
	opBBR
	opBBS
)

type instruction struct {
	operation operation
	mode      addressMode
	bit       uint8 // RMB/SMB/BBR/BBS 的位元編號
}

var opcodeTable = [256]instruction{
	0x00: {opBRK, modeImplied, 0},
	0x01: {opORA, modeIndirectX, 0}, 0x05: {opORA, modeZeroPage, 0},
	0x09: {opORA, modeImmediate, 0}, 0x0d: {opORA, modeAbsolute, 0},
	0x11: {opORA, modeIndirectY, 0}, 0x12: {opORA, modeZeroPageIndirect, 0},
	0x15: {opORA, modeZeroPageX, 0}, 0x19: {opORA, modeAbsoluteY, 0},
	0x1d: {opORA, modeAbsoluteX, 0},

	0x06: {opASL, modeZeroPage, 0}, 0x0a: {opASL, modeAccumulator, 0},
	0x0e: {opASL, modeAbsolute, 0}, 0x16: {opASL, modeZeroPageX, 0},
	0x1e: {opASL, modeAbsoluteX, 0},

	0x04: {opTSB, modeZeroPage, 0}, 0x0c: {opTSB, modeAbsolute, 0},
	0x14: {opTRB, modeZeroPage, 0}, 0x1c: {opTRB, modeAbsolute, 0},

	0x08: {opPHP, modeImplied, 0}, 0x28: {opPLP, modeImplied, 0},
	0x48: {opPHA, modeImplied, 0}, 0x68: {opPLA, modeImplied, 0},
	0x5a: {opPHY, modeImplied, 0}, 0x7a: {opPLY, modeImplied, 0},
	0xda: {opPHX, modeImplied, 0}, 0xfa: {opPLX, modeImplied, 0},

	0x10: {opBPL, modeRelative, 0}, 0x30: {opBMI, modeRelative, 0},
	0x50: {opBVC, modeRelative, 0}, 0x70: {opBVS, modeRelative, 0},
	0x80: {opBRA, modeRelative, 0}, 0x90: {opBCC, modeRelative, 0},
	0xb0: {opBCS, modeRelative, 0}, 0xd0: {opBNE, modeRelative, 0},
	0xf0: {opBEQ, modeRelative, 0},

	0x18: {opCLC, modeImplied, 0}, 0x38: {opSEC, modeImplied, 0},
	0x58: {opCLI, modeImplied, 0}, 0x78: {opSEI, modeImplied, 0},
	0xb8: {opCLV, modeImplied, 0}, 0xd8: {opCLD, modeImplied, 0},
	0xf8: {opSED, modeImplied, 0},

	0x20: {opJSR, modeAbsolute, 0}, 0x40: {opRTI, modeImplied, 0},
	0x60: {opRTS, modeImplied, 0}, 0x4c: {opJMP, modeAbsolute, 0},
	0x6c: {opJMP, modeIndirect, 0}, 0x7c: {opJMP, modeAbsoluteIndirectX, 0},

	0x21: {opAND, modeIndirectX, 0}, 0x25: {opAND, modeZeroPage, 0},
	0x29: {opAND, modeImmediate, 0}, 0x2d: {opAND, modeAbsolute, 0},
	0x31: {opAND, modeIndirectY, 0}, 0x32: {opAND, modeZeroPageIndirect, 0},
	0x35: {opAND, modeZeroPageX, 0}, 0x39: {opAND, modeAbsoluteY, 0},
	0x3d: {opAND, modeAbsoluteX, 0},

	0x24: {opBIT, modeZeroPage, 0}, 0x2c: {opBIT, modeAbsolute, 0},
	0x34: {opBIT, modeZeroPageX, 0}, 0x3c: {opBIT, modeAbsoluteX, 0},
	0x89: {opBIT, modeImmediate, 0},

	0x26: {opROL, modeZeroPage, 0}, 0x2a: {opROL, modeAccumulator, 0},
	0x2e: {opROL, modeAbsolute, 0}, 0x36: {opROL, modeZeroPageX, 0},
	0x3e: {opROL, modeAbsoluteX, 0},

	0x41: {opEOR, modeIndirectX, 0}, 0x45: {opEOR, modeZeroPage, 0},
	0x49: {opEOR, modeImmediate, 0}, 0x4d: {opEOR, modeAbsolute, 0},
	0x51: {opEOR, modeIndirectY, 0}, 0x52: {opEOR, modeZeroPageIndirect, 0},
	0x55: {opEOR, modeZeroPageX, 0}, 0x59: {opEOR, modeAbsoluteY, 0},
	0x5d: {opEOR, modeAbsoluteX, 0},

	0x46: {opLSR, modeZeroPage, 0}, 0x4a: {opLSR, modeAccumulator, 0},
	0x4e: {opLSR, modeAbsolute, 0}, 0x56: {opLSR, modeZeroPageX, 0},
	0x5e: {opLSR, modeAbsoluteX, 0},

	0x61: {opADC, modeIndirectX, 0}, 0x65: {opADC, modeZeroPage, 0},
	0x69: {opADC, modeImmediate, 0}, 0x6d: {opADC, modeAbsolute, 0},
	0x71: {opADC, modeIndirectY, 0}, 0x72: {opADC, modeZeroPageIndirect, 0},
	0x75: {opADC, modeZeroPageX, 0}, 0x79: {opADC, modeAbsoluteY, 0},
	0x7d: {opADC, modeAbsoluteX, 0},

	0x66: {opROR, modeZeroPage, 0}, 0x6a: {opROR, modeAccumulator, 0},
	0x6e: {opROR, modeAbsolute, 0}, 0x76: {opROR, modeZeroPageX, 0},
	0x7e: {opROR, modeAbsoluteX, 0},

	0x81: {opSTA, modeIndirectX, 0}, 0x85: {opSTA, modeZeroPage, 0},
	0x8d: {opSTA, modeAbsolute, 0}, 0x91: {opSTA, modeIndirectY, 0},
	0x92: {opSTA, modeZeroPageIndirect, 0}, 0x95: {opSTA, modeZeroPageX, 0},
	0x99: {opSTA, modeAbsoluteY, 0}, 0x9d: {opSTA, modeAbsoluteX, 0},

	0x84: {opSTY, modeZeroPage, 0}, 0x8c: {opSTY, modeAbsolute, 0},
	0x94: {opSTY, modeZeroPageX, 0},
	0x86: {opSTX, modeZeroPage, 0}, 0x8e: {opSTX, modeAbsolute, 0},
	0x96: {opSTX, modeZeroPageY, 0},
	0x64: {opSTZ, modeZeroPage, 0}, 0x74: {opSTZ, modeZeroPageX, 0},
	0x9c: {opSTZ, modeAbsolute, 0}, 0x9e: {opSTZ, modeAbsoluteX, 0},

	0x88: {opDEY, modeImplied, 0}, 0x8a: {opTXA, modeImplied, 0},
	0x98: {opTYA, modeImplied, 0}, 0x9a: {opTXS, modeImplied, 0},
	0xa8: {opTAY, modeImplied, 0}, 0xaa: {opTAX, modeImplied, 0},
	0xba: {opTSX, modeImplied, 0}, 0xc8: {opINY, modeImplied, 0},
	0xca: {opDEX, modeImplied, 0}, 0xe8: {opINX, modeImplied, 0},
	0x1a: {opINC, modeAccumulator, 0}, 0x3a: {opDEC, modeAccumulator, 0},
	0xea: {opNOP, modeImplied, 0}, 0xcb: {opWAI, modeImplied, 0},

	0xa0: {opLDY, modeImmediate, 0}, 0xa4: {opLDY, modeZeroPage, 0},
	0xac: {opLDY, modeAbsolute, 0}, 0xb4: {opLDY, modeZeroPageX, 0},
	0xbc: {opLDY, modeAbsoluteX, 0},
	0xa2: {opLDX, modeImmediate, 0}, 0xa6: {opLDX, modeZeroPage, 0},
	0xae: {opLDX, modeAbsolute, 0}, 0xb6: {opLDX, modeZeroPageY, 0},
	0xbe: {opLDX, modeAbsoluteY, 0},
	0xa1: {opLDA, modeIndirectX, 0}, 0xa5: {opLDA, modeZeroPage, 0},
	0xa9: {opLDA, modeImmediate, 0}, 0xad: {opLDA, modeAbsolute, 0},
	0xb1: {opLDA, modeIndirectY, 0}, 0xb2: {opLDA, modeZeroPageIndirect, 0},
	0xb5: {opLDA, modeZeroPageX, 0}, 0xb9: {opLDA, modeAbsoluteY, 0},
	0xbd: {opLDA, modeAbsoluteX, 0},

	0xc0: {opCPY, modeImmediate, 0}, 0xc4: {opCPY, modeZeroPage, 0},
	0xcc: {opCPY, modeAbsolute, 0},
	0xe0: {opCPX, modeImmediate, 0}, 0xe4: {opCPX, modeZeroPage, 0},
	0xec: {opCPX, modeAbsolute, 0},
	0xc1: {opCMP, modeIndirectX, 0}, 0xc5: {opCMP, modeZeroPage, 0},
	0xc9: {opCMP, modeImmediate, 0}, 0xcd: {opCMP, modeAbsolute, 0},
	0xd1: {opCMP, modeIndirectY, 0}, 0xd2: {opCMP, modeZeroPageIndirect, 0},
	0xd5: {opCMP, modeZeroPageX, 0}, 0xd9: {opCMP, modeAbsoluteY, 0},
	0xdd: {opCMP, modeAbsoluteX, 0},

	0xc6: {opDEC, modeZeroPage, 0}, 0xce: {opDEC, modeAbsolute, 0},
	0xd6: {opDEC, modeZeroPageX, 0}, 0xde: {opDEC, modeAbsoluteX, 0},
	0xe6: {opINC, modeZeroPage, 0}, 0xee: {opINC, modeAbsolute, 0},
	0xf6: {opINC, modeZeroPageX, 0}, 0xfe: {opINC, modeAbsoluteX, 0},

	0xe1: {opSBC, modeIndirectX, 0}, 0xe5: {opSBC, modeZeroPage, 0},
	0xe9: {opSBC, modeImmediate, 0}, 0xed: {opSBC, modeAbsolute, 0},
	0xf1: {opSBC, modeIndirectY, 0}, 0xf2: {opSBC, modeZeroPageIndirect, 0},
	0xf5: {opSBC, modeZeroPageX, 0}, 0xf9: {opSBC, modeAbsoluteY, 0},
	0xfd: {opSBC, modeAbsoluteX, 0},

	0x07: {opRMB, modeZeroPage, 0}, 0x17: {opRMB, modeZeroPage, 1},
	0x27: {opRMB, modeZeroPage, 2}, 0x37: {opRMB, modeZeroPage, 3},
	0x47: {opRMB, modeZeroPage, 4}, 0x57: {opRMB, modeZeroPage, 5},
	0x67: {opRMB, modeZeroPage, 6}, 0x77: {opRMB, modeZeroPage, 7},
	0x87: {opSMB, modeZeroPage, 0}, 0x97: {opSMB, modeZeroPage, 1},
	0xa7: {opSMB, modeZeroPage, 2}, 0xb7: {opSMB, modeZeroPage, 3},
	0xc7: {opSMB, modeZeroPage, 4}, 0xd7: {opSMB, modeZeroPage, 5},
	0xe7: {opSMB, modeZeroPage, 6}, 0xf7: {opSMB, modeZeroPage, 7},
	0x0f: {opBBR, modeZeroPage, 0}, 0x1f: {opBBR, modeZeroPage, 1},
	0x2f: {opBBR, modeZeroPage, 2}, 0x3f: {opBBR, modeZeroPage, 3},
	0x4f: {opBBR, modeZeroPage, 4}, 0x5f: {opBBR, modeZeroPage, 5},
	0x6f: {opBBR, modeZeroPage, 6}, 0x7f: {opBBR, modeZeroPage, 7},
	0x8f: {opBBS, modeZeroPage, 0}, 0x9f: {opBBS, modeZeroPage, 1},
	0xaf: {opBBS, modeZeroPage, 2}, 0xbf: {opBBS, modeZeroPage, 3},
	0xcf: {opBBS, modeZeroPage, 4}, 0xdf: {opBBS, modeZeroPage, 5},
	0xef: {opBBS, modeZeroPage, 6}, 0xff: {opBBS, modeZeroPage, 7},
}

// executeGeneric 回傳 handled=false 表示表裡沒有這個編碼，由呼叫端 fail-closed。
func (c *CPU) executeGeneric(opcode uint8) (bool, error) {
	entry := opcodeTable[opcode]
	if entry.operation == opNone {
		undefined, ok := undefinedOpcode(opcode)
		if !ok {
			return false, nil
		}
		c.state.PC++
		return true, c.executeUndefined(undefined)
	}
	c.state.PC++
	err := c.executeEntry(entry)
	return true, err
}

// isWriteOperation 決定索引定址是否一律付出額外週期：寫入與讀改寫不能靠
// 「沒有跨頁就省一個週期」的快捷路徑。
func isWriteOperation(op operation) bool {
	switch op {
	case opSTA, opSTX, opSTY, opSTZ, opASL, opLSR, opROL, opROR, opINC, opDEC, opTSB, opTRB:
		return true
	}
	return false
}

func (c *CPU) resolveAddress(mode addressMode, write bool) (uint16, error) {
	switch mode {
	case modeZeroPage:
		value, err := c.fetch()
		return uint16(value), err
	case modeZeroPageX:
		value, err := c.fetch()
		if err != nil {
			return 0, err
		}
		if err := c.internal(); err != nil {
			return 0, err
		}
		return uint16(value + c.state.X), nil
	case modeZeroPageY:
		value, err := c.fetch()
		if err != nil {
			return 0, err
		}
		if err := c.internal(); err != nil {
			return 0, err
		}
		return uint16(value + c.state.Y), nil
	case modeAbsolute:
		return c.fetchWord()
	case modeAbsoluteX, modeAbsoluteY:
		base, err := c.fetchWord()
		if err != nil {
			return 0, err
		}
		index := uint16(c.state.X)
		if mode == modeAbsoluteY {
			index = uint16(c.state.Y)
		}
		target := base + index
		if write || base&0xff00 != target&0xff00 {
			if err := c.internal(); err != nil {
				return 0, err
			}
		}
		return target, nil
	case modeIndirectX:
		pointer, err := c.fetch()
		if err != nil {
			return 0, err
		}
		if err := c.internal(); err != nil {
			return 0, err
		}
		return c.readPointer(uint8(pointer + c.state.X))
	case modeIndirectY:
		pointer, err := c.fetch()
		if err != nil {
			return 0, err
		}
		base, err := c.readPointer(pointer)
		if err != nil {
			return 0, err
		}
		target := base + uint16(c.state.Y)
		if write || base&0xff00 != target&0xff00 {
			if err := c.internal(); err != nil {
				return 0, err
			}
		}
		return target, nil
	case modeZeroPageIndirect:
		pointer, err := c.fetch()
		if err != nil {
			return 0, err
		}
		return c.readPointer(pointer)
	case modeIndirect:
		pointer, err := c.fetchWord()
		if err != nil {
			return 0, err
		}
		// 65C02 修正了 NMOS 的頁邊界錯誤，指標可以跨頁。
		lo, err := c.read(pointer)
		if err != nil {
			return 0, err
		}
		hi, err := c.read(pointer + 1)
		return uint16(hi)<<8 | uint16(lo), err
	case modeAbsoluteIndirectX:
		pointer, err := c.fetchWord()
		if err != nil {
			return 0, err
		}
		pointer += uint16(c.state.X)
		if err := c.internal(); err != nil {
			return 0, err
		}
		lo, err := c.read(pointer)
		if err != nil {
			return 0, err
		}
		hi, err := c.read(pointer + 1)
		return uint16(hi)<<8 | uint16(lo), err
	}
	return 0, nil
}

func (c *CPU) fetchWord() (uint16, error) {
	lo, err := c.fetch()
	if err != nil {
		return 0, err
	}
	hi, err := c.fetch()
	return uint16(hi)<<8 | uint16(lo), err
}

// readPointer 讀取零頁指標；高位元組的位址在零頁內回繞。
func (c *CPU) readPointer(pointer uint8) (uint16, error) {
	lo, err := c.read(uint16(pointer))
	if err != nil {
		return 0, err
	}
	hi, err := c.read(uint16(pointer + 1))
	return uint16(hi)<<8 | uint16(lo), err
}

func (c *CPU) loadOperand(entry instruction) (uint8, uint16, error) {
	if entry.mode == modeImmediate {
		value, err := c.fetch()
		return value, 0, err
	}
	address, err := c.resolveAddress(entry.mode, isWriteOperation(entry.operation))
	if err != nil {
		return 0, 0, err
	}
	value, err := c.read(address)
	return value, address, err
}

// undefinedNOP 描述 W65C02S 上未指派的編碼：它們不是非法指令，而是有明確
// 長度與週期數的 NOP。cycles 含 opcode 取指令的那一個週期。
type undefinedNOP struct {
	bytes  uint8
	cycles uint8
}

// 依 W65C02S 資料手冊的未指派編碼分組。$CB（WAI）與 $DB（STP）已另有定義，
// 不屬於此表。
func undefinedOpcode(opcode uint8) (undefinedNOP, bool) {
	switch opcode {
	case 0x02, 0x22, 0x42, 0x62, 0x82, 0xc2, 0xe2:
		return undefinedNOP{bytes: 2, cycles: 2}, true
	case 0x44:
		return undefinedNOP{bytes: 2, cycles: 3}, true
	case 0x54, 0xd4, 0xf4:
		return undefinedNOP{bytes: 2, cycles: 4}, true
	case 0x5c:
		return undefinedNOP{bytes: 3, cycles: 8}, true
	case 0xdc, 0xfc:
		return undefinedNOP{bytes: 3, cycles: 4}, true
	case 0xdb: // STP：停機需要外部 reset，維持 fail-closed
		return undefinedNOP{}, false
	}
	if opcode&0x0f == 0x03 || opcode&0x0f == 0x0b {
		return undefinedNOP{bytes: 1, cycles: 1}, true
	}
	return undefinedNOP{}, false
}

func (c *CPU) executeUndefined(entry undefinedNOP) error {
	// opcode 取指令已經計過一個週期；其餘的位元組照樣讀入，剩下的補內部週期。
	for index := uint8(1); index < entry.bytes; index++ {
		if _, err := c.fetch(); err != nil {
			return err
		}
	}
	for index := entry.bytes; index < entry.cycles; index++ {
		if err := c.internal(); err != nil {
			return err
		}
	}
	return nil
}

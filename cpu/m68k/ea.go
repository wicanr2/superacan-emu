package m68k

// 本檔是 68000 定址模式的一般化實作。原本的 decoder 為每一組
// 「操作 × 大小 × 定址模式」寫一個 case，每遇到新組合就要新增一條；
// 這裡改成先把 mode/register 解析成 operand，再由指令族共用讀寫路徑。
//
// 時間模型沿用既有約定：每次 bus transaction 由 readWord/writeWord 自動計 4 cycle，
// 額外的內部 cycle 由呼叫端明確 advance。因此本檔只負責定址模式本身的內部 cycle
// （預減 2、brief-indexed 2），指令族的基礎 cycle 由 generic.go 補。

type operandKind uint8

const (
	operandDataRegister operandKind = iota
	operandAddressRegister
	operandMemory
	operandImmediate
)

// operand 是已解析的來源或目的。解析時 (An)+／-(An) 的副作用已經套用一次，
// 之後的讀寫不得再改動位址暫存器。
type operand struct {
	kind      operandKind
	register  uint8
	address   uint32
	immediate uint32
}

// operandStride 是 (An)+／-(An) 的位址增減量。位元組存取用 A7 時是 2，
// 因為堆疊指標必須保持偶數。
func operandStride(register uint8, size Width) uint32 {
	switch size {
	case WidthByte:
		if register == 7 {
			return 2
		}
		return 1
	case WidthLong:
		return 4
	default:
		return 2
	}
}

// currentAddress 是下一個 nextWord 會回傳的延伸字所在位址，
// 也就是 PC 相對定址的基底。
func (s *instructionStream) currentAddress() uint32 {
	return (s.nextAddress - 2) & addressMask
}

// resolveOperand 解析一個 68000 effective address。mode 7 的 register 欄位再細分
// 絕對定址、PC 相對與立即值；不支援的組合回傳 false 讓呼叫端 fail-closed。
//
// 位址暫存器與計算出來的位址一律保留完整 32 位元：68000 的 An 是 32 位元暫存器，
// 只有位址匯流排是 24 位元，遮罩發生在 readWord／writeWord。若在這裡就截成
// 24 位元，`CMPA.L #$FFFFA122,A5` 這類與 Work RAM 高位址比較的迴圈永遠不會結束。
func (c *CPU) resolveOperand(stream *instructionStream, mode, register uint8, size Width) (operand, bool, error) {
	switch mode {
	case 0:
		return operand{kind: operandDataRegister, register: register}, true, nil
	case 1:
		return operand{kind: operandAddressRegister, register: register}, true, nil
	case 2:
		return operand{kind: operandMemory, address: c.state.A[register]}, true, nil
	case 3:
		address := c.state.A[register]
		c.state.A[register] += operandStride(register, size)
		return operand{kind: operandMemory, address: address}, true, nil
	case 4:
		if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
			return operand{}, false, err
		}
		c.state.A[register] -= operandStride(register, size)
		return operand{kind: operandMemory, address: c.state.A[register]}, true, nil
	case 5:
		displacement, err := stream.nextWord()
		if err != nil {
			return operand{}, false, err
		}
		address := uint32(int32(c.state.A[register]) + int32(int16(displacement)))
		return operand{kind: operandMemory, address: address}, true, nil
	case 6:
		address, err := stream.nextBriefIndexedAddress(c.state.A[register])
		if err != nil {
			return operand{}, false, err
		}
		if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
			return operand{}, false, err
		}
		return operand{kind: operandMemory, address: address}, true, nil
	case 7:
		switch register {
		case 0: // (xxx).W：延伸字帶正負號
			word, err := stream.nextWord()
			if err != nil {
				return operand{}, false, err
			}
			return operand{kind: operandMemory, address: uint32(int32(int16(word)))}, true, nil
		case 1: // (xxx).L
			address, err := stream.nextLong()
			if err != nil {
				return operand{}, false, err
			}
			return operand{kind: operandMemory, address: address}, true, nil
		case 2: // (d16,PC)
			base := stream.currentAddress()
			displacement, err := stream.nextWord()
			if err != nil {
				return operand{}, false, err
			}
			address := uint32(int32(base) + int32(int16(displacement)))
			return operand{kind: operandMemory, address: address}, true, nil
		case 3: // (d8,PC,Xn)
			base := stream.currentAddress()
			address, err := stream.nextBriefIndexedAddress(base)
			if err != nil {
				return operand{}, false, err
			}
			if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 2}); err != nil {
				return operand{}, false, err
			}
			return operand{kind: operandMemory, address: address}, true, nil
		case 4: // #imm
			switch size {
			case WidthLong:
				value, err := stream.nextLong()
				if err != nil {
					return operand{}, false, err
				}
				return operand{kind: operandImmediate, immediate: value}, true, nil
			case WidthByte:
				word, err := stream.nextWord()
				if err != nil {
					return operand{}, false, err
				}
				return operand{kind: operandImmediate, immediate: uint32(word & 0xff)}, true, nil
			default:
				word, err := stream.nextWord()
				if err != nil {
					return operand{}, false, err
				}
				return operand{kind: operandImmediate, immediate: uint32(word)}, true, nil
			}
		}
	}
	return operand{}, false, nil
}

// readOperand 取出 operand 目前的值，記憶體存取會產生對應的 bus transaction。
func (c *CPU) readOperand(source operand, size Width) (uint32, error) {
	switch source.kind {
	case operandDataRegister:
		return truncate(c.state.D[source.register], size), nil
	case operandAddressRegister:
		return truncate(c.state.A[source.register], size), nil
	case operandImmediate:
		return truncate(source.immediate, size), nil
	}
	switch size {
	case WidthByte:
		value, err := c.readByte(source.address, FCSupervisorData)
		return uint32(value), err
	case WidthLong:
		return c.readLong(source.address, FCSupervisorData)
	default:
		value, err := c.readWord(source.address, FCSupervisorData, PhaseDataRead)
		return uint32(value), err
	}
}

// writeOperand 寫回 operand。暫存器目的只改動對應寬度的位元，
// 位址暫存器目的則一律做符號延伸成 32 位元（68000 的 An 沒有部分寫入）。
func (c *CPU) writeOperand(destination operand, size Width, value uint32) error {
	switch destination.kind {
	case operandDataRegister:
		c.state.D[destination.register] = merge(c.state.D[destination.register], value, size)
		return nil
	case operandAddressRegister:
		c.state.A[destination.register] = signExtend(value, size)
		return nil
	case operandImmediate:
		return errUnsupportedOperand
	}
	switch size {
	case WidthByte:
		return c.writeByte(destination.address, uint8(value), FCSupervisorData)
	case WidthLong:
		return c.writeLong(destination.address, value, FCSupervisorData)
	default:
		return c.writeWord(destination.address, uint16(value), FCSupervisorData)
	}
}

func truncate(value uint32, size Width) uint32 {
	switch size {
	case WidthByte:
		return value & 0xff
	case WidthWord:
		return value & 0xffff
	default:
		return value
	}
}

func merge(original, value uint32, size Width) uint32 {
	switch size {
	case WidthByte:
		return original&0xffff_ff00 | value&0xff
	case WidthWord:
		return original&0xffff_0000 | value&0xffff
	default:
		return value
	}
}

func signExtend(value uint32, size Width) uint32 {
	switch size {
	case WidthByte:
		return uint32(int32(int8(value)))
	case WidthWord:
		return uint32(int32(int16(value)))
	default:
		return value
	}
}

// sizeFromField 解析標準的 bits 7-6 大小欄位；11 在多數指令族是非法值。
func sizeFromField(field uint16) (Width, bool) {
	switch field {
	case 0:
		return WidthByte, true
	case 1:
		return WidthWord, true
	case 2:
		return WidthLong, true
	default:
		return WidthNone, false
	}
}

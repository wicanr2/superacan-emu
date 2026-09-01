package m68k

import "errors"

// 一般化指令執行層。既有的逐一 case decoder 仍然優先，只有它判定為
// InstructionIllegal 時才進入本檔；因此既有已釘住的行為與時序不受影響。
//
// 時序來源是 M68000 Programmer's Reference Manual 的指令時間表：
// 本檔在自動計費的 bus transaction（每次 4 cycle）之外，補上該表要求的內部 cycle。
// 這些值尚未與 Moira 做逐指令差分，屬 `strong-inference`，不是實機量測。

var (
	errUnsupportedOperand = errors.New("m68k: operand not writable")
	errPrivilege          = errors.New("m68k: privileged instruction in user mode")
)

// executeGeneric 回傳 handled=false 表示本層不認識這個 opcode，
// 由呼叫端 fail-closed，不得靜默當成 NOP。
func (c *CPU) executeGeneric(opcode uint16) (bool, error) {
	switch opcode >> 12 {
	case 0x0:
		return c.genericImmediateAndBit(opcode)
	case 0x1, 0x2, 0x3:
		return c.genericMove(opcode)
	case 0x4:
		return c.genericMiscellaneous(opcode)
	case 0x5:
		return c.genericQuickConditional(opcode)
	case 0x8:
		return c.genericOr(opcode)
	case 0x9:
		return c.genericSub(opcode)
	case 0xb:
		return c.genericCompareEor(opcode)
	case 0xc:
		return c.genericAnd(opcode)
	case 0xd:
		return c.genericAdd(opcode)
	case 0xe:
		return c.genericShift(opcode)
	}
	return false, nil
}

func (c *CPU) internal(cycles uint8) error {
	if cycles == 0 {
		return nil
	}
	return c.advance(Phase{Kind: PhaseInternal, Cycles: cycles})
}

func (c *CPU) addSized(destination, source uint32, size Width) uint32 {
	switch size {
	case WidthByte:
		return uint32(c.add8(uint8(destination), uint8(source)))
	case WidthLong:
		return c.add32(destination, source)
	default:
		return uint32(c.add16(uint16(destination), uint16(source)))
	}
}

func (c *CPU) subSized(destination, source uint32, size Width) uint32 {
	switch size {
	case WidthByte:
		return uint32(c.sub8(uint8(destination), uint8(source)))
	case WidthLong:
		return c.sub32(destination, source)
	default:
		return uint32(c.sub16(uint16(destination), uint16(source)))
	}
}

func (c *CPU) compareSized(destination, source uint32, size Width) {
	switch size {
	case WidthByte:
		c.setCompare8(uint8(destination), uint8(source))
	case WidthLong:
		c.setCompare32(destination, source)
	default:
		c.setCompare16(uint16(destination), uint16(source))
	}
}

func (c *CPU) setNZSized(value uint32, size Width) {
	switch size {
	case WidthByte:
		c.setNZ8(uint8(value))
	case WidthLong:
		c.setNZ32(value)
	default:
		c.setNZ16(uint16(value))
	}
}

func isRegisterOrImmediate(source operand) bool {
	return source.kind != operandMemory
}

// longSourceInternal 是 ADD／SUB／AND／OR 的 long 形式在
// 「<ea> → Dn」方向的額外內部 cycle：暫存器或立即值來源多 2 cycle。
func longSourceInternal(source operand) uint8 {
	if isRegisterOrImmediate(source) {
		return 4
	}
	return 2
}

// --- MOVE / MOVEA -----------------------------------------------------------

func (c *CPU) genericMove(opcode uint16) (bool, error) {
	var size Width
	switch opcode >> 12 {
	case 0x1:
		size = WidthByte
	case 0x3:
		size = WidthWord
	default:
		size = WidthLong
	}
	destinationMode := uint8(opcode >> 6 & 7)
	destinationRegister := uint8(opcode >> 9 & 7)
	sourceMode := uint8(opcode >> 3 & 7)
	sourceRegister := uint8(opcode & 7)
	if size == WidthByte && sourceMode == 1 {
		return false, nil // MOVE.B 不能以位址暫存器為來源
	}

	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, sourceMode, sourceRegister, size)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(source, size)
	if err != nil {
		return true, err
	}

	if destinationMode == 1 { // MOVEA：不影響旗標，一律符號延伸成 32 位元
		if size == WidthByte {
			return false, nil
		}
		c.state.A[destinationRegister] = signExtend(value, size)
		return true, stream.finish()
	}

	// MOVE 的目的端預減不收 EA 計算的 2 cycle：位址計算與寫入重疊。
	var destination operand
	if destinationMode == 4 {
		c.state.A[destinationRegister] = (c.state.A[destinationRegister] - operandStride(destinationRegister, size)) & addressMask
		destination = operand{kind: operandMemory, address: c.state.A[destinationRegister] & addressMask}
	} else {
		destination, ok, err = c.resolveOperand(stream, destinationMode, destinationRegister, size)
		if err != nil || !ok {
			return ok, err
		}
	}
	c.setNZSized(value, size)
	if err := c.writeOperand(destination, size, value); err != nil {
		return true, err
	}
	return true, stream.finish()
}

// --- 立即值與位元操作 -------------------------------------------------------

func (c *CPU) genericImmediateAndBit(opcode uint16) (bool, error) {
	if opcode&0x0138 == 0x0108 {
		return false, nil // MOVEP 尚未實作，維持 fail-closed
	}
	if opcode&0x0100 != 0 { // 動態位元操作：位元編號來自 Dn
		return c.genericBitOperation(opcode, true)
	}
	if opcode&0x0f00 == 0x0800 { // 靜態位元操作：位元編號來自立即字
		return c.genericBitOperation(opcode, false)
	}

	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	operation := opcode >> 9 & 7

	// ORI/ANDI/EORI to CCR 與 to SR 的 <ea> 欄位固定是立即值。
	if mode == 7 && register == 4 && (operation == 0 || operation == 1 || operation == 5) {
		return c.genericImmediateToStatus(opcode, operation, size)
	}

	stream := c.newInstructionStream()
	immediate, ok, err := c.resolveOperand(stream, 7, 4, size)
	if err != nil || !ok {
		return ok, err
	}
	destination, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	if destination.kind == operandAddressRegister {
		return false, nil
	}
	current, err := c.readOperand(destination, size)
	if err != nil {
		return true, err
	}

	var result uint32
	compareOnly := false
	switch operation {
	case 0: // ORI
		result = current | immediate.immediate
		c.setNZSized(result, size)
	case 1: // ANDI
		result = current & immediate.immediate
		c.setNZSized(result, size)
	case 2: // SUBI
		result = c.subSized(current, immediate.immediate, size)
	case 3: // ADDI
		result = c.addSized(current, immediate.immediate, size)
	case 5: // EORI
		result = current ^ immediate.immediate
		c.setNZSized(result, size)
	case 6: // CMPI
		c.compareSized(current, immediate.immediate, size)
		compareOnly = true
	default:
		return false, nil
	}

	if err := c.immediateInternalCycles(operation, size, destination); err != nil {
		return true, err
	}
	if !compareOnly {
		if err := c.writeOperand(destination, size, result); err != nil {
			return true, err
		}
	}
	return true, stream.finish()
}

// immediateInternalCycles 補上 PRM 表要求、但不對應任何 bus transaction 的時間。
func (c *CPU) immediateInternalCycles(operation uint16, size Width, destination operand) error {
	if size != WidthLong || destination.kind != operandDataRegister {
		return nil
	}
	if operation == 6 { // CMPI.L Dn 為 14 cycle
		return c.internal(2)
	}
	return c.internal(4)
}

func (c *CPU) genericImmediateToStatus(opcode uint16, operation uint16, size Width) (bool, error) {
	if size == WidthLong {
		return false, nil
	}
	if size == WidthWord && c.state.SR&0x2000 == 0 {
		return true, errPrivilege
	}
	stream := c.newInstructionStream()
	immediate, err := stream.nextWord()
	if err != nil {
		return true, err
	}
	mask := uint16(0x001f)
	if size == WidthWord {
		mask = 0xffff
	}
	value := immediate & mask
	switch operation {
	case 0:
		c.state.SR |= value
	case 1:
		c.state.SR &= value | ^mask
	case 5:
		c.state.SR ^= value
	}
	if err := c.internal(12); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericBitOperation(opcode uint16, dynamic bool) (bool, error) {
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	operation := opcode >> 6 & 3
	if mode == 1 {
		return false, nil
	}
	size := WidthByte
	if mode == 0 {
		size = WidthLong
	}

	stream := c.newInstructionStream()
	var bit uint32
	if dynamic {
		bit = c.state.D[opcode>>9&7]
	} else {
		word, err := stream.nextWord()
		if err != nil {
			return true, err
		}
		bit = uint32(word)
	}
	if size == WidthLong {
		bit &= 31
	} else {
		bit &= 7
	}

	destination, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	current, err := c.readOperand(destination, size)
	if err != nil {
		return true, err
	}

	mask := uint32(1) << bit
	c.state.SR &^= flagZero
	if current&mask == 0 {
		c.state.SR |= flagZero
	}

	if err := c.internal(bitOperationInternal(operation, destination.kind == operandDataRegister)); err != nil {
		return true, err
	}
	if operation != 0 {
		var result uint32
		switch operation {
		case 1:
			result = current ^ mask
		case 2:
			result = current &^ mask
		default:
			result = current | mask
		}
		if err := c.writeOperand(destination, size, result); err != nil {
			return true, err
		}
	}
	return true, stream.finish()
}

func bitOperationInternal(operation uint16, dataRegister bool) uint8 {
	if !dataRegister {
		return 0
	}
	switch operation {
	case 0: // BTST
		return 2
	case 2: // BCLR 多一個內部循環
		return 6
	default: // BCHG／BSET
		return 4
	}
}

// --- ADDQ／SUBQ／Scc／DBcc ---------------------------------------------------

func (c *CPU) genericQuickConditional(opcode uint16) (bool, error) {
	if opcode&0x00c0 == 0x00c0 {
		if opcode&0x0038 == 0x0008 {
			return false, nil // DBcc 已由既有 decoder 處理
		}
		return c.genericScc(opcode)
	}
	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	quick := uint32(opcode >> 9 & 7)
	if quick == 0 {
		quick = 8
	}
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)

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
	if destination.kind == operandAddressRegister {
		// 位址暫存器目的不影響旗標，且一律以 long 運算。
		if size == WidthByte {
			return false, nil
		}
		if opcode&0x0100 == 0 {
			result = c.state.A[register] + quick
		} else {
			result = c.state.A[register] - quick
		}
		c.state.A[register] = result & addressMask
		if err := c.internal(4); err != nil {
			return true, err
		}
		return true, stream.finish()
	}
	if opcode&0x0100 == 0 {
		result = c.addSized(current, quick, size)
	} else {
		result = c.subSized(current, quick, size)
	}
	if size == WidthLong && destination.kind == operandDataRegister {
		if err := c.internal(4); err != nil {
			return true, err
		}
	}
	if err := c.writeOperand(destination, size, result); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericScc(opcode uint16) (bool, error) {
	condition := uint8(opcode >> 8 & 0x0f)
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
	value := uint32(0)
	if conditionTrue(condition, c.state.SR) {
		value = 0xff
	}
	if destination.kind == operandDataRegister {
		// Scc Dn 為 4 cycle，條件成立時 6。
		if value != 0 {
			if err := c.internal(2); err != nil {
				return true, err
			}
		}
	}
	if err := c.writeOperand(destination, WidthByte, value); err != nil {
		return true, err
	}
	return true, stream.finish()
}

// --- OR／DIVU／DIVS／SBCD ----------------------------------------------------

func (c *CPU) genericOr(opcode uint16) (bool, error) {
	switch opcode & 0x01c0 {
	case 0x00c0:
		return c.genericDivide(opcode, false)
	case 0x01c0:
		return c.genericDivide(opcode, true)
	}
	if opcode&0x01f0 == 0x0100 {
		return false, nil // SBCD 尚未實作
	}
	return c.genericBinary(opcode, binaryOr)
}

// --- SUB／SUBA／SUBX ---------------------------------------------------------

func (c *CPU) genericSub(opcode uint16) (bool, error) {
	if opcode&0x00c0 == 0x00c0 {
		return c.genericAddressArithmetic(opcode, false)
	}
	if opcode&0x0130 == 0x0100 {
		return false, nil // SUBX 由既有 decoder 或後續切片處理
	}
	return c.genericBinary(opcode, binarySub)
}

// --- CMP／CMPA／EOR／CMPM ----------------------------------------------------

func (c *CPU) genericCompareEor(opcode uint16) (bool, error) {
	if opcode&0x00c0 == 0x00c0 {
		return c.genericCompareAddress(opcode)
	}
	if opcode&0x0100 != 0 {
		if opcode&0x0038 == 0x0008 {
			return false, nil // CMPM 已由既有 decoder 處理
		}
		return c.genericBinary(opcode, binaryEor)
	}
	return c.genericBinary(opcode, binaryCmp)
}

// --- AND／MULU／MULS／ABCD／EXG ----------------------------------------------

func (c *CPU) genericAnd(opcode uint16) (bool, error) {
	switch opcode & 0x01c0 {
	case 0x00c0:
		return c.genericMultiply(opcode, false)
	case 0x01c0:
		return c.genericMultiply(opcode, true)
	}
	if opcode&0x01f0 == 0x0100 {
		return false, nil // ABCD 尚未實作
	}
	if opcode&0x0100 != 0 {
		switch opcode & 0x00f8 {
		case 0x0040, 0x0048, 0x0088:
			return c.genericExchange(opcode)
		}
	}
	return c.genericBinary(opcode, binaryAnd)
}

// --- ADD／ADDA／ADDX ---------------------------------------------------------

func (c *CPU) genericAdd(opcode uint16) (bool, error) {
	if opcode&0x00c0 == 0x00c0 {
		return c.genericAddressArithmetic(opcode, true)
	}
	if opcode&0x0130 == 0x0100 {
		return false, nil // ADDX 由既有 decoder 或後續切片處理
	}
	return c.genericBinary(opcode, binaryAdd)
}

type binaryOperation uint8

const (
	binaryAdd binaryOperation = iota
	binarySub
	binaryAnd
	binaryOr
	binaryEor
	binaryCmp
)

// genericBinary 處理 <ea>,Dn 與 Dn,<ea> 兩個方向的雙運算元指令。
func (c *CPU) genericBinary(opcode uint16, operation binaryOperation) (bool, error) {
	size, ok := sizeFromField(opcode >> 6 & 3)
	if !ok {
		return false, nil
	}
	toMemory := opcode&0x0100 != 0
	if operation == binaryCmp && toMemory {
		return false, nil
	}
	dataRegister := uint8(opcode >> 9 & 7)
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)
	if mode == 1 && (size == WidthByte || operation == binaryEor || operation == binaryAnd || operation == binaryOr) {
		return false, nil
	}
	if operation == binaryEor && !toMemory {
		return false, nil
	}

	stream := c.newInstructionStream()
	memory, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	memoryValue, err := c.readOperand(memory, size)
	if err != nil {
		return true, err
	}

	if !toMemory { // <ea> → Dn
		current := truncate(c.state.D[dataRegister], size)
		result := c.applyBinary(operation, current, memoryValue, size)
		if size == WidthLong && operation != binaryCmp {
			if err := c.internal(longSourceInternal(memory)); err != nil {
				return true, err
			}
		} else if size == WidthLong {
			if err := c.internal(2); err != nil {
				return true, err
			}
		}
		if operation != binaryCmp {
			c.state.D[dataRegister] = merge(c.state.D[dataRegister], result, size)
		}
		return true, stream.finish()
	}

	// Dn → <ea>
	if memory.kind == operandDataRegister && operation != binaryEor {
		return false, nil // 這個編碼在非 EOR 的族是 ADDX/SUBX/ABCD/SBCD
	}
	if memory.kind == operandAddressRegister {
		return false, nil
	}
	source := truncate(c.state.D[dataRegister], size)
	result := c.applyBinary(operation, memoryValue, source, size)
	if operation == binaryEor && memory.kind == operandDataRegister {
		if size == WidthLong {
			if err := c.internal(4); err != nil {
				return true, err
			}
		}
	}
	if err := c.writeOperand(memory, size, result); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) applyBinary(operation binaryOperation, destination, source uint32, size Width) uint32 {
	switch operation {
	case binaryAdd:
		return c.addSized(destination, source, size)
	case binarySub:
		return c.subSized(destination, source, size)
	case binaryAnd:
		result := destination & source
		c.setNZSized(result, size)
		return result
	case binaryOr:
		result := destination | source
		c.setNZSized(result, size)
		return result
	case binaryEor:
		result := destination ^ source
		c.setNZSized(result, size)
		return result
	default:
		c.compareSized(destination, source, size)
		return destination
	}
}

// genericAddressArithmetic 實作 ADDA／SUBA：不影響旗標，來源符號延伸成 32 位元。
func (c *CPU) genericAddressArithmetic(opcode uint16, isAdd bool) (bool, error) {
	size := WidthWord
	if opcode&0x0100 != 0 {
		size = WidthLong
	}
	destination := uint8(opcode >> 9 & 7)
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)

	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(source, size)
	if err != nil {
		return true, err
	}
	extended := signExtend(value, size)
	if isAdd {
		c.state.A[destination] = (c.state.A[destination] + extended) & addressMask
	} else {
		c.state.A[destination] = (c.state.A[destination] - extended) & addressMask
	}
	internal := uint8(2)
	if size == WidthWord || isRegisterOrImmediate(source) {
		internal = 4
	}
	if err := c.internal(internal); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericCompareAddress(opcode uint16) (bool, error) {
	size := WidthWord
	if opcode&0x0100 != 0 {
		size = WidthLong
	}
	destination := uint8(opcode >> 9 & 7)
	mode := uint8(opcode >> 3 & 7)
	register := uint8(opcode & 7)

	stream := c.newInstructionStream()
	source, ok, err := c.resolveOperand(stream, mode, register, size)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(source, size)
	if err != nil {
		return true, err
	}
	c.setCompare32(c.state.A[destination], signExtend(value, size))
	if err := c.internal(2); err != nil {
		return true, err
	}
	return true, stream.finish()
}

func (c *CPU) genericExchange(opcode uint16) (bool, error) {
	x := uint8(opcode >> 9 & 7)
	y := uint8(opcode & 7)
	switch opcode & 0x00f8 {
	case 0x0040:
		c.state.D[x], c.state.D[y] = c.state.D[y], c.state.D[x]
	case 0x0048:
		c.state.A[x], c.state.A[y] = c.state.A[y], c.state.A[x]
	case 0x0088:
		c.state.D[x], c.state.A[y] = c.state.A[y], c.state.D[x]
	default:
		return false, nil
	}
	if err := c.internal(2); err != nil {
		return true, err
	}
	return true, c.prefetch()
}

// genericMultiply 以 68000 的實際位元計數規則計時：基礎 38 cycle 再加來源位元。
func (c *CPU) genericMultiply(opcode uint16, signed bool) (bool, error) {
	destination := uint8(opcode >> 9 & 7)
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
	operand16 := uint16(value)
	multiplicand := uint16(c.state.D[destination])
	var result uint32
	if signed {
		result = uint32(int32(int16(multiplicand)) * int32(int16(operand16)))
	} else {
		result = uint32(multiplicand) * uint32(operand16)
	}
	c.state.D[destination] = result
	c.setNZ32(result)
	if err := c.internal(multiplyCycles(operand16, signed)); err != nil {
		return true, err
	}
	return true, stream.finish()
}

// multiplyCycles 是扣掉 prefetch 之後的內部時間：MULU 為 34 + 2×(來源 1 的個數)，
// MULS 以位元對變化次數計算。上限受 uint8 限制，故以 70 封頂。
func multiplyCycles(value uint16, signed bool) uint8 {
	count := 0
	if signed {
		previous := uint16(0)
		for bit := 0; bit < 16; bit++ {
			current := value >> bit & 1
			if current != previous {
				count++
			}
			previous = current
		}
	} else {
		for bit := 0; bit < 16; bit++ {
			if value>>bit&1 != 0 {
				count++
			}
		}
	}
	total := 34 + 2*count
	if total > 70 {
		total = 70
	}
	return uint8(total)
}

// genericDivide 目前採 PRM 的最壞情況時間；精確的逐步時間仍待收斂。
func (c *CPU) genericDivide(opcode uint16, signed bool) (bool, error) {
	destination := uint8(opcode >> 9 & 7)
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
	divisor := uint16(value)
	dividend := c.state.D[destination]
	if divisor == 0 {
		// 除以零是 vector 5 例外；在補上完整例外路徑之前 fail-closed。
		return true, errors.New("m68k: divide by zero")
	}
	if signed {
		quotient := int32(dividend) / int32(int16(divisor))
		remainder := int32(dividend) % int32(int16(divisor))
		if quotient > 32767 || quotient < -32768 {
			c.state.SR |= flagOverflow
		} else {
			c.state.SR &^= flagOverflow
			c.state.D[destination] = uint32(uint16(remainder))<<16 | uint32(uint16(quotient))
			c.setNZ16(uint16(quotient))
		}
	} else {
		quotient := dividend / uint32(divisor)
		remainder := dividend % uint32(divisor)
		if quotient > 0xffff {
			c.state.SR |= flagOverflow
		} else {
			c.state.SR &^= flagOverflow
			c.state.D[destination] = remainder<<16 | quotient
			c.setNZ16(uint16(quotient))
		}
	}
	worstCase := uint8(136)
	if signed {
		worstCase = 154
	}
	if err := c.internal(worstCase); err != nil {
		return true, err
	}
	return true, stream.finish()
}

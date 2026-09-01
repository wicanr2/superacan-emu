package m68k

// 68000 的例外向量編號。向量位址是編號乘以四。
const (
	VectorBusError          uint8 = 2
	VectorAddressError      uint8 = 3
	VectorIllegalInstruction uint8 = 4
	VectorDivideByZero      uint8 = 5
	VectorCHK               uint8 = 6
	VectorTRAPV             uint8 = 7
	VectorPrivilegeViolation uint8 = 8
	VectorLineA             uint8 = 10
	VectorLineF             uint8 = 11
	VectorTRAPBase          uint8 = 32
)

// exception 產生一個 68000 例外：推入回返 PC 與 SR、進入監督者模式並關閉 trace、
// 讀取向量、重灌 prefetch。
//
// returnPC 依例外種類不同：TRAP／TRAPV／CHK／除以零推的是下一條指令的位址，
// 非法指令與特權違例推的是造成例外那條指令自己的位址。
//
// internalCycles 是 PRM 指令時間扣掉本函式實際發生的七次 bus transaction
// （兩次 PC 寫、一次 SR 寫、兩次向量讀、兩次 prefetch 補位）之後剩下的時間。
func (c *CPU) exception(vector uint8, returnPC uint32, internalCycles uint8) error {
	status := c.state.SR
	if err := c.internal(internalCycles); err != nil {
		return err
	}
	c.setStatusRegister(status&0xf8ff | 0x2000)

	c.state.A[7] -= 4
	if err := c.writeLong(c.state.A[7], returnPC, FCSupervisorData); err != nil {
		return err
	}
	c.state.A[7] -= 2
	if err := c.writeWord(c.state.A[7], status, FCSupervisorData); err != nil {
		return err
	}
	target, err := c.readLong(uint32(vector)*4, FCSupervisorData)
	if err != nil {
		return err
	}
	return c.refillPrefetch(target&addressMask, 0)
}

// setStatusRegister 是所有 SR 寫入的唯一入口。S 位元改變時要交換堆疊指標：
// 68000 有兩個 A7，監督者模式看到 SSP、使用者模式看到 USP，切換由 S 位元決定。
func (c *CPU) setStatusRegister(value uint16) {
	wasSupervisor := c.state.SR&0x2000 != 0
	isSupervisor := value&0x2000 != 0
	if wasSupervisor != isSupervisor {
		c.state.A[7], c.state.InactiveSP = c.state.InactiveSP, c.state.A[7]
	}
	c.state.SR = value
}

// trap 實作 TRAP #n。向量是 32 + n。
func (c *CPU) genericTrap(number uint8) (bool, error) {
	return true, c.exception(VectorTRAPBase+number&0x0f, (c.state.PC+2)&addressMask, 6)
}

// genericTrapOnOverflow 實作 TRAPV：V 沒設就只是繼續執行。
func (c *CPU) genericTrapOnOverflow() (bool, error) {
	if c.state.SR&flagOverflow == 0 {
		return true, c.prefetch()
	}
	return true, c.exception(VectorTRAPV, (c.state.PC+2)&addressMask, 6)
}

// genericCheck 實作 CHK <ea>,Dn：Dn 的字為負或大於上界就進向量 6。
func (c *CPU) genericCheck(opcode uint16) (bool, error) {
	register := uint8(opcode >> 9 & 7)
	mode := uint8(opcode >> 3 & 7)
	source := uint8(opcode & 7)
	if mode == 1 {
		return false, nil
	}
	stream := c.newInstructionStream()
	bound, ok, err := c.resolveOperand(stream, mode, source, WidthWord)
	if err != nil || !ok {
		return ok, err
	}
	value, err := c.readOperand(bound, WidthWord)
	if err != nil {
		return true, err
	}
	subject := int16(uint16(c.state.D[register]))
	limit := int16(uint16(value))

	// N 反映比較結果；Z、V、C 在 68000 上未定義，這裡保持不變。
	c.state.SR &^= flagNegative
	switch {
	case subject < 0:
		c.state.SR |= flagNegative
	case subject > limit:
		// 超出上界時 N 清除，與 68000 一致。
	default:
		if err := c.internal(6); err != nil {
			return true, err
		}
		return true, stream.finish()
	}
	if err := stream.finish(); err != nil {
		return true, err
	}
	return true, c.exception(VectorCHK, c.state.PC, 4)
}

// failClosedOrException 把「這個編碼在 68000 上就是非法」與「我們還沒實作」分開：
// 前者產生例外，後者維持 fail-closed，不得靜默當成 NOP。
func (c *CPU) illegalInstruction(opcode uint16) (bool, error) {
	switch {
	case opcode == 0x4afc:
		return true, c.exception(VectorIllegalInstruction, c.state.PC, 6)
	case opcode&0xf000 == 0xa000:
		return true, c.exception(VectorLineA, c.state.PC, 6)
	case opcode&0xf000 == 0xf000:
		return true, c.exception(VectorLineF, c.state.PC, 6)
	}
	return false, nil
}

// privilegeViolation 在使用者模式執行特權指令時進向量 8。
func (c *CPU) privilegeViolation() error {
	return c.exception(VectorPrivilegeViolation, c.state.PC, 6)
}

func (c *CPU) divideByZero() error {
	return c.exception(VectorDivideByZero, (c.state.PC+2)&addressMask, 10)
}

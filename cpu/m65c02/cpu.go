// Package m65c02 implements the WDC 65C02 as an independent Go core.
package m65c02

import "fmt"

const (
	flagCarry            uint8 = 1 << 0
	flagZero             uint8 = 1 << 1
	flagInterruptDisable uint8 = 1 << 2
	flagDecimal          uint8 = 1 << 3
	flagUnused           uint8 = 1 << 5
	flagOverflow         uint8 = 1 << 6
	flagNegative         uint8 = 1 << 7
)

type Bus interface {
	Read8(address uint16) (uint8, error)
	Write8(address uint16, value uint8) error
}

type Cycle struct {
	Address  uint16
	Write    bool
	Value    uint8
	Internal bool
}

type Scheduler interface {
	Advance(Cycle) error
}

type State struct {
	PC     uint16
	A      uint8
	X      uint8
	Y      uint8
	SP     uint8
	P      uint8
	Cycles uint64
}

type StepResult struct {
	PCBefore uint16
	PCAfter  uint16
	Opcode   uint8
	Cycles   uint64
}

type CPU struct {
	bus       Bus
	scheduler Scheduler
	state     State
}

func New(bus Bus, scheduler Scheduler) *CPU {
	if bus == nil || scheduler == nil {
		panic("m65c02: nil bus or scheduler")
	}
	return &CPU{bus: bus, scheduler: scheduler}
}

func (c *CPU) State() State { return c.state }

// Reset models the seven-cycle reset entry and reads the little-endian vector
// at $FFFC/$FFFD. The machine keeps reset asserted until the sound driver has
// been uploaded and $E9001C bit 0 is released.
func (c *CPU) Reset() error {
	c.state = State{SP: 0xfd, P: flagInterruptDisable | flagUnused}
	for range 5 {
		if err := c.internal(); err != nil {
			return err
		}
	}
	lo, err := c.read(0xfffc)
	if err != nil {
		return fmt.Errorf("m65c02 reset vector low: %w", err)
	}
	hi, err := c.read(0xfffd)
	if err != nil {
		return fmt.Errorf("m65c02 reset vector high: %w", err)
	}
	c.state.PC = uint16(hi)<<8 | uint16(lo)
	return nil
}

func (c *CPU) Step() (StepResult, error) {
	result := StepResult{PCBefore: c.state.PC}
	start := c.state.Cycles
	opcode, err := c.read(c.state.PC)
	if err != nil {
		return result, err
	}
	result.Opcode = opcode
	switch opcode {
	case 0x78: // SEI
		c.state.PC++
		c.state.P |= flagInterruptDisable
		err = c.internal()
	case 0xea: // NOP
		c.state.PC++
		err = c.internal()
	case 0xd8: // CLD
		c.state.PC++
		c.state.P &^= flagDecimal
		err = c.internal()
	case 0x58: // CLI
		c.state.PC++
		c.state.P &^= flagInterruptDisable
		err = c.internal()
	case 0x18: // CLC
		c.state.PC++
		c.state.P &^= flagCarry
		err = c.internal()
	case 0x38: // SEC
		c.state.PC++
		c.state.P |= flagCarry
		err = c.internal()
	case 0xb8: // CLV
		c.state.PC++
		c.state.P &^= flagOverflow
		err = c.internal()
	case 0xf8: // SED
		c.state.PC++
		c.state.P |= flagDecimal
		err = c.internal()
	case 0xa2: // LDX #imm
		c.state.PC++
		c.state.X, err = c.fetch()
		if err == nil {
			c.setNZ(c.state.X)
		}
	case 0xa9: // LDA #imm
		c.state.PC++
		c.state.A, err = c.fetch()
		if err == nil {
			c.setNZ(c.state.A)
		}
	case 0xa5, 0xa6, 0xa4: // LDA/LDX/LDY zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			switch opcode {
			case 0xa5:
				c.state.A = value
			case 0xa6:
				c.state.X = value
			case 0xa4:
				c.state.Y = value
			}
			c.setNZ(value)
		}
	case 0xa0: // LDY #imm
		c.state.PC++
		c.state.Y, err = c.fetch()
		if err == nil {
			c.setNZ(c.state.Y)
		}
	case 0xad: // LDA abs
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		if err == nil {
			c.state.A, err = c.read(uint16(hi)<<8 | uint16(lo))
		}
		if err == nil {
			c.setNZ(c.state.A)
		}
	case 0xb9, 0xbd: // LDA abs,Y / abs,X
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		base := uint16(hi)<<8 | uint16(lo)
		index := uint16(c.state.Y)
		if opcode == 0xbd {
			index = uint16(c.state.X)
		}
		address := base + index
		if err == nil && base&0xff00 != address&0xff00 {
			err = c.internal()
		}
		if err == nil {
			c.state.A, err = c.read(address)
		}
		if err == nil {
			c.setNZ(c.state.A)
		}
	case 0xcc: // CPY abs
		c.state.PC++
		var lo, hi, value uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		if err == nil {
			value, err = c.read(uint16(hi)<<8 | uint16(lo))
		}
		if err == nil {
			c.compare(c.state.Y, value)
		}
	case 0x29, 0x09, 0x49: // AND/ORA/EOR #imm
		c.state.PC++
		var value uint8
		value, err = c.fetch()
		if err == nil {
			switch opcode {
			case 0x29:
				c.state.A &= value
			case 0x09:
				c.state.A |= value
			case 0x49:
				c.state.A ^= value
			}
			c.setNZ(c.state.A)
		}
	case 0x9a: // TXS
		c.state.PC++
		c.state.SP = c.state.X
		err = c.internal()
	case 0xaa: // TAX
		c.state.PC++
		c.state.X = c.state.A
		c.setNZ(c.state.X)
		err = c.internal()
	case 0xa8: // TAY
		c.state.PC++
		c.state.Y = c.state.A
		c.setNZ(c.state.Y)
		err = c.internal()
	case 0x8a: // TXA
		c.state.PC++
		c.state.A = c.state.X
		c.setNZ(c.state.A)
		err = c.internal()
	case 0x98: // TYA
		c.state.PC++
		c.state.A = c.state.Y
		c.setNZ(c.state.A)
		err = c.internal()
	case 0xba: // TSX
		c.state.PC++
		c.state.X = c.state.SP
		c.setNZ(c.state.X)
		err = c.internal()
	case 0x20: // JSR abs
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		if err == nil {
			err = c.internal()
		}
		returnAddress := c.state.PC - 1
		if err == nil {
			err = c.push(uint8(returnAddress >> 8))
		}
		if err == nil {
			err = c.push(uint8(returnAddress))
		}
		if err == nil {
			c.state.PC = uint16(hi)<<8 | uint16(lo)
		}
	case 0x4c: // JMP abs
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		if err == nil {
			c.state.PC = uint16(hi)<<8 | uint16(lo)
		}
	case 0x48: // PHA
		c.state.PC++
		err = c.internal()
		if err == nil {
			err = c.push(c.state.A)
		}
	case 0xda: // PHX (65C02)
		c.state.PC++
		err = c.internal()
		if err == nil {
			err = c.push(c.state.X)
		}
	case 0x5a: // PHY (65C02)
		c.state.PC++
		err = c.internal()
		if err == nil {
			err = c.push(c.state.Y)
		}
	case 0x68, 0xfa, 0x7a: // PLA/PLX/PLY
		c.state.PC++
		err = c.internal()
		var value uint8
		if err == nil {
			value, err = c.pull()
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			switch opcode {
			case 0x68:
				c.state.A = value
			case 0xfa:
				c.state.X = value
			case 0x7a:
				c.state.Y = value
			}
			c.setNZ(value)
		}
	case 0x60: // RTS
		c.state.PC++
		err = c.internal()
		var lo, hi uint8
		if err == nil {
			lo, err = c.pull()
		}
		if err == nil {
			hi, err = c.pull()
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			c.state.PC = (uint16(hi)<<8 | uint16(lo)) + 1
		}
	case 0x95: // STA zp,X
		c.state.PC++
		var zeroPage uint8
		zeroPage, err = c.fetch()
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.write(uint16(zeroPage+c.state.X), c.state.A)
		}
	case 0x9d: // STA abs,X
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.write((uint16(hi)<<8|uint16(lo))+uint16(c.state.X), c.state.A)
		}
	case 0x8d, 0x8e, 0x8c: // STA/STX/STY abs
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		value := c.state.A
		if opcode == 0x8e {
			value = c.state.X
		} else if opcode == 0x8c {
			value = c.state.Y
		}
		if err == nil {
			err = c.write(uint16(hi)<<8|uint16(lo), value)
		}
	case 0x85, 0x86, 0x84: // STA/STX/STY zp
		c.state.PC++
		var zeroPage uint8
		zeroPage, err = c.fetch()
		value := c.state.A
		if opcode == 0x86 {
			value = c.state.X
		} else if opcode == 0x84 {
			value = c.state.Y
		}
		if err == nil {
			err = c.write(uint16(zeroPage), value)
		}
	case 0xe6, 0xc6: // INC/DEC zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if opcode == 0xe6 {
			value++
		} else {
			value--
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.write(uint16(zeroPage), value)
		}
		if err == nil {
			c.setNZ(value)
		}
	case 0xca: // DEX
		c.state.PC++
		c.state.X--
		c.setNZ(c.state.X)
		err = c.internal()
	case 0xe8: // INX
		c.state.PC++
		c.state.X++
		c.setNZ(c.state.X)
		err = c.internal()
	case 0xc8: // INY
		c.state.PC++
		c.state.Y++
		c.setNZ(c.state.Y)
		err = c.internal()
	case 0x88: // DEY
		c.state.PC++
		c.state.Y--
		c.setNZ(c.state.Y)
		err = c.internal()
	case 0x10: // BPL
		err = c.branch(c.state.P&flagNegative == 0)
	case 0x30: // BMI
		err = c.branch(c.state.P&flagNegative != 0)
	case 0x50: // BVC
		err = c.branch(c.state.P&flagOverflow == 0)
	case 0x70: // BVS
		err = c.branch(c.state.P&flagOverflow != 0)
	case 0x90: // BCC
		err = c.branch(c.state.P&flagCarry == 0)
	case 0xb0: // BCS
		err = c.branch(c.state.P&flagCarry != 0)
	case 0xd0: // BNE
		err = c.branch(c.state.P&flagZero == 0)
	case 0xf0: // BEQ
		err = c.branch(c.state.P&flagZero != 0)
	case 0x80: // BRA (65C02)
		err = c.branch(true)
	default:
		err = fmt.Errorf("m65c02: unimplemented opcode $%02X at $%04X", opcode, c.state.PC)
	}
	result.PCAfter = c.state.PC
	result.Cycles = c.state.Cycles - start
	return result, err
}

func (c *CPU) branch(taken bool) error {
	c.state.PC++
	displacement, err := c.fetch()
	if err != nil || !taken {
		return err
	}
	oldPC := c.state.PC
	target := uint16(int32(oldPC) + int32(int8(displacement)))
	if err := c.internal(); err != nil {
		return err
	}
	if oldPC&0xff00 != target&0xff00 {
		if err := c.internal(); err != nil {
			return err
		}
	}
	c.state.PC = target
	return nil
}

func (c *CPU) fetch() (uint8, error) {
	value, err := c.read(c.state.PC)
	if err == nil {
		c.state.PC++
	}
	return value, err
}

func (c *CPU) push(value uint8) error {
	if err := c.write(0x0100|uint16(c.state.SP), value); err != nil {
		return err
	}
	c.state.SP--
	return nil
}

func (c *CPU) pull() (uint8, error) {
	c.state.SP++
	return c.read(0x0100 | uint16(c.state.SP))
}

func (c *CPU) setNZ(value uint8) {
	c.state.P &^= flagZero | flagNegative
	if value == 0 {
		c.state.P |= flagZero
	}
	if value&0x80 != 0 {
		c.state.P |= flagNegative
	}
}

func (c *CPU) compare(register, value uint8) {
	c.state.P &^= flagCarry
	if register >= value {
		c.state.P |= flagCarry
	}
	c.setNZ(register - value)
}

func (c *CPU) read(address uint16) (uint8, error) {
	if err := c.scheduler.Advance(Cycle{Address: address}); err != nil {
		return 0, err
	}
	c.state.Cycles++
	return c.bus.Read8(address)
}

func (c *CPU) write(address uint16, value uint8) error {
	if err := c.scheduler.Advance(Cycle{Address: address, Write: true, Value: value}); err != nil {
		return err
	}
	c.state.Cycles++
	return c.bus.Write8(address, value)
}

func (c *CPU) internal() error {
	if err := c.scheduler.Advance(Cycle{Internal: true}); err != nil {
		return err
	}
	c.state.Cycles++
	return nil
}

// Package m65c02 implements the WDC 65C02 as an independent Go core.
package m65c02

import "fmt"

const (
	flagCarry            uint8 = 1 << 0
	flagZero             uint8 = 1 << 1
	flagInterruptDisable uint8 = 1 << 2
	flagDecimal          uint8 = 1 << 3
	flagBreak            uint8 = 1 << 4
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
	PCBefore  uint16
	PCAfter   uint16
	Opcode    uint8
	Cycles    uint64
	Interrupt bool
	NMI       bool
	Waiting   bool
}

type CPU struct {
	bus        Bus
	scheduler  Scheduler
	state      State
	irqLine    bool
	nmiPending bool
	waiting    bool
}

func New(bus Bus, scheduler Scheduler) *CPU {
	if bus == nil || scheduler == nil {
		panic("m65c02: nil bus or scheduler")
	}
	return &CPU{bus: bus, scheduler: scheduler}
}

func (c *CPU) State() State { return c.state }

// SetIRQ drives the level-sensitive maskable interrupt input. The input is
// sampled only at instruction boundaries, as on the physical processor.
func (c *CPU) SetIRQ(asserted bool) { c.irqLine = asserted }

// PulseNMI latches a rising edge until the next instruction boundary.
func (c *CPU) PulseNMI() { c.nmiPending = true }

// Reset models the seven-cycle reset entry and reads the little-endian vector
// at $FFFC/$FFFD. The machine keeps reset asserted until the sound driver has
// been uploaded and $E9001C bit 0 is released.
func (c *CPU) Reset() error {
	c.state = State{SP: 0xfd, P: flagInterruptDisable | flagUnused}
	c.waiting = false
	c.nmiPending = false
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
	if c.waiting {
		if !c.irqLine && !c.nmiPending {
			err := c.internal()
			result.PCAfter = c.state.PC
			result.Cycles = c.state.Cycles - start
			result.Waiting = true
			return result, err
		}
		c.waiting = false
	}
	if c.nmiPending {
		c.nmiPending = false
		err := c.serviceInterrupt(0xfffa)
		result.PCAfter = c.state.PC
		result.Cycles = c.state.Cycles - start
		result.Interrupt = true
		result.NMI = true
		return result, err
	}
	if c.irqLine && c.state.P&flagInterruptDisable == 0 {
		err := c.serviceInterrupt(0xfffe)
		result.PCAfter = c.state.PC
		result.Cycles = c.state.Cycles - start
		result.Interrupt = true
		return result, err
	}
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
	case 0xb5, 0xb6, 0xb4: // LDA zp,X / LDX zp,Y / LDY zp,X
		c.state.PC++
		var zeroPage, value, index uint8
		zeroPage, err = c.fetch()
		index = c.state.X
		if opcode == 0xb6 {
			index = c.state.Y
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			value, err = c.read(uint16(zeroPage + index))
		}
		if err == nil {
			switch opcode {
			case 0xb5:
				c.state.A = value
			case 0xb6:
				c.state.X = value
			case 0xb4:
				c.state.Y = value
			}
			c.setNZ(value)
		}
	case 0xa1: // LDA (zp,X)
		c.state.PC++
		var zeroPage, lo, hi uint8
		zeroPage, err = c.fetch()
		if err == nil {
			err = c.internal()
		}
		pointer := zeroPage + c.state.X
		if err == nil {
			lo, err = c.read(uint16(pointer))
		}
		if err == nil {
			hi, err = c.read(uint16(pointer + 1))
		}
		if err == nil {
			c.state.A, err = c.read(uint16(hi)<<8 | uint16(lo))
		}
		if err == nil {
			c.setNZ(c.state.A)
		}
	case 0xb1: // LDA (zp),Y
		c.state.PC++
		var zeroPage, lo, hi uint8
		zeroPage, err = c.fetch()
		if err == nil {
			lo, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			hi, err = c.read(uint16(zeroPage + 1))
		}
		base := uint16(hi)<<8 | uint16(lo)
		address := base + uint16(c.state.Y)
		if err == nil && base&0xff00 != address&0xff00 {
			err = c.internal()
		}
		if err == nil {
			c.state.A, err = c.read(address)
		}
		if err == nil {
			c.setNZ(c.state.A)
		}
	case 0xa0: // LDY #imm
		c.state.PC++
		c.state.Y, err = c.fetch()
		if err == nil {
			c.setNZ(c.state.Y)
		}
	case 0xad, 0xae, 0xac: // LDA/LDX/LDY abs
		c.state.PC++
		var lo, hi uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		var value uint8
		if err == nil {
			value, err = c.read(uint16(hi)<<8 | uint16(lo))
		}
		if err == nil {
			switch opcode {
			case 0xad:
				c.state.A = value
			case 0xae:
				c.state.X = value
			case 0xac:
				c.state.Y = value
			}
			c.setNZ(value)
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
	case 0xcc, 0xec: // CPY/CPX abs
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
			if opcode == 0xec {
				c.compare(c.state.X, value)
			} else {
				c.compare(c.state.Y, value)
			}
		}
	case 0xe0, 0xc0: // CPX/CPY #imm
		c.state.PC++
		var value uint8
		value, err = c.fetch()
		if err == nil {
			if opcode == 0xe0 {
				c.compare(c.state.X, value)
			} else {
				c.compare(c.state.Y, value)
			}
		}
	case 0xc9: // CMP #imm
		c.state.PC++
		var value uint8
		value, err = c.fetch()
		if err == nil {
			c.compare(c.state.A, value)
		}
	case 0xc5: // CMP zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			c.compare(c.state.A, value)
		}
	case 0xcd: // CMP abs
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
			c.compare(c.state.A, value)
		}
	case 0x69: // ADC #imm
		c.state.PC++
		var value uint8
		value, err = c.fetch()
		if err == nil {
			err = c.adc(value)
		}
	case 0x65: // ADC zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			err = c.adc(value)
		}
	case 0x79: // ADC abs,Y
		c.state.PC++
		var lo, hi, value uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		base := uint16(hi)<<8 | uint16(lo)
		address := base + uint16(c.state.Y)
		if err == nil && base&0xff00 != address&0xff00 {
			err = c.internal()
		}
		if err == nil {
			value, err = c.read(address)
		}
		if err == nil {
			err = c.adc(value)
		}
	case 0x6d: // ADC abs
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
			err = c.adc(value)
		}
	case 0xe9: // SBC #imm
		c.state.PC++
		var value uint8
		value, err = c.fetch()
		if err == nil {
			err = c.sbc(value)
		}
	case 0xe5: // SBC zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			err = c.sbc(value)
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
	case 0x25, 0x05, 0x45: // AND/ORA/EOR zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			switch opcode {
			case 0x25:
				c.state.A &= value
			case 0x05:
				c.state.A |= value
			case 0x45:
				c.state.A ^= value
			}
			c.setNZ(c.state.A)
		}
	case 0x35, 0x15, 0x55: // AND/ORA/EOR zp,X
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			value, err = c.read(uint16(zeroPage + c.state.X))
		}
		if err == nil {
			switch opcode {
			case 0x35:
				c.state.A &= value
			case 0x15:
				c.state.A |= value
			case 0x55:
				c.state.A ^= value
			}
			c.setNZ(c.state.A)
		}
	case 0x3d, 0x1d, 0x5d: // AND/ORA/EOR abs,X
		c.state.PC++
		var lo, hi, value uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		base := uint16(hi)<<8 | uint16(lo)
		address := base + uint16(c.state.X)
		if err == nil && base&0xff00 != address&0xff00 {
			err = c.internal()
		}
		if err == nil {
			value, err = c.read(address)
		}
		if err == nil {
			switch opcode {
			case 0x3d:
				c.state.A &= value
			case 0x1d:
				c.state.A |= value
			case 0x5d:
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
	case 0x6c: // JMP (abs), with the 65C02 page-boundary correction
		c.state.PC++
		var pointerLo, pointerHi, targetLo, targetHi uint8
		pointerLo, err = c.fetch()
		if err == nil {
			pointerHi, err = c.fetch()
		}
		pointer := uint16(pointerHi)<<8 | uint16(pointerLo)
		if err == nil {
			targetLo, err = c.read(pointer)
		}
		if err == nil {
			targetHi, err = c.read(pointer + 1)
		}
		if err == nil {
			c.state.PC = uint16(targetHi)<<8 | uint16(targetLo)
		}
	case 0x48: // PHA
		c.state.PC++
		err = c.internal()
		if err == nil {
			err = c.push(c.state.A)
		}
	case 0x08: // PHP
		c.state.PC++
		err = c.internal()
		if err == nil {
			err = c.push(c.state.P | flagBreak | flagUnused)
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
	case 0x28: // PLP
		c.state.PC++
		err = c.internal()
		var status uint8
		if err == nil {
			status, err = c.pull()
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			c.state.P = status | flagUnused
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
	case 0x40: // RTI
		c.state.PC++
		err = c.internal()
		var status, lo, hi uint8
		if err == nil {
			status, err = c.pull()
		}
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
			c.state.P = status | flagUnused
			c.state.PC = uint16(hi)<<8 | uint16(lo)
		}
	case 0xcb: // WAI (65C02)
		c.state.PC++
		err = c.internal()
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			c.waiting = true
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
	case 0x91: // STA (zp),Y
		c.state.PC++
		var zeroPage, lo, hi uint8
		zeroPage, err = c.fetch()
		if err == nil {
			lo, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			hi, err = c.read(uint16(zeroPage + 1))
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.write((uint16(hi)<<8|uint16(lo))+uint16(c.state.Y), c.state.A)
		}
	case 0x9d, 0x99: // STA abs,X / STA abs,Y
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
			index := c.state.X
			if opcode == 0x99 {
				index = c.state.Y
			}
			err = c.write((uint16(hi)<<8|uint16(lo))+uint16(index), c.state.A)
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
	case 0xf6, 0xd6: // INC/DEC zp,X
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			err = c.internal()
		}
		address := uint16(zeroPage + c.state.X)
		if err == nil {
			value, err = c.read(address)
		}
		if opcode == 0xf6 {
			value++
		} else {
			value--
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.write(address, value)
		}
		if err == nil {
			c.setNZ(value)
		}
	case 0xee, 0xce: // INC/DEC abs
		c.state.PC++
		var lo, hi, value uint8
		lo, err = c.fetch()
		if err == nil {
			hi, err = c.fetch()
		}
		address := uint16(hi)<<8 | uint16(lo)
		if err == nil {
			value, err = c.read(address)
		}
		if opcode == 0xee {
			value++
		} else {
			value--
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			err = c.write(address, value)
		}
		if err == nil {
			c.setNZ(value)
		}
	case 0x46: // LSR zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			c.state.P &^= flagCarry
			if value&1 != 0 {
				c.state.P |= flagCarry
			}
			value >>= 1
			err = c.write(uint16(zeroPage), value)
		}
		if err == nil {
			c.setNZ(value)
		}
	case 0x06, 0x26, 0x66: // ASL/ROL/ROR zp
		c.state.PC++
		var zeroPage, value uint8
		zeroPage, err = c.fetch()
		if err == nil {
			value, err = c.read(uint16(zeroPage))
		}
		if err == nil {
			err = c.internal()
		}
		if err == nil {
			oldCarry := c.state.P&flagCarry != 0
			c.state.P &^= flagCarry
			switch opcode {
			case 0x06:
				if value&0x80 != 0 {
					c.state.P |= flagCarry
				}
				value <<= 1
			case 0x26:
				if value&0x80 != 0 {
					c.state.P |= flagCarry
				}
				value <<= 1
				if oldCarry {
					value |= 1
				}
			case 0x66:
				if value&1 != 0 {
					c.state.P |= flagCarry
				}
				value >>= 1
				if oldCarry {
					value |= 0x80
				}
			}
			err = c.write(uint16(zeroPage), value)
		}
		if err == nil {
			c.setNZ(value)
		}
	case 0x0a, 0x2a, 0x4a, 0x6a: // ASL/ROL/LSR/ROR A
		c.state.PC++
		oldCarry := c.state.P&flagCarry != 0
		c.state.P &^= flagCarry
		switch opcode {
		case 0x0a:
			if c.state.A&0x80 != 0 {
				c.state.P |= flagCarry
			}
			c.state.A <<= 1
		case 0x2a:
			if c.state.A&0x80 != 0 {
				c.state.P |= flagCarry
			}
			c.state.A <<= 1
			if oldCarry {
				c.state.A |= 1
			}
		case 0x4a:
			if c.state.A&1 != 0 {
				c.state.P |= flagCarry
			}
			c.state.A >>= 1
		case 0x6a:
			if c.state.A&1 != 0 {
				c.state.P |= flagCarry
			}
			c.state.A >>= 1
			if oldCarry {
				c.state.A |= 0x80
			}
		}
		c.setNZ(c.state.A)
		err = c.internal()
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

func (c *CPU) serviceInterrupt(vector uint16) error {
	if err := c.internal(); err != nil {
		return err
	}
	if err := c.internal(); err != nil {
		return err
	}
	if err := c.push(uint8(c.state.PC >> 8)); err != nil {
		return err
	}
	if err := c.push(uint8(c.state.PC)); err != nil {
		return err
	}
	status := (c.state.P | flagUnused) &^ flagBreak
	if err := c.push(status); err != nil {
		return err
	}
	c.state.P = (c.state.P | flagInterruptDisable | flagUnused) &^ flagDecimal
	lo, err := c.read(vector)
	if err != nil {
		return err
	}
	hi, err := c.read(vector + 1)
	if err != nil {
		return err
	}
	c.state.PC = uint16(hi)<<8 | uint16(lo)
	return nil
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

func (c *CPU) adc(value uint8) error {
	a := c.state.A
	carry := uint16(0)
	if c.state.P&flagCarry != 0 {
		carry = 1
	}
	binary := uint16(a) + uint16(value) + carry
	c.state.P &^= flagCarry | flagOverflow
	if (^(a ^ value) & (a ^ uint8(binary)) & 0x80) != 0 {
		c.state.P |= flagOverflow
	}
	if c.state.P&flagDecimal != 0 {
		low := uint16(a&0x0f) + uint16(value&0x0f) + carry
		if low > 9 {
			low += 6
		}
		decimal := uint16(a&0xf0) + uint16(value&0xf0) + low
		if decimal > 0x9f {
			decimal += 0x60
		}
		if decimal > 0xff {
			c.state.P |= flagCarry
		}
		c.state.A = uint8(decimal)
		c.setNZ(c.state.A)
		return c.internal() // W65C02 decimal arithmetic takes one extra cycle.
	}
	if binary > 0xff {
		c.state.P |= flagCarry
	}
	c.state.A = uint8(binary)
	c.setNZ(c.state.A)
	return nil
}

func (c *CPU) sbc(value uint8) error {
	a := c.state.A
	borrow := int16(1)
	if c.state.P&flagCarry != 0 {
		borrow = 0
	}
	binary := int16(a) - int16(value) - borrow
	result := uint8(binary)
	c.state.P &^= flagCarry | flagOverflow
	if binary >= 0 {
		c.state.P |= flagCarry
	}
	if ((a ^ result) & (a ^ value) & 0x80) != 0 {
		c.state.P |= flagOverflow
	}
	if c.state.P&flagDecimal != 0 {
		low := int16(a&0x0f) - int16(value&0x0f) - borrow
		high := int16(a>>4) - int16(value>>4)
		if low < 0 {
			low -= 6
			high--
		}
		if high < 0 {
			high -= 6
		}
		result = uint8(high<<4)&0xf0 | uint8(low)&0x0f
		c.state.A = result
		c.setNZ(result)
		return c.internal()
	}
	c.state.A = result
	c.setNZ(result)
	return nil
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

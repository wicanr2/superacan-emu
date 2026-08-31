package machine

import (
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/cpu/m65c02"
)

// SoundBus is the W65C02-visible 64 KiB address space. RAM is physically
// shared with the 68000 $E80000 window; $0400-$04FF is decoded as controller I/O.
type SoundBus struct {
	ram       *[65536]byte
	audio     *umc6619.Device
	irqEnable uint8
	irqStatus uint8
	io        [256]uint8
	pads      [2]uint16
	shiftCtrl uint8
	shiftRegs [2]uint8
	latched   [2]uint16
	latch     [2]uint8
	latchFull [2]bool
	onIRQ6    func()
}

func newSoundBus(ram *[65536]byte) *SoundBus {
	bus := &SoundBus{ram: ram, audio: umc6619.New(), pads: [2]uint16{0xffff, 0xffff}}
	bus.audio.SetRAMReader(func(address uint16) uint8 { return ram[address] })
	bus.audio.SetIRQHandler(func(mask uint8, asserted bool) {
		if asserted {
			bus.irqStatus |= mask
		} else {
			bus.irqStatus &^= mask
		}
	})
	return bus
}

func (b *SoundBus) Audio() *umc6619.Device { return b.audio }
func (b *SoundBus) IRQAsserted() bool      { return b.irqStatus&b.irqEnable != 0 }
func (b *SoundBus) IRQStatus() uint8       { return b.irqStatus }
func (b *SoundBus) SetPad(player int, activeLow uint16) {
	b.pads[player&1] = activeLow
}
func (b *SoundBus) Pad(player int) uint16         { return b.pads[player&1] }
func (b *SoundBus) SetIRQ6Handler(handler func()) { b.onIRQ6 = handler }

func (b *SoundBus) Reset() {
	pads, onIRQ6 := b.pads, b.onIRQ6
	b.audio.Reset()
	b.irqEnable, b.irqStatus = 0, 0
	b.io = [256]uint8{}
	b.shiftCtrl = 0
	b.shiftRegs = [2]uint8{}
	b.latched = [2]uint16{}
	b.latch = [2]uint8{}
	b.latchFull = [2]bool{}
	b.pads, b.onIRQ6 = pads, onIRQ6
}

// RequestFrom68K raises the level-held sound command source (IRQ bit 5).
func (b *SoundBus) RequestFrom68K() { b.irqStatus |= 0x20 }

// WriteFrom68K supplies the two byte latches exposed through the shared
// $E80404/$E80405 window. Other shared RAM writes require no side effect.
func (b *SoundBus) WriteFrom68K(address uint16, value uint8) {
	if address != 0x0404 && address != 0x0405 {
		return
	}
	index := int(address - 0x0404)
	b.latch[index], b.latchFull[index] = value, true
	if index == 0 {
		b.irqStatus |= 0x08
	} else {
		b.irqStatus |= 0x04
	}
}

func (b *SoundBus) Read8(address uint16) (uint8, error) {
	if address < 0x0400 || address > 0x04ff {
		return b.ram[address], nil
	}
	switch address {
	case 0x0402, 0x0403:
		return b.shiftRegs[address-0x0402], nil
	case 0x0404, 0x0405:
		index := int(address - 0x0404)
		if index == 0 {
			b.irqStatus &^= 0x08
		} else {
			b.irqStatus &^= 0x04
		}
		if b.latchFull[index] {
			b.latchFull[index] = false
			return b.latch[index], nil
		}
		return 0xcd, nil
	case 0x0406:
		return 0, nil
	case 0x0409:
		b.irqStatus &^= 0x10
		return 0, nil
	case 0x040a:
		b.irqStatus &^= 0x20
		return b.ram[address], nil
	case 0x0410:
		return b.irqEnable, nil
	case 0x0411:
		return b.irqStatus, nil
	case 0x0412:
		return 0, nil
	case 0x0420:
		return b.audio.Status(), nil
	case 0x0422:
		return b.audio.ReadData(), nil
	default:
		return b.io[address&0xff], nil
	}
}

func (b *SoundBus) Write8(address uint16, value uint8) error {
	if address < 0x0400 || address > 0x04ff {
		b.ram[address] = value
		return nil
	}
	switch address {
	case 0x0407:
		lowered := b.shiftCtrl &^ value
		b.shiftCtrl = value
		for player := 0; player < 2; player++ {
			if lowered&(1<<player) != 0 {
				b.latched[player] = b.pads[player]
			}
			if lowered&(4<<player) != 0 {
				b.shiftRegs[player] = b.shiftRegs[player]<<1 | uint8(b.latched[player]>>15)
				b.latched[player] <<= 1
			}
			if lowered&(0x10<<player) != 0 {
				b.shiftRegs[player] = 0
				if player == 0 {
					b.irqStatus |= 0x08
				} else {
					b.irqStatus |= 0x04
				}
			}
		}
	case 0x040a:
		b.ram[address] = value
		if b.onIRQ6 != nil {
			b.onIRQ6()
		}
	case 0x0410:
		b.irqEnable = value
	case 0x0420:
		b.audio.WriteAddress(value)
	case 0x0422:
		b.audio.WriteData(value)
	default:
		b.io[address&0xff] = value
	}
	return nil
}

type SoundTimeline struct {
	Cycles    uint64
	Last      m65c02.Cycle
	OnAdvance func(cycles uint64)
}

func (t *SoundTimeline) Advance(cycle m65c02.Cycle) error {
	t.Cycles++
	t.Last = cycle
	if t.OnAdvance != nil {
		t.OnAdvance(1)
	}
	return nil
}

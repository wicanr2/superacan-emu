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
}

func newSoundBus(ram *[65536]byte) *SoundBus {
	bus := &SoundBus{ram: ram, audio: umc6619.New()}
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

func (b *SoundBus) Read8(address uint16) (uint8, error) {
	if address < 0x0400 || address > 0x04ff {
		return b.ram[address], nil
	}
	switch address {
	case 0x0404, 0x0405:
		return 0xcd, nil
	case 0x0410:
		return b.irqEnable, nil
	case 0x0411:
		return b.irqStatus, nil
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

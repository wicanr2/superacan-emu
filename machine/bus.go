package machine

import (
	"fmt"

	"github.com/wicanr2/superacan-emu/chip/umc6650"
)

const (
	IPLSize  = 4096
	SRAMSize = 32768
)

// Bus is the 68000-visible 24-bit system bus. Video, audio and DMA windows are
// deliberately fail-closed as unmapped until their independent chips attach.
type Bus struct {
	rom      []byte
	ipl      [IPLSize]byte
	soundRAM [65536]byte
	workRAM  [65536]byte
	sram     [SRAMSize]byte

	lockout *umc6650.Device
	e90b3c  uint16
	control uint16
	loOff   bool
	hiOff   bool
}

func NewBus(ipl, rom, key []byte) (*Bus, error) {
	if len(ipl) != IPLSize {
		return nil, fmt.Errorf("machine: IPL size %d, want %d", len(ipl), IPLSize)
	}
	if len(rom) == 0 {
		return nil, fmt.Errorf("machine: empty cartridge ROM")
	}
	if len(key) != umc6650.KeySize {
		return nil, fmt.Errorf("machine: UMC6650 key size %d, want %d", len(key), umc6650.KeySize)
	}
	b := &Bus{rom: append([]byte(nil), rom...), lockout: umc6650.New(key)}
	copy(b.ipl[:], ipl)
	return b, nil
}

func (b *Bus) Lockout() *umc6650.Device { return b.lockout }
func (b *Bus) LowOverlayEnabled() bool  { return !b.loOff }
func (b *Bus) HighOverlayEnabled() bool { return !b.hiOff }
func (b *Bus) Control() uint16          { return b.control }

func (b *Bus) romByte(offset uint32) uint8 {
	if offset < uint32(len(b.rom)) {
		return b.rom[offset]
	}
	return 0xff
}

func (b *Bus) Read8(address uint32) (uint8, error) {
	address &= 0x00ff_ffff
	switch {
	case address < 0x400000:
		if address < IPLSize && !b.loOff {
			return b.ipl[address], nil
		}
		return b.romByte(address), nil
	case address >= 0xe80000 && address < 0xe90000:
		return b.soundRAM[address&0xffff], nil
	case address == 0xe9001c:
		return uint8(b.control >> 8), nil
	case address == 0xe9001d:
		return uint8(b.control), nil
	case address == 0xe90b3c:
		return uint8(b.e90b3c >> 8), nil
	case address == 0xe90b3d:
		return uint8(b.e90b3c), nil
	case address == 0xeb0d01:
		return b.lockout.ReadData(), nil
	case address >= 0xeb0d00 && address <= 0xeb0d03:
		return 0xff, nil
	case address >= 0xec0000 && address < 0xed0000:
		if address&1 == 0 {
			return 0xff, nil
		}
		return b.sram[(address&0xffff)>>1], nil
	case address >= 0xf80000 && address < 0xfc0000:
		offset := address & 0x3ffff
		if offset < IPLSize && !b.hiOff {
			return b.ipl[offset], nil
		}
		return b.romByte(offset), nil
	case address >= 0xfc0000:
		return b.workRAM[address&0xffff], nil
	default:
		return 0xff, nil
	}
}

func (b *Bus) Read16(address uint32) (uint16, error) {
	hi, err := b.Read8(address)
	if err != nil {
		return 0, err
	}
	lo, err := b.Read8(address + 1)
	return uint16(hi)<<8 | uint16(lo), err
}

func (b *Bus) Write8(address uint32, value uint8) error {
	address &= 0x00ff_ffff
	switch {
	case address >= 0xe80000 && address < 0xe90000:
		b.soundRAM[address&0xffff] = value
	case address == 0xe9001c:
		b.setControl(b.control&0x00ff | uint16(value)<<8)
	case address == 0xe9001d:
		b.setControl(b.control&0xff00 | uint16(value))
	case address == 0xe90b3c:
		b.e90b3c = b.e90b3c&0x00ff | uint16(value)<<8
	case address == 0xe90b3d:
		b.e90b3c = b.e90b3c&0xff00 | uint16(value)
	case address == 0xeb0d03:
		b.lockout.WriteAddress(value)
	case address == 0xeb0d01:
		b.lockout.WriteData(value)
	case address >= 0xec0000 && address < 0xed0000:
		if address&1 != 0 {
			b.sram[(address&0xffff)>>1] = value
		}
	case address >= 0xfc0000:
		b.workRAM[address&0xffff] = value
	}
	return nil
}

func (b *Bus) Write16(address uint32, value uint16) error {
	if err := b.Write8(address, uint8(value>>8)); err != nil {
		return err
	}
	return b.Write8(address+1, uint8(value))
}

func (b *Bus) setControl(value uint16) {
	b.control = value
	if value&0x0002 != 0 {
		b.loOff = true
	}
	if value&0x0008 != 0 {
		b.hiOff = true
	}
}

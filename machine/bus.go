package machine

import (
	"fmt"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6650"
)

const (
	IPLSize  = 4096
	SRAMSize = 32768
)

// Bus is the 68000-visible 24-bit system bus. Implemented windows route to
// independent devices; unknown audio and DMA registers remain fail-closed.
type Bus struct {
	rom      []byte
	ipl      [IPLSize]byte
	soundRAM [65536]byte
	workRAM  [65536]byte
	sram     [SRAMSize]byte

	lockout         *umc6650.Device
	video           *umc6618.Device
	e90b3c          uint16
	control         uint16
	loOff           bool
	hiOff           bool
	observer        func(Transaction)
	controlObserver func(oldValue, newValue uint16) error
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
	b := &Bus{rom: append([]byte(nil), rom...), lockout: umc6650.New(key), video: umc6618.New()}
	copy(b.ipl[:], ipl)
	return b, nil
}

func (b *Bus) Lockout() *umc6650.Device { return b.lockout }
func (b *Bus) Video() *umc6618.Device   { return b.video }
func (b *Bus) LowOverlayEnabled() bool  { return !b.loOff }
func (b *Bus) HighOverlayEnabled() bool { return !b.hiOff }
func (b *Bus) Control() uint16          { return b.control }

// SetObserver installs an optional synchronous observer for complete CPU bus
// transactions. A 16-bit access produces one notification, even though plain
// RAM storage is backed by bytes.
func (b *Bus) SetObserver(observer func(Transaction)) { b.observer = observer }

func (b *Bus) setControlObserver(observer func(oldValue, newValue uint16) error) {
	b.controlObserver = observer
}

func (b *Bus) observe(transaction Transaction) {
	if b.observer != nil {
		b.observer(transaction)
	}
}

func (b *Bus) romByte(offset uint32) uint8 {
	if offset < uint32(len(b.rom)) {
		return b.rom[offset]
	}
	return 0xff
}

func (b *Bus) Read8(address uint32) (uint8, error) {
	address &= 0x00ff_ffff
	value, err := b.read8(address)
	if err == nil {
		b.observe(Transaction{Address: address, Width: 1, Value: uint16(value)})
	}
	return value, err
}

func (b *Bus) read8(address uint32) (uint8, error) {
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
	case address == 0xe90010 || address == 0xe90011:
		return b.video.IRQMask(), nil
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
	case address >= 0xf00000 && address < 0xf00200:
		value := b.video.ReadRegister(uint16(address&0x1ff) >> 1)
		if address&1 != 0 {
			return uint8(value), nil
		}
		return uint8(value >> 8), nil
	case address >= 0xf00200 && address < 0xf00400:
		value := b.video.ReadPalette(uint16(address&0x1ff) >> 1)
		if address&1 != 0 {
			return uint8(value), nil
		}
		return uint8(value >> 8), nil
	case address >= 0xf40000 && address < 0xf60000:
		return b.video.ReadVRAM8(address & 0x1ffff), nil
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
	address &= 0x00ff_ffff
	var value uint16
	switch {
	case address >= 0xf00000 && address < 0xf00200:
		value = b.video.ReadRegister(uint16(address&0x1ff) >> 1)
	case address >= 0xf00200 && address < 0xf00400:
		value = b.video.ReadPalette(uint16(address&0x1ff) >> 1)
	case address >= 0xf40000 && address < 0xf60000:
		value = b.video.ReadVRAM16(address & 0x1ffff)
	default:
		hi, err := b.read8(address)
		if err != nil {
			return 0, err
		}
		lo, err := b.read8((address + 1) & 0x00ff_ffff)
		if err != nil {
			return 0, err
		}
		value = uint16(hi)<<8 | uint16(lo)
	}
	b.observe(Transaction{Address: address, Width: 2, Value: value})
	return value, nil
}

func (b *Bus) Write8(address uint32, value uint8) error {
	address &= 0x00ff_ffff
	if err := b.write8(address, value); err != nil {
		return err
	}
	b.observe(Transaction{Address: address, Width: 1, Write: true, Value: uint16(value)})
	return nil
}

func (b *Bus) write8(address uint32, value uint8) error {
	switch {
	case address >= 0xe80000 && address < 0xe90000:
		b.soundRAM[address&0xffff] = value
	case address == 0xe9001c:
		return b.setControl(b.control&0x00ff | uint16(value)<<8)
	case address == 0xe9001d:
		return b.setControl(b.control&0xff00 | uint16(value))
	case address == 0xe90010 || address == 0xe90011:
		b.video.SetIRQMask(value)
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
	case address >= 0xf00000 && address < 0xf00200:
		index := uint16(address&0x1ff) >> 1
		word := b.video.ReadRegister(index)
		if address&1 != 0 {
			word = word&0xff00 | uint16(value)
		} else {
			word = word&0x00ff | uint16(value)<<8
		}
		b.video.WriteRegister(index, word)
	case address >= 0xf00200 && address < 0xf00400:
		index := uint16(address&0x1ff) >> 1
		word := b.video.ReadPalette(index)
		if address&1 != 0 {
			word = word&0xff00 | uint16(value)
		} else {
			word = word&0x00ff | uint16(value)<<8
		}
		b.video.WritePalette(index, word)
	case address >= 0xf40000 && address < 0xf60000:
		b.video.WriteVRAM8(address&0x1ffff, value)
	case address >= 0xfc0000:
		b.workRAM[address&0xffff] = value
	}
	return nil
}

func (b *Bus) Write16(address uint32, value uint16) error {
	address &= 0x00ff_ffff
	switch {
	case address >= 0xf00000 && address < 0xf00200:
		b.video.WriteRegister(uint16(address&0x1ff)>>1, value)
	case address >= 0xf00200 && address < 0xf00400:
		b.video.WritePalette(uint16(address&0x1ff)>>1, value)
	case address >= 0xf40000 && address < 0xf60000:
		b.video.WriteVRAM16(address&0x1ffff, value)
	default:
		if err := b.write8(address, uint8(value>>8)); err != nil {
			return err
		}
		if err := b.write8((address+1)&0x00ff_ffff, uint8(value)); err != nil {
			return err
		}
	}
	b.observe(Transaction{Address: address, Width: 2, Write: true, Value: value})
	return nil
}

func (b *Bus) setControl(value uint16) error {
	oldValue := b.control
	b.control = value
	if value&0x0002 != 0 {
		b.loOff = true
	}
	if value&0x0008 != 0 {
		b.hiOff = true
	}
	if b.controlObserver != nil && oldValue != value {
		return b.controlObserver(oldValue, value)
	}
	return nil
}

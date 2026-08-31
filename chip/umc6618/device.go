// Package umc6618 models the Super A'Can video controller as an independent
// device. Rendering, DMA and scanline IRQs build on the register/palette/VRAM
// transaction contract established here.
package umc6618

import "crypto/sha256"

const (
	RegisterCount = 256
	PaletteCount  = 256
	VRAMSize      = 0x20000
)

type Device struct {
	registers [RegisterCount]uint16
	palette   [PaletteCount]uint16
	vram      [VRAMSize]uint8

	videoFlags      uint16
	pixelMode       uint16
	scanline        uint16
	frame           uint64
	vblankIRQ       bool
	spriteDMAStarts uint64
	irqMask         uint8
	lineCycles      uint16
}

func New() *Device { return &Device{} }

func (d *Device) ReadRegister(index uint16) uint16 {
	index &= 0xff
	switch index {
	case 0:
		value := uint16(0)
		if d.scanline >= 240 {
			value |= 0x8000
		}
		if d.frame&1 != 0 {
			value |= 2
		}
		d.vblankIRQ = false
		return value
	case 1:
		return d.scanline
	case 4:
		return d.videoFlags
	case 0xf8:
		return d.pixelMode
	default:
		return d.registers[index]
	}
}

func (d *Device) WriteRegister(index, value uint16) {
	index &= 0xff
	d.registers[index] = value
	switch index {
	case 4:
		d.videoFlags = value
	case 0x0f:
		if value&0x8000 != 0 {
			d.spriteDMAStarts++
		}
	case 0xf8:
		d.pixelMode = value & 0x1f
	}
}

func (d *Device) Register(index uint16) uint16 { return d.registers[index&0xff] }
func (d *Device) VideoFlags() uint16           { return d.videoFlags }
func (d *Device) PixelMode() uint16            { return d.pixelMode }
func (d *Device) SpriteDMAStarts() uint64      { return d.spriteDMAStarts }

func (d *Device) ReadPalette(index uint16) uint16  { return d.palette[index&0xff] }
func (d *Device) WritePalette(index, value uint16) { d.palette[index&0xff] = value }

func (d *Device) ReadVRAM8(offset uint32) uint8         { return d.vram[offset&(VRAMSize-1)] }
func (d *Device) WriteVRAM8(offset uint32, value uint8) { d.vram[offset&(VRAMSize-1)] = value }
func (d *Device) ReadVRAM16(offset uint32) uint16 {
	offset &= VRAMSize - 1
	return uint16(d.vram[offset])<<8 | uint16(d.vram[(offset+1)&(VRAMSize-1)])
}
func (d *Device) WriteVRAM16(offset uint32, value uint16) {
	offset &= VRAMSize - 1
	d.vram[offset] = uint8(value >> 8)
	d.vram[(offset+1)&(VRAMSize-1)] = uint8(value)
}

func (d *Device) VRAMSHA256() [32]byte { return sha256.Sum256(d.vram[:]) }

func (d *Device) NonzeroVRAMBytes() uint32 {
	var count uint32
	for _, value := range d.vram {
		if value != 0 {
			count++
		}
	}
	return count
}

func (d *Device) SetScanline(scanline uint16) { d.scanline = scanline % 262 }
func (d *Device) Scanline() uint16            { return d.scanline }
func (d *Device) TriggerVBlank(enabled bool) {
	d.frame++
	d.vblankIRQ = enabled
}
func (d *Device) VBlankPending() bool { return d.vblankIRQ }

func (d *Device) SetIRQMask(mask uint8) { d.irqMask = mask }
func (d *Device) IRQMask() uint8        { return d.irqMask }
func (d *Device) Frame() uint64         { return d.frame }

// AdvanceM68KCycles advances the 262-line video timing domain. The line
// budgets are the software-observed modes used by the deprecated oracle.
func (d *Device) AdvanceM68KCycles(cycles uint8) {
	d.lineCycles += uint16(cycles)
	budget := uint16(684)
	if d.videoFlags&0x0100 != 0 {
		budget = 728
	}
	for d.lineCycles >= budget {
		d.lineCycles -= budget
		d.scanline = (d.scanline + 1) % 262
		if d.scanline == 240 {
			d.frame++
			if d.irqMask&0x80 != 0 {
				d.vblankIRQ = true
			}
		}
	}
}

// Package umc6650 implements the Super A'Can lockout/security chip.
package umc6650

const KeySize = 16

// Device models the independently addressed internal byte space.
type Device struct {
	memory  [128]byte
	address uint8
}

func New(key []byte) *Device {
	if len(key) != KeySize {
		panic("umc6650: key must contain exactly 16 bytes")
	}
	d := &Device{}
	copy(d.memory[0x20:0x30], key)
	return d
}

func (d *Device) WriteAddress(value uint8) { d.address = value & 0x7f }
func (d *Device) Address() uint8           { return d.address }
func (d *Device) ReadData() uint8          { return d.memory[d.address] }

func (d *Device) WriteData(value uint8) {
	if d.address >= 0x20 && d.address <= 0x2f {
		return
	}
	d.memory[d.address] = value
}

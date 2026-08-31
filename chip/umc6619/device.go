// Package umc6619 models the Super A'Can system/audio controller's indirect
// sound register port. PCM, timer and DMA execution are added as independently
// tested slices; the register file here establishes the observable port bus.
package umc6619

type Device struct {
	address   uint8
	registers [256]uint8
}

func New() *Device { return &Device{} }

// Status bit 0 is the indirect-port busy flag. The first slice has no delayed
// transactions, so it is always ready rather than inventing a wait duration.
func (d *Device) Status() uint8                { return 0 }
func (d *Device) WriteAddress(address uint8)   { d.address = address }
func (d *Device) Address() uint8               { return d.address }
func (d *Device) ReadData() uint8              { return d.registers[d.address] }
func (d *Device) WriteData(value uint8)        { d.registers[d.address] = value }
func (d *Device) Register(address uint8) uint8 { return d.registers[address] }

// Package frc models the Super A'Can host free-running counter/timer exposed
// at $E90014/$E90016. Its period table is MAME-derived and intentionally kept
// separate from confirmed hardware contracts.
package frc

const OneHzM68KCycles int64 = 10_738_635

type Device struct {
	control   uint16
	frequency uint16
	remaining int64
	active    bool
	pending   bool
	supported bool
}

func New() *Device { return &Device{} }

func (d *Device) Reset()                 { *d = Device{} }
func (d *Device) Control() uint16        { return d.control }
func (d *Device) Frequency() uint16      { return d.frequency }
func (d *Device) Pending() bool          { return d.pending }
func (d *Device) Active() bool           { return d.active }
func (d *Device) SupportedMode() bool    { return d.supported }
func (d *Device) RemainingCycles() int64 { return d.remaining }

func (d *Device) WriteControl(value uint16) {
	d.control = value
	d.schedule()
}

func (d *Device) WriteFrequency(value uint16) {
	d.frequency = value
	d.schedule()
}

func (d *Device) AdvanceM68KCycles(cycles uint64) {
	if !d.active || d.pending {
		return
	}
	d.remaining -= int64(cycles)
	if d.remaining <= 0 {
		d.remaining = 0
		d.active = false
		d.pending = true
	}
}

// Acknowledge applies HOLD_LINE semantics and starts a fresh period from the
// acknowledge boundary, matching the pinned MAME-derived behavior.
func (d *Device) Acknowledge() {
	if !d.pending {
		return
	}
	d.pending = false
	d.schedule()
}

func (d *Device) schedule() {
	d.active = false
	d.supported = false
	d.remaining = 0
	if d.control&0xff00 != 0xa200 {
		return
	}
	period := int64(uint32(d.control&0x00ff)<<16 | uint32(d.frequency))
	var cycles int64
	switch d.control & 0x000f {
	case 0:
		cycles = OneHzM68KCycles
	case 1:
		cycles = 1024 * period
	case 0x0f:
		cycles = 8192 * period
	default:
		return
	}
	d.supported = true
	if cycles <= 0 {
		return
	}
	d.remaining = cycles
	d.active = true
}

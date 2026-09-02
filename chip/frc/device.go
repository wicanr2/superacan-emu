// Package frc models the Super A'Can host counter/timer exposed at
// $E90014/$E90016. The period formula is decompiled from Bcan 0.0.8b
// (sub_1400A7420/sub_1400A7510); see the knowledge base, docs/memory-map.md.
package frc

const (
	// The counter's input clock is master/12. Bcan expresses every interval in
	// master ticks, so this device does too and converts at the call boundary.
	masterTicksPerM68KCycle int64 = 10
	oneSecondMasterTicks    int64 = 107_386_350
	// Mode $1 counts 1024 input ticks per unit, mode $F counts 9040; ×12 turns
	// each into master ticks.
	mode1MasterTicks int64 = 12 * 1024
	modeFMasterTicks int64 = 12 * 9040
)

type Device struct {
	control   uint16
	frequency uint16
	remaining int64
	active    bool
	pending   bool
	supported bool
}

func New() *Device { return &Device{} }

func (d *Device) Reset()              { *d = Device{} }
func (d *Device) Control() uint16     { return d.control }
func (d *Device) Frequency() uint16   { return d.frequency }
func (d *Device) Pending() bool       { return d.pending }
func (d *Device) Active() bool        { return d.active }
func (d *Device) SupportedMode() bool { return d.supported }

// RemainingMasterTicks 是距離下一次觸發的主時脈 tick 數（master = 68k × 10）。
func (d *Device) RemainingMasterTicks() int64 { return d.remaining }

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
	d.remaining -= int64(cycles) * masterTicksPerM68KCycle
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
	// Bcan：控制暫存器高位元組必須是 $A2，低 4 bit 選速率，週期一律算 (n+1)。
	// 常數是 1024×master 與 9040×master 除以 ⌊master/12⌋，C 會約掉，剩下純 tick 數。
	period := int64(d.frequency) + 1
	var cycles int64
	switch d.control & 0x000f {
	case 0:
		cycles = oneSecondMasterTicks // 固定一秒，與週期暫存器無關
	case 1:
		cycles = mode1MasterTicks * period
	case 0x0f:
		cycles = modeFMasterTicks * period
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

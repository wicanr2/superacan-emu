// Package umc6619 models the Super A'Can PCM audio controller as an
// independent device in the W65C02 clock domain.
package umc6619

const (
	ClockHz         = 3_579_545
	CyclesPerSample = 80
	TimerIRQ        = 0x80
	DMAIRQ          = 0x40
)

type Sample struct{ Left, Right int16 }

type channel struct {
	pitch       uint16
	increment   uint32
	start       uint16
	length      uint16
	current     uint16
	end         uint16
	fraction    uint32
	dma         uint8
	volumeLeft  uint8
	volumeRight uint8
	oneShot     bool
}

type ChannelState struct {
	Pitch, Start, Length, Current uint16
	Increment, Fraction           uint32
	DMA, VolumeLeft, VolumeRight  uint8
	OneShot                       bool
	Active                        bool
}

type Device struct {
	address   uint8
	registers [256]uint8
	channels  [16]channel
	active    uint16

	sampleCycles   uint64
	timerRemaining int64
	timerRunning   bool
	timerIRQ       bool
	dmaIRQ         bool

	readRAM     func(uint16) uint8
	onSample    func(Sample)
	onIRQ       func(mask uint8, asserted bool)
	sampleCount uint64
}

func New() *Device { return &Device{} }

func (d *Device) Reset() {
	readRAM, onSample, onIRQ := d.readRAM, d.onSample, d.onIRQ
	if onIRQ != nil {
		if d.timerIRQ {
			onIRQ(TimerIRQ, false)
		}
		if d.dmaIRQ {
			onIRQ(DMAIRQ, false)
		}
	}
	*d = Device{readRAM: readRAM, onSample: onSample, onIRQ: onIRQ}
}

func (d *Device) SetRAMReader(reader func(uint16) uint8)  { d.readRAM = reader }
func (d *Device) SetSampleSink(sink func(Sample))         { d.onSample = sink }
func (d *Device) SetIRQHandler(handler func(uint8, bool)) { d.onIRQ = handler }
func (d *Device) Status() uint8                           { return 0 }
func (d *Device) WriteAddress(address uint8)              { d.address = address }
func (d *Device) Address() uint8                          { return d.address }
func (d *Device) Register(address uint8) uint8            { return d.registers[address] }
func (d *Device) ActiveChannels() uint16                  { return d.active }
func (d *Device) SampleCount() uint64                     { return d.sampleCount }
func (d *Device) TimerPending() bool                      { return d.timerIRQ }
func (d *Device) DMAPending() bool                        { return d.dmaIRQ }

func (d *Device) ReadData() uint8 {
	value := d.registers[d.address]
	switch d.address {
	case 0x14:
		d.setIRQ(TimerIRQ, false)
	case 0x16:
		d.setIRQ(DMAIRQ, false)
		value &^= DMAIRQ
		if d.dmaBusy() {
			value |= DMAIRQ
		}
	}
	return value
}

func (d *Device) WriteData(value uint8) {
	d.registers[d.address] = value
	d.applyRegister(d.address, value)
}

func (d *Device) Channel(index uint8) ChannelState {
	c := d.channels[index&15]
	return ChannelState{
		Pitch: c.pitch, Start: c.start, Length: c.length, Current: c.current,
		Increment: c.increment, Fraction: c.fraction, DMA: c.dma,
		VolumeLeft: c.volumeLeft, VolumeRight: c.volumeRight, OneShot: c.oneShot,
		Active: d.active&(1<<(index&15)) != 0,
	}
}

func (d *Device) applyRegister(address, value uint8) {
	channelIndex := address & 15
	c := &d.channels[channelIndex]
	switch address >> 4 {
	case 0x1:
		switch address {
		case 0x14:
			if value&0x80 != 0 {
				d.timerRunning = true
				d.timerRemaining = d.timerPeriod()
			}
		case 0x17:
			channelIndex = value & 15
			if value&0xf0 != 0 {
				d.keyOn(channelIndex)
			} else {
				d.active &^= 1 << channelIndex
			}
		}
	case 0x2:
		c.pitch = c.pitch&0xff00 | uint16(value)
		c.increment = uint32(c.pitch) << 6
	case 0x3:
		c.pitch = c.pitch&0x00ff | uint16(value)<<8
		c.increment = uint32(c.pitch) << 6
	case 0x5:
		c.length = uint16(0x40 << ((value & 0x0e) >> 1))
		c.oneShot = value&1 != 0
	case 0x6:
		c.start = c.start&0x00ff | uint16(value)<<8
	case 0x7:
		c.start = c.start&0xff00 | uint16(value)
	case 0x9:
		c.dma = value
	case 0xe:
		c.volumeLeft = value&0xf0 | value>>4
		c.volumeRight = value&0x0f | value<<4
	}
}

func (d *Device) keyOn(index uint8) {
	c := &d.channels[index&15]
	c.current = c.start << 6
	c.end = c.current + c.length
	c.fraction = 0
	d.active |= 1 << (index & 15)
}

func (d *Device) timerPeriod() int64 {
	value := uint16(d.registers[0x12])<<8 | uint16(d.registers[0x11])
	return int64(10 * (0x10000 - uint32(value)))
}

func (d *Device) Advance(cycles uint64) {
	if d.timerRunning {
		d.timerRemaining -= int64(cycles)
		period := d.timerPeriod()
		for d.timerRemaining <= 0 {
			d.timerRemaining += period
			if d.registers[0x14]&0x40 != 0 {
				d.setIRQ(TimerIRQ, true)
			}
		}
	}
	d.sampleCycles += cycles
	for d.sampleCycles >= CyclesPerSample {
		d.sampleCycles -= CyclesPerSample
		d.mixSample()
	}
}

func (d *Device) mixSample() {
	var left, right int32
	for index := uint8(0); index < 16; index++ {
		if d.active&(1<<index) == 0 {
			continue
		}
		c := &d.channels[index]
		value := uint8(0x80)
		if d.readRAM != nil {
			value = d.readRAM(c.current)
		}
		sample := int32(int(value)-128) << 8
		left += sample * int32(c.volumeLeft) >> 8
		right += sample * int32(c.volumeRight) >> 8
		c.fraction += c.increment
		c.current += uint16(c.fraction >> 16)
		c.fraction &= 0xffff
		if c.current >= c.end {
			switch {
			case c.dma != 0:
				d.setIRQ(DMAIRQ, true)
				d.keyOn(index)
			case c.oneShot:
				d.active &^= 1 << index
			case c.length != 0:
				c.current -= c.length
			}
		}
	}
	d.sampleCount++
	if d.onSample != nil {
		d.onSample(Sample{Left: clamp16(left >> 1), Right: clamp16(right >> 1)})
	}
}

func clamp16(value int32) int16 {
	if value < -32768 {
		return -32768
	}
	if value > 32767 {
		return 32767
	}
	return int16(value)
}

func (d *Device) dmaBusy() bool {
	for index := uint8(0); index < 16; index++ {
		if d.active&(1<<index) != 0 && d.channels[index].dma != 0 {
			return true
		}
	}
	return false
}

func (d *Device) setIRQ(mask uint8, asserted bool) {
	pending := &d.dmaIRQ
	if mask == TimerIRQ {
		pending = &d.timerIRQ
	}
	if *pending == asserted {
		return
	}
	*pending = asserted
	if d.onIRQ != nil {
		d.onIRQ(mask, asserted)
	}
}

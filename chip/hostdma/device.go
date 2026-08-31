// Package hostdma models the two Super A'Can host DMA channels at
// $E90020-$E9003F. Transfer semantics are MAME-derived; unknown timing remains
// explicit because the available contract executes the bus burst at trigger.
package hostdma

import "fmt"

type Bus interface {
	Read8(address uint32) (uint8, error)
	Read16(address uint32) (uint16, error)
	Write8(address uint32, value uint8) error
	Write16(address uint32, value uint16) error
}

type ChannelState struct {
	Source, Destination uint32
	Count, Control      uint16
	Triggers            uint64
}

type Device struct {
	bus      Bus
	channels [2]ChannelState
	busy     bool
}

func New(bus Bus) *Device {
	if bus == nil {
		panic("hostdma: nil bus")
	}
	return &Device{bus: bus}
}

func (d *Device) Reset()                         { d.channels = [2]ChannelState{}; d.busy = false }
func (d *Device) Channel(index int) ChannelState { return d.channels[index&1] }

func (d *Device) ReadRegister(channel, index int) uint16 {
	c := d.channels[channel&1]
	switch index & 7 {
	case 0:
		return uint16(c.Source >> 16)
	case 1:
		return uint16(c.Source)
	case 2:
		return uint16(c.Destination >> 16)
	case 3:
		return uint16(c.Destination)
	case 4:
		return c.Count
	case 5:
		return c.Control
	default:
		return 0
	}
}

func (d *Device) WriteRegister(channel, index int, value uint16) error {
	c := &d.channels[channel&1]
	switch index & 7 {
	case 0:
		c.Source = c.Source&0x0000ffff | uint32(value)<<16
	case 1:
		c.Source = c.Source&0xffff0000 | uint32(value)
	case 2:
		c.Destination = c.Destination&0x0000ffff | uint32(value)<<16
	case 3:
		c.Destination = c.Destination&0xffff0000 | uint32(value)
	case 4:
		c.Count = value
	case 5:
		c.Control = value
		if value&0x8800 != 0 {
			return d.trigger(channel & 1)
		}
	}
	return nil
}

// WriteRegisterByte performs big-endian RMW. Control triggers only after its
// low byte arrives, so a CPU word transaction cannot start two bursts.
func (d *Device) WriteRegisterByte(channel, index int, lowByte bool, value uint8) error {
	word := d.ReadRegister(channel, index)
	if lowByte {
		word = word&0xff00 | uint16(value)
	} else {
		word = word&0x00ff | uint16(value)<<8
	}
	if index&7 == 5 && !lowByte {
		d.channels[channel&1].Control = word
		return nil
	}
	return d.WriteRegister(channel, index, word)
}

func (d *Device) trigger(channel int) error {
	if d.busy {
		return fmt.Errorf("hostdma: reentrant trigger on channel %d", channel)
	}
	d.busy = true
	defer func() { d.busy = false }()
	c := &d.channels[channel]
	c.Triggers++
	control := c.Control
	destinationDecrement := control&0x0400 != 0
	sourceDecrement := control&0x0200 != 0
	for index := uint32(0); index <= uint32(c.Count); index++ {
		switch {
		case control == 0xa800:
			if c.Destination&0xfe0000 == 0xf40000 {
				if err := d.bus.Write16(c.Destination&0x00ffffff, 0); err != nil {
					return err
				}
				c.Destination = step(c.Destination, 2, destinationDecrement)
			} else {
				value, err := d.bus.Read8((c.Source + (c.Destination & 1)) & 0x00ffffff)
				if err != nil {
					return err
				}
				if err := d.bus.Write8(c.Destination&0x00ffffff, value); err != nil {
					return err
				}
				c.Destination = step(c.Destination, 1, destinationDecrement)
			}
		case control&0x1000 != 0:
			value, err := d.bus.Read16(c.Source & 0x00ffffff)
			if err != nil {
				return err
			}
			if err := d.bus.Write16(c.Destination&0x00ffffff, value); err != nil {
				return err
			}
			c.Destination = step(c.Destination, 2, destinationDecrement)
			c.Source = step(c.Source, 2, sourceDecrement)
			if control&0x0100 != 0 && c.Destination&0x0f == 0 {
				c.Destination -= 0x10
			}
		default:
			value, err := d.bus.Read8(c.Source & 0x00ffffff)
			if err != nil {
				return err
			}
			if err := d.bus.Write8(c.Destination&0x00ffffff, value); err != nil {
				return err
			}
			c.Destination = step(c.Destination, 1, destinationDecrement)
			c.Source = step(c.Source, 1, sourceDecrement)
		}
	}
	return nil
}

func step(address uint32, amount uint32, decrement bool) uint32 {
	if decrement {
		return address - amount
	}
	return address + amount
}

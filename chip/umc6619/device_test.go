package umc6619

import "testing"

func writeRegister(device *Device, address, value uint8) {
	device.WriteAddress(address)
	device.WriteData(value)
}

func TestIndirectRegisterPort(t *testing.T) {
	device := New()
	writeRegister(device, 0x07, 0x80)
	if device.Status() != 0 || device.Address() != 0x07 || device.ReadData() != 0x80 || device.Register(0x07) != 0x80 {
		t.Fatalf("address=$%02X data=$%02X", device.Address(), device.ReadData())
	}
}

func TestPCMChannelUsesKeyDataChannelAndNativeSampleClock(t *testing.T) {
	device := New()
	ram := [65536]uint8{}
	ram[0x1000] = 0xff
	device.SetRAMReader(func(address uint16) uint8 { return ram[address] })
	var samples []Sample
	device.SetSampleSink(func(sample Sample) { samples = append(samples, sample) })

	writeRegister(device, 0x23, 0x00)
	writeRegister(device, 0x33, 0x04)
	writeRegister(device, 0x53, 0x01)
	writeRegister(device, 0x63, 0x00)
	writeRegister(device, 0x73, 0x40)
	writeRegister(device, 0xe3, 0xff)
	writeRegister(device, 0x17, 0x13)
	if device.ActiveChannels() != 1<<3 || !device.Channel(3).Active || device.Channel(7).Active {
		t.Fatalf("active=$%04X", device.ActiveChannels())
	}
	device.Advance(79)
	if len(samples) != 0 {
		t.Fatal("sample emitted before 80 clocks")
	}
	device.Advance(1)
	if len(samples) != 1 || samples[0] != (Sample{Left: 16192, Right: 16192}) {
		t.Fatalf("samples=%v", samples)
	}
	if state := device.Channel(3); state.Current != 0x1001 || state.Increment != 0x10000 {
		t.Fatalf("channel=%+v", state)
	}
	device.Advance(63 * CyclesPerSample)
	if device.Channel(3).Active {
		t.Fatal("one-shot channel did not stop at configured length")
	}
}

func TestTimerIRQIsLevelHeldUntilRegisterRead(t *testing.T) {
	device := New()
	var transitions []struct {
		mask     uint8
		asserted bool
	}
	device.SetIRQHandler(func(mask uint8, asserted bool) {
		transitions = append(transitions, struct {
			mask     uint8
			asserted bool
		}{mask, asserted})
	})
	writeRegister(device, 0x11, 0xff)
	writeRegister(device, 0x12, 0xff)
	writeRegister(device, 0x14, 0xc0)
	device.Advance(10)
	if !device.TimerPending() || len(transitions) != 1 || transitions[0].mask != TimerIRQ || !transitions[0].asserted {
		t.Fatalf("pending=%v transitions=%v", device.TimerPending(), transitions)
	}
	device.Advance(20)
	if len(transitions) != 1 {
		t.Fatal("held IRQ emitted duplicate assertion")
	}
	device.WriteAddress(0x14)
	if got := device.ReadData(); got != 0xc0 || device.TimerPending() || len(transitions) != 2 || transitions[1].asserted {
		t.Fatalf("read=$%02X pending=%v transitions=%v", got, device.TimerPending(), transitions)
	}
}

func TestDMACompletionIRQBusyAndAcknowledge(t *testing.T) {
	device := New()
	writeRegister(device, 0x20, 0xff)
	writeRegister(device, 0x30, 0xff)
	writeRegister(device, 0x50, 0x00)
	writeRegister(device, 0x90, 0xff)
	writeRegister(device, 0x17, 0x10)
	device.WriteAddress(0x16)
	if device.ReadData()&DMAIRQ == 0 {
		t.Fatal("active DMA channel did not report busy")
	}
	device.Advance(2 * CyclesPerSample)
	if !device.DMAPending() {
		t.Fatal("DMA channel completion did not assert IRQ")
	}
	device.WriteAddress(0x16)
	if got := device.ReadData(); got&DMAIRQ == 0 || device.DMAPending() {
		t.Fatalf("status=$%02X pending=%v", got, device.DMAPending())
	}
}

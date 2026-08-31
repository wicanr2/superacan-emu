package hostdma

import "testing"

type testBus struct{ memory [1 << 16]byte }

func (b *testBus) Read8(address uint32) (uint8, error) { return b.memory[address&0xffff], nil }
func (b *testBus) Read16(address uint32) (uint16, error) {
	return uint16(b.memory[address&0xffff])<<8 | uint16(b.memory[(address+1)&0xffff]), nil
}
func (b *testBus) Write8(address uint32, value uint8) error {
	b.memory[address&0xffff] = value
	return nil
}
func (b *testBus) Write16(address uint32, value uint16) error {
	b.memory[address&0xffff], b.memory[(address+1)&0xffff] = uint8(value>>8), uint8(value)
	return nil
}

func TestWordTransferCountPlusOneAndPointerAdvance(t *testing.T) {
	bus := &testBus{}
	copy(bus.memory[0x100:], []byte{0x12, 0x34, 0x56, 0x78})
	d := New(bus)
	_ = d.WriteRegister(0, 1, 0x0100)
	_ = d.WriteRegister(0, 3, 0x0200)
	_ = d.WriteRegister(0, 4, 1)
	if err := d.WriteRegister(0, 5, 0x9000); err != nil {
		t.Fatal(err)
	}
	if got := bus.memory[0x200:0x204]; string(got) != string([]byte{0x12, 0x34, 0x56, 0x78}) {
		t.Fatalf("destination=% X", got)
	}
	c := d.Channel(0)
	if c.Source != 0x104 || c.Destination != 0x204 || c.Triggers != 1 {
		t.Fatalf("channel=%+v", c)
	}
}

func TestControlByteWriteTriggersOnlyOnLowByte(t *testing.T) {
	bus := &testBus{}
	bus.memory[0x10] = 0xa5
	d := New(bus)
	_ = d.WriteRegister(1, 1, 0x0010)
	_ = d.WriteRegister(1, 3, 0x0020)
	if err := d.WriteRegisterByte(1, 5, false, 0x88); err != nil {
		t.Fatal(err)
	}
	if d.Channel(1).Triggers != 0 {
		t.Fatal("high control byte triggered DMA")
	}
	if err := d.WriteRegisterByte(1, 5, true, 0x00); err != nil {
		t.Fatal(err)
	}
	if d.Channel(1).Triggers != 1 || bus.memory[0x20] != 0xa5 {
		t.Fatalf("channel=%+v destination=$%02X", d.Channel(1), bus.memory[0x20])
	}
}

func TestIndirectWordDestinationWrapsEverySixteenBytes(t *testing.T) {
	bus := &testBus{}
	for i := range 16 {
		bus.memory[0x100+i] = byte(i)
	}
	d := New(bus)
	_ = d.WriteRegister(0, 1, 0x0100)
	_ = d.WriteRegister(0, 3, 0x020e)
	_ = d.WriteRegister(0, 4, 3)
	if err := d.WriteRegister(0, 5, 0x9900); err != nil {
		t.Fatal(err)
	}
	if d.Channel(0).Destination != 0x0206 {
		t.Fatalf("destination=$%08X", d.Channel(0).Destination)
	}
}

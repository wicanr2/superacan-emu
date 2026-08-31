package umc6650

import "testing"

func TestPortsKeyRAMAndAddressMask(t *testing.T) {
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = uint8(0x80 + i)
	}
	d := New(key)
	d.WriteAddress(0xa3)
	if d.Address() != 0x23 || d.ReadData() != 0x83 {
		t.Fatalf("masked key read: address=$%02X data=$%02X", d.Address(), d.ReadData())
	}
	d.WriteData(0)
	if d.ReadData() != 0x83 {
		t.Fatal("key region accepted a write")
	}
	d.WriteAddress(0x5f)
	d.WriteData(0xa5)
	if d.ReadData() != 0xa5 {
		t.Fatalf("RAM readback=$%02X", d.ReadData())
	}
	d.WriteAddress(0x09)
	d.WriteData(0xff)
	if d.ReadData() != 0xff {
		t.Fatalf("output register readback=$%02X", d.ReadData())
	}
}

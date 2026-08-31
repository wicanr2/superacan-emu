package umc6619

import "testing"

func TestIndirectRegisterPort(t *testing.T) {
	device := New()
	device.WriteAddress(0x14)
	device.WriteData(0xcf)
	if device.Status() != 0 || device.Address() != 0x14 || device.ReadData() != 0xcf || device.Register(0x14) != 0xcf {
		t.Fatalf("device=%+v", device)
	}
}

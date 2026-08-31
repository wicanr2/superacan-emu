package machine

import "testing"

func testMachineBus(t *testing.T) *Bus {
	t.Helper()
	ipl := make([]byte, IPLSize)
	rom := make([]byte, 0x400000)
	key := make([]byte, 16)
	ipl[0], ipl[1], ipl[0x604] = 0xfd, 0x00, 0x4e
	rom[0], rom[1], rom[0x604] = 0x12, 0x34, 0xa5
	b, err := NewBus(ipl, rom, key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOverlayLatchesAreIndependentAndOneWay(t *testing.T) {
	b := testMachineBus(t)
	if low, _ := b.Read16(0); low != 0xfd00 {
		t.Fatalf("low IPL=$%04X", low)
	}
	if high, _ := b.Read8(0xf80604); high != 0x4e {
		t.Fatalf("high IPL=$%02X", high)
	}
	if err := b.Write16(0xe9001c, 0x0002); err != nil {
		t.Fatal(err)
	}
	if low, _ := b.Read16(0); low != 0x1234 || !b.HighOverlayEnabled() {
		t.Fatalf("after low disable: low=$%04X high=%v", low, b.HighOverlayEnabled())
	}
	if err := b.Write16(0xe9001c, 0); err != nil {
		t.Fatal(err)
	}
	if b.LowOverlayEnabled() {
		t.Fatal("low overlay re-enabled after control clear")
	}
	if err := b.Write16(0xe9001c, 0x0008); err != nil {
		t.Fatal(err)
	}
	if high, _ := b.Read8(0xf80604); high != 0xa5 || b.HighOverlayEnabled() {
		t.Fatalf("after high disable: high=$%02X enabled=%v", high, b.HighOverlayEnabled())
	}
}

func TestLockoutWorkRAMMirrorsAndSRAMLanes(t *testing.T) {
	b := testMachineBus(t)
	if err := b.Write8(0xeb0d03, 0x5f); err != nil {
		t.Fatal(err)
	}
	if err := b.Write8(0xeb0d01, 0xa5); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Read8(0xeb0d01); got != 0xa5 {
		t.Fatalf("lockout RAM=$%02X", got)
	}
	if err := b.Write8(0xfc1234, 0x6a); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Read8(0xff1234); got != 0x6a {
		t.Fatalf("Work RAM mirror=$%02X", got)
	}
	_ = b.Write8(0xec0000, 0x11)
	_ = b.Write8(0xec0001, 0x22)
	if even, _ := b.Read8(0xec0000); even != 0xff {
		t.Fatalf("SRAM even lane=$%02X", even)
	}
	if odd, _ := b.Read8(0xec0001); odd != 0x22 {
		t.Fatalf("SRAM odd lane=$%02X", odd)
	}
}

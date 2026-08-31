package umc6618

import "testing"

func TestRegisterPaletteAndVRAMStorage(t *testing.T) {
	device := New()
	device.WriteRegister(4, 0x01a8)
	device.WriteRegister(0xf8, 0xffff)
	device.WritePalette(7, 0x7c1f)
	device.WriteVRAM16(0x4400, 0x1234)
	if device.ReadRegister(4) != 0x01a8 || device.VideoFlags() != 0x01a8 {
		t.Fatalf("video flags=$%04X", device.VideoFlags())
	}
	if device.ReadRegister(0xf8) != 0x001f || device.ReadPalette(7) != 0x7c1f {
		t.Fatalf("pixel=$%04X palette=$%04X", device.ReadRegister(0xf8), device.ReadPalette(7))
	}
	if device.ReadVRAM16(0x4400) != 0x1234 || device.ReadVRAM8(0x4401) != 0x34 {
		t.Fatalf("VRAM=$%04X", device.ReadVRAM16(0x4400))
	}
}

func TestIRQStatusReadAndSingleSpriteDMATrigger(t *testing.T) {
	device := New()
	device.SetScanline(240)
	device.TriggerVBlank(true)
	if status := device.ReadRegister(0); status&0x8000 == 0 || status&2 == 0 {
		t.Fatalf("status=$%04X", status)
	}
	if device.VBlankPending() {
		t.Fatal("status read did not acknowledge vblank")
	}
	device.WriteRegister(0x0f, 0x8000)
	if device.SpriteDMAStarts() != 1 {
		t.Fatalf("sprite DMA starts=%d", device.SpriteDMAStarts())
	}
}

func TestScanlineTimingUsesVideoWidthAndRaisesMaskedVBlank(t *testing.T) {
	device := New()
	device.SetIRQMask(0x80)
	for range 239 {
		for range 4 {
			device.AdvanceM68KCycles(171)
		}
	}
	if device.Scanline() != 239 || device.Frame() != 0 {
		t.Fatalf("before vblank line=%d frame=%d", device.Scanline(), device.Frame())
	}
	for range 4 {
		device.AdvanceM68KCycles(171)
	}
	if device.Scanline() != 240 || device.Frame() != 1 || !device.VBlankPending() {
		t.Fatalf("vblank line=%d frame=%d pending=%v", device.Scanline(), device.Frame(), device.VBlankPending())
	}
	device.WriteRegister(4, 0x0100)
	before := device.Scanline()
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(217)
	if device.Scanline() != before {
		t.Fatal("320-wide line advanced before 728 cycles")
	}
	device.AdvanceM68KCycles(1)
	if device.Scanline() != (before+1)%262 {
		t.Fatal("320-wide line did not advance at 728 cycles")
	}
}

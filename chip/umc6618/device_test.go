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

func TestRenderWindowAndBlankedWideArea(t *testing.T) {
	device := New()
	device.WritePalette(1, 0x001f)
	device.WriteRegister(4, 0x0002)
	device.WriteRegister(0xe8, 0x0001)
	device.WriteRegister(0xe9, 0x0100)
	device.WriteVRAM16(0x0400, 0)
	device.WriteVRAM16(0x0402, 2)

	device.RenderFrame()
	frame := device.Framebuffer()
	if frame[0] != 0xfff80000 || frame[1] != 0xfff80000 {
		t.Fatalf("window pixels=$%08X,$%08X", frame[0], frame[1])
	}
	if frame[2] != 0xff000000 || frame[256] != 0xff000000 {
		t.Fatalf("blank pixels=$%08X,$%08X", frame[2], frame[256])
	}
	if count := device.NonblackPixels(); count != Height*2 {
		t.Fatalf("nonblack pixels=%d", count)
	}
}

func TestTilePixelPackedModes(t *testing.T) {
	device := New()
	device.WriteVRAM8(0, 0xe4)
	if got := device.tilePixel(0, 0, 0, 0); got != 0xe4 {
		t.Fatalf("8bpp pixel=$%02X", got)
	}
	if low, high := device.tilePixel(1, 0, 0, 0), device.tilePixel(1, 0, 1, 0); low != 4 || high != 0x0e {
		t.Fatalf("4bpp pixels=$%X,$%X", low, high)
	}
	for x, want := range []uint8{0, 1, 2, 3} {
		if got := device.tilePixel(2, 0, x, 0); got != want {
			t.Fatalf("2bpp x=%d pixel=%d want=%d", x, got, want)
		}
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

func TestSpriteDMACopyAndZeroFill(t *testing.T) {
	device := New()
	memory := map[uint32]uint16{0x1000: 0x1234, 0x1002: 0xabcd}
	device.SetDMAAccess(func(address uint32) uint16 { return memory[address] }, func(address uint32, value uint16) { memory[address] = value })
	device.WriteRegister(0x08, 1)
	device.WriteRegister(0x09, 0)
	device.WriteRegister(0x0a, 0x2000)
	device.WriteRegister(0x0b, 1)
	device.WriteRegister(0x0c, 0)
	device.WriteRegister(0x0d, 0x1000)
	device.WriteRegister(0x0e, 1)
	device.WriteRegister(0x0f, 0x8000)
	if memory[0x2000] != 0x1234 || memory[0x2002] != 0xabcd {
		t.Fatalf("copy=$%04X,$%04X", memory[0x2000], memory[0x2002])
	}
	device.WriteRegister(0x08, 0)
	device.WriteRegister(0x0a, 0x3000)
	device.WriteRegister(0x0f, 0x8100)
	if memory[0x3000] != 0 {
		t.Fatalf("fill=$%04X", memory[0x3000])
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

func TestRasterAndProgrammableLineIRQs(t *testing.T) {
	device := New()
	device.SetIRQMask(0x10)
	device.WriteRegister(5, 0x8002)
	device.WriteRegister(6, 0x8003)
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(174)
	if !device.RasterPending() || device.LinePending() || device.HighestIRQLevel() != 4 {
		t.Fatalf("line 1 raster=%v line=%v level=%d", device.RasterPending(), device.LinePending(), device.HighestIRQLevel())
	}
	device.ClearIRQ(4)
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(174)
	if !device.LinePending() || device.HighestIRQLevel() != 5 {
		t.Fatalf("line 2 pending=%v level=%d", device.LinePending(), device.HighestIRQLevel())
	}
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(255)
	device.AdvanceM68KCycles(174)
	if device.LinePending() {
		t.Fatal("line-off target did not clear IRQ5")
	}
}

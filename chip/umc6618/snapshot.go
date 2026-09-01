package umc6618

// Snapshot 是 UM6618 會影響後續輸出的全部狀態。framebuffer 雖然是衍生資料，
// 仍然保存：載入後到下一個 vblank 之間畫面不會重新合成，不存就會顯示上一份內容。
type Snapshot struct {
	Registers       [RegisterCount]uint16
	Palette         [PaletteCount]uint16
	VRAM            [VRAMSize]uint8
	Framebuffer     [Width * Height]uint32
	VideoFlags      uint16
	PixelMode       uint16
	Scanline        uint16
	Frame           uint64
	VBlankIRQ       bool
	RasterIRQ       bool
	LineIRQ         bool
	LineOn          int16
	LineOff         int16
	SpriteDMAStarts uint64
	IRQMask         uint8
	LineCycles      uint16
}

func (d *Device) Snapshot() Snapshot {
	return Snapshot{
		Registers: d.registers, Palette: d.palette, VRAM: d.vram, Framebuffer: d.framebuffer,
		VideoFlags: d.videoFlags, PixelMode: d.pixelMode, Scanline: d.scanline, Frame: d.frame,
		VBlankIRQ: d.vblankIRQ, RasterIRQ: d.rasterIRQ, LineIRQ: d.lineIRQ,
		LineOn: d.lineOn, LineOff: d.lineOff, SpriteDMAStarts: d.spriteDMAStarts,
		IRQMask: d.irqMask, LineCycles: d.lineCycles,
	}
}

func (d *Device) Restore(s Snapshot) {
	d.registers, d.palette, d.vram, d.framebuffer = s.Registers, s.Palette, s.VRAM, s.Framebuffer
	d.videoFlags, d.pixelMode, d.scanline, d.frame = s.VideoFlags, s.PixelMode, s.Scanline, s.Frame
	d.vblankIRQ, d.rasterIRQ, d.lineIRQ = s.VBlankIRQ, s.RasterIRQ, s.LineIRQ
	d.lineOn, d.lineOff, d.spriteDMAStarts = s.LineOn, s.LineOff, s.SpriteDMAStarts
	d.irqMask, d.lineCycles = s.IRQMask, s.LineCycles
}

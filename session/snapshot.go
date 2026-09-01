package session

import (
	"image"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/presentation"
	"github.com/wicanr2/superacan-emu/ui"
)

// snapshot 把 machine.System 包成 ui.Snapshot。所有方法都只讀，
// 唯一的可變狀態是重複使用的 RGBA 緩衝，避免每一幀配置一張新圖。
type snapshot struct{ session *Session }

func (a snapshot) Framebuffer() *image.RGBA {
	s := a.session
	if s.frame == nil {
		s.frame = image.NewRGBA(image.Rect(0, 0, umc6618.Width, umc6618.Height))
	}
	presentation.ARGBToRGBA(s.frame.Pix, s.System.Bus.Video().Framebuffer())
	return s.frame
}

func (a snapshot) FrameIndex() uint64 { return a.session.System.Bus.Video().Frame() }

func (a snapshot) Instructions() (uint64, uint64) {
	return a.session.System.Instructions, a.session.System.SoundInstructions
}

func (a snapshot) Halt() (ui.HaltReason, bool) {
	if a.session.halt == ui.HaltNone {
		return ui.HaltNone, false
	}
	return a.session.halt, true
}

func (a snapshot) Cartridge() (string, [32]byte, int64) {
	s := a.session
	return s.Title, s.System.ROMSHA256, s.romSize
}

func (a snapshot) Firmware() ui.FirmwareIDs { return a.session.firmware }

// ReadWorkRAM 是金手指搜尋的來源。越界一律回 0 而不是截斷後照讀，
// 這樣呼叫端不會誤以為讀到了資料。
func (a snapshot) ReadWorkRAM(dst []byte, addr uint32) int {
	if addr < ui.WorkRAMBase || uint64(addr)+uint64(len(dst))-1 > ui.WorkRAMEnd {
		return 0
	}
	bus := a.session.System.Bus
	for i := range dst {
		value, err := bus.Read8(addr + uint32(i))
		if err != nil {
			return i
		}
		dst[i] = value
	}
	return len(dst)
}

func (a snapshot) VideoRegisters() [256]uint16 {
	var out [umc6618.RegisterCount]uint16
	video := a.session.System.Bus.Video()
	for index := range out {
		out[index] = video.Register(uint16(index))
	}
	return out
}

func (a snapshot) LayerMask() uint32 { return a.session.layerMask }

package ui

import "image"

// HaltReason 是 fail-closed 停機的原因分類。
type HaltReason uint8

const (
	HaltNone HaltReason = iota
	HaltUnimplemented
	HaltBusFault
	HaltTimingBound
	HaltMediaFault
)

// FirmwareIDs 是四份主機韌體的 SHA-256。空值表示未載入。
type FirmwareIDs struct {
	IPL      [32]byte
	Key      [32]byte
	SoundA   [32]byte
	SoundB   [32]byte
	IPLOK    bool
	KeyOK    bool
	SoundAOK bool
	SoundBOK bool
}

// Snapshot 是 ui 對模擬核心的唯讀視角。所有方法都不得改變核心狀態，
// Framebuffer 回傳的是借用，ui 只讀不寫。
type Snapshot interface {
	Framebuffer() *image.RGBA
	FrameIndex() uint64
	Instructions() (m68k, m65c02 uint64)
	Halt() (HaltReason, bool)
	Cartridge() (title string, sum [32]byte, size int64)
	Firmware() FirmwareIDs
	ReadWorkRAM(dst []byte, addr uint32) int
	VideoRegisters() [256]uint16
	LayerMask() uint32
}

// SlotInfo 是一個存檔槽的狀態。縮圖由入口從 payload 內的 framebuffer 等比縮到
// 160×120，ui 不解析存檔格式。
type SlotInfo struct {
	Index    int
	Present  bool
	Rejected bool
	Reason   string
	Stamp    string
	Frame    uint64
	Thumb    *image.RGBA
}

// SlotSource 提供十個存檔槽的現況。入口只讀標頭就能判斷可否讀取，
// payload 的完整性留到實際載入時驗。
type SlotSource interface {
	Slots() []SlotInfo
}

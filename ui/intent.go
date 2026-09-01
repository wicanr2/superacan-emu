package ui

// Intent 是 ui 要求入口做的事。ui 自己不做任何一件，回傳之後由入口在 frame
// 邊界執行；這是「UI 不能寫入模擬核心」在型別上的落實。
type Intent interface{ isIntent() }

// ResetKind 區分冷開機與軟重設。
type ResetKind uint8

const (
	ResetCold ResetKind = iota
	ResetSoft
)

// CaptureKind 是擷取動作。
type CaptureKind uint8

const (
	CaptureScreenshot CaptureKind = iota
	CaptureClipStart
	CaptureClipStop
)

// WorkRAMBase 與 WorkRAMEnd 是 PokeWorkRAM 唯一允許的範圍。
const (
	WorkRAMBase = 0xfc0000
	WorkRAMEnd  = 0xfcffff
)

type (
	// LoadCartridge 載入卡帶檔案。
	LoadCartridge struct{ Path string }
	// UnloadCartridge 退出目前卡帶。
	UnloadCartridge struct{}
	// Reset 重設主機。
	Reset struct{ Kind ResetKind }
	// SaveState 寫入存檔槽。
	SaveState struct{ Slot int }
	// LoadState 讀取存檔槽。
	LoadState struct{ Slot int }
	// DeleteState 刪除存檔槽。
	DeleteState struct{ Slot int }
	// SetPaused 切換暫停。
	SetPaused struct{ Paused bool }
	// SetPacing 切換是否依實時速度執行；false 為全速。
	SetPacing struct{ Paced bool }
	// SetVolume 設定音量，0 到 100。
	SetVolume struct{ Percent int }
	// Capture 要求截圖或錄影。
	Capture struct{ Kind CaptureKind }
	// ApplyConfig 要求套用整份設定。
	ApplyConfig struct{ Config Config }
	// SetLayerMask 設定圖層遮罩，供診斷畫面使用。
	SetLayerMask struct{ Mask uint32 }
	// PokeWorkRAM 是唯一的寫入通道，只接受 Work RAM 範圍；金手指用。
	PokeWorkRAM struct {
		Addr  uint32
		Width uint8
		Value uint32
	}
	// Quit 結束模擬器。
	Quit struct{}
)

// Valid 回報這次寫入是否落在 Work RAM 內。越界的寫入由入口拒絕並記錄，
// ui 不得繞過這個檢查。
func (p PokeWorkRAM) Valid() bool {
	if p.Width != 1 && p.Width != 2 && p.Width != 4 {
		return false
	}
	last := uint64(p.Addr) + uint64(p.Width) - 1
	return uint64(p.Addr) >= WorkRAMBase && last <= WorkRAMEnd
}

func (LoadCartridge) isIntent()   {}
func (UnloadCartridge) isIntent() {}
func (Reset) isIntent()           {}
func (SaveState) isIntent()       {}
func (LoadState) isIntent()       {}
func (DeleteState) isIntent()     {}
func (SetPaused) isIntent()       {}
func (SetPacing) isIntent()       {}
func (SetVolume) isIntent()       {}
func (Capture) isIntent()         {}
func (ApplyConfig) isIntent()     {}
func (SetLayerMask) isIntent()    {}
func (PokeWorkRAM) isIntent()     {}
func (Quit) isIntent()            {}

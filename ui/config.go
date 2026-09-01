package ui

// Config 是會寫進設定檔的使用者選擇。P1 只用到其中幾項，其餘欄位在 P3 與 P5
// 補齊；讀入時未知欄位忽略，寫回時保留，因此這個結構可以逐步長大。
type Config struct {
	// Language 是介面語言的 BCP 47 標籤，空字串表示跟隨系統。
	Language string `json:"language,omitempty"`
	// Volume 是 0 到 100。
	Volume int `json:"volume"`
	// SuppressInfoToasts 抑制操作訊息，錯誤仍然顯示。
	SuppressInfoToasts bool `json:"suppress_info_toasts,omitempty"`
	// SaveSlot 是目前選定的存檔槽。
	SaveSlot int `json:"save_slot"`
	// MuteWhenUnpaced 在全速執行時靜音。
	MuteWhenUnpaced bool `json:"mute_when_unpaced,omitempty"`
}

// DefaultConfig 是全新安裝的設定。
func DefaultConfig() Config {
	return Config{Volume: 80, SaveSlot: 0}
}

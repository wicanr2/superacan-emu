package ui

import "encoding/json"

// ConfigVersion 只在需要語意遷移時提高。提高之後舊版讀新檔仍然套用「未知鍵忽略」，
// 所以單純新增欄位不必動它。
const ConfigVersion = 1

// Binding 是一組實體輸入到抽象動作的對應。
//
// Frontend 是寫入這組綁定的前端識別字串，而且一定要跟著存：同一個實體鍵在 X11
// keysym 與 ebiten.Key 底下是不同數值，沒有這個欄位，跨前端讀入會得到錯誤的按鍵。
type Binding struct {
	Frontend string `json:"frontend,omitempty"`
	Code     uint32 `json:"code"`
	Label    string `json:"label,omitempty"`
}

// Empty 回報這一組綁定是不是空的。
func (b Binding) Empty() bool { return b.Frontend == "" && b.Code == 0 }

// Usable 回報這組綁定在指定前端上能不能用。前端不符時不套用，
// 而不是拿別人的鍵碼硬套。
func (b Binding) Usable(frontend string) bool {
	return !b.Empty() && b.Frontend == frontend
}

// PlayerBindings 是一位玩家的鍵盤與手把綁定。兩者可以同時存在。
type PlayerBindings struct {
	Keyboard map[string]Binding `json:"keyboard,omitempty"`
	Gamepad  map[string]Binding `json:"gamepad,omitempty"`
}

// FirmwareConfig 只存路徑，不存韌體內容。
type FirmwareConfig struct {
	IPL        string `json:"ipl"`
	Key        string `json:"key"`
	SoundBIOS1 string `json:"sound_bios1"`
	SoundBIOS2 string `json:"sound_bios2"`
}

// PathsConfig 是各種目錄。
type PathsConfig struct {
	CartridgeDirs []string `json:"cartridge_dirs"`
	StateDir      string   `json:"state_dir"`
	CaptureDir    string   `json:"capture_dir"`
	CheatDir      string   `json:"cheat_dir"`
	BatteryDir    string   `json:"battery_dir"`
}

// RecentEntry 是最近開啟過的卡帶。只存路徑與雜湊，開啟前重新驗算。
type RecentEntry struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Title      string `json:"title"`
	LastPlayed string `json:"last_played"`
}

// VideoConfig 是影像設定。
type VideoConfig struct {
	Scale        int    `json:"scale"`
	IntegerScale bool   `json:"integer_scale"`
	Aspect       string `json:"aspect"`
	Filter       string `json:"filter"`
	Fullscreen   bool   `json:"fullscreen"`
	FrameBlend   bool   `json:"frame_blend"`
	ShowFPS      bool   `json:"show_fps"`
}

// AudioConfig 是音訊設定。
type AudioConfig struct {
	MasterVolume  int    `json:"master_volume"`
	MuteOnFastFwd bool   `json:"mute_on_fast_forward"`
	BufferMS      int    `json:"buffer_ms"`
	Sink          string `json:"sink,omitempty"`
}

// TouchConfig 是觸控版面設定。
type TouchConfig struct {
	Layout       string `json:"layout"`
	Opacity      int    `json:"opacity"`
	Scale        int    `json:"scale"`
	Haptics      bool   `json:"haptics"`
	DPadDeadzone int    `json:"dpad_deadzone"`
	StickMode    bool   `json:"stick_mode"`
	SwapHands    bool   `json:"swap_hands,omitempty"`
}

// InputConfig 是輸入設定。
type InputConfig struct {
	Players [2]PlayerBindings  `json:"players"`
	Hotkeys map[string]Binding `json:"hotkeys,omitempty"`
	Touch   TouchConfig        `json:"touch"`
}

// InterfaceConfig 是介面本身的設定。
type InterfaceConfig struct {
	Language           string `json:"language"`
	Metrics            string `json:"metrics"`
	SuppressInfoToasts bool   `json:"suppress_info_toasts"`
	SaveSlot           int    `json:"save_slot"`
}

// DiagnosticsConfig 是診斷設定。
type DiagnosticsConfig struct {
	LayerMask      uint32 `json:"layer_mask"`
	VideoRegisters bool   `json:"video_registers"`
}

// CheatsConfig 是金手指設定。
type CheatsConfig struct {
	Enabled bool `json:"enabled"`
	LockAll bool `json:"lock_all"`
}

// Config 是整份設定。
//
// Unknown 保存讀入時無法辨識的頂層鍵，寫回時原樣寫出。只忽略不保留的話，
// 舊版模擬器寫一次設定就會把新版的欄位刪光。
type Config struct {
	ConfigVersion int               `json:"config_version"`
	Firmware      FirmwareConfig    `json:"firmware"`
	Paths         PathsConfig       `json:"paths"`
	Recent        []RecentEntry     `json:"recent"`
	Video         VideoConfig       `json:"video"`
	Audio         AudioConfig       `json:"audio"`
	Input         InputConfig       `json:"input"`
	Interface     InterfaceConfig   `json:"ui"`
	Diagnostics   DiagnosticsConfig `json:"diagnostics"`
	Cheats        CheatsConfig      `json:"cheats"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// DefaultConfig 是全新安裝的設定。
func DefaultConfig() Config {
	config := Config{
		ConfigVersion: ConfigVersion,
		Video: VideoConfig{
			Scale: 3, IntegerScale: true, Aspect: "4:3", Filter: "nearest",
		},
		Audio: AudioConfig{MasterVolume: 80, MuteOnFastFwd: true, BufferMS: 200},
		Interface: InterfaceConfig{
			Language: "", Metrics: "auto", SaveSlot: 0,
		},
		Diagnostics: DiagnosticsConfig{LayerMask: 0x1f},
		Input: InputConfig{
			Touch: TouchConfig{Layout: "landscape_default", Opacity: 60, Scale: 100,
				Haptics: true, DPadDeadzone: 15},
		},
	}
	config.Input.Hotkeys = map[string]Binding{}
	for index := range config.Input.Players {
		config.Input.Players[index] = PlayerBindings{
			Keyboard: map[string]Binding{}, Gamepad: map[string]Binding{},
		}
	}
	return config
}

// PadButtons 是十二個手把按鈕的識別字串，順序即畫面上的列順序。
var PadButtons = []string{
	"up", "down", "left", "right", "a", "b", "x", "y", "l", "r", "start", "select",
}

// Hotkeys 是熱鍵動作，順序即畫面上的列順序。
var Hotkeys = []string{
	"menu", "pause", "save_state", "load_state", "next_slot", "prev_slot",
	"screenshot", "capture", "show_fps", "fast_forward", "fast_forward_lock",
	"mute", "fullscreen", "soft_reset", "load_cartridge", "lock_cheats",
	"cycle_layer_mask",
}

var padButtonLabels = map[string]string{
	"up": "上", "down": "下", "left": "左", "right": "右",
	"a": "A", "b": "B", "x": "X", "y": "Y", "l": "L", "r": "R",
	"start": "Start", "select": "Select",
}

var hotkeyLabels = map[string]string{
	"menu": "開啟選單", "pause": "暫停／繼續",
	"save_state": "存檔到目前槽", "load_state": "從目前槽讀檔",
	"next_slot": "下一個存檔槽", "prev_slot": "上一個存檔槽",
	"screenshot": "截圖", "capture": "開始／停止擷取",
	"show_fps": "顯示／隱藏 FPS", "fast_forward": "全速（按住）",
	"fast_forward_lock": "全速（鎖定）", "mute": "靜音",
	"fullscreen": "全螢幕", "soft_reset": "重設主機（軟）",
	"load_cartridge": "載入卡帶", "lock_cheats": "鎖定全部金手指",
	"cycle_layer_mask": "循環圖層遮罩（診斷）",
}

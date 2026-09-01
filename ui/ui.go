package ui

import (
	"image"
	"time"
)

// UI 是介面的狀態機。它不持有模擬核心，也不知道自己被畫到哪一種視窗上；
// 要求動作時把 Intent 排進佇列，由入口在 frame 邊界取走執行。
type UI struct {
	surface Surface
	metrics Metrics
	theme   Theme
	font    *Font
	config  Config
	slots   SlotSource

	library   Library
	firmware  FirmwareSource
	about     AboutInfo
	audio     AudioStatsSource
	diag      DiagnosticsSource
	cheat     CheatSource
	mask      uint32
	mode      Mode
	haltNote  string

	stack     []screen
	modal     *confirm
	toasts    []toastItem
	errorText string
	intents   []Intent
	now       time.Duration
	paused    bool
}

// Mode 決定介面的常駐畫面。
type Mode uint8

const (
	// ModeGame 是有卡帶在跑：覆蓋層預設隱藏，按 Menu 才出現。
	ModeGame Mode = iota
	// ModeShell 是沒有卡帶：S0 啟動畫面常駐，關不掉。
	ModeShell
	// ModeHalt 是 fail-closed 停機：S9 常駐且不能用返回鍵略過。
	ModeHalt
)

// Options 是建立 UI 的參數。Font 留空時用嵌入的 bitmapfont/v4。
type Options struct {
	Surface  Surface
	Config   Config
	Slots    SlotSource
	Library     Library
	Firmware    FirmwareSource
	About       AboutInfo
	AudioStats  AudioStatsSource
	Diagnostics DiagnosticsSource
	Cheats      CheatSource
	Theme    *Theme
	Font     *Font
}

// New 建立介面狀態機。
func New(options Options) *UI {
	theme := DefaultTheme()
	if options.Theme != nil {
		theme = *options.Theme
	}
	font := options.Font
	if font == nil {
		font = DefaultFont()
	}
	surface := options.Surface
	if surface.Scale < 1 {
		surface.Scale = 1
	}
	return &UI{
		surface:  surface,
		metrics:  MetricsFor(surface.Profile),
		theme:    theme,
		font:     font,
		config:   options.Config,
		slots:    options.Slots,
		library:  options.Library,
		firmware: options.Firmware,
		about:    options.About,
		audio:    options.AudioStats,
		diag:     options.Diagnostics,
		cheat:    options.Cheats,
		mask:     AllLayers,
	}
}

// SetMode 切換常駐畫面。ModeShell 與 ModeHalt 的根畫面關不掉：
// 沒有卡帶時沒有東西可以回去，停機時不能假裝沒事發生。
func (u *UI) SetMode(mode Mode, note string) {
	u.mode = mode
	u.modal = nil
	u.haltNote = note
	switch mode {
	case ModeShell:
		u.stack = []screen{&startScreen{}}
	case ModeHalt:
		u.stack = []screen{&haltScreen{}}
	default:
		u.stack = nil
	}
}

// Mode 回報目前的常駐畫面。
func (u *UI) Mode() Mode { return u.mode }

// SetLayerMask 讓入口把實際生效的遮罩回報給介面。介面自己不改 machine，
// 它送出 SetLayerMask intent，套用成功之後入口再叫這個函式。
func (u *UI) SetLayerMask(mask uint32) { u.mask = mask }

// rawCapturer 是正在等待實體輸入的畫面。
type rawCapturer interface{ capturing() bool }

// WantsRawInput 回報介面正在等待「按下哪一個鍵」。為 true 時前端只送 RawKey／
// RawPad 與取消，不要送翻譯過的導覽事件——否則 Enter 會同時被當成確認與被指定。
func (u *UI) WantsRawInput() bool {
	if !u.Visible() || u.modal != nil {
		return false
	}
	capturer, ok := u.stack[len(u.stack)-1].(rawCapturer)
	return ok && capturer.capturing()
}

// firmwareEntries 取得四份韌體的現況；沒有來源時視為全部未設定。
func (u *UI) firmwareEntries() []FirmwareEntry {
	if u.firmware == nil {
		entries := make([]FirmwareEntry, FirmwareCount)
		for i := range entries {
			entries[i].Kind = FirmwareKind(i)
		}
		return entries
	}
	return u.firmware.FirmwareEntries()
}

// firmwareReady 回報四份韌體是否齊備。缺任一份就不啟動任何卡帶。
func (u *UI) firmwareReady() bool {
	for _, entry := range u.firmwareEntries() {
		if !entry.Loaded {
			return false
		}
	}
	return true
}

func (u *UI) recentEntries() []CartridgeEntry {
	if u.library == nil {
		return nil
	}
	return u.library.Recent()
}

// Visible 回報覆蓋層是否正在顯示。為 true 時入口停止推進模擬時間，
// 並把輸入全部交給 UI。
func (u *UI) Visible() bool { return len(u.stack) > 0 }

// Config 回傳目前設定，入口寫回設定檔時用。
func (u *UI) Config() Config { return u.config }

// Metrics 回傳目前生效的度量。
func (u *UI) Metrics() Metrics { return u.metrics }

// Open 叫出覆蓋選單。
func (u *UI) Open() {
	if u.mode != ModeGame || u.Visible() {
		return
	}
	u.stack = []screen{&overlayScreen{}}
	u.paused = true
	u.emit(SetPaused{Paused: true})
}

// Close 關掉整個覆蓋層，回到遊戲。ModeShell 與 ModeHalt 有常駐根畫面，
// 這時只退回根畫面而不是隱藏介面。
func (u *UI) Close() {
	if !u.Visible() {
		return
	}
	u.modal = nil
	if u.mode != ModeGame {
		u.stack = u.stack[:1]
		return
	}
	u.stack = nil
	u.paused = false
	u.emit(SetPaused{Paused: false})
}

func (u *UI) push(s screen) { u.stack = append(u.stack, s) }

func (u *UI) pop() {
	if len(u.stack) <= 1 {
		u.Close()
		return
	}
	u.stack = u.stack[:len(u.stack)-1]
}

// SetHaltNote 更新停機說明。
func (u *UI) SetHaltNote(note string) { u.haltNote = note }

func (u *UI) emit(intent Intent) { u.intents = append(u.intents, intent) }

// TakeIntents 取走並清空待執行的 Intent。入口必須在 frame 邊界呼叫。
func (u *UI) TakeIntents() []Intent {
	if len(u.intents) == 0 {
		return nil
	}
	out := u.intents
	u.intents = nil
	return out
}

// Handle 消化一個事件。回傳是否被 UI 吃掉；false 表示前端應該把它當成遊戲輸入。
func (u *UI) Handle(ev Event) bool {
	if surface, ok := ev.(Surface); ok {
		if surface.Scale < 1 {
			surface.Scale = 1
		}
		u.surface = surface
		u.metrics = MetricsFor(surface.Profile)
		return true
	}
	if life, ok := ev.(Life); ok && life.Kind == LifeBack {
		return u.handleBack()
	}
	if !u.Visible() {
		if action, ok := ev.(Action); ok && action.Kind == ActMenu {
			u.Open()
			return true
		}
		return false
	}
	if u.errorText != "" {
		if action, ok := ev.(Action); ok && (action.Kind == ActConfirm || action.Kind == ActCancel) {
			u.errorText = ""
			return true
		}
	}
	if u.modal != nil {
		return u.modal.handle(u, ev)
	}
	return u.stack[len(u.stack)-1].handle(u, ev)
}

// handleBack 實作 §9.3 的返回鍵順序：modal、錯誤列、堆疊、覆蓋層、根畫面。
func (u *UI) handleBack() bool {
	switch {
	case u.modal != nil:
		u.modal = nil
	case u.errorText != "":
		u.errorText = ""
	case len(u.stack) > 1:
		u.pop()
	case u.mode != ModeGame:
		// 根畫面關不掉：沒有卡帶時沒有東西可以回去，停機不能被略過。
	case u.Visible():
		u.Close()
	default:
		u.Open()
	}
	return true
}

// Update 推進 UI 自己的時間。now 是單調時間，由入口提供；UI 不讀掛鐘，
// 也不因為它而改變模擬排程。
func (u *UI) Update(now time.Duration) {
	u.now = now
	u.expireToasts()
}

// Draw 把覆蓋層畫到 dst。dst 是表面原生解析度的 RGBA，遊戲畫面應已畫在上面；
// snap 可以是 nil（尚未載入卡帶）。
func (u *UI) Draw(dst *image.RGBA, snap Snapshot) {
	c := &canvas{dst: dst, scale: u.surface.Scale, metrics: u.metrics, font: u.font, theme: u.theme}
	if u.Visible() {
		// 只畫最上層。每一層都自己負責畫滿需要的底，堆疊在視覺上不疊加；
		// 半透明面板疊在半透明面板上會讓下層的字透出來。
		u.stack[len(u.stack)-1].draw(u, c, snap)
		if u.modal != nil {
			u.modal.draw(u, c)
		}
	}
	// 金手指標記畫在覆蓋層之外：一旦這個工作階段寫過 Work RAM，
	// 不論選單開不開，畫面上都必須看得出來。
	u.drawCheatMarker(c)
	u.drawErrorBar(c)
	u.drawToasts(c)
}

// fillPage 把整頁塗成不透明的面板底。整頁畫面要遮住遊戲畫面：面板色本身帶
// alpha 是為了讓覆蓋選單透出後面的畫面，整頁畫面沿用同一個 alpha 會讓遊戲的
// 高飽和色塊從字底下透出來。
func (u *UI) fillPage(c *canvas) {
	solid := u.theme.Panel
	solid.A = 0xff
	c.rect(0, 0, c.width(), c.height(), solid)
}

// slotInfo 取一個槽的現況；沒有 SlotSource 時視為空槽。
func (u *UI) slotInfo(slot int) SlotInfo {
	if u.slots == nil {
		return SlotInfo{Index: slot}
	}
	for _, info := range u.slots.Slots() {
		if info.Index == slot {
			return info
		}
	}
	return SlotInfo{Index: slot}
}

package session

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

// MaxFrameInstructions 是單一 frame 的有界執行上限。超過即停機，不無限迴圈。
const MaxFrameInstructions = 20_000_000

// Session 把模擬核心、介面與存檔目錄綁在一起。前端只需要餵事件、呼叫 Advance、
// 再把 Compose 的結果貼上去。
type Session struct {
	System   *machine.System
	UI       *ui.UI
	Title    string
	StateDir string

	// Stamp 可覆寫存檔槽顯示的時間，供測試固定畫面。
	Stamp func(os.FileInfo) string
	// Screenshot 收到截圖要求時被呼叫，由前端決定寫到哪裡。
	Screenshot func(frame *image.RGBA) error
	// OnQuit 在使用者要求離開時被呼叫。
	OnQuit func()

	firmware  ui.FirmwareIDs
	romSize   int64
	layerMask uint32
	halt      ui.HaltReason
	haltNote  string

	frame  *image.RGBA
	paused bool
	pacing bool
	quit   bool
	pads   [2]uint16
}

// Options 建立 Session 需要的東西。
type Options struct {
	System   *machine.System
	Title    string
	StateDir string
	ROMSize  int64
	Firmware ui.FirmwareIDs
	Surface  ui.Surface
	Config   ui.Config
}

// New 建立一個 session。UI 的 SlotSource 就是 session 自己：存檔槽的真相在檔案
// 系統上，不在介面的記憶體裡。
func New(options Options) *Session {
	s := &Session{
		System:    options.System,
		Title:     options.Title,
		StateDir:  options.StateDir,
		firmware:  options.Firmware,
		romSize:   options.ROMSize,
		layerMask: uint32(umc6618.AllLayers),
		pacing:    true,
		pads:      [2]uint16{0xffff, 0xffff},
	}
	s.UI = ui.New(ui.Options{Surface: options.Surface, Config: options.Config, Slots: s})
	return s
}

// Snapshot 是給 UI 的唯讀視角。
func (s *Session) Snapshot() ui.Snapshot { return snapshot{s} }

// Paused 回報模擬時間是否停住。覆蓋層開著時一定是停住的。
func (s *Session) Paused() bool { return s.paused || s.UI.Visible() }

// Quitting 回報使用者是否要求離開。
func (s *Session) Quitting() bool { return s.quit }

// Halt 回報停機原因。
func (s *Session) Halt() (ui.HaltReason, string) { return s.halt, s.haltNote }

// SetPad 設定手把狀態（active-low）。覆蓋層開著時一律送「全部放開」，
// 這是輸入來源的閘門，不是改寫晶片狀態。
func (s *Session) SetPad(player int, activeLow uint16) {
	if player < 0 || player >= len(s.pads) {
		return
	}
	s.pads[player] = activeLow
}

// Handle 把一個介面事件交給 UI，回傳是否被吃掉。
func (s *Session) Handle(event ui.Event) bool { return s.UI.Handle(event) }

// Advance 推進一個 frame，並執行這一輪累積的 Intent。覆蓋層開著時不推進時間，
// 但 Intent 照樣執行——存檔與讀檔本來就發生在 frame 邊界。
func (s *Session) Advance(now time.Duration) error {
	s.UI.Update(now)
	if err := s.drainIntents(); err != nil {
		return err
	}
	if s.Paused() || s.halt != ui.HaltNone {
		s.applyPads(true)
		return nil
	}
	s.applyPads(false)
	if _, err := s.System.RunFrame(MaxFrameInstructions); err != nil {
		s.halt = classifyHalt(err)
		s.haltNote = err.Error()
		return err
	}
	return nil
}

func (s *Session) applyPads(released bool) {
	for player := range s.pads {
		value := s.pads[player]
		if released {
			value = 0xffff
		}
		s.System.SoundBus.SetPad(player, value)
	}
}

// classifyHalt 目前只分「有沒有停」。machine 還沒有可供程式判別的錯誤型別，
// 拿錯誤字串去猜分類會得到看起來合理但站不住的結果，所以先一律歸到
// HaltUnimplemented，實際原因由 haltNote 原文呈現。machine 導入 sentinel error
// 之後這裡再細分。
func classifyHalt(err error) ui.HaltReason {
	if err == nil {
		return ui.HaltNone
	}
	return ui.HaltUnimplemented
}

// Compose 把遊戲畫面與覆蓋層畫進 dst。遊戲畫面以最近鄰整數放大，
// 覆蓋層畫在 dst 的原生解析度上。
func (s *Session) Compose(dst *image.RGBA) {
	snap := snapshot{s}
	game := snap.Framebuffer()
	blitNearest(dst, game)
	s.UI.Draw(dst, snap)
}

// blitNearest 把來源等比例填滿目的地。整數倍時等同像素複製，
// 非整數倍時仍是最近鄰，不做取樣模糊——硬體輸出是點陣，插值只會製造假訊息。
func blitNearest(dst, src *image.RGBA) {
	dstB, srcB := dst.Bounds(), src.Bounds()
	if dstB.Empty() || srcB.Empty() {
		return
	}
	for y := 0; y < dstB.Dy(); y++ {
		sy := srcB.Min.Y + y*srcB.Dy()/dstB.Dy()
		srcRow := src.PixOffset(srcB.Min.X, sy)
		dstRow := dst.PixOffset(dstB.Min.X, dstB.Min.Y+y)
		for x := 0; x < dstB.Dx(); x++ {
			sx := x * srcB.Dx() / dstB.Dx()
			copy(dst.Pix[dstRow+x*4:dstRow+x*4+4], src.Pix[srcRow+sx*4:srcRow+sx*4+4])
		}
	}
}

// drainIntents 執行 UI 排出的動作。這是唯一改變模擬狀態的入口；
// 失敗一律回報到介面的錯誤列，不靜默吞掉。
func (s *Session) drainIntents() error {
	for _, intent := range s.UI.TakeIntents() {
		if err := s.apply(intent); err != nil {
			s.UI.Fail(err.Error())
		}
	}
	return nil
}

func (s *Session) apply(intent ui.Intent) error {
	switch value := intent.(type) {
	case ui.SetPaused:
		s.paused = value.Paused
	case ui.SetPacing:
		s.pacing = value.Paced
	case ui.Reset:
		return s.reset(value.Kind)
	case ui.SaveState:
		return s.saveSlot(value.Slot)
	case ui.LoadState:
		return s.loadSlot(value.Slot)
	case ui.DeleteState:
		return s.deleteSlot(value.Slot)
	case ui.SetLayerMask:
		s.layerMask = value.Mask
	case ui.Capture:
		return s.capture(value.Kind)
	case ui.PokeWorkRAM:
		return s.poke(value)
	case ui.Quit:
		s.quit = true
		if s.OnQuit != nil {
			s.OnQuit()
		}
	case ui.UnloadCartridge:
		s.quit = true
	}
	return nil
}

func (s *Session) reset(kind ui.ResetKind) error {
	// 軟重設與冷開機目前都走同一條 68000 reset；兩者的差別（RAM 是否保留）
	// 要等 machine 提供冷開機路徑才成立，在那之前不假裝有分別。
	_ = kind
	return s.System.Reset()
}

func (s *Session) saveSlot(slot int) error {
	if s.StateDir == "" {
		return fmt.Errorf("session: 沒有指定存檔目錄")
	}
	if err := os.MkdirAll(s.StateDir, 0o755); err != nil {
		return err
	}
	// 先寫暫存檔再改名：中途失敗不會留下半份存檔覆蓋掉原本可用的那一格。
	temporary := s.slotPath(slot) + ".tmp"
	file, err := os.Create(temporary)
	if err != nil {
		return err
	}
	if err := s.System.SaveState(file); err != nil {
		file.Close()
		os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, s.slotPath(slot))
}

func (s *Session) loadSlot(slot int) error {
	file, err := os.Open(s.slotPath(slot))
	if err != nil {
		return err
	}
	defer file.Close()
	return s.System.LoadState(file)
}

func (s *Session) deleteSlot(slot int) error {
	if err := os.Remove(s.slotPath(slot)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Session) capture(kind ui.CaptureKind) error {
	if kind != ui.CaptureScreenshot {
		return fmt.Errorf("session: 錄影尚未實作")
	}
	if s.Screenshot == nil {
		return fmt.Errorf("session: 這個前端沒有提供截圖輸出")
	}
	return s.Screenshot(snapshot{s}.Framebuffer())
}

// poke 是金手指的寫入路徑。範圍檢查在這裡再做一次：ui.PokeWorkRAM.Valid 是
// 型別層的保證，這裡是執行層的保證，兩層都不能省。
func (s *Session) poke(value ui.PokeWorkRAM) error {
	if !value.Valid() {
		return fmt.Errorf("session: $%06X 不在 Work RAM 範圍內", value.Addr)
	}
	bus := s.System.Bus
	switch value.Width {
	case 1:
		return bus.Write8(value.Addr, uint8(value.Value))
	case 2:
		return bus.Write16(value.Addr, uint16(value.Value))
	default:
		if err := bus.Write16(value.Addr, uint16(value.Value>>16)); err != nil {
			return err
		}
		return bus.Write16(value.Addr+2, uint16(value.Value))
	}
}

// TitleFromPath 由檔名推出顯示用的卡帶名稱。這是檔名不是卡帶標頭裡的欄位：
// 目前沒有證據指出 Super A'Can 的卡帶標頭有標題欄位，所以不假裝有。
func TitleFromPath(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if index := strings.Index(name, " ("); index > 0 {
		name = name[:index]
	}
	return strings.ToUpper(strings.TrimSpace(name))
}

// StateDirFor 是「每個卡帶一個存檔目錄」的預設規則。
func StateDirFor(root, romPath string) string {
	name := filepath.Base(romPath)
	return filepath.Join(root, name)
}

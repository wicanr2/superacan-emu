package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

// newTestSession 造一台最小可執行的機器：reset 向量指到一個原地迴圈，
// 這樣 RunFrame 永遠有指令可跑，測試不必依賴商業 ROM。
func newTestSession(t *testing.T) *Session {
	t.Helper()
	ipl := make([]byte, machine.IPLSize)
	rom := make([]byte, 0x10000)
	key := make([]byte, 16)
	ipl[0], ipl[1], ipl[2], ipl[3] = 0x00, 0x00, 0x10, 0x00 // SSP=$00001000
	ipl[4], ipl[5], ipl[6], ipl[7] = 0x00, 0x00, 0x04, 0x00 // PC=$00000400
	ipl[0x400], ipl[0x401] = 0x60, 0xfe                     // BRA.S *

	system, err := machine.NewSystem(ipl, rom, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := system.Reset(); err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		System:   system,
		Title:    "TEST CART",
		StateDir: t.TempDir(),
		ROMSize:  int64(len(rom)),
		Surface:  ui.Surface{W: 960, H: 720, Scale: 1, Profile: ui.ProfileCompact},
		Config:   ui.DefaultConfig(),
	})
	// 時間戳是環境不是介面行為，固定它才能對畫面取雜湊。
	s.Stamp = func(os.FileInfo) string { return "01-01 00:00" }
	return s
}

func advance(t *testing.T, s *Session, frames int) {
	t.Helper()
	for i := 0; i < frames; i++ {
		if _, err := s.Advance(time.Duration(i) * 16 * time.Millisecond); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
}

// 這是 P1 驗收條件裡「叫出選單並完成存讀檔」的 headless 版本。手動在視窗前面按
// 一次證明不了任何可重跑的事；這條測試每次都跑。
func TestMenuSaveAndLoadRoundTripHeadless(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 4)
	before := s.System.Instructions

	// 開選單：模擬時間必須停住。
	s.Handle(ui.Action{Kind: ui.ActMenu})
	advance(t, s, 4)
	if !s.UI.Visible() {
		t.Fatal("Menu 事件應該叫出覆蓋選單")
	}
	if s.System.Instructions != before {
		t.Fatalf("選單開著時模擬時間前進了 %d 條指令", s.System.Instructions-before)
	}

	// 繼續遊戲(0) → 存檔(1) → 進存檔槽 → 對空的槽 0 按確認。
	s.Handle(ui.Nav{Dir: ui.DirDown})
	s.Handle(ui.Action{Kind: ui.ActConfirm})
	s.Handle(ui.Action{Kind: ui.ActConfirm})
	advance(t, s, 1)

	path := filepath.Join(s.StateDir, SlotFileName(0))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("存檔槽 0 應該有檔案：%v", err)
	}
	slots := s.Slots()
	if !slots[0].Present || slots[0].Rejected {
		t.Fatalf("槽 0 應該可讀取，得到 %+v", slots[0])
	}
	if slots[0].Thumb == nil {
		t.Fatal("縮圖應該直接來自 payload 內的 framebuffer")
	}

	// 關掉選單再跑一段，狀態必須與存檔當下不同。
	s.UI.Close()
	advance(t, s, 8)
	afterRun := s.System.Instructions
	if afterRun == before {
		t.Fatal("關掉選單之後模擬時間應該恢復前進")
	}

	// 讀檔：繼續遊戲(0) → 讀檔(2) → 槽 0 → 確認。
	s.Handle(ui.Action{Kind: ui.ActMenu})
	s.Handle(ui.Nav{Dir: ui.DirDown})
	s.Handle(ui.Nav{Dir: ui.DirDown})
	s.Handle(ui.Action{Kind: ui.ActConfirm})
	s.Handle(ui.Action{Kind: ui.ActConfirm})
	advance(t, s, 1)

	if s.UI.Visible() {
		t.Fatal("讀檔成功之後應該回到遊戲")
	}
	if s.System.Instructions >= afterRun {
		t.Fatalf("讀檔之後指令數 %d 應該退回存檔當下的 %d", s.System.Instructions, afterRun)
	}
}

// 讀檔要能完全取代現行狀態：從存檔續跑，與一路跑到底的結果必須逐位元相同。
func TestLoadStateResumesIdentically(t *testing.T) {
	reference := newTestSession(t)
	advance(t, reference, 6)
	referenceState := reference.System.Instructions

	continuous := newTestSession(t)
	advance(t, continuous, 6)
	if err := continuous.saveSlot(0); err != nil {
		t.Fatal(err)
	}
	advance(t, continuous, 6)
	wantInstructions := continuous.System.Instructions
	wantFrame := continuous.System.Bus.Video().Frame()
	wantPixels := continuous.System.Bus.Video().FramebufferSHA256()

	resumed := newTestSession(t)
	resumed.StateDir = continuous.StateDir
	if err := resumed.loadSlot(0); err != nil {
		t.Fatal(err)
	}
	if resumed.System.Instructions != referenceState {
		t.Fatalf("載入後指令數 %d，want %d", resumed.System.Instructions, referenceState)
	}
	advance(t, resumed, 6)
	if resumed.System.Instructions != wantInstructions {
		t.Fatalf("續跑後指令數 %d，want %d", resumed.System.Instructions, wantInstructions)
	}
	if resumed.System.Bus.Video().Frame() != wantFrame {
		t.Fatalf("續跑後 frame %d，want %d", resumed.System.Bus.Video().Frame(), wantFrame)
	}
	if resumed.System.Bus.Video().FramebufferSHA256() != wantPixels {
		t.Fatal("續跑後的 framebuffer 與連續執行不同")
	}
}

// 壞掉的存檔要在畫面上先標成拒絕，而不是等按下去才失敗。
func TestCorruptSlotIsRejectedBeforeUse(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)
	if err := s.saveSlot(3); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.StateDir, SlotFileName(3))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	slot := s.Slots()[3]
	if !slot.Rejected || slot.Present {
		t.Fatalf("損壞的存檔應該標成拒絕，得到 %+v", slot)
	}
	if slot.Reason == "" {
		t.Fatal("拒絕必須說明理由")
	}
	if err := s.loadSlot(3); err == nil {
		t.Fatal("損壞的存檔必須無法載入")
	}
}

// 覆蓋層開著時輸入要被閘住：machine 收到的是「全部放開」。
func TestOverlayGatesPadInput(t *testing.T) {
	s := newTestSession(t)
	s.SetPad(0, 0x7fff)
	advance(t, s, 1)
	if got := s.System.SoundBus.Pad(0); got != 0x7fff {
		t.Fatalf("遊戲中手把=$%04X，want $7FFF", got)
	}
	s.Handle(ui.Action{Kind: ui.ActMenu})
	advance(t, s, 1)
	if got := s.System.SoundBus.Pad(0); got != 0xffff {
		t.Fatalf("選單開著時手把=$%04X，want $FFFF（全部放開）", got)
	}
}

// 合成畫面在固定狀態下要可重現，這是前端接上去之後唯一能自動比對的東西。
func TestComposeIsDeterministic(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 3)
	if err := s.saveSlot(0); err != nil {
		t.Fatal(err)
	}
	s.Handle(ui.Action{Kind: ui.ActMenu})
	advance(t, s, 1)

	first := composeHash(s)
	second := composeHash(s)
	if first != second {
		t.Fatalf("同一狀態合成兩次不一致：%s / %s", first, second)
	}
}

func composeHash(s *Session) string {
	dst := image.NewRGBA(image.Rect(0, 0, 960, 720))
	s.Compose(dst)
	sum := sha256.Sum256(dst.Pix)
	return hex.EncodeToString(sum[:])
}

// 熱鍵要與選單走同一條路：不開選單、只按熱鍵，存檔與讀檔的結果必須相同。
// 這條在沒有視窗的容器裡跑完整條流程，不靠人在視窗前面按一次。
func TestHotkeySaveAndLoadRoundTripHeadless(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 4)

	s.Handle(ui.HotkeyEvent{Action: "save_state"})
	advance(t, s, 1)
	if s.UI.Visible() {
		t.Fatal("熱鍵存檔不應該叫出選單")
	}
	path := filepath.Join(s.StateDir, SlotFileName(0))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("熱鍵存檔之後槽 0 應該有檔案：%v", err)
	}
	saved := s.System.Instructions

	advance(t, s, 8)
	if s.System.Instructions == saved {
		t.Fatal("存檔之後模擬時間應該繼續前進")
	}

	s.Handle(ui.HotkeyEvent{Action: "load_state"})
	advance(t, s, 1)
	if got := s.System.Instructions; got != saved {
		t.Fatalf("熱鍵讀檔之後指令數 %d，預期退回存檔當下的 %d", got, saved)
	}
}

// 換槽熱鍵要真的換掉存檔的目標槽，而不是只改畫面上的數字。
func TestHotkeyNextSlotChangesTheTarget(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)

	s.Handle(ui.HotkeyEvent{Action: "next_slot"})
	s.Handle(ui.HotkeyEvent{Action: "next_slot"})
	s.Handle(ui.HotkeyEvent{Action: "save_state"})
	advance(t, s, 1)

	if _, err := os.Stat(filepath.Join(s.StateDir, SlotFileName(2))); err != nil {
		t.Fatalf("換兩次槽之後應該存進槽 2：%v", err)
	}
	if _, err := os.Stat(filepath.Join(s.StateDir, SlotFileName(0))); err == nil {
		t.Fatal("槽 0 不該有檔案")
	}
}

// 暫停熱鍵要停住模擬時間，再按一次要恢復。
func TestHotkeyPauseStopsEmulatedTime(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)

	s.Handle(ui.HotkeyEvent{Action: "pause"})
	advance(t, s, 4)
	paused := s.System.Instructions
	advance(t, s, 4)
	if s.System.Instructions != paused {
		t.Fatalf("暫停之後模擬時間前進了 %d 條指令", s.System.Instructions-paused)
	}

	s.Handle(ui.HotkeyEvent{Action: "pause"})
	advance(t, s, 4)
	if s.System.Instructions == paused {
		t.Fatal("解除暫停之後模擬時間應該恢復前進")
	}
}

// 全速熱鍵改的是主機的節奏，不是模擬器的時間線：同樣的 frame 數要得到同樣的
// 指令數，否則「全速」就變成改變硬體行為。
func TestHotkeyFastForwardDoesNotChangeEmulatedWork(t *testing.T) {
	paced := newTestSession(t)
	advance(t, paced, 12)

	fast := newTestSession(t)
	fast.Handle(ui.HotkeyEvent{Action: "fast_forward"})
	advance(t, fast, 12)
	if fast.Pacing() {
		t.Fatal("全速熱鍵沒有關掉節奏")
	}
	if got, want := fast.System.Instructions, paced.System.Instructions; got != want {
		t.Fatalf("全速下 %d 條指令，實時下 %d 條", got, want)
	}

	fast.Handle(ui.HotkeyEvent{Action: "fast_forward", Released: true})
	advance(t, fast, 1)
	if !fast.Pacing() {
		t.Fatal("放開全速熱鍵沒有回到實時速度")
	}
}

// 靜音是主機端的音量，不能影響模擬核心；Volume 同時要反映全速靜音的設定。
func TestHotkeyMuteOnlyChangesHostVolume(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)
	if got := s.Volume(); got != ui.DefaultConfig().Audio.MasterVolume {
		t.Fatalf("預設音量 %d", got)
	}

	s.Handle(ui.HotkeyEvent{Action: "mute"})
	advance(t, s, 1)
	if got := s.Volume(); got != 0 {
		t.Fatalf("靜音後音量 %d，預期 0", got)
	}
	s.Handle(ui.HotkeyEvent{Action: "mute"})
	advance(t, s, 1)
	if got := s.Volume(); got != ui.DefaultConfig().Audio.MasterVolume {
		t.Fatalf("解除靜音後音量 %d", got)
	}

	// MuteOnFastFwd 預設開啟：全速時音量為 0，設定本身不變。
	s.Handle(ui.HotkeyEvent{Action: "fast_forward"})
	advance(t, s, 1)
	if got := s.Volume(); got != 0 {
		t.Fatalf("全速時音量 %d，預期依 MuteOnFastFwd 靜音", got)
	}
}

// 離開前景要把卡帶電池記憶體寫出去，並叫出覆蓋選單：行動平台沒有正常結束，
// 切走的那一刻就是最後一次能寫檔的機會；而回到前景時凍住的畫面要有說明。
func TestSuspendFlushesAndOpensTheOverlay(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 4)
	before := s.System.Instructions

	flushed := 0
	s.Flush = func() error { flushed++; return nil }

	if !s.Handle(ui.Life{Kind: ui.LifeSuspend}) {
		t.Fatal("LifeSuspend 應該被 session 吃掉")
	}
	if flushed != 1 {
		t.Fatalf("Flush 被呼叫 %d 次，預期 1 次", flushed)
	}
	if !s.UI.Visible() {
		t.Fatal("離開前景應該叫出覆蓋選單")
	}
	advance(t, s, 8)
	if s.System.Instructions != before {
		t.Fatalf("離開前景之後模擬時間前進了 %d 條指令", s.System.Instructions-before)
	}

	// 回到前景不自動恢復執行：選單留在畫面上，由使用者按「繼續遊戲」。
	if !s.Handle(ui.Life{Kind: ui.LifeResume}) {
		t.Fatal("LifeResume 應該被 session 吃掉")
	}
	advance(t, s, 4)
	if !s.UI.Visible() {
		t.Fatal("回到前景不該自動關掉選單")
	}
	if s.System.Instructions != before {
		t.Fatal("回到前景不該自動恢復執行")
	}
}

// 落地失敗要留在錯誤列上，不能安靜吞掉：使用者的存檔沒寫成功是他必須知道的事。
func TestSuspendReportsFlushFailure(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)
	s.Flush = func() error { return errors.New("磁碟已滿") }
	s.Handle(ui.Life{Kind: ui.LifeSuspend})
	if got := s.UI.ErrorText(); got != "磁碟已滿" {
		t.Fatalf("錯誤列是 %q，預期落地失敗的原因", got)
	}
}

// 返回鍵仍然是介面導覽，不能被生命週期那一層攔走。
func TestBackIsStillHandledByTheInterface(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 2)
	if !s.Handle(ui.Life{Kind: ui.LifeBack}) {
		t.Fatal("LifeBack 應該被吃掉")
	}
	if !s.UI.Visible() {
		t.Fatal("遊戲中按返回鍵應該開啟選單")
	}
}

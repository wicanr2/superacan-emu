package session

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/presentation"
	"github.com/wicanr2/superacan-emu/ui"
)

// 截圖必須等同硬體輸出：PNG 解出來的像素要與 --screenshot 走的
// presentation.EncodePNG 位元組相同。兩條路徑不同就表示截圖不能當畫面證據。
func TestScreenshotMatchesHardwareOutput(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 3)
	paintTestFramebuffer(s)

	var viaUI []byte
	s.Screenshot = func(frame *image.RGBA) error {
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, frame); err != nil {
			return err
		}
		viaUI = buffer.Bytes()
		return nil
	}
	if err := s.capture(ui.CaptureScreenshot); err != nil {
		t.Fatal(err)
	}

	var viaHeadless bytes.Buffer
	if err := presentation.EncodePNG(&viaHeadless, umc6618.Width, umc6618.Height,
		s.System.Bus.Video().Framebuffer()); err != nil {
		t.Fatal(err)
	}

	first, err := png.Decode(bytes.NewReader(viaUI))
	if err != nil {
		t.Fatal(err)
	}
	second, err := png.Decode(bytes.NewReader(viaHeadless.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	bounds := first.Bounds()
	if bounds != second.Bounds() {
		t.Fatalf("尺寸不同：%v / %v", bounds, second.Bounds())
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := first.At(x, y).RGBA()
			r2, g2, b2, _ := second.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				t.Fatalf("(%d,%d) 兩條路徑的像素不同", x, y)
			}
		}
	}
}

// 外部接收端拿到的位元組數必須等於 幀數 × 320 × 240 × 4。
func TestCaptureSinkReceivesRawFrames(t *testing.T) {
	s := newTestSession(t)
	var sink bytes.Buffer
	s.SetCaptureSink(&sink)
	if err := s.StartCapture(""); err != nil {
		t.Fatal(err)
	}
	const frames = 7
	advance(t, s, frames)
	if err := s.StopCapture(); err != nil {
		t.Fatal(err)
	}
	want := frames * umc6618.Width * umc6618.Height * 4
	if sink.Len() != want {
		t.Fatalf("接收端拿到 %d 位元組，want %d", sink.Len(), want)
	}
	if s.CaptureFrames() != frames {
		t.Fatalf("擷取 %d 幀", s.CaptureFrames())
	}
}

// 錄影的幀數要等於實際跑掉的 frame 數，而且檔案的長度欄位要一致。
func TestClipFrameCountMatchesEmulatedFrames(t *testing.T) {
	s := newTestSession(t)
	s.CaptureDir = t.TempDir()
	path := filepath.Join(s.CaptureDir, "clip.avi")
	if err := s.StartCapture(path); err != nil {
		t.Fatal(err)
	}
	const frames = 10
	advance(t, s, frames)
	if err := s.StopCapture(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// avih 的 dwTotalFrames 在 RIFF 標頭之後的固定位置；這裡只驗它等於 frames。
	index := bytes.Index(raw, []byte("avih"))
	if index < 0 {
		t.Fatal("找不到 avih")
	}
	total := binary.LittleEndian.Uint32(raw[index+8+16 : index+8+20])
	if total != frames {
		t.Fatalf("dwTotalFrames=%d，want %d", total, frames)
	}
	if s.CaptureFrames() != frames {
		t.Fatalf("擷取 %d 幀", s.CaptureFrames())
	}
}

// 開著擷取跑，指令數與 framebuffer 雜湊不得改變。
func TestCaptureDoesNotChangeTiming(t *testing.T) {
	plain := newTestSession(t)
	advance(t, plain, 20)
	wantInstructions := plain.System.Instructions
	wantPixels := plain.System.Bus.Video().FramebufferSHA256()

	recording := newTestSession(t)
	recording.CaptureDir = t.TempDir()
	if err := recording.StartCapture(filepath.Join(recording.CaptureDir, "clip.avi")); err != nil {
		t.Fatal(err)
	}
	advance(t, recording, 20)
	if err := recording.StopCapture(); err != nil {
		t.Fatal(err)
	}
	if recording.System.Instructions != wantInstructions {
		t.Fatalf("開著錄影的指令數 %d，want %d", recording.System.Instructions, wantInstructions)
	}
	if recording.System.Bus.Video().FramebufferSHA256() != wantPixels {
		t.Fatal("開著錄影的 framebuffer 不同")
	}
}

// 沒有按停止就結束的錄影仍然要是完整的檔案。長度欄位是在收尾才回填的，
// 少了這一步，資料都在、標頭全是 0，播放器一幀也讀不出來。
func TestShutdownFinalisesAnOpenClip(t *testing.T) {
	s := newTestSession(t)
	s.CaptureDir = t.TempDir()
	path := filepath.Join(s.CaptureDir, "unfinished.avi")
	if err := s.StartCapture(path); err != nil {
		t.Fatal(err)
	}
	advance(t, s, 6)
	if err := s.Shutdown(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(raw[4:8]); got != uint32(len(raw)-8) {
		t.Fatalf("RIFF 大小 %d，檔案是 %d：收尾沒有回填", got, len(raw)-8)
	}
	index := bytes.Index(raw, []byte("avih"))
	if total := binary.LittleEndian.Uint32(raw[index+8+16 : index+8+20]); total != 6 {
		t.Fatalf("dwTotalFrames=%d，want 6", total)
	}
	// 重複呼叫要安全。
	if err := s.Shutdown(); err != nil {
		t.Fatal(err)
	}
}

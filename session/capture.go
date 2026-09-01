package session

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/wicanr2/superacan-emu/capture"
	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/ui"
)

// 擷取的來源一律是 UM6618 的顯示孔徑：不含覆蓋層、不套濾鏡，所以錄下來的畫面
// 與截圖是同一個東西，也因此可以當畫面證據用。
//
// 編碼發生在模擬迴圈之外的意義是：來不及時丟掉的是編碼工作，不是模擬幀。
// 目前的實作是同步編碼，丟棄計數永遠是 0；改成非同步時這個計數要開始有值。
type captureState struct {
	recorder *capture.Recorder
	sink     io.Writer
	path     string
	frames   int
	dropped  int
}

// Recording 回報是否正在擷取。
func (s *Session) Recording() bool {
	return s.clip.recorder != nil || s.clip.sink != nil
}

// CapturePath 是目前的錄影檔路徑。
func (s *Session) CapturePath() string { return s.clip.path }

// CaptureFrames 是已擷取的幀數。
func (s *Session) CaptureFrames() int { return s.clip.frames }

// SetCaptureSink 指定一個外部接收端。設了之後擷取會把原始 RGBA 幀寫給它，
// 不寫 AVI；由使用者自己接 ffmpeg 之類的工具。
func (s *Session) SetCaptureSink(sink io.Writer) { s.clip.sink = sink }

// StartCapture 開始錄影。沒有指定路徑時依 CaptureDir 與時間自動命名。
func (s *Session) StartCapture(path string) error {
	if s.System == nil {
		return fmt.Errorf("session: 沒有卡帶可以錄影")
	}
	if s.clip.recorder != nil {
		return fmt.Errorf("session: 已經在錄影")
	}
	if s.clip.sink != nil {
		// 外部接收端不需要建立檔案，直接開始送幀。
		s.clip.frames = 0
		return nil
	}
	if path == "" {
		path = filepath.Join(s.CaptureDir, fmt.Sprintf("acan-%s.avi", time.Now().Format("20060102-150405")))
	}
	recorder, err := capture.NewRecorder(path, umc6618.Width, umc6618.Height, 80)
	if err != nil {
		return err
	}
	s.clip.recorder = recorder
	s.clip.path = path
	s.clip.frames = 0
	return nil
}

// StopCapture 結束錄影並回填檔案的長度欄位。
func (s *Session) StopCapture() error {
	if s.clip.recorder == nil {
		if s.clip.sink != nil {
			s.clip.sink = nil
			return nil
		}
		return fmt.Errorf("session: 沒有在錄影")
	}
	recorder := s.clip.recorder
	s.clip.recorder = nil
	return recorder.Close()
}

// Shutdown 在結束前收尾。錄影檔的長度欄位是在 Close 才回填的，沒有收尾的檔案
// 資料都在、標頭全是 0，播放器一幀也讀不出來——整份錄影等於白錄。入口必須 defer
// 這個函式，不能寄望使用者記得先按停止。
func (s *Session) Shutdown() error {
	if s.clip.recorder == nil {
		return nil
	}
	return s.StopCapture()
}

// PushCapturePCM 讓前端把已經重取樣成 48 kHz 16-bit stereo 的 PCM 交給錄影。
// 播放與錄影共用同一份樣本，兩者不會不同步。
func (s *Session) PushCapturePCM(pcm []byte) {
	if s.clip.recorder == nil || len(pcm) == 0 {
		return
	}
	if err := s.clip.recorder.WritePCM(pcm); err != nil {
		s.UI.Fail(err.Error())
	}
}

// captureFrame 在每個完成的 frame 之後被呼叫。
func (s *Session) captureFrame() {
	if !s.Recording() {
		return
	}
	frame := snapshot{s}.Framebuffer()
	if s.clip.sink != nil {
		if _, err := s.clip.sink.Write(frame.Pix); err != nil {
			s.UI.Fail(err.Error())
			s.clip.sink = nil
			return
		}
		s.clip.frames++
		return
	}
	if err := s.clip.recorder.WriteFrame(frame); err != nil {
		s.UI.Fail(err.Error())
		return
	}
	s.clip.frames++
}

func (s *Session) capture(kind ui.CaptureKind) error {
	switch kind {
	case ui.CaptureScreenshot:
		if s.Screenshot == nil {
			return fmt.Errorf("session: 這個前端沒有提供截圖輸出")
		}
		if s.System == nil {
			return fmt.Errorf("session: 沒有畫面可以截圖")
		}
		return s.Screenshot(snapshot{s}.Framebuffer())
	case ui.CaptureClipStart:
		return s.StartCapture("")
	default:
		return s.StopCapture()
	}
}

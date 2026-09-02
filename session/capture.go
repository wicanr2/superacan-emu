package session

import (
	"fmt"
	"image"
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

	// pcmBytes 是已寫入錄影的音訊位元組數。錄合成畫面時用它把音訊補齊到與
	// 畫面同長：覆蓋層開著時畫面照錄但模擬時間停住，不補的話音訊會愈跑愈前面。
	pcmBytes int

	// composed 不為 nil 時錄的是合成後的視窗（含覆蓋層），尺寸就是它的大小。
	// 這條路是給展示與教學用的：它錄下來的是「使用者看到的畫面」，
	// 因此**不能當畫面證據**——證據要用不含覆蓋層的那一條。
	composed *image.RGBA
}

// Recording 回報是否正在擷取。
func (s *Session) Recording() bool {
	return s.clip.recorder != nil || s.clip.sink != nil
}

// CapturePath 是目前的錄影檔路徑。
func (s *Session) CapturePath() string { return s.clip.path }

// CapturePCMBytes 是已寫入錄影的音訊位元組數，供驗證音畫是否等長。
func (s *Session) CapturePCMBytes() int { return s.clip.pcmBytes }

// CaptureFrames 是已擷取的幀數。
func (s *Session) CaptureFrames() int { return s.clip.frames }

// SetCaptureComposed 把錄影來源換成合成後的視窗，尺寸為 width×height。
// 錄下來的畫面含覆蓋層與觸控版面，而且以主機迴圈的節奏取樣（覆蓋層開著時模擬
// 時間停住，但畫面仍然要錄下來，否則走選單那一段會變成靜止）。
//
// 這條路的產物是展示影片，不是畫面證據：證據要用不含覆蓋層的預設來源。
func (s *Session) SetCaptureComposed(width, height int) {
	if width <= 0 || height <= 0 {
		s.clip.composed = nil
		return
	}
	s.clip.composed = image.NewRGBA(image.Rect(0, 0, width, height))
}

// CaptureComposed 回報目前是不是在錄合成畫面。
func (s *Session) CaptureComposed() bool { return s.clip.composed != nil }

// SetCaptureSink 指定一個外部接收端。設了之後擷取會把原始 RGBA 幀寫給它，
// 不寫 AVI；由使用者自己接 ffmpeg 之類的工具。
func (s *Session) SetCaptureSink(sink io.Writer) { s.clip.sink = sink }

// StartCapture 開始錄影。沒有指定路徑時依 CaptureDir 與時間自動命名。
func (s *Session) StartCapture(path string) error {
	// 合成畫面不需要卡帶：啟動畫面與卡帶瀏覽器本來就是要錄下來的東西。
	if s.System == nil && s.clip.composed == nil {
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
	width, height := umc6618.Width, umc6618.Height
	if s.clip.composed != nil {
		width, height = s.clip.composed.Bounds().Dx(), s.clip.composed.Bounds().Dy()
	}
	recorder, err := capture.NewRecorder(path, width, height, 80)
	if err != nil {
		return err
	}
	s.clip.recorder = recorder
	s.clip.path = path
	s.clip.frames = 0
	s.clip.pcmBytes = 0
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
	// 錄合成畫面時音訊要與畫面等長，所以多出來的部分要削掉。重取樣一幀不會
	// 剛好是 800 個取樣，只補不削的話誤差會單向累積——量到的是四百幀多 181 個
	// block，換算到一分鐘的影片就是幾十毫秒的偏移。削掉的量遠小於一個 block
	// 的聽覺門檻。
	if limit := s.captureAudioLimit(); limit >= 0 {
		if s.clip.pcmBytes >= limit {
			return
		}
		if room := limit - s.clip.pcmBytes; len(pcm) > room {
			pcm = pcm[:room]
		}
	}
	if err := s.clip.recorder.WritePCM(pcm); err != nil {
		s.UI.Fail(err.Error())
		return
	}
	s.clip.pcmBytes += len(pcm)
}

// captureAudioLimit 是這一刻音訊最多可以寫到哪裡，只在錄合成畫面時有意義。
// 上限給到「下一幀」，正在進行中的那一幀才寫得完。回傳負值代表不設限。
func (s *Session) captureAudioLimit() int {
	if s.clip.composed == nil {
		return -1
	}
	return (s.clip.frames + 1) * captureBytesPerFrame
}

// captureBytesPerFrame 是一個錄影幀對應的音訊位元組數：每秒 60 幀、
// 每幀 48000/60 個取樣、每個取樣四個位元組（16-bit stereo）。
const captureBytesPerFrame = capture.AudioSampleRate / capture.FrameRate * capture.AudioBlockAlign

// padCaptureAudio 把音訊補到與已錄畫面同長。每幀 48000/60 個取樣、每個取樣
// 四個位元組（16-bit stereo）；模擬時間停住的那些幀沒有樣本，補靜音。
//
// 不補的後果不是「那一段沒聲音」，而是**整份錄影從那裡開始音畫不同步**：
// AVI 的音訊是一條連續的串流，少掉的部分會讓後面的聲音整體提前。
func (s *Session) padCaptureAudio() {
	if s.clip.recorder == nil || s.clip.composed == nil {
		return
	}
	want := s.clip.frames * captureBytesPerFrame
	if s.clip.pcmBytes >= want {
		return
	}
	silence := make([]byte, want-s.clip.pcmBytes)
	if err := s.clip.recorder.WritePCM(silence); err != nil {
		s.UI.Fail(err.Error())
		return
	}
	s.clip.pcmBytes += len(silence)
}

// captureSource 取這一幀要寫進錄影的畫面。
func (s *Session) captureSource() *image.RGBA {
	if s.clip.composed == nil {
		return snapshot{s}.Framebuffer()
	}
	s.Compose(s.clip.composed)
	return s.clip.composed
}

// captureFrame 在每個完成的 frame 之後被呼叫。
func (s *Session) captureFrame() {
	if !s.Recording() {
		return
	}
	frame := s.captureSource()
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

// captureComposedTick 在模擬時間沒有前進的那些主機迴圈裡補一幀。只有錄合成畫面
// 時才有意義：覆蓋層開著時模擬時間停住，但使用者看到的畫面仍然在動（選單、
// 提示、捲動），不補這一幀的話走選單那一段在影片裡會是靜止的。
func (s *Session) captureComposedTick() {
	if s.clip.composed == nil {
		return
	}
	s.captureFrame()
	s.padCaptureAudio()
}

// Package hostio 是桌面前端共用的主機側輸入輸出：媒體載入、卡帶電池記憶體、
// 截圖輸出與外部音訊播放程序。
//
// 它不決定任何硬體語意，也不碰介面；X11 與 macOS 兩個入口共用同一份，
// 兩邊才不會各自長出不一樣的載入規則。這裡的函式一律回傳錯誤而不是直接結束
// 行程——要不要結束是入口的決定。
package hostio

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
	"github.com/wicanr2/superacan-emu/presentation"
)

// LoadWordSwapped 讀入需要 word swap 的映像（IPL）。
func LoadWordSwapped(path string, expectedSize int) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	image, err := media.DecodeWordSwapped(path, raw, expectedSize)
	if err != nil {
		return nil, err
	}
	return image.Bytes, nil
}

// LoadLinear 讀入不需要轉換的映像（key、音效 BIOS）。
func LoadLinear(path string, expectedSize int) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	image, err := media.DecodeLinear(path, raw, expectedSize)
	if err != nil {
		return nil, err
	}
	return image.Bytes, nil
}

// LoadCartridge 接受 raw 卡帶與 ZIP（單一成員或雙部分）。
func LoadCartridge(path string) (media.Image, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return media.Image{}, fmt.Errorf("read %s: %w", path, err)
	}
	return media.DecodeCartridge(path, raw)
}

// WriteScreenshot 把顯示孔徑寫成 PNG。截圖不含覆蓋層也不套濾鏡，
// 所以它可以當畫面證據用。
func WriteScreenshot(path string, framebuffer []uint32) error {
	output, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create screenshot: %w", err)
	}
	if err := presentation.EncodePNG(output, umc6618.Width, umc6618.Height, framebuffer); err != nil {
		_ = output.Close()
		return fmt.Errorf("encode screenshot: %w", err)
	}
	return output.Close()
}

// LoadCartridgeSave 把卡帶電池記憶體讀進機器。檔案不存在不是錯誤：
// 第一次玩本來就沒有存檔。
func LoadCartridgeSave(system *machine.System, path string) error {
	if path == "" {
		return nil
	}
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return system.Bus.LoadCartridgeSave(payload)
}

// WriteCartridgeSave 寫回卡帶電池記憶體。先寫暫存檔再改名，中途失敗不會留下
// 半份存檔覆蓋掉原本可用的那一個。
func WriteCartridgeSave(system *machine.System, path string) error {
	if path == "" || system == nil {
		return nil
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, system.Bus.CartridgeSave(), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// WriteSaveState 寫出一份 ACANGOS1 存檔。
func WriteSaveState(system *machine.System, path string) error {
	var buffer bytes.Buffer
	if err := system.SaveState(&buffer); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, buffer.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// ReadSaveState 讀入一份存檔。載入是交易式的：驗證失敗時現行狀態不變。
func ReadSaveState(system *machine.System, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return system.LoadState(file)
}

// AudioSink 把 UMC6619 的原生樣本重取樣成 48 kHz stereo，交給外部播放程序。
// 佇列滿了就丟掉最舊的樣本：播放端的狀態不回饋到模擬器時間線。
//
// clip 不為 nil 時同一份重取樣輸出也交給它，播放與錄影因此不會不同步。
//
// volume 不為 nil 時，送給播放程序的樣本依它縮放（0–100）。縮放做在送出前的
// 那一段，不做在取樣回呼裡：回呼一秒跑四萬多次，在那裡讀設定會把介面狀態
// 拉進模擬迴圈。錄影拿到的是未縮放的樣本——靜音是監聽控制，不該讓錄下來的
// 檔案跟著變成無聲。
func AudioSink(system *machine.System, command string, clip func([]byte), volume *atomic.Int32) (func(), error) {
	stream := presentation.NewPCM16StereoStream(48000 / 5)
	resampler := presentation.NewStereoResampler(umc6619.ClockHz, umc6619.CyclesPerSample, 48000,
		func(left, right int16) {
			stream.Push(left, right)
			if clip != nil {
				clip([]byte{
					byte(uint16(left)), byte(uint16(left) >> 8),
					byte(uint16(right)), byte(uint16(right) >> 8),
				})
			}
		})
	system.SoundBus.Audio().SetSampleSink(func(sample umc6619.Sample) {
		resampler.Push(sample.Left, sample.Right)
	})

	process := exec.Command("sh", "-c", command)
	pipe, err := process.StdinPipe()
	if err != nil {
		return nil, err
	}
	process.Stderr = os.Stderr
	if err := process.Start(); err != nil {
		return nil, err
	}

	done := make(chan struct{})
	go func() {
		buffer := make([]byte, 4*480) // 每次 10 ms
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				_ = pipe.Close()
				return
			case <-ticker.C:
				n, err := stream.Read(buffer)
				if err != nil || n == 0 {
					continue
				}
				scalePCM16(buffer[:n], volume)
				if _, err := pipe.Write(buffer[:n]); err != nil {
					_ = pipe.Close()
					return
				}
			}
		}
	}()

	return func() {
		close(done)
		_ = process.Process.Kill()
		_, _ = process.Process.Wait()
	}, nil
}

// scalePCM16 就地縮放 16-bit little-endian 樣本。100 直接跳過，
// 讓預設音量不付任何代價。
func scalePCM16(buffer []byte, volume *atomic.Int32) {
	if volume == nil {
		return
	}
	percent := volume.Load()
	if percent >= 100 {
		return
	}
	if percent <= 0 {
		for i := range buffer {
			buffer[i] = 0
		}
		return
	}
	for i := 0; i+1 < len(buffer); i += 2 {
		sample := int32(int16(uint16(buffer[i]) | uint16(buffer[i+1])<<8))
		sample = sample * percent / 100
		buffer[i] = byte(uint16(int16(sample)))
		buffer[i+1] = byte(uint16(int16(sample)) >> 8)
	}
}

// CaptureSink 把原始 RGBA 幀送給一個外部程序。這條路存在的理由是純 Go 沒有
// H.264 編碼器：想要小檔案的使用者可以自備 ffmpeg，內建的 AVI/MJPEG 則不需要
// 任何外部工具。
func CaptureSink(command string) (io.WriteCloser, func(), error) {
	process := exec.Command("sh", "-c", command)
	pipe, err := process.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	process.Stderr = os.Stderr
	if err := process.Start(); err != nil {
		return nil, nil, err
	}
	return pipe, func() {
		_ = pipe.Close()
		_ = process.Wait()
	}, nil
}

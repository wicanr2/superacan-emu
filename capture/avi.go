// Package capture 產生可播放的錄影檔與截圖。
//
// 格式是 AVI 容器加 MJPEG 視訊與 PCM 音訊，全部用標準函式庫，沒有執行期相依。
// 選這個組合的理由見 docs/capture-formats.md：純 Go 沒有 H.264 與 AAC 編碼器，
// 而 MJPEG 在 AVI 裡的播放器支援比在 MP4 裡好，Bcan 自己的第二種錄影格式也是
// AVI/MJPEG。要 H.264 的話是另一條路（執行期載入 OpenH264），不在這個套件裡。
package capture

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
)

// FrameRate 是錄影的視訊幀率。Super A'Can 的一個硬體 frame 對一個錄影 frame，
// 不做抽幀：抽幀會讓錄下來的動作與實際執行對不上。
const FrameRate = 60

// 音訊格式固定為 48 kHz 16-bit stereo，與呈現層的重取樣輸出一致。
const (
	AudioSampleRate = 48000
	AudioChannels   = 2
	AudioBits       = 16
	// AudioBlockAlign 是一組左右聲道取樣的位元組數，也是 AVI 音訊串流的時間單位。
	AudioBlockAlign = AudioChannels * AudioBits / 8
)

// Recorder 寫出一個 AVI 檔。大小欄位要在結束時回填，所以它需要可 seek 的目的地。
type Recorder struct {
	file    *os.File
	quality int

	offsets    headerOffsets
	moviStart  int64
	frames     int
	audioBytes int
	maxJPEG    int
	index      []indexEntry
	width      int
	height     int
	closed     bool
}

type indexEntry struct {
	id     [4]byte
	flags  uint32
	offset uint32
	size   uint32
}

// NewRecorder 建立錄影檔。quality 是 JPEG 品質（1–100）。
func NewRecorder(path string, width, height, quality int) (*Recorder, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("capture: 畫面大小 %dx%d 不合法", width, height)
	}
	if quality <= 0 || quality > 100 {
		quality = 80
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	recorder := &Recorder{file: file, quality: quality, width: width, height: height}
	if err := recorder.writeHeaders(); err != nil {
		file.Close()
		os.Remove(path)
		return nil, err
	}
	return recorder, nil
}

// Frames 是已寫入的視訊幀數。
func (r *Recorder) Frames() int { return r.frames }

// WriteFrame 寫一幀。來源是顯示孔徑的原始像素，不含覆蓋層也不套濾鏡，
// 所以錄下來的畫面與截圖是同一個東西。
func (r *Recorder) WriteFrame(frame *image.RGBA) error {
	if r.closed {
		return fmt.Errorf("capture: 錄影已結束")
	}
	if frame.Bounds().Dx() != r.width || frame.Bounds().Dy() != r.height {
		return fmt.Errorf("capture: 這一幀是 %dx%d，錄影檔是 %dx%d",
			frame.Bounds().Dx(), frame.Bounds().Dy(), r.width, r.height)
	}
	var encoded chunkBuffer
	if err := jpeg.Encode(&encoded, frame, &jpeg.Options{Quality: r.quality}); err != nil {
		return err
	}
	if encoded.Len() > r.maxJPEG {
		r.maxJPEG = encoded.Len()
	}
	r.frames++
	return r.writeChunk([4]byte{'0', '0', 'd', 'c'}, 0x10, encoded.Bytes())
}

// WritePCM 寫一段 48 kHz 16-bit stereo 的 PCM。長度必須是 4 的倍數。
func (r *Recorder) WritePCM(pcm []byte) error {
	if r.closed {
		return fmt.Errorf("capture: 錄影已結束")
	}
	if len(pcm) == 0 {
		return nil
	}
	if len(pcm)%4 != 0 {
		return fmt.Errorf("capture: PCM 長度 %d 不是 4 的倍數", len(pcm))
	}
	r.audioBytes += len(pcm)
	return r.writeChunk([4]byte{'0', '1', 'w', 'b'}, 0x10, pcm)
}

// Close 回填所有大小欄位並寫出索引。
func (r *Recorder) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if err := r.writeIndex(); err != nil {
		r.file.Close()
		return err
	}
	if err := r.patchHeaders(); err != nil {
		r.file.Close()
		return err
	}
	return r.file.Close()
}

func (r *Recorder) writeChunk(id [4]byte, flags uint32, payload []byte) error {
	offset, err := r.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	header := make([]byte, 8)
	copy(header, id[:])
	binary.LittleEndian.PutUint32(header[4:], uint32(len(payload)))
	if _, err := r.file.Write(header); err != nil {
		return err
	}
	if _, err := r.file.Write(payload); err != nil {
		return err
	}
	// RIFF 的 chunk 要對齊到偶數位元組。
	if len(payload)%2 == 1 {
		if _, err := r.file.Write([]byte{0}); err != nil {
			return err
		}
	}
	r.index = append(r.index, indexEntry{
		id: id, flags: flags,
		offset: uint32(offset - r.moviStart + 4),
		size:   uint32(len(payload)),
	})
	return nil
}

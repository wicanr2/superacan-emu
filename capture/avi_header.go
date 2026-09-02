package capture

import (
	"bytes"
	"encoding/binary"
	"io"
)

// chunkBuffer 只是 bytes.Buffer 的別名，讓 jpeg.Encode 有地方寫。
type chunkBuffer = bytes.Buffer

// 需要在 Close 回填的欄位位移。這些值在寫標頭時記下來，不是手算的常數：
// 手算的位移錯了之後檔案看起來還是「有東西」，播放器才會拒絕，而那時候已經
// 離出錯的地方很遠了。
type headerOffsets struct {
	riffSize    int64
	maxBytes    int64
	totalFrames int64
	videoLength int64
	audioLength int64
	moviSize    int64
}

func putU32(out *bytes.Buffer, value uint32) {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], value)
	out.Write(encoded[:])
}

func putU16(out *bytes.Buffer, value uint16) {
	var encoded [2]byte
	binary.LittleEndian.PutUint16(encoded[:], value)
	out.Write(encoded[:])
}

// writeHeaders 寫出 RIFF/AVI 的標頭。長度欄位先寫 0，Close 再回填。
func (r *Recorder) writeHeaders() error {
	var out bytes.Buffer
	out.WriteString("RIFF")
	r.offsets.riffSize = int64(out.Len())
	putU32(&out, 0) // RIFF 大小，之後回填
	out.WriteString("AVI ")

	// LIST hdrl
	out.WriteString("LIST")
	putU32(&out, 4+8+56+8+4+8+56+8+40+8+4+8+56+8+18)
	out.WriteString("hdrl")

	// avih
	out.WriteString("avih")
	putU32(&out, 56)
	putU32(&out, 1000000/FrameRate) // dwMicroSecPerFrame
	r.offsets.maxBytes = int64(out.Len())
	putU32(&out, 0)    // dwMaxBytesPerSec，之後回填
	putU32(&out, 0)    // dwPaddingGranularity
	putU32(&out, 0x10) // dwFlags: AVIF_HASINDEX
	r.offsets.totalFrames = int64(out.Len())
	putU32(&out, 0) // dwTotalFrames，之後回填
	putU32(&out, 0) // dwInitialFrames
	putU32(&out, 2) // dwStreams
	putU32(&out, 0) // dwSuggestedBufferSize，之後回填
	putU32(&out, uint32(r.width))
	putU32(&out, uint32(r.height))
	for i := 0; i < 4; i++ {
		putU32(&out, 0) // dwReserved
	}

	// LIST strl（視訊）
	out.WriteString("LIST")
	putU32(&out, 4+8+56+8+40)
	out.WriteString("strl")
	out.WriteString("strh")
	putU32(&out, 56)
	out.WriteString("vids")
	out.WriteString("MJPG")
	putU32(&out, 0) // dwFlags
	putU16(&out, 0) // wPriority
	putU16(&out, 0) // wLanguage
	putU32(&out, 0) // dwInitialFrames
	putU32(&out, 1) // dwScale
	putU32(&out, FrameRate)
	putU32(&out, 0) // dwStart
	r.offsets.videoLength = int64(out.Len())
	putU32(&out, 0) // dwLength，之後回填
	putU32(&out, 0) // dwSuggestedBufferSize
	putU32(&out, 0) // dwQuality
	putU32(&out, 0) // dwSampleSize
	putU16(&out, 0)
	putU16(&out, 0)
	putU16(&out, uint16(r.width))
	putU16(&out, uint16(r.height))

	// strf（BITMAPINFOHEADER）
	out.WriteString("strf")
	putU32(&out, 40)
	putU32(&out, 40)
	putU32(&out, uint32(r.width))
	putU32(&out, uint32(r.height))
	putU16(&out, 1)  // biPlanes
	putU16(&out, 24) // biBitCount
	out.WriteString("MJPG")
	putU32(&out, uint32(r.width*r.height*3))
	putU32(&out, 0)
	putU32(&out, 0)
	putU32(&out, 0)
	putU32(&out, 0)

	// LIST strl（音訊）
	out.WriteString("LIST")
	putU32(&out, 4+8+56+8+18)
	out.WriteString("strl")
	out.WriteString("strh")
	putU32(&out, 56)
	out.WriteString("auds")
	putU32(&out, 0) // fccHandler：PCM 沒有 handler
	putU32(&out, 0) // dwFlags
	putU16(&out, 0)
	putU16(&out, 0)
	putU32(&out, 0) // dwInitialFrames
	// PCM 串流的時間單位是「一個 block（一組左右聲道取樣）」，不是位元組：
	// dwScale 取 nBlockAlign、dwRate 取每秒位元組，兩者相除就是取樣率；
	// dwSampleSize 同樣是 nBlockAlign，dwLength 因此以 block 計。
	//
	// 用「一個位元組當一個取樣」也自洽，讀得回原始資料，但那不是 VfW 的慣例：
	// 解碼器算不出串流長度（ffprobe 的 duration 是 N/A），在有視訊一起轉檔時
	// 會把音訊截掉一大截。
	putU32(&out, AudioBlockAlign)                 // dwScale
	putU32(&out, AudioSampleRate*AudioBlockAlign) // dwRate
	putU32(&out, 0)                               // dwStart
	r.offsets.audioLength = int64(out.Len())
	putU32(&out, 0)               // dwLength，之後回填（以 block 計）
	putU32(&out, 0)               // dwSuggestedBufferSize
	putU32(&out, 0)               // dwQuality
	putU32(&out, AudioBlockAlign) // dwSampleSize
	putU16(&out, 0)
	putU16(&out, 0)
	putU16(&out, 0)
	putU16(&out, 0)

	// strf（WAVEFORMATEX）
	out.WriteString("strf")
	putU32(&out, 18)
	putU16(&out, 1) // WAVE_FORMAT_PCM
	putU16(&out, AudioChannels)
	putU32(&out, AudioSampleRate)
	putU32(&out, AudioSampleRate*AudioChannels*AudioBits/8)
	putU16(&out, AudioChannels*AudioBits/8)
	putU16(&out, AudioBits)
	putU16(&out, 0) // cbSize

	// LIST movi
	out.WriteString("LIST")
	r.offsets.moviSize = int64(out.Len())
	putU32(&out, 0) // movi 大小，之後回填
	out.WriteString("movi")

	if _, err := r.file.Write(out.Bytes()); err != nil {
		return err
	}
	position, err := r.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	// movi 的資料從 "movi" 這四個字之後開始；索引的位移以 LIST 的資料起點為基準。
	r.moviStart = position - 4
	return nil
}

func (r *Recorder) writeIndex() error {
	var out bytes.Buffer
	out.WriteString("idx1")
	putU32(&out, uint32(len(r.index)*16))
	for _, entry := range r.index {
		out.Write(entry.id[:])
		putU32(&out, entry.flags)
		putU32(&out, entry.offset)
		putU32(&out, entry.size)
	}
	_, err := r.file.Write(out.Bytes())
	return err
}

func (r *Recorder) patchHeaders() error {
	end, err := r.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	indexBytes := int64(len(r.index)*16 + 8)
	moviEnd := end - indexBytes
	moviSize := moviEnd - r.moviStart

	patches := []struct {
		offset int64
		value  uint32
	}{
		{r.offsets.riffSize, uint32(end - 8)},
		{r.offsets.maxBytes, uint32(r.maxJPEG)},
		{r.offsets.totalFrames, uint32(r.frames)},
		{r.offsets.videoLength, uint32(r.frames)},
		// dwLength 以 block 計，與 strh 的 dwScale／dwSampleSize 同單位。
		{r.offsets.audioLength, uint32(r.audioBytes / AudioBlockAlign)},
		{r.offsets.moviSize, uint32(moviSize)},
	}
	for _, patch := range patches {
		var encoded [4]byte
		binary.LittleEndian.PutUint32(encoded[:], patch.value)
		if _, err := r.file.WriteAt(encoded[:], patch.offset); err != nil {
			return err
		}
	}
	_, err = r.file.Seek(end, io.SeekStart)
	return err
}

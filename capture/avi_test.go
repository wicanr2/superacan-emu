package capture

import (
	"encoding/binary"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
)

// riffNode 是走訪 RIFF 結構時的一個節點。
type riffNode struct {
	id       string
	form     string
	payload  []byte
	offset   int64
	children []riffNode
}

// walkRIFF 把整個檔案拆成樹。標頭的位移是手算的，靠這個走訪把它們算一次再比對，
// 不是拿常數去驗常數。
func walkRIFF(t *testing.T, raw []byte, base int64) []riffNode {
	t.Helper()
	var nodes []riffNode
	for offset := 0; offset+8 <= len(raw); {
		id := string(raw[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		if offset+8+size > len(raw) {
			size = len(raw) - offset - 8
		}
		payload := raw[offset+8 : offset+8+size]
		node := riffNode{id: id, payload: payload, offset: base + int64(offset)}
		if id == "LIST" || id == "RIFF" {
			node.form = string(payload[:4])
			if node.form == "hdrl" || node.form == "strl" || node.form == "AVI " {
				node.children = walkRIFF(t, payload[4:], base+int64(offset)+12)
			}
		}
		nodes = append(nodes, node)
		advance := 8 + size
		if size%2 == 1 {
			advance++
		}
		offset += advance
	}
	return nodes
}

func find(nodes []riffNode, path ...string) *riffNode {
	current := nodes
	var found *riffNode
	for _, want := range path {
		found = nil
		for index := range current {
			node := &current[index]
			if node.id == want || node.form == want {
				found = node
				break
			}
		}
		if found == nil {
			return nil
		}
		current = found.children
	}
	return found
}

func testFrame(width, height int, seed byte) *image.RGBA {
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := frame.PixOffset(x, y)
			frame.Pix[offset+0] = byte(x) + seed
			frame.Pix[offset+1] = byte(y)
			frame.Pix[offset+2] = byte(x^y) + seed
			frame.Pix[offset+3] = 0xff
		}
	}
	return frame
}

// 錄影檔的每個長度欄位都要與實際內容相符。這些欄位是在結束時回填的，
// 位移算錯的話檔案看起來還是「有東西」，但播放器會拒絕或播出垃圾。
func TestRecorderWritesConsistentHeaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.avi")
	recorder, err := NewRecorder(path, 320, 240, 80)
	if err != nil {
		t.Fatal(err)
	}
	const frames = 12
	audioPerFrame := AudioSampleRate * AudioChannels * AudioBits / 8 / FrameRate
	for i := 0; i < frames; i++ {
		if err := recorder.WriteFrame(testFrame(320, 240, byte(i))); err != nil {
			t.Fatal(err)
		}
		if err := recorder.WritePCM(make([]byte, audioPerFrame)); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	nodes := walkRIFF(t, raw, 0)
	riff := find(nodes, "RIFF")
	if riff == nil || riff.form != "AVI " {
		t.Fatal("不是 AVI 檔")
	}
	if got := binary.LittleEndian.Uint32(raw[4:8]); got != uint32(len(raw)-8) {
		t.Fatalf("RIFF 大小 %d，檔案是 %d", got, len(raw)-8)
	}

	avih := find(riff.children, "hdrl", "avih")
	if avih == nil {
		t.Fatal("找不到 avih")
	}
	if got := binary.LittleEndian.Uint32(avih.payload[16:20]); got != frames {
		t.Fatalf("dwTotalFrames=%d，want %d", got, frames)
	}

	// 兩條 strl：視訊在前、音訊在後。
	hdrl := find(riff.children, "hdrl")
	var streams []*riffNode
	for index := range hdrl.children {
		if hdrl.children[index].form == "strl" {
			streams = append(streams, &hdrl.children[index])
		}
	}
	if len(streams) != 2 {
		t.Fatalf("找到 %d 條串流", len(streams))
	}
	videoLength := binary.LittleEndian.Uint32(find(streams[0].children, "strh").payload[32:36])
	if videoLength != frames {
		t.Fatalf("視訊 dwLength=%d，want %d", videoLength, frames)
	}
	audioLength := binary.LittleEndian.Uint32(find(streams[1].children, "strh").payload[32:36])
	if audioLength != uint32(frames*audioPerFrame) {
		t.Fatalf("音訊 dwLength=%d，want %d", audioLength, frames*audioPerFrame)
	}

	var moviNode *riffNode
	for index := range riff.children {
		if riff.children[index].form == "movi" {
			moviNode = &riff.children[index]
		}
	}
	if moviNode == nil {
		t.Fatal("找不到 movi")
	}
	// movi 之後緊接 idx1，兩者相加要等於檔案結尾。
	idx1 := find(riff.children, "idx1")
	if idx1 == nil {
		t.Fatal("找不到 idx1")
	}
	if len(idx1.payload) != frames*2*16 {
		t.Fatalf("索引 %d 位元組，want %d", len(idx1.payload), frames*2*16)
	}
	if idx1.offset+int64(len(idx1.payload))+8 != int64(len(raw)) {
		t.Fatalf("idx1 沒有結束在檔尾：%d + %d != %d",
			idx1.offset, len(idx1.payload)+8, len(raw))
	}
}

// 每一幀都要能被 image/jpeg 解回來——寫進去的是不是有效的 JPEG，
// 只有解一次才知道。
func TestRecordedFramesDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.avi")
	recorder, err := NewRecorder(path, 64, 48, 90)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := recorder.WriteFrame(testFrame(64, 48, byte(i*40))); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	nodes := walkRIFF(t, raw, 0)
	riff := find(nodes, "RIFF")
	var movi *riffNode
	for index := range riff.children {
		if riff.children[index].form == "movi" {
			movi = &riff.children[index]
		}
	}
	if movi == nil {
		t.Fatal("找不到 movi")
	}
	decoded := 0
	// 逐 chunk 走訪，不掃位元組：JPEG 資料裡本來就可能出現 "00dc" 這四個位元組。
	for offset := 4; offset+8 <= len(movi.payload); {
		id := string(movi.payload[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(movi.payload[offset+4 : offset+8]))
		if id == "00dc" {
			if err := decodeJPEG(movi.payload[offset+8 : offset+8+size]); err != nil {
				t.Fatalf("第 %d 幀解不開：%v", decoded, err)
			}
			decoded++
		}
		offset += 8 + size
		if size%2 == 1 {
			offset++
		}
	}
	if decoded != 3 {
		t.Fatalf("解出 %d 幀", decoded)
	}
}

// 尺寸不符的幀要被拒絕，不能默默寫進去讓播放器自己撞牆。
func TestRecorderRejectsWrongFrameSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.avi")
	recorder, err := NewRecorder(path, 320, 240, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer recorder.Close()
	if err := recorder.WriteFrame(testFrame(160, 120, 0)); err == nil {
		t.Fatal("尺寸不符必須被拒絕")
	}
	if err := recorder.WritePCM(make([]byte, 3)); err == nil {
		t.Fatal("PCM 長度不是 4 的倍數必須被拒絕")
	}
}

var _ = fmt.Sprintf

package session

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"testing"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/ui"
)

// paintTestFramebuffer 直接寫進 UM6618 的 framebuffer。測試用的機器只跑一個原地
// 迴圈，畫面全黑，而全黑的畫面套 scanline 之後還是全黑——那證明不了濾鏡有作用。
func paintTestFramebuffer(s *Session) {
	framebuffer := s.System.Bus.Video().Framebuffer()
	for i := range framebuffer {
		framebuffer[i] = 0xff000000 | uint32(i%251)<<16 | uint32(i%199)<<8 | uint32(i%173)
	}
	s.frame = nil
}

func composeWithFilter(t *testing.T, s *Session, filter string) string {
	t.Helper()
	config := s.config
	config.Video.Filter = filter
	s.SetConfig(config)
	dst := image.NewRGBA(image.Rect(0, 0, 640, 480))
	s.Compose(dst)
	sum := sha256.Sum256(dst.Pix)
	return hex.EncodeToString(sum[:])
}

// 三段 scanline 必須各自產生不同且可重現的畫面。
func TestScanlineFiltersAreDistinctAndReproducible(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 3)
	paintTestFramebuffer(s)

	seen := map[string]string{}
	for _, filter := range []string{"nearest", "scanline25", "scanline50", "scanline75"} {
		first := composeWithFilter(t, s, filter)
		second := composeWithFilter(t, s, filter)
		if first != second {
			t.Fatalf("%s 兩次合成不一致", filter)
		}
		for other, hash := range seen {
			if hash == first {
				t.Fatalf("%s 與 %s 產生相同畫面", filter, other)
			}
		}
		seen[filter] = first
	}
	// 未知的濾鏡名稱視為不套濾鏡，而不是猜一個：設定檔可能來自更新的版本。
	if composeWithFilter(t, s, "crt-royale") != seen["nearest"] {
		t.Fatal("未知濾鏡應該等同不套濾鏡")
	}
}

// 濾鏡不得進截圖：截圖直接取自 UM6618 的顯示孔徑，所以它可以當畫面證據用。
func TestFilterDoesNotReachScreenshots(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 3)

	paintTestFramebuffer(s)
	var captured []byte
	s.Screenshot = func(frame *image.RGBA) error {
		captured = append([]byte(nil), frame.Pix...)
		return nil
	}
	config := s.config
	config.Video.Filter = "nearest"
	s.SetConfig(config)
	if err := s.capture(ui.CaptureScreenshot); err != nil {
		t.Fatal(err)
	}
	plain := append([]byte(nil), captured...)

	config.Video.Filter = "scanline75"
	s.SetConfig(config)
	if err := s.capture(ui.CaptureScreenshot); err != nil {
		t.Fatal(err)
	}
	if string(plain) != string(captured) {
		t.Fatal("套了濾鏡之後截圖不得改變")
	}
}

// 圖層遮罩只影響 framebuffer 合成，不影響指令數與硬體時序。
func TestLayerMaskDoesNotChangeMachine(t *testing.T) {
	s := newTestSession(t)
	advance(t, s, 4)
	before := s.System.Instructions
	beforeFrame := s.System.Bus.Video().Frame()

	if err := s.apply(ui.SetLayerMask{Mask: ui.LayerSprite}); err != nil {
		t.Fatal(err)
	}
	if s.System.Instructions != before || s.System.Bus.Video().Frame() != beforeFrame {
		t.Fatalf("遮罩改變了 machine：指令 %d→%d frame %d→%d",
			before, s.System.Instructions, beforeFrame, s.System.Bus.Video().Frame())
	}
	if s.layerMask != ui.LayerSprite {
		t.Fatalf("遮罩 %d", s.layerMask)
	}

	// 恢復全部圖層之後仍然不影響指令數。
	if err := s.apply(ui.SetLayerMask{Mask: uint32(umc6618.AllLayers)}); err != nil {
		t.Fatal(err)
	}
	if s.System.Instructions != before {
		t.Fatal("恢復遮罩不得改變指令數")
	}
}

// 診斷顯示的數字要等於 machine 的實際值，不能是另外算的。
func TestDiagnosticsReadMachineDirectly(t *testing.T) {
	s := newTestSession(t)
	s.FrontendName = "test"
	advance(t, s, 5)
	facts := s.Diagnostics()
	if facts.M68K != s.System.Instructions {
		t.Fatalf("68000 指令 %d，machine 是 %d", facts.M68K, s.System.Instructions)
	}
	if facts.M65C02 != s.System.SoundInstructions {
		t.Fatalf("65C02 指令 %d", facts.M65C02)
	}
	if facts.Frame != s.System.Bus.Video().Frame() {
		t.Fatalf("frame %d", facts.Frame)
	}
	if facts.IPL != s.System.IPLSHA256 || facts.Cartridge != s.System.ROMSHA256 {
		t.Fatal("媒體身分不符")
	}
	if facts.Frontend != "test" {
		t.Fatalf("前端 %q", facts.Frontend)
	}
}

// 音量與緩衝只影響主機播放，不改變 UM6619 的取樣。改音量前後，
// 送進 sink 的樣本序列必須逐位元相同。
func TestAudioSettingsDoNotFeedBackIntoTheCore(t *testing.T) {
	reference := newTestSession(t)
	hashA := audioHashOver(t, reference, 3)

	quiet := newTestSession(t)
	config := quiet.config
	config.Audio.MasterVolume = 10
	config.Audio.BufferMS = 50
	quiet.SetConfig(config)
	hashB := audioHashOver(t, quiet, 3)

	if hashA != hashB {
		t.Fatal("改音量或緩衝之後 UM6619 的取樣序列變了")
	}
}

func audioHashOver(t *testing.T, s *Session, frames int) string {
	t.Helper()
	digest := sha256.New()
	s.System.SoundBus.Audio().SetSampleSink(func(sample umc6619.Sample) {
		encoded := [4]byte{
			byte(sample.Left), byte(sample.Left >> 8),
			byte(sample.Right), byte(sample.Right >> 8),
		}
		_, _ = digest.Write(encoded[:])
	})
	advance(t, s, frames)
	return hex.EncodeToString(digest.Sum(nil))
}

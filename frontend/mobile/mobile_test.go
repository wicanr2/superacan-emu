package mobile

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/superacan-emu/ui"
)

func TestSurfaceFollowsDensityUntilSpaceRunsOut(t *testing.T) {
	cases := []struct {
		name          string
		w, h          int
		density       float64
		wantScale     int
		wantDesignedW int
	}{
		{"1080p 手機橫向", 2400, 1080, 3, 3, 800},
		{"1080×1920 密度 2.625", 1920, 1080, 2.625, 3, 640},
		{"720p 密度 2", 1280, 720, 2, 2, 640},
		{"低密度小螢幕降回 1", 854, 480, 1.5, 1, 854},
		{"平板密度 2", 2560, 1600, 2, 2, 1280},
		{"密度超出上限時夾住", 3840, 2160, 6, 4, 960},
		{"密度回報 0 時當成 1", 1280, 720, 0, 1, 1280},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			surface := Surface(tc.w, tc.h, tc.density)
			if surface.Scale != tc.wantScale {
				t.Fatalf("Scale=%d，預期 %d", surface.Scale, tc.wantScale)
			}
			if got := surface.W / surface.Scale; got != tc.wantDesignedW {
				t.Fatalf("設計單位寬 %d，預期 %d", got, tc.wantDesignedW)
			}
			if surface.Profile != ui.ProfileTouch {
				t.Fatal("行動平台一律用觸控版面")
			}
			if surface.W/surface.Scale < MinDesignWidth && surface.Scale != 1 {
				t.Fatalf("設計單位寬 %d 低於下限卻沒有降 Scale", surface.W/surface.Scale)
			}
		})
	}
}

// 觸控目標的下限是 44 設計單位；Scale 等於密度時那就是 44 dp。
// 這條把「為什麼 Scale 取密度」釘住，換算方式改掉時測試要跟著改。
func TestSurfaceKeepsTouchTargetsAtLeast44dp(t *testing.T) {
	const density = 3
	surface := Surface(2400, 1080, density)
	targetPx := float64(ui.TouchMinTarget * surface.Scale)
	if dp := targetPx / density; dp < 44 {
		t.Fatalf("最小觸控目標 %.1f dp，低於 44 dp", dp)
	}
}

func TestPathsUnderKeepsEverythingInsideTheAppDirectory(t *testing.T) {
	root := t.TempDir()
	paths := PathsUnder(root)
	for name, path := range map[string]string{
		"Firmware": paths.Firmware, "Cartridges": paths.Cartridges,
		"StateRoot": paths.StateRoot, "SaveDir": paths.SaveDir,
		"CheatDir": paths.CheatDir, "CaptureDir": paths.CaptureDir,
		"ConfigFile": paths.ConfigFile,
	} {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) > 2 && rel[:3] == "../" {
			t.Errorf("%s 落在 app 目錄之外：%s", name, path)
		}
	}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("Ensure：%v", err)
	}
	// 再跑一次要是無害的：每次啟動都會呼叫。
	if err := paths.Ensure(); err != nil {
		t.Fatalf("第二次 Ensure：%v", err)
	}
}

func TestFirmwareFilesMatchTheDesktopNames(t *testing.T) {
	paths := PathsUnder("/data/files")
	ipl, key, one, two := paths.FirmwareFiles()
	want := []string{
		"/data/files/firmware/internal_68k.bin",
		"/data/files/firmware/umc6650.bin",
		"/data/files/firmware/internal_6502_1.bin",
		"/data/files/firmware/internal_6502_2.bin",
	}
	for index, got := range []string{ipl, key, one, two} {
		if got != want[index] {
			t.Errorf("韌體 %d 是 %s，預期 %s", index, got, want[index])
		}
	}
}

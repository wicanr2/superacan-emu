// Package mobile 是行動平台前端的與平台無關部分：表面尺寸政策與檔案位置。
//
// 它刻意不匯入 Ebitengine，所以能在 `GOOS=android CGO_ENABLED=0` 下建置與測試。
// 需要 NDK 的只有真正接上視窗與觸控的那一層（`mobile/acan`）。這樣分的理由是
// 「什麼尺寸配什麼版面」「存檔寫到哪裡」這兩件事會被實機行為打臉，但不需要實機
// 才能寫對——它們可以現在就被測試釘住。
package mobile

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/wicanr2/superacan-emu/ui"
)

const (
	// MinDesignWidth 與 MinDesignHeight 是介面可用的最小設計單位尺寸。
	// 橫向版面要放得下置中的 4:3 畫面與兩側的方向鍵、面鍵；低於這個尺寸時
	// 寧可讓觸控目標小於 44 dp，也不要讓控制項疊在一起。
	MinDesignWidth  = 640
	MinDesignHeight = 360
	// MaxScale 是設計單位到像素的最大倍率。再高只是把同樣的版面畫得更大，
	// 不會多出可用空間。
	MaxScale = 4
)

// Surface 依實體像素尺寸與顯示密度決定介面表面。
//
// Scale 取顯示密度：介面的最小觸控目標是 44 設計單位，Scale 等於密度時那 44 單位
// 剛好是 44 dp，也就是 Android 建議的最小觸控尺寸。密度高的螢幕因此得到的是
// 「一樣大的按鍵、更清楚的字」，而不是「一樣多的像素、更小的按鍵」。
//
// 低密度小螢幕上照密度取值會讓設計單位不夠放控制項，這時降低 Scale：
// 按鍵小一點還能按，控制項互相重疊就不能用了。
func Surface(widthPx, heightPx int, density float64) ui.Surface {
	scale := int(math.Round(density))
	if scale < 1 {
		scale = 1
	}
	if scale > MaxScale {
		scale = MaxScale
	}
	for scale > 1 && (widthPx/scale < MinDesignWidth || heightPx/scale < MinDesignHeight) {
		scale--
	}
	return ui.Surface{W: widthPx, H: heightPx, Scale: scale, Profile: ui.ProfileTouch}
}

// Paths 是 app 私有目錄底下的分區。行動平台沒有命令列旗標，這些位置由程式決定，
// 所以它們必須是可預期而且寫得下來的。
type Paths struct {
	// Firmware 放四份主機韌體，Cartridges 放卡帶。兩者都由使用者自己放進去：
	// 受版權保護的內容不隨程式散布，程式也不代為下載。
	Firmware   string
	Cartridges string

	StateRoot  string
	SaveDir    string
	CheatDir   string
	CaptureDir string
	ConfigFile string
}

// PathsUnder 依 app 的檔案目錄組出分區。Android 端傳進來的應該是
// getExternalFilesDir(null)：那個位置不需要任何權限，使用者又能用檔案管理程式
// 或 USB 放入韌體與卡帶。
func PathsUnder(filesDir string) Paths {
	return Paths{
		Firmware:   filepath.Join(filesDir, "firmware"),
		Cartridges: filepath.Join(filesDir, "cartridges"),
		StateRoot:  filepath.Join(filesDir, "states"),
		SaveDir:    filepath.Join(filesDir, "saves"),
		CheatDir:   filepath.Join(filesDir, "cheats"),
		CaptureDir: filepath.Join(filesDir, "captures"),
		ConfigFile: filepath.Join(filesDir, "settings.json"),
	}
}

// Ensure 建立所有目錄。缺目錄時的失敗要在啟動時就看得見，而不是等到第一次存檔。
func (p Paths) Ensure() error {
	for _, dir := range []string{p.Firmware, p.Cartridges, p.StateRoot, p.SaveDir, p.CheatDir, p.CaptureDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mobile: 建立 %s：%w", dir, err)
		}
	}
	return nil
}

// FirmwareFiles 是四份韌體在 Firmware 目錄下的檔名，與桌面旗標的名稱一致。
func (p Paths) FirmwareFiles() (ipl, key, soundBIOS1, soundBIOS2 string) {
	return filepath.Join(p.Firmware, "internal_68k.bin"),
		filepath.Join(p.Firmware, "umc6650.bin"),
		filepath.Join(p.Firmware, "internal_6502_1.bin"),
		filepath.Join(p.Firmware, "internal_6502_2.bin")
}

// Package acan 是 Android 應用程式的 Go 端。gomobile 把這裡匯出的函式變成 Java
// 方法，Activity 只做四件事：啟動、返回鍵、離開前景、回到前景。
//
// 這一層要 cgo：Android 應用程式的原生碼必須是被 Java runtime 載入的共享程式庫，
// 而 -buildmode=c-shared 在任何平台都要求 cgo。禁 cgo 的範圍因此限定在模擬核心
// 與 Linux、macOS 的發行 binary，見 docs/platform-targets.md。
//
// 與平台無關的判斷（表面尺寸、檔案位置）在 frontend/mobile，那個套件在
// GOOS=android CGO_ENABLED=0 下就能建置與測試。
package acan

import (
	"fmt"
	"image"
	"path/filepath"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	ebitenmobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/frontend/hostio"
	"github.com/wicanr2/superacan-emu/frontend/mobile"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/presentation"
	"github.com/wicanr2/superacan-emu/session"
	"github.com/wicanr2/superacan-emu/ui"
)

// frontendName 是寫進設定檔的前端識別字串。Android 的鍵碼與 X11 keysym、macOS
// 虛擬鍵碼都是不同的數值空間，綁定必須記得是誰寫的。
const frontendName = "android"

// 版本資訊。發行時由 -ldflags 覆蓋。
var (
	buildVersion = "dev"
	buildDate    = "unknown"
)

var (
	mutex   sync.Mutex
	current *game
)

// Start 由 Activity 的 onCreate 呼叫，filesDir 應該是 getExternalFilesDir(null)：
// 那個位置不需要任何權限，使用者又能用檔案管理程式或 USB 把韌體與卡帶放進去。
//
// 沒有卡帶、沒有韌體都不是錯誤：介面本來就有啟動畫面與韌體設定畫面，會把缺什麼
// 寫在畫面上。這裡只在連目錄都建不出來時失敗。
func Start(filesDir string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if current != nil {
		return nil
	}
	paths := mobile.PathsUnder(filesDir)
	if err := paths.Ensure(); err != nil {
		return err
	}
	g, err := newGame(paths)
	if err != nil {
		return err
	}
	current = g
	ebitenmobile.SetGame(g)
	return nil
}

// Back 是 Android 的返回鍵。回傳 true 代表介面吃掉了這次按鍵，Activity 不要再
// 交給系統——ui.handleBack 一律吃掉：在遊戲中返回鍵開啟選單，在選單中往回退一層，
// 所以返回鍵永遠不會把應用程式關掉。要離開請用選單裡的「離開模擬器」。
func Back() bool {
	mutex.Lock()
	defer mutex.Unlock()
	if current == nil {
		return false
	}
	return current.session.Handle(ui.Life{Kind: ui.LifeBack})
}

// Suspend 由 onPause 呼叫。行動平台沒有「正常結束」：切走之後程式可能直接被回收，
// 所以這一刻是最後一次能寫檔的機會。
func Suspend() {
	mutex.Lock()
	defer mutex.Unlock()
	if current != nil {
		current.session.Handle(ui.Life{Kind: ui.LifeSuspend})
	}
}

// Resume 由 onResume 呼叫。不自動恢復執行：畫面上是覆蓋選單，由使用者按「繼續遊戲」。
func Resume() {
	mutex.Lock()
	defer mutex.Unlock()
	if current != nil {
		current.session.Handle(ui.Life{Kind: ui.LifeResume})
	}
}

// game 是 ebiten.Game 的實作。它只做「推進、合成、貼圖、收觸控」，
// 所有硬體語意仍然由 machine 與 session 決定。
type game struct {
	session *session.Session
	paths   mobile.Paths

	firmware  [4][]byte
	surface   ui.Surface
	composite *image.RGBA
	screen    *ebiten.Image
	started   time.Time

	audioContext *audio.Context
	player       *audio.Player
	stream       *presentation.PCM16StereoStream

	current     *machine.System
	currentSave string
	touches     map[ebiten.TouchID]struct{}
}

func newGame(paths mobile.Paths) (*game, error) {
	g := &game{
		paths:   paths,
		started: time.Now(),
		touches: map[ebiten.TouchID]struct{}{},
	}
	g.loadFirmware()

	config := ui.DefaultConfig()
	if loaded, _, err := session.LoadConfig(paths.ConfigFile); err == nil {
		config = loaded
	}
	config.Paths.CartridgeDirs = []string{paths.Cartridges}

	// 表面尺寸在第一次 Layout 才知道，先用一個合理的預設值建立介面；
	// Layout 會立刻送出真正的尺寸。
	g.surface = mobile.Surface(1280, 720, 1)
	ipl, key, soundA, soundB := paths.FirmwareFiles()
	library := session.NewLibrary([]string{paths.Cartridges}, nil, paths.StateRoot, paths.SaveDir)
	g.session = session.New(session.Options{
		Surface:     g.surface,
		Config:      config,
		Library:     library,
		FirmwareSet: session.DescribeFirmwareSet(ipl, key, soundA, soundB),
		About:       session.About(buildVersion, buildDate),
	})
	g.session.StateRoot = paths.StateRoot
	g.session.ConfigPath = paths.ConfigFile
	g.session.CaptureDir = paths.CaptureDir
	g.session.FrontendName = frontendName
	g.session.ScriptFrontend = frontendName
	g.session.Loader = g.loadCartridge
	g.session.Flush = g.flush
	g.session.Screenshot = func(*image.RGBA) error {
		if g.current == nil {
			return fmt.Errorf("acan: 沒有卡帶可以截圖")
		}
		return hostio.WriteScreenshot(
			filepath.Join(g.paths.CaptureDir, screenshotName()),
			g.current.Bus.Video().Framebuffer())
	}
	g.session.Shell()
	return g, nil
}

// loadFirmware 讀四份韌體。缺檔不是錯誤：介面的韌體畫面會列出缺哪一份，
// 那比啟動失敗更有用——使用者要知道該把檔案放到哪裡。
func (g *game) loadFirmware() {
	ipl, key, soundA, soundB := g.paths.FirmwareFiles()
	if bytes, err := hostio.LoadWordSwapped(ipl, machine.IPLSize); err == nil {
		g.firmware[0] = bytes
	}
	if bytes, err := hostio.LoadLinear(key, 16); err == nil {
		g.firmware[1] = bytes
	}
	if bytes, err := hostio.LoadLinear(soundA, machine.SoundBIOSBankSize); err == nil {
		g.firmware[2] = bytes
	}
	if bytes, err := hostio.LoadLinear(soundB, machine.SoundBIOSBankSize); err == nil {
		g.firmware[3] = bytes
	}
}

func (g *game) loadCartridge(path string) (*machine.System, string, error) {
	if g.firmware[0] == nil || g.firmware[1] == nil {
		return nil, "", fmt.Errorf("acan: 韌體不齊，無法啟動卡帶")
	}
	image, err := hostio.LoadCartridge(path)
	if err != nil {
		return nil, "", err
	}
	system, err := machine.NewSystem(g.firmware[0], image.Bytes, g.firmware[1])
	if err != nil {
		return nil, "", err
	}
	if g.firmware[2] != nil && g.firmware[3] != nil {
		if err := system.LoadSoundBIOS(0, g.firmware[2]); err != nil {
			return nil, "", err
		}
		if err := system.LoadSoundBIOS(1, g.firmware[3]); err != nil {
			return nil, "", err
		}
	}
	// 換卡帶前先把上一片的電池記憶體寫回去。
	if g.current != nil {
		_ = hostio.WriteCartridgeSave(g.current, g.currentSave)
	}
	save := session.BatteryPathFor(g.paths.SaveDir, path)
	if err := hostio.LoadCartridgeSave(system, save); err != nil {
		return nil, "", err
	}
	if err := system.Reset(); err != nil {
		return nil, "", err
	}
	g.current, g.currentSave = system, save
	g.attachAudio(system)
	return system, session.TitleFromPath(path), nil
}

// attachAudio 接上主機音訊。取樣回呼只做重取樣與入列，不讀設定：
// 它一秒跑四萬多次，在那裡讀介面狀態會把介面拉進模擬迴圈。音量在每一幀套用。
func (g *game) attachAudio(system *machine.System) {
	if g.audioContext == nil {
		g.audioContext = audio.NewContext(48000)
	}
	g.stream = presentation.NewPCM16StereoStream(48000 / 5)
	resampler := presentation.NewStereoResampler(umc6619.ClockHz, umc6619.CyclesPerSample, 48000,
		func(left, right int16) {
			g.stream.Push(left, right)
			g.session.PushCapturePCM([]byte{
				byte(uint16(left)), byte(uint16(left) >> 8),
				byte(uint16(right)), byte(uint16(right) >> 8),
			})
		})
	system.SoundBus.Audio().SetSampleSink(func(sample umc6619.Sample) {
		resampler.Push(sample.Left, sample.Right)
	})
	if g.player != nil {
		_ = g.player.Close()
	}
	player, err := g.audioContext.NewPlayer(g.stream)
	if err != nil {
		g.session.UI.Fail(err.Error())
		return
	}
	g.player = player
	g.player.Play()
}

// flush 把卡帶電池記憶體寫回去。離開前景時由 session 呼叫。
func (g *game) flush() error {
	if g.current == nil || g.currentSave == "" {
		return nil
	}
	return hostio.WriteCartridgeSave(g.current, g.currentSave)
}

func (g *game) Update() error {
	g.collectTouches()
	g.session.SetPad(0, uint16(g.session.UI.TouchPad()))
	if _, err := g.session.Advance(time.Since(g.started)); err != nil {
		return err
	}
	if g.player != nil {
		g.player.SetVolume(float64(g.session.Volume()) / 100)
	}
	if g.session.Quitting() {
		return ebiten.Termination
	}
	return nil
}

// collectTouches 把這一幀的觸控翻成 ui.Pointer。座標是實體像素，
// ui 自己依表面的 Scale 換算成設計單位。
func (g *game) collectTouches() {
	active := map[ebiten.TouchID]struct{}{}
	for _, id := range ebiten.AppendTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		phase := ui.PhaseMove
		if _, seen := g.touches[id]; !seen {
			phase = ui.PhaseDown
		}
		active[id] = struct{}{}
		g.session.Handle(ui.Pointer{ID: int(id), X: x, Y: y, Phase: phase})
	}
	for id := range g.touches {
		if _, still := active[id]; !still {
			g.session.Handle(ui.Pointer{ID: int(id), Phase: ui.PhaseUp})
		}
	}
	g.touches = active
}

func (g *game) Draw(screen *ebiten.Image) {
	bounds := screen.Bounds()
	if g.composite == nil || g.composite.Bounds() != bounds {
		g.composite = image.NewRGBA(bounds)
	}
	g.session.Compose(g.composite)
	screen.WritePixels(g.composite.Pix)
}

// Layout 是 ebiten.Game 要求的整數版本；實際使用的是 LayoutF，
// 因為那才拿得到裝置的縮放倍率。
func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

// LayoutF 回傳實體像素尺寸：介面在原生解析度上作畫，文字才不會被放大兩次。
// 尺寸或方向改變時把新的表面送給介面，觸控版面會跟著重算。
func (g *game) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	scale := ebiten.Monitor().DeviceScaleFactor()
	if scale <= 0 {
		scale = 1
	}
	widthPx := int(outsideWidth * scale)
	heightPx := int(outsideHeight * scale)
	if widthPx < 1 || heightPx < 1 {
		return outsideWidth, outsideHeight
	}
	if surface := mobile.Surface(widthPx, heightPx, scale); surface != g.surface {
		g.surface = surface
		g.session.Handle(surface)
	}
	return float64(widthPx), float64(heightPx)
}

// screenshotName 以本地時間命名截圖。
func screenshotName() string {
	return fmt.Sprintf("acan-%s.png", time.Now().Format("20060102-150405"))
}

// acan-x11 是不需要 cgo 的 Linux 桌面前端：以純 Go 的 X11 呈現層輸出畫面與讀取鍵盤，
// 音訊則交給外部播放程序，讓整個 binary 在 CGO_ENABLED=0 下可以建置。
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/frontend/hostio"
	"github.com/wicanr2/superacan-emu/frontend/x11"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/session"
	"github.com/wicanr2/superacan-emu/ui"
)

// X11 keysym；數值出自 X11 的 keysymdef.h。
const (
	keysymLeft   = 0xff51
	keysymUp     = 0xff52
	keysymRight  = 0xff53
	keysymDown   = 0xff54
	keysymReturn = 0xff0d
	keysymShiftR = 0xffe2
	keysymEscape = 0xff1b
	keysymF2     = 0xffbf
	keysymF3     = 0xffc0
	keysymF4     = 0xffc1
	keysymF5     = 0xffc2
	keysymF6     = 0xffc3
	keysymF7     = 0xffc4
	keysymF8     = 0xffc5
	keysymF9     = 0xffc6
	keysymF10    = 0xffc7
	keysymF11    = 0xffc8
	keysymF12    = 0xffc9
	keysymLowerA = 0x0061
	keysymLowerQ = 0x0071
	keysymLowerS = 0x0073
	keysymLowerW = 0x0077
	keysymLowerX = 0x0078
	keysymLowerZ = 0x007a
	keysymDigit2 = 0x0032
	keysymDigit6 = 0x0036
	keysymLowerD = 0x0064
	keysymLowerF = 0x0066
	keysymLowerG = 0x0067
	keysymLowerI = 0x0069
	keysymLowerK = 0x006b
	keysymLowerO = 0x006f
	keysymLowerP = 0x0070
	keysymLowerR = 0x0072
	keysymLowerU = 0x0075
	keysymLowerY = 0x0079
)

type keyBinding struct {
	keysym uint32
	button uint16
}

func main() {
	iplPath := flag.String("ipl", "", "path to word-swapped 4096-byte internal_68k.bin")
	keyPath := flag.String("key", "", "path to 16-byte umc6650.bin")
	soundBIOS1Path := flag.String("sound-bios1", "", "path to 8192-byte internal_6502_1.bin")
	soundBIOS2Path := flag.String("sound-bios2", "", "path to 8192-byte internal_6502_2.bin")
	romPath := flag.String("rom", "", "path to a raw cartridge dump or a cartridge ZIP")
	savePath := flag.String("save", "", "32768-byte cartridge battery file, loaded at start and written on exit")
	statePath := flag.String("state", "", "save state file; F5 writes it and F7 restores it")
	scale := flag.Int("scale", 3, "integer window scale")
	frames := flag.Uint64("frames", 0, "exit after this many emulated frames (0 runs until the window closes)")
	screenshot := flag.String("screenshot", "", "write the final emulated framebuffer as PNG")
	pace := flag.Bool("pace", true, "wait for the 60 Hz host tick between frames; disable for bounded smoke runs")
	audioSink := flag.String("audio-sink", "", "shell command receiving 48000 Hz signed 16-bit stereo PCM on stdin, for example \"aplay -f cd -t raw\"")
	stateDir := flag.String("state-dir", "", "directory holding the ten save-state slots the overlay menu reads and writes")
	uiScript := flag.String("ui-script", "", "scripted overlay events for smoke runs: frame:EVENT,... where EVENT is one of "+session.ScriptEventNames())
	press := flag.String("press", "", "P1 input timeline: tick:BUTTON+BUTTON[*frames],... for scripted runs")
	press2 := flag.String("press2", "", "P2 input timeline, same syntax as --press")
	record := flag.String("record", "", "record the composed window (game plus overlay) to this AVI file")
	romDirs := flag.String("rom-dir", "", "comma-separated directories the cartridge browser scans")
	stateRoot := flag.String("state-root", "", "root directory for per-cartridge save-state directories")
	saveDir := flag.String("save-dir", "", "directory holding per-cartridge battery files")
	configPath := flag.String("config", "", "settings file; defaults to the platform config directory, \"none\" disables it")
	captureDir := flag.String("capture-dir", ".", "directory for screenshots and clips")
	captureSink := flag.String("capture-sink", "", "shell command receiving raw 320x240 RGBA frames on stdin instead of writing an AVI")
	maxTicks := flag.Uint64("max-ticks", 0, "exit after this many host loop iterations regardless of pause state; for scripted smoke runs")
	flag.Parse()

	// 沒給路徑時用使用者資料目錄底下的預設位置。發行包要能直接點兩下就開，
	// 缺韌體或缺卡帶不是啟動失敗——介面本來就有啟動畫面與韌體畫面會說明缺什麼。
	defaults := defaultPaths()
	if *iplPath == "" {
		*iplPath, *keyPath = defaults.ipl, defaults.key
		*soundBIOS1Path, *soundBIOS2Path = defaults.soundA, defaults.soundB
	}
	if *romPath == "" && *romDirs == "" {
		*romDirs = defaults.cartridges
	}
	if (*soundBIOS1Path == "") != (*soundBIOS2Path == "") {
		fail("--sound-bios1 and --sound-bios2 must be supplied together")
	}
	if *scale < 1 {
		fail("--scale must be at least 1")
	}

	// 設定檔在建視窗之前讀：綁定與音量都要在第一個 frame 就生效。
	settingsPath := *configPath
	if settingsPath == "" {
		if resolved, err := session.ConfigPath(); err == nil {
			settingsPath = resolved
		}
	}
	if settingsPath == "none" {
		settingsPath = ""
	}
	config := ui.DefaultConfig()
	if settingsPath != "" {
		loaded, warnings, err := session.LoadConfig(settingsPath)
		if err != nil {
			fail(fmt.Sprintf("settings: %v", err))
		}
		config = loaded
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "settings: %s\n", warning)
		}
	}
	playerOneKeys := bindingsFor(config, 0)
	playerTwoKeys := bindingsFor(config, 1)
	// --state 是覆蓋層之前就有的單檔存讀路徑，仍然佔用 F5／F7。給了它就把
	// 兩個槽位熱鍵讓開，同一個鍵不會有兩種存檔語意。
	var skipHotkeys []string
	if *statePath != "" {
		skipHotkeys = []string{"save_state", "load_state"}
	}
	hotkeys := hotkeyBindings(config, skipHotkeys...)

	// 韌體只讀一次；換卡帶時整台機器重建，但韌體位元組沿用同一份。
	// 讀不到不是錯誤：DescribeFirmwareSet 會把缺哪一份寫在啟動畫面上，
	// 那比啟動失敗有用——使用者要知道檔案該放到哪裡。
	iplBytes, _ := hostio.LoadWordSwapped(*iplPath, machine.IPLSize)
	keyBytes, _ := hostio.LoadLinear(*keyPath, 16)
	var soundBIOS [2][]byte
	if *soundBIOS1Path != "" {
		soundBIOS[0], _ = hostio.LoadLinear(*soundBIOS1Path, machine.SoundBIOSBankSize)
		soundBIOS[1], _ = hostio.LoadLinear(*soundBIOS2Path, machine.SoundBIOSBankSize)
	}

	stopAudio := func() {}
	var overlayRef *session.Session
	// audioVolume 由主迴圈每幀寫入、由音訊送出的那一段讀取。
	var audioVolume atomic.Int32
	audioVolume.Store(100)
	attachAudio := func(system *machine.System) {
		if *audioSink == "" {
			return
		}
		stopAudio()
		stop, err := hostio.AudioSink(system, *audioSink, func(pcm []byte) {
			if overlayRef != nil {
				overlayRef.PushCapturePCM(pcm)
			}
		}, &audioVolume)
		if err != nil {
			fail(fmt.Sprintf("audio sink: %v", err))
		}
		stopAudio = stop
	}
	defer func() { stopAudio() }()

	// 目前掛著的卡帶，退出時要把電池記憶體寫回去。
	var current *machine.System
	var currentSave string
	newSystem := func(path string) (*machine.System, string, error) {
		if iplBytes == nil || keyBytes == nil {
			return nil, "", fmt.Errorf("韌體不齊，無法啟動卡帶")
		}
		image, err := hostio.LoadCartridge(path)
		if err != nil {
			return nil, "", err
		}
		system, err := machine.NewSystem(iplBytes, image.Bytes, keyBytes)
		if err != nil {
			return nil, "", err
		}
		if soundBIOS[0] != nil {
			if err := system.LoadSoundBIOS(0, soundBIOS[0]); err != nil {
				return nil, "", err
			}
			if err := system.LoadSoundBIOS(1, soundBIOS[1]); err != nil {
				return nil, "", err
			}
		}
		if current != nil {
			_ = hostio.WriteCartridgeSave(current, currentSave)
		}
		save := *savePath
		if save == "" && *saveDir != "" {
			save = session.BatteryPathFor(*saveDir, path)
		}
		if err := hostio.LoadCartridgeSave(system, save); err != nil {
			return nil, "", err
		}
		if err := system.Reset(); err != nil {
			return nil, "", err
		}
		attachAudio(system)
		current, currentSave = system, save
		return system, session.TitleFromPath(path), nil
	}

	var system *machine.System
	if *romPath != "" {
		var err error
		system, _, err = newSystem(*romPath)
		if err != nil {
			fail(err.Error())
		}
	}
	defer func() { _ = hostio.WriteCartridgeSave(current, currentSave) }()

	window, err := x11.New("Super A'Can Emulator", umc6618.Width, umc6618.Height, *scale)
	if err != nil {
		fail(err.Error())
	}
	defer window.Close()

	windowW, windowH := window.Size()
	library := session.NewLibrary(splitList(*romDirs), recentFor(*romPath), *stateRoot, *saveDir)
	overlay := session.New(session.Options{
		System:      system,
		Title:       session.TitleFromPath(*romPath),
		StateDir:    *stateDir,
		Surface:     ui.Surface{W: windowW, H: windowH, Scale: 1, Profile: ui.ProfileCompact},
		Config:      config,
		Library:     library,
		FirmwareSet: session.DescribeFirmwareSet(*iplPath, *keyPath, *soundBIOS1Path, *soundBIOS2Path),
		About:       session.About(buildVersion, buildDate),
	})
	overlayRef = overlay
	// 錄影檔的長度欄位在收尾時才回填；沒有收尾的檔案播放器一幀也讀不出來。
	defer func() {
		if err := overlay.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "capture: %v\n", err)
		}
	}()
	overlay.StateRoot = *stateRoot
	overlay.ConfigPath = settingsPath
	overlay.CaptureDir = *captureDir
	overlay.FrontendName = frontendName
	overlay.UI.SetDefaultHotkeys(defaultHotkeyBindings())
	if *captureSink != "" {
		sink, stopSink, sinkErr := hostio.CaptureSink(*captureSink)
		if sinkErr != nil {
			fail(fmt.Sprintf("capture sink: %v", sinkErr))
		}
		overlay.SetCaptureSink(sink)
		defer stopSink()
	}
	overlay.ScriptFrontend = frontendName
	overlay.Loader = newSystem
	overlay.Screenshot = func(frame *image.RGBA) error {
		return hostio.WriteScreenshot(filepath.Join(*captureDir, screenshotName()), current.Bus.Video().Framebuffer())
	}
	input := newOverlayInput()
	script, err := session.ParseScript(*uiScript)
	if err != nil {
		fail(err.Error())
	}
	presses, err := session.ParsePresses(*press)
	if err != nil {
		fail(err.Error())
	}
	presses2, err := session.ParsePresses(*press2)
	if err != nil {
		fail(err.Error())
	}
	// 錄的是合成後的視窗：展示影片要看得到覆蓋層與觸控版面，
	// 而不含覆蓋層的那一條是給畫面證據用的，兩者不共用。
	if *record != "" {
		overlay.SetCaptureComposed(windowW, windowH)
		if err := overlay.StartCapture(*record); err != nil {
			fail(err.Error())
		}
	}
	injected, injected2 := machine.PadReleased, machine.PadReleased

	// SetTPS 的等價物：主機以 60 Hz 請求下一個硬體 frame，硬體 frame 邊界仍由
	// cycle scheduler 決定，不用改變核心 cycle 數來配合主機。
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	var completed, tick uint64
	var savePressed, loadPressed bool
	started := time.Now()
	for {
		if !window.Poll() || overlay.Quitting() {
			break
		}
		// 覆蓋層沒開的時候 Esc 仍是離開，這是現行行為；改成「開啟選單」
		// 要等 WORKLIST A1 拍板。
		if !overlay.UI.Visible() && window.KeysymPressed(keysymEscape) {
			break
		}
		// 腳本以主機迴圈的次數計時，不用模擬 frame 數：覆蓋層開著時模擬時間
		// 停住，用 frame 數當索引會讓腳本永遠等不到下一個事件。
		// 腳本與按鍵注入用同一個計數，否則同一份時間表在兩邊會差一拍。
		now := tick
		overlay.Play(script, now)
		tick++
		// --frames 只數真正跑掉的 frame，覆蓋層開著時它不會前進。腳本會停在
		// 選單裡，所以還需要一個不受暫停影響的上限，否則 smoke run 不會結束。
		if *maxTicks != 0 && tick > *maxTicks {
			break
		}
		// 等待指定綁定時只送原始按鍵：否則 Enter 會同時被當成確認與被指定。
		if overlay.UI.WantsRawInput() {
			for _, keysym := range window.TakeKeyPresses() {
				if keysym == keysymEscape {
					overlay.Handle(ui.Action{Kind: ui.ActCancel})
					continue
				}
				overlay.Handle(ui.RawKey{
					Frontend: frontendName, Code: keysym, Label: keysymLabel(keysym),
				})
			}
			for _, key := range overlayKeys {
				input.edge(window, key.keysym)
			}
			input.edge(window, keysymEscape)
			for _, hotkey := range hotkeys {
				input.transition(window, hotkey.keysym)
			}
		} else {
			window.TakeKeyPresses()
			// 熱鍵一律走 ui.Hotkey：哪些動作在什麼狀態下生效由介面決定，
			// 前端只負責把「這個鍵剛按下／剛放開」翻譯成動作名稱。
			for _, hotkey := range hotkeys {
				down, up := input.transition(window, hotkey.keysym)
				if down {
					overlay.UI.Hotkey(hotkey.action)
				}
				if up {
					overlay.UI.HotkeyRelease(hotkey.action)
				}
			}
		}
		if overlay.UI.WantsRawInput() {
			// 已經處理完，這一輪不再翻譯導覽事件。
		} else if overlay.UI.Visible() {
			if input.edge(window, keysymEscape) {
				overlay.Handle(ui.Action{Kind: ui.ActCancel})
			}
			for _, key := range overlayKeys {
				if input.edge(window, key.keysym) {
					overlay.Handle(key.event)
				}
			}
		} else {
			// 覆蓋層關著時仍要更新邊緣狀態，否則關掉選單後第一次按鍵會被吃掉。
			for _, key := range overlayKeys {
				input.edge(window, key.keysym)
			}
			input.edge(window, keysymEscape)
		}
		// --state 的單檔存讀：只在給了路徑時生效，鍵位是 F5／F7。
		// 取按下的那一瞬間，按著不放不會重複觸發。
		if *statePath != "" {
			if pressed := window.KeysymPressed(keysymF5); pressed && !savePressed {
				reportStateResult("save", hostio.WriteSaveState(overlay.System, *statePath))
			}
			savePressed = window.KeysymPressed(keysymF5)
			if pressed := window.KeysymPressed(keysymF7); pressed && !loadPressed {
				reportStateResult("load", hostio.ReadSaveState(overlay.System, *statePath))
			}
			loadPressed = window.KeysymPressed(keysymF7)
		}
		// 音量交給音訊執行緒讀：那一段每 10 ms 跑一次，不能在那裡讀介面狀態。
		audioVolume.Store(int32(overlay.Volume()))
		// 注入與鍵盤是聯集：兩者都是 active-low，AND 起來就是「任一按下即按下」。
		injected = session.ApplyPresses(now, injected, presses)
		injected2 = session.ApplyPresses(now, injected2, presses2)
		overlay.SetPad(0, padState(window, playerOneKeys)&injected)
		overlay.SetPad(1, padState(window, playerTwoKeys)&injected2)
		advanced, err := overlay.Advance(time.Since(started))
		if err != nil {
			fail(err.Error())
		}
		if advanced {
			completed++
		}
		if overlay.UI.Visible() {
			// 覆蓋層開著時走合成路徑：畫面停在最後一個完成的 frame，
			// 選單畫在視窗的原生解析度上。
			canvas := input.canvas(windowW, windowH)
			overlay.Compose(canvas)
			if err := window.PresentRGBA(canvas.Pix, windowW, windowH); err != nil {
				fail(err.Error())
			}
		} else if err := window.Present(overlay.System.Bus.Video().Framebuffer()); err != nil {
			fail(err.Error())
		}
		if *frames != 0 && completed >= *frames {
			break
		}
		if *pace {
			<-ticker.C
		}
	}

	if *frames != 0 && overlay.System != nil {
		sha := overlay.System.Bus.Video().FramebufferSHA256()
		fmt.Printf("frames=%d instructions=%d framebuffer_sha256=%x\n", completed, overlay.System.Instructions, sha)
	}
	if *screenshot != "" && overlay.System != nil {
		if err := hostio.WriteScreenshot(*screenshot, overlay.System.Bus.Video().Framebuffer()); err != nil {
			fail(err.Error())
		}
	}
}

func padState(window *x11.Window, bindings []keyBinding) uint16 {
	state := machine.PadReleased
	for _, binding := range bindings {
		if window.KeysymPressed(binding.keysym) {
			state |= binding.button
		}
	}
	return state
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "acan-x11:", message)
	os.Exit(1)
}

func reportStateResult(action string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "acan-x11: %s state: %v\n", action, err)
		return
	}
	fmt.Fprintf(os.Stderr, "acan-x11: %s state ok\n", action)
}

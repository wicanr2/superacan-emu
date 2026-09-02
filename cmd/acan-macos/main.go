//go:build darwin

// acan-macos 是 macOS 桌面前端：視窗與輸入透過 purego 直接呼叫 Objective-C
// runtime，音訊交給外部播放程序，所以整個 binary 在 CGO_ENABLED=0 下可以建置。
//
// 模擬核心、介面與 Intent 執行全部由 session 提供，與 X11 前端共用同一份；
// 這個檔案只做「開視窗、翻譯輸入、貼圖」三件事。
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/frontend/cocoa"
	"github.com/wicanr2/superacan-emu/frontend/hostio"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/session"
	"github.com/wicanr2/superacan-emu/ui"
)

func main() {
	// AppKit 只能在主執行緒操作。cocoa.New 也會鎖，這裡先鎖是為了讓
	// 「這個行程的 main goroutine 就是 UI 執行緒」這件事在入口就成立。
	runtime.LockOSThread()

	iplPath := flag.String("ipl", "", "path to word-swapped 4096-byte internal_68k.bin")
	keyPath := flag.String("key", "", "path to 16-byte umc6650.bin")
	soundBIOS1Path := flag.String("sound-bios1", "", "path to 8192-byte internal_6502_1.bin")
	soundBIOS2Path := flag.String("sound-bios2", "", "path to 8192-byte internal_6502_2.bin")
	romPath := flag.String("rom", "", "path to a raw cartridge dump or a cartridge ZIP")
	romDirs := flag.String("rom-dir", "", "comma-separated directories the cartridge browser scans")
	savePath := flag.String("save", "", "32768-byte cartridge battery file")
	saveDir := flag.String("save-dir", "", "directory holding per-cartridge battery files")
	stateDir := flag.String("state-dir", "", "directory holding the ten save-state slots")
	stateRoot := flag.String("state-root", "", "root directory for per-cartridge save-state directories")
	configPath := flag.String("config", "", "settings file; defaults to the platform config directory, \"none\" disables it")
	captureDir := flag.String("capture-dir", ".", "directory for screenshots and clips")
	captureSink := flag.String("capture-sink", "", "shell command receiving raw 320x240 RGBA frames on stdin")
	audioSink := flag.String("audio-sink", "", "shell command receiving 48000 Hz signed 16-bit stereo PCM on stdin")
	scale := flag.Int("scale", 3, "integer window scale")
	frames := flag.Uint64("frames", 0, "exit after this many emulated frames (0 runs until the window closes)")
	pace := flag.Bool("pace", true, "wait for the 60 Hz host tick between frames")
	uiScript := flag.String("ui-script", "", "scripted overlay events for smoke runs: frame:EVENT,... where EVENT is one of "+session.ScriptEventNames())
	maxTicks := flag.Uint64("max-ticks", 0, "exit after this many host loop iterations regardless of pause state")
	flag.Parse()

	if *iplPath == "" || *keyPath == "" {
		fail("--ipl and --key are required")
	}
	if *romPath == "" && *romDirs == "" {
		fail("--rom or --rom-dir is required")
	}
	if (*soundBIOS1Path == "") != (*soundBIOS2Path == "") {
		fail("--sound-bios1 and --sound-bios2 must be supplied together")
	}
	if *scale < 1 {
		fail("--scale must be at least 1")
	}

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
	hotkeys := hotkeyBindings(config)

	iplBytes := must(hostio.LoadWordSwapped(*iplPath, machine.IPLSize))
	keyBytes := must(hostio.LoadLinear(*keyPath, 16))
	var soundBIOS [2][]byte
	if *soundBIOS1Path != "" {
		soundBIOS[0] = must(hostio.LoadLinear(*soundBIOS1Path, machine.SoundBIOSBankSize))
		soundBIOS[1] = must(hostio.LoadLinear(*soundBIOS2Path, machine.SoundBIOSBankSize))
	}

	stopAudio := func() {}
	var overlay *session.Session
	// audioVolume 由主迴圈每幀寫入、由音訊送出的那一段讀取。
	var audioVolume atomic.Int32
	audioVolume.Store(100)
	attachAudio := func(system *machine.System) {
		if *audioSink == "" {
			return
		}
		stopAudio()
		stop, err := hostio.AudioSink(system, *audioSink, func(pcm []byte) {
			if overlay != nil {
				overlay.PushCapturePCM(pcm)
			}
		}, &audioVolume)
		if err != nil {
			fail(fmt.Sprintf("audio sink: %v", err))
		}
		stopAudio = stop
	}
	defer func() { stopAudio() }()

	var current *machine.System
	var currentSave string
	newSystem := func(path string) (*machine.System, string, error) {
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

	window, err := cocoa.New("Super A'Can Emulator", umc6618.Width, umc6618.Height, *scale)
	if err != nil {
		fail(err.Error())
	}
	defer window.Close()

	windowW, windowH := window.Size()
	library := session.NewLibrary(splitList(*romDirs), recentFor(*romPath), *stateRoot, *saveDir)
	overlay = session.New(session.Options{
		System:      system,
		Title:       session.TitleFromPath(*romPath),
		StateDir:    *stateDir,
		Surface:     ui.Surface{W: windowW, H: windowH, Scale: 1, Profile: ui.ProfileCompact},
		Config:      config,
		Library:     library,
		FirmwareSet: session.DescribeFirmwareSet(*iplPath, *keyPath, *soundBIOS1Path, *soundBIOS2Path),
		About:       session.About(buildVersion, buildDate),
	})
	overlay.StateRoot = *stateRoot
	overlay.ConfigPath = settingsPath
	overlay.ScriptFrontend = frontendName
	overlay.FrontendName = frontendName
	overlay.UI.SetDefaultHotkeys(defaultHotkeyBindings())
	overlay.CaptureDir = *captureDir
	overlay.Loader = newSystem
	overlay.Screenshot = func(frame *image.RGBA) error {
		return hostio.WriteScreenshot(filepath.Join(*captureDir, screenshotName()),
			current.Bus.Video().Framebuffer())
	}
	// 錄影檔的長度欄位在收尾時才回填；沒有收尾的檔案播放器一幀也讀不出來。
	defer func() {
		if err := overlay.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "capture: %v\n", err)
		}
	}()
	if *captureSink != "" {
		sink, stop, err := hostio.CaptureSink(*captureSink)
		if err != nil {
			fail(fmt.Sprintf("capture sink: %v", err))
		}
		overlay.SetCaptureSink(sink)
		defer stop()
	}

	script, err := session.ParseScript(*uiScript)
	if err != nil {
		fail(err.Error())
	}

	input := newInputState()
	surface := image.NewRGBA(image.Rect(0, 0, windowW, windowH))
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	var completed, tick uint64
	started := time.Now()
	for {
		if !window.Poll() || overlay.Quitting() {
			break
		}
		overlay.Play(script, tick)
		tick++
		if *maxTicks != 0 && tick > *maxTicks {
			break
		}
		// 等待指定綁定時只送原始按鍵：否則 Return 會同時被當成確認與被指定。
		if overlay.UI.WantsRawInput() {
			for _, code := range window.TakeKeyPresses() {
				if code == cocoa.KeyEscape {
					overlay.Handle(ui.Action{Kind: ui.ActCancel})
					continue
				}
				overlay.Handle(ui.RawKey{
					Frontend: frontendName, Code: code, Label: cocoa.KeyLabel(uint16(code)),
				})
			}
			input.sync(window, hotkeys)
		} else {
			window.TakeKeyPresses()
			// 熱鍵一律走 ui.Hotkey：哪些動作在什麼狀態下生效由介面決定，
			// 前端只把「這個鍵剛按下／剛放開」翻譯成動作名稱。
			for _, hotkey := range hotkeys {
				down, up := input.transition(window, hotkey.code)
				if down {
					overlay.UI.Hotkey(hotkey.action)
				}
				if up {
					overlay.UI.HotkeyRelease(hotkey.action)
				}
			}
			if overlay.UI.Visible() {
				if input.edge(window, cocoa.KeyEscape) {
					overlay.Handle(ui.Action{Kind: ui.ActCancel})
				}
				for _, key := range overlayKeys {
					if input.edge(window, key.code) {
						overlay.Handle(key.event)
					}
				}
			} else {
				input.sync(window, hotkeys)
			}
		}
		// 音量交給音訊執行緒讀：那一段每 10 ms 跑一次，不能在那裡讀介面狀態。
		audioVolume.Store(int32(overlay.Volume()))

		overlay.SetPad(0, padState(window, playerOneKeys))
		overlay.SetPad(1, padState(window, playerTwoKeys))
		advanced, err := overlay.Advance(time.Since(started))
		if err != nil {
			fail(err.Error())
		}
		if advanced {
			completed++
		}
		if overlay.System != nil || overlay.UI.Visible() {
			overlay.Compose(surface)
			if err := window.Present(surface); err != nil {
				fail(err.Error())
			}
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
		fmt.Printf("frames=%d instructions=%d framebuffer_sha256=%x\n",
			completed, overlay.System.Instructions, sha)
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		fail(err.Error())
	}
	return value
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "acan-macos: %s\n", message)
	os.Exit(1)
}

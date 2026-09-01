// acan-x11 是不需要 cgo 的 Linux 桌面前端：以純 Go 的 X11 呈現層輸出畫面與讀取鍵盤，
// 音訊則交給外部播放程序，讓整個 binary 在 CGO_ENABLED=0 下可以建置。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"os"
	"os/exec"
	"time"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/frontend/x11"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
	"github.com/wicanr2/superacan-emu/presentation"
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
	keysymF5     = 0xffc2
	keysymF7     = 0xffc4
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
	romDirs := flag.String("rom-dir", "", "comma-separated directories the cartridge browser scans")
	stateRoot := flag.String("state-root", "", "root directory for per-cartridge save-state directories")
	saveDir := flag.String("save-dir", "", "directory holding per-cartridge battery files")
	configPath := flag.String("config", "", "settings file; defaults to the platform config directory, \"none\" disables it")
	maxTicks := flag.Uint64("max-ticks", 0, "exit after this many host loop iterations regardless of pause state; for scripted smoke runs")
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
	menuKeysym := hotkeyKeysym(config, "menu", keysymF1)

	// 韌體只讀一次；換卡帶時整台機器重建，但韌體位元組沿用同一份。
	iplBytes := loadWordSwapped(*iplPath, machine.IPLSize)
	keyBytes := loadLinear(*keyPath, 16)
	var soundBIOS [2][]byte
	if *soundBIOS1Path != "" {
		soundBIOS[0] = loadLinear(*soundBIOS1Path, machine.SoundBIOSBankSize)
		soundBIOS[1] = loadLinear(*soundBIOS2Path, machine.SoundBIOSBankSize)
	}

	stopAudio := func() {}
	attachAudio := func(system *machine.System) {
		if *audioSink == "" {
			return
		}
		stopAudio()
		stop, err := startAudioSink(system, *audioSink)
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
		image, err := media.DecodeCartridge(path, mustRead(path))
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
			writeCartridgeSave(current, currentSave)
		}
		save := *savePath
		if save == "" && *saveDir != "" {
			save = session.BatteryPathFor(*saveDir, path)
		}
		loadCartridgeSave(system, save)
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
	defer func() { writeCartridgeSave(current, currentSave) }()

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
	overlay.StateRoot = *stateRoot
	overlay.ConfigPath = settingsPath
	overlay.ScriptFrontend = frontendName
	overlay.Loader = newSystem
	overlay.Screenshot = func(frame *image.RGBA) error {
		return writeScreenshot(screenshotName(), current.Bus.Video().Framebuffer())
	}
	input := newOverlayInput()
	script, err := session.ParseScript(*uiScript)
	if err != nil {
		fail(err.Error())
	}

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
		overlay.Play(script, tick)
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
			input.edge(window, menuKeysym)
		} else {
			window.TakeKeyPresses()
			if input.edge(window, menuKeysym) {
				overlay.Handle(ui.Action{Kind: ui.ActMenu})
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
		// 熱鍵取按下的那一瞬間，按著不放不會重複觸發。
		if pressed := window.KeysymPressed(keysymF5); pressed && !savePressed {
			reportStateResult("save", writeSaveState(system, *statePath))
		}
		savePressed = window.KeysymPressed(keysymF5)
		if pressed := window.KeysymPressed(keysymF7); pressed && !loadPressed {
			reportStateResult("load", readSaveState(system, *statePath))
		}
		loadPressed = window.KeysymPressed(keysymF7)
		overlay.SetPad(0, padState(window, playerOneKeys))
		overlay.SetPad(1, padState(window, playerTwoKeys))
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
		if err := writeScreenshot(*screenshot, overlay.System.Bus.Video().Framebuffer()); err != nil {
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

// startAudioSink 把 UMC6619 的原生樣本重取樣成 48 kHz stereo，交給外部播放程序。
// 佇列滿了就丟掉最舊的樣本，播放端的狀態不回饋到模擬器時間線。
func startAudioSink(system *machine.System, command string) (func(), error) {
	stream := presentation.NewPCM16StereoStream(48000 / 5)
	resampler := presentation.NewStereoResampler(umc6619.ClockHz, umc6619.CyclesPerSample, 48000,
		func(left, right int16) { stream.Push(left, right) })
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

func writeScreenshot(path string, framebuffer []uint32) error {
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

func loadWordSwapped(path string, expectedSize int) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("read %s: %v", path, err))
	}
	image, err := media.DecodeWordSwapped(path, raw, expectedSize)
	if err != nil {
		fail(err.Error())
	}
	return image.Bytes
}

// loadCartridge 接受 raw 卡帶與 ZIP（單一成員或雙部分）。
func loadCartridge(path string) media.Image {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("read %s: %v", path, err))
	}
	image, err := media.DecodeCartridge(path, raw)
	if err != nil {
		fail(err.Error())
	}
	return image
}

func loadLinear(path string, expectedSize int) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("read %s: %v", path, err))
	}
	image, err := media.DecodeLinear(path, raw, expectedSize)
	if err != nil {
		fail(err.Error())
	}
	return image.Bytes
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "acan-x11:", message)
	os.Exit(1)
}

// writeSaveState 與 readSaveState 是熱鍵路徑：失敗只回報，不中斷正在進行的遊戲。
func writeSaveState(system *machine.System, path string) error {
	if path == "" {
		return fmt.Errorf("no --state path given")
	}
	var encoded bytes.Buffer
	if err := system.SaveState(&encoded); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, encoded.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readSaveState(system *machine.System, path string) error {
	if path == "" {
		return fmt.Errorf("no --state path given")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return system.LoadState(bytes.NewReader(payload))
}

func reportStateResult(action string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "acan-x11: %s state: %v\n", action, err)
		return
	}
	fmt.Fprintf(os.Stderr, "acan-x11: %s state ok\n", action)
}

// loadCartridgeSave 在檔案存在時載入電池記憶體；不存在視為全新卡帶，不是錯誤。
func loadCartridgeSave(system *machine.System, path string) {
	if path == "" {
		return
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fail(fmt.Sprintf("read save %s: %v", path, err))
	}
	if err := system.Bus.LoadCartridgeSave(payload); err != nil {
		fail(err.Error())
	}
}

// writeCartridgeSave 以先寫暫存檔再改名的方式落地，避免中途失敗留下半套存檔。
func writeCartridgeSave(system *machine.System, path string) {
	if path == "" {
		return
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, system.Bus.CartridgeSave(), 0o644); err != nil {
		fail(fmt.Sprintf("write save %s: %v", temporary, err))
	}
	if err := os.Rename(temporary, path); err != nil {
		fail(fmt.Sprintf("rename save %s: %v", path, err))
	}
}

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/frontend"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
	"github.com/wicanr2/superacan-emu/presentation"
)

func main() {
	iplPath := flag.String("ipl", "", "path to word-swapped 4096-byte internal_68k.bin")
	keyPath := flag.String("key", "", "path to 16-byte umc6650.bin")
	soundBIOS1Path := flag.String("sound-bios1", "", "path to 8192-byte internal_6502_1.bin")
	soundBIOS2Path := flag.String("sound-bios2", "", "path to 8192-byte internal_6502_2.bin")
	romPath := flag.String("rom", "", "path to a raw cartridge dump or a cartridge ZIP")
	savePath := flag.String("save", "", "32768-byte cartridge battery file, loaded at start and written on exit")
	statePath := flag.String("state", "", "save state file; F5 writes it and F7 restores it")
	scale := flag.Int("scale", 3, "initial integer window scale")
	frames := flag.Uint64("frames", 0, "exit after this many emulated frames (0 runs until window close)")
	screenshot := flag.String("screenshot", "", "write the final emulated framebuffer as PNG")
	hostAudio := flag.Bool("audio", true, "play emulated audio through the host device")
	flag.Parse()
	if *iplPath == "" || *keyPath == "" || *romPath == "" {
		fail("--ipl, --key and --rom are required")
	}
	if (*soundBIOS1Path == "") != (*soundBIOS2Path == "") {
		fail("--sound-bios1 and --sound-bios2 must be supplied together")
	}
	if *scale < 1 {
		fail("--scale must be at least 1")
	}
	system, err := machine.NewSystem(
		loadWordSwapped(*iplPath, machine.IPLSize),
		loadCartridge(*romPath).Bytes,
		loadLinear(*keyPath, 16),
	)
	if err != nil {
		fail(err.Error())
	}
	if *soundBIOS1Path != "" {
		if err := system.LoadSoundBIOS(0, loadLinear(*soundBIOS1Path, machine.SoundBIOSBankSize)); err != nil {
			fail(err.Error())
		}
		if err := system.LoadSoundBIOS(1, loadLinear(*soundBIOS2Path, machine.SoundBIOSBankSize)); err != nil {
			fail(err.Error())
		}
	}
	loadCartridgeSave(system, *savePath)
	if err := system.Reset(); err != nil {
		fail(fmt.Sprintf("reset: %v", err))
	}
	defer writeCartridgeSave(system, *savePath)
	ebiten.SetWindowSize(320**scale, 240**scale)
	ebiten.SetWindowTitle("Super A'Can Emulator")
	ebiten.SetTPS(60)
	game := frontend.NewGame(system)
	game.MaxFrames = *frames
	game.SaveState = func() error { return writeSaveState(system, *statePath) }
	game.LoadState = func() error { return readSaveState(system, *statePath) }
	game.OnStatus = func(message string) { fmt.Fprintln(os.Stderr, "acan:", message) }
	var audioOutput *frontend.Audio
	if *hostAudio {
		audioOutput, err = frontend.NewAudio(system)
		if err != nil {
			fail(fmt.Sprintf("audio: %v", err))
		}
		defer func() { _ = audioOutput.Close() }()
	}
	if err := ebiten.RunGame(game); err != nil {
		fail(err.Error())
	}
	if *frames != 0 {
		sha := system.Bus.Video().FramebufferSHA256()
		fmt.Printf("frames=%d instructions=%d framebuffer_sha256=%x\n", game.CompletedFrames, system.Instructions, sha)
	}
	if *screenshot != "" {
		if err := writeScreenshot(*screenshot, system.Bus.Video().Framebuffer()); err != nil {
			fail(err.Error())
		}
	}
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
	if err := output.Close(); err != nil {
		return fmt.Errorf("close screenshot: %w", err)
	}
	return nil
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
	fmt.Fprintln(os.Stderr, "acan:", message)
	os.Exit(1)
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

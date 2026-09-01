package main

import (
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
	romPath := flag.String("rom", "", "path to word-swapped cartridge ROM")
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
		loadWordSwapped(*romPath, 0),
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
	if err := system.Reset(); err != nil {
		fail(fmt.Sprintf("reset: %v", err))
	}
	ebiten.SetWindowSize(320**scale, 240**scale)
	ebiten.SetWindowTitle("Super A'Can Emulator")
	ebiten.SetTPS(60)
	game := frontend.NewGame(system)
	game.MaxFrames = *frames
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

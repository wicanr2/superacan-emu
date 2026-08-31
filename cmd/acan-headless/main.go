package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
)

func main() {
	iplPath := flag.String("ipl", "", "path to word-swapped 4096-byte internal_68k.bin")
	keyPath := flag.String("key", "", "path to 16-byte umc6650.bin")
	romPath := flag.String("rom", "", "path to word-swapped cartridge ROM")
	steps := flag.Uint64("instructions", 1, "number of 68000 instructions to execute")
	flag.Parse()

	if *iplPath == "" || *keyPath == "" || *romPath == "" {
		fail("--ipl, --key and --rom are required")
	}
	ipl := loadWordSwapped(*iplPath, machine.IPLSize)
	key := loadLinear(*keyPath, 16)
	rom := loadWordSwapped(*romPath, 0)

	system, err := machine.NewSystem(ipl.Bytes, rom.Bytes, key.Bytes)
	if err != nil {
		fail(err.Error())
	}
	if err := system.Reset(); err != nil {
		fail(fmt.Sprintf("reset: %v", err))
	}
	result, err := system.RunInstructions(*steps)
	state := system.M68K.State()
	fmt.Printf("ipl_sha256=%s rom_sha256=%s steps=%d pc=$%06X opcode=$%04X cycles=%d\n",
		hex.EncodeToString(ipl.RawSHA256[:]), hex.EncodeToString(rom.RawSHA256[:]),
		system.Instructions, state.PC, result.Opcode, state.Cycles)
	if err != nil {
		fail(err.Error())
	}
}

func loadWordSwapped(path string, expectedSize int) media.Image {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("read %s: %v", path, err))
	}
	image, err := media.DecodeWordSwapped(path, raw, expectedSize)
	if err != nil {
		fail(err.Error())
	}
	return image
}

func loadLinear(path string, expectedSize int) media.Image {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("read %s: %v", path, err))
	}
	image, err := media.DecodeLinear(path, raw, expectedSize)
	if err != nil {
		fail(err.Error())
	}
	return image
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "acan-headless:", message)
	os.Exit(1)
}

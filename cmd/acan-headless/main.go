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
	watch := flag.String("watch", "", "comma-separated hexadecimal bus addresses/ranges")
	watchLimit := flag.Uint64("watch-limit", 64, "maximum matching bus transactions to retain")
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
	type observedTransaction struct {
		machine.Transaction
		Instruction uint64
		PC          uint32
		Opcode      uint16
	}
	var trace *machine.TransactionTrace
	var observed []observedTransaction
	if *watch != "" {
		ranges, err := machine.ParseAddressRanges(*watch)
		if err != nil {
			fail(err.Error())
		}
		trace = &machine.TransactionTrace{Ranges: ranges, Limit: *watchLimit}
		system.Bus.SetObserver(func(transaction machine.Transaction) {
			before := len(trace.Records)
			trace.Observe(transaction)
			if len(trace.Records) != before {
				state := system.M68K.State()
				observed = append(observed, observedTransaction{
					Transaction: transaction, Instruction: system.Instructions,
					PC: state.PC, Opcode: state.IRD,
				})
			}
		})
	}
	if err := system.Reset(); err != nil {
		fail(fmt.Sprintf("reset: %v", err))
	}
	result, err := system.RunInstructions(*steps)
	state := system.M68K.State()
	soundState := system.M65C02.State()
	vramSHA := system.Bus.Video().VRAMSHA256()
	framebufferSHA := system.Bus.Video().FramebufferSHA256()
	fmt.Printf("ipl_sha256=%s rom_sha256=%s steps=%d pc=$%06X opcode=$%04X cycles=%d overlays=low:%t,high:%t sound_steps=%d sound_pc=$%04X sound_cycles=%d sound_reset=%t video_frame=%d scanline=%d video_flags=$%04X vram_nonzero=%d vram_sha256=%s framebuffer_nonblack=%d framebuffer_sha256=%s\n",
		hex.EncodeToString(ipl.RawSHA256[:]), hex.EncodeToString(rom.RawSHA256[:]),
		system.Instructions, state.PC, result.Opcode, state.Cycles,
		system.Bus.LowOverlayEnabled(), system.Bus.HighOverlayEnabled(),
		system.SoundInstructions, soundState.PC, soundState.Cycles, system.SoundResetAsserted(),
		system.Bus.Video().Frame(), system.Bus.Video().Scanline(), system.Bus.Video().VideoFlags(),
		system.Bus.Video().NonzeroVRAMBytes(), hex.EncodeToString(vramSHA[:]),
		system.Bus.Video().NonblackPixels(), hex.EncodeToString(framebufferSHA[:]))
	for _, record := range observed {
		direction := "R"
		if record.Write {
			direction = "W"
		}
		fmt.Printf("bus step=%d pc=$%06X opcode=$%04X %s%d $%06X=$%0*X\n",
			record.Instruction, record.PC, record.Opcode, direction, record.Width*8,
			record.Address, int(record.Width)*2, record.Value)
	}
	if trace != nil {
		fmt.Printf("bus_matches=%d bus_retained=%d bus_omitted=%d\n",
			trace.Matched, len(trace.Records), trace.Matched-uint64(len(trace.Records)))
	}
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

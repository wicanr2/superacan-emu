package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/cpu/m68k"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
	"github.com/wicanr2/superacan-emu/presentation"
)

func main() {
	iplPath := flag.String("ipl", "", "path to word-swapped 4096-byte internal_68k.bin")
	keyPath := flag.String("key", "", "path to 16-byte umc6650.bin")
	romPath := flag.String("rom", "", "path to word-swapped cartridge ROM")
	steps := flag.Uint64("instructions", 1, "number of 68000 instructions to execute")
	frames := flag.Uint64("frames", 0, "run this many completed hardware frames instead of --instructions")
	screenshot := flag.String("screenshot", "", "write the final framebuffer as PNG")
	layerMask := flag.Uint("layer-mask", uint(umc6618.AllLayers), "diagnostic render mask: tilemaps=1/2/4 sprite=8 ROZ=16 windows=32")
	disableROZLineTables := flag.Bool("disable-roz-line-tables", false, "diagnostic: bypass MAME-derived ROZ per-line tables on final render")
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
	audioHash := sha256.New()
	var audioNonzero uint64
	system.SoundBus.Audio().SetSampleSink(func(sample umc6619.Sample) {
		var encoded [4]byte
		binary.LittleEndian.PutUint16(encoded[0:2], uint16(sample.Left))
		binary.LittleEndian.PutUint16(encoded[2:4], uint16(sample.Right))
		_, _ = audioHash.Write(encoded[:])
		if sample.Left != 0 || sample.Right != 0 {
			audioNonzero++
		}
	})
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
	var result m68k.StepResult
	if *frames == 0 {
		result, err = system.RunInstructions(*steps)
	} else {
		for range *frames {
			if _, err = system.RunFrame(2_000_000); err != nil {
				break
			}
		}
		result.Opcode = system.M68K.State().IRD
	}
	state := system.M68K.State()
	if *disableROZLineTables {
		video := system.Bus.Video()
		video.WriteRegister(0xc0, video.Register(0xc0)|0x0200)
		video.RenderFrame()
	}
	if uint8(*layerMask) != umc6618.AllLayers {
		system.Bus.Video().RenderFrameLayers(uint8(*layerMask))
	}
	soundState := system.M65C02.State()
	vramSHA := system.Bus.Video().VRAMSHA256()
	framebufferSHA := system.Bus.Video().FramebufferSHA256()
	fmt.Printf("ipl_sha256=%s rom_sha256=%s steps=%d pc=$%06X opcode=$%04X cycles=%d overlays=low:%t,high:%t sound_steps=%d sound_pc=$%04X sound_cycles=%d sound_samples=%d audio_nonzero=%d audio_sha256=%x sound_irq=$%02X sound_reset=%t video_frame=%d scanline=%d video_flags=$%04X irq_ack=7:%d,5:%d,4:%d vram_nonzero=%d vram_sha256=%s framebuffer_nonblack=%d framebuffer_sha256=%s\n",
		hex.EncodeToString(ipl.RawSHA256[:]), hex.EncodeToString(rom.RawSHA256[:]),
		system.Instructions, state.PC, result.Opcode, state.Cycles,
		system.Bus.LowOverlayEnabled(), system.Bus.HighOverlayEnabled(),
		system.SoundInstructions, soundState.PC, soundState.Cycles, system.SoundBus.Audio().SampleCount(), audioNonzero, audioHash.Sum(nil), system.SoundBus.IRQStatus(), system.SoundResetAsserted(),
		system.Bus.Video().Frame(), system.Bus.Video().Scanline(), system.Bus.Video().VideoFlags(),
		system.IRQAcknowledgements[7], system.IRQAcknowledgements[5], system.IRQAcknowledgements[4],
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
	if *screenshot != "" {
		output, createErr := os.Create(*screenshot)
		if createErr != nil {
			fail(fmt.Sprintf("create screenshot: %v", createErr))
		}
		encodeErr := presentation.EncodePNG(output, umc6618.Width, umc6618.Height, system.Bus.Video().Framebuffer())
		closeErr := output.Close()
		if encodeErr != nil {
			fail(fmt.Sprintf("encode screenshot: %v", encodeErr))
		}
		if closeErr != nil {
			fail(fmt.Sprintf("close screenshot: %v", closeErr))
		}
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

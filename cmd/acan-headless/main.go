package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/cpu/m68k"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
	"github.com/wicanr2/superacan-emu/presentation"
	"github.com/wicanr2/superacan-emu/session"
	"github.com/wicanr2/superacan-emu/ui"
)

func main() {
	iplPath := flag.String("ipl", "", "path to word-swapped 4096-byte internal_68k.bin")
	keyPath := flag.String("key", "", "path to 16-byte umc6650.bin")
	soundBIOS1Path := flag.String("sound-bios1", "", "path to 8192-byte internal_6502_1.bin")
	soundBIOS2Path := flag.String("sound-bios2", "", "path to 8192-byte internal_6502_2.bin")
	romPath := flag.String("rom", "", "path to a raw cartridge dump or a cartridge ZIP")
	savePath := flag.String("save", "", "32768-byte cartridge battery file, loaded at start and written on exit")
	loadStatePath := flag.String("load-state", "", "restore this save state before running")
	saveStatePath := flag.String("save-state", "", "write a save state when the run finishes")
	steps := flag.Uint64("instructions", 1, "number of 68000 instructions to execute")
	frames := flag.Uint64("frames", 0, "run this many completed hardware frames instead of --instructions")
	screenshot := flag.String("screenshot", "", "write the final framebuffer as PNG")
	screenshotDir := flag.String("screenshot-dir", "", "write one PNG per sampled frame into this existing directory")
	screenshotEvery := flag.Uint64("screenshot-every", 1, "with --screenshot-dir, sample every Nth completed frame")
	wavPath := flag.String("wav", "", "write resampled 48000 Hz signed 16-bit stereo WAV")
	layerMask := flag.Uint("layer-mask", uint(umc6618.AllLayers), "diagnostic render mask: tilemaps=1/2/4 sprite=8 ROZ=16 windows=32")
	disableROZLineTables := flag.Bool("disable-roz-line-tables", false, "diagnostic: bypass MAME-derived ROZ per-line tables on final render")
	watch := flag.String("watch", "", "comma-separated hexadecimal bus addresses/ranges")
	traceInstructions := flag.Int("trace-instructions", 0, "retain the last N 68000 instructions and print them if execution stops")
	dumpVideoRegisters := flag.Bool("video-registers", false, "print the 256 UM6618 registers and the derived per-layer state")
	watchLimit := flag.Uint64("watch-limit", 64, "maximum matching bus transactions to retain")
	soundRAMAlias := flag.Bool("sound-ram-alias", false, "diagnostic: model the sound SRAM as a single 32 KiB device (drop A15 for RAM accesses)")
	press := flag.String("press", "", "P1 input timeline: frame:BUTTON+BUTTON,... (held for 10 frames)")
	press2 := flag.String("press2", "", "P2 input timeline: frame:BUTTON+BUTTON,... (held for 10 frames)")
	uiScript := flag.String("ui-script", "", "overlay UI event timeline: frame:EVENT,... where EVENT is one of "+session.ScriptEventNames())
	uiSurfaceSpec := flag.String("ui-surface", "960x720", "overlay UI surface size WxH")
	uiScale := flag.Int("ui-scale", 1, "overlay UI design-unit scale")
	uiTouch := flag.Bool("ui-touch", false, "use the touch layout profile instead of compact")
	uiStateDir := flag.String("ui-state-dir", "", "directory holding the ten save-state slots the overlay UI reads and writes")
	uiCompose := flag.String("ui-compose", "", "write the final composed game+overlay frame as PNG")
	flag.Parse()

	if *iplPath == "" || *keyPath == "" || *romPath == "" {
		fail("--ipl, --key and --rom are required")
	}
	if (*soundBIOS1Path == "") != (*soundBIOS2Path == "") {
		fail("--sound-bios1 and --sound-bios2 must be supplied together")
	}
	presses, err := parsePresses(*press)
	if err != nil {
		fail(err.Error())
	}
	presses2, err := parsePresses(*press2)
	if err != nil {
		fail(err.Error())
	}
	uiEnabled := *uiScript != "" || *uiCompose != ""
	script, err := session.ParseScript(*uiScript)
	if err != nil {
		fail(err.Error())
	}
	profile := ui.ProfileCompact
	if *uiTouch {
		profile = ui.ProfileTouch
	}
	surface, err := uiSurface(*uiSurfaceSpec, profile, *uiScale)
	if err != nil {
		fail(err.Error())
	}
	ipl := loadWordSwapped(*iplPath, machine.IPLSize)
	key := loadLinear(*keyPath, 16)
	rom := loadCartridge(*romPath)

	system, err := machine.NewSystem(ipl.Bytes, rom.Bytes, key.Bytes)
	if err != nil {
		fail(err.Error())
	}
	system.Bus.SetSoundRAMAlias(*soundRAMAlias)
	if *soundBIOS1Path != "" {
		if err := system.LoadSoundBIOS(0, loadLinear(*soundBIOS1Path, machine.SoundBIOSBankSize).Bytes); err != nil {
			fail(err.Error())
		}
		if err := system.LoadSoundBIOS(1, loadLinear(*soundBIOS2Path, machine.SoundBIOSBankSize).Bytes); err != nil {
			fail(err.Error())
		}
	}
	audioHash := sha256.New()
	var audioNonzero uint64
	var pcm48 []byte
	var resampler *presentation.StereoResampler
	if *wavPath != "" {
		resampler = presentation.NewStereoResampler(umc6619.ClockHz, umc6619.CyclesPerSample, 48000, func(left, right int16) {
			pcm48 = binary.LittleEndian.AppendUint16(pcm48, uint16(left))
			pcm48 = binary.LittleEndian.AppendUint16(pcm48, uint16(right))
		})
	}
	system.SoundBus.Audio().SetSampleSink(func(sample umc6619.Sample) {
		var encoded [4]byte
		binary.LittleEndian.PutUint16(encoded[0:2], uint16(sample.Left))
		binary.LittleEndian.PutUint16(encoded[2:4], uint16(sample.Right))
		_, _ = audioHash.Write(encoded[:])
		if sample.Left != 0 || sample.Right != 0 {
			audioNonzero++
		}
		if resampler != nil {
			resampler.Push(sample.Left, sample.Right)
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
	system.Trace = machine.NewInstructionRing(*traceInstructions)
	loadCartridgeSave(system, *savePath)
	if err := system.Reset(); err != nil {
		fail(fmt.Sprintf("reset: %v", err))
	}
	if *loadStatePath != "" {
		file, openErr := os.Open(*loadStatePath)
		if openErr != nil {
			fail(fmt.Sprintf("open save state %s: %v", *loadStatePath, openErr))
		}
		loadErr := system.LoadState(file)
		_ = file.Close()
		if loadErr != nil {
			fail(loadErr.Error())
		}
	}
	var result m68k.StepResult
	if *frames == 0 {
		result, err = system.RunInstructions(*steps)
	} else {
		var overlay *session.Session
		if uiEnabled {
			overlay = session.New(session.Options{
				System:   system,
				Title:    session.TitleFromPath(*romPath),
				StateDir: *uiStateDir,
				ROMSize:  int64(len(rom.Bytes)),
				Surface:  surface,
				Config:   ui.DefaultConfig(),
			})
			// 存檔槽的時間戳是環境不是行為，固定它才能對合成畫面取雜湊。
			overlay.Stamp = func(os.FileInfo) string { return "01-01 00:00" }
			// 診斷畫面要回報這一輪是哪個入口跑的；名稱是常數，不影響雜湊的可重現性。
			overlay.FrontendName = "headless"
		}
		p1, p2 := machine.PadReleased, machine.PadReleased
		for frame := uint64(0); frame < *frames; frame++ {
			p1 = applyPresses(frame, p1, presses)
			p2 = applyPresses(frame, p2, presses2)
			if overlay != nil {
				overlay.Play(script, frame)
				overlay.SetPad(0, p1)
				overlay.SetPad(1, p2)
				if _, err = overlay.Advance(frameClock(frame)); err != nil {
					break
				}
			} else {
				system.SoundBus.SetPad(0, p1)
				system.SoundBus.SetPad(1, p2)
				if _, err = system.RunFrame(2_000_000); err != nil {
					break
				}
			}
			if *screenshotDir != "" && *screenshotEvery > 0 && (frame+1)%*screenshotEvery == 0 {
				video := system.Bus.Video()
				name := filepath.Join(*screenshotDir, fmt.Sprintf("frame-%06d.png", frame+1))
				if writeErr := writePNG(name, video.Framebuffer()); writeErr != nil {
					fail(writeErr.Error())
				}
				sum := video.FramebufferSHA256()
				fmt.Printf("sample frame=%d nonblack=%d framebuffer_sha256=%s file=%s\n",
					frame+1, video.NonblackPixels(), hex.EncodeToString(sum[:]), name)
			}
		}
		result.Opcode = system.M68K.State().IRD
		if overlay != nil {
			if shutdownErr := overlay.Shutdown(); shutdownErr != nil {
				fail(shutdownErr.Error())
			}
			composed := composeFrame(overlay, surface)
			sum := sha256.Sum256(composed.Pix)
			visible, reason := overlay.Halt()
			fmt.Printf("ui_surface=%dx%d ui_scale=%d ui_visible=%t ui_halt=%d ui_halt_note=%q ui_sha256=%s\n",
				surface.W, surface.H, surface.Scale, overlay.UI.Visible(), visible, reason,
				hex.EncodeToString(sum[:]))
			for _, slot := range overlay.Slots() {
				if !slot.Present && !slot.Rejected {
					continue
				}
				fmt.Printf("ui_slot=%d present=%t rejected=%t frame=%d reason=%q\n",
					slot.Index, slot.Present, slot.Rejected, slot.Frame, slot.Reason)
			}
			if *uiCompose != "" {
				if writeErr := writeComposedPNG(*uiCompose, composed); writeErr != nil {
					fail(writeErr.Error())
				}
			}
		}
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
	dma0, dma1 := system.Bus.HostDMA().Channel(0), system.Bus.HostDMA().Channel(1)
	vramSHA := system.Bus.Video().VRAMSHA256()
	framebufferSHA := system.Bus.Video().FramebufferSHA256()
	fmt.Printf("ipl_sha256=%s rom_sha256=%s steps=%d pc=$%06X opcode=$%04X cycles=%d overlays=low:%t,high:%t sound_bios=%t sound_ram_alias=%t sound_steps=%d sound_pc=$%04X sound_cycles=%d sound_samples=%d audio_nonzero=%d audio_sha256=%x sound_irq=$%02X sound_reset=%t video_frame=%d scanline=%d video_flags=$%04X frc=$%04X/$%04X,pending:%t,supported:%t dma_triggers=0:%d,1:%d irq_ack=7:%d,6:%d,5:%d,4:%d,3:%d vram_nonzero=%d vram_sha256=%s framebuffer_nonblack=%d framebuffer_sha256=%s\n",
		hex.EncodeToString(ipl.RawSHA256[:]), hex.EncodeToString(rom.RawSHA256[:]),
		system.Instructions, state.PC, result.Opcode, state.Cycles,
		system.Bus.LowOverlayEnabled(), system.Bus.HighOverlayEnabled(),
		*soundBIOS1Path != "", system.Bus.SoundRAMAlias(), system.SoundInstructions, soundState.PC, soundState.Cycles, system.SoundBus.Audio().SampleCount(), audioNonzero, audioHash.Sum(nil), system.SoundBus.IRQStatus(), system.SoundResetAsserted(),
		system.Bus.Video().Frame(), system.Bus.Video().Scanline(), system.Bus.Video().VideoFlags(),
		system.Bus.FRC().Control(), system.Bus.FRC().Frequency(), system.Bus.FRC().Pending(), system.Bus.FRC().SupportedMode(),
		dma0.Triggers, dma1.Triggers,
		system.IRQAcknowledgements[7], system.IRQAcknowledgements[6], system.IRQAcknowledgements[5], system.IRQAcknowledgements[4],
		system.IRQAcknowledgements[3],
		system.Bus.Video().NonzeroVRAMBytes(), hex.EncodeToString(vramSHA[:]),
		system.Bus.Video().NonblackPixels(), hex.EncodeToString(framebufferSHA[:]))
	if clashes := system.Bus.SoundRAMClashes(); len(clashes) > 0 {
		cells := make([]int, 0, len(clashes))
		for cell := range clashes {
			cells = append(cells, int(cell))
		}
		sort.Ints(cells)
		total := uint32(0)
		for _, cell := range cells {
			total += clashes[uint16(cell)]
		}
		fmt.Printf("sound_ram_clash_cells=%d sound_ram_clash_writes=%d\n", len(cells), total)
		for i, cell := range cells {
			if i >= 24 {
				fmt.Printf("sound_ram_clash ... %d more cells\n", len(cells)-i)
				break
			}
			fmt.Printf("sound_ram_clash $%04X (also $%04X) writes=%d\n", cell, cell|0x8000, clashes[uint16(cell)])
		}
	}
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
	if *dumpVideoRegisters {
		video := system.Bus.Video()
		for index := 0; index < 256; index += 8 {
			fmt.Printf("vreg $%03X:", index*2)
			for offset := 0; offset < 8; offset++ {
				fmt.Printf(" %04X", video.Register(uint16(index+offset)))
			}
			fmt.Println()
		}
		for layer := 0; layer < 3; layer++ {
			base := 0x80 + layer*0x10
			fmt.Printf("layer %d flags=$%04X tilemode=$%04X scrollx=$%04X scrolly=$%04X base=$%04X mode=$%04X\n",
				layer, video.Register(uint16(base)), video.Register(uint16(base+1)),
				video.Register(uint16(base+2)), video.Register(uint16(base+3)),
				video.Register(uint16(base+4)), video.Register(uint16(base+5)))
		}
	}
	for _, record := range system.Trace.Records() {
		fmt.Printf("trace step=%d pc=$%06X opcode=$%04X cycles=%d\n",
			record.Index, record.PC, record.Opcode, record.Cycles)
	}
	if err != nil {
		fail(err.Error())
	}
	if *screenshot != "" {
		if writeErr := writePNG(*screenshot, system.Bus.Video().Framebuffer()); writeErr != nil {
			fail(writeErr.Error())
		}
	}
	writeCartridgeSave(system, *savePath)
	if *saveStatePath != "" {
		var encoded bytes.Buffer
		if stateErr := system.SaveState(&encoded); stateErr != nil {
			fail(stateErr.Error())
		}
		temporary := *saveStatePath + ".tmp"
		if writeErr := os.WriteFile(temporary, encoded.Bytes(), 0o644); writeErr != nil {
			fail(fmt.Sprintf("write save state %s: %v", temporary, writeErr))
		}
		if renameErr := os.Rename(temporary, *saveStatePath); renameErr != nil {
			fail(fmt.Sprintf("rename save state %s: %v", *saveStatePath, renameErr))
		}
	}
	if *wavPath != "" {
		output, createErr := os.Create(*wavPath)
		if createErr != nil {
			fail(fmt.Sprintf("create WAV: %v", createErr))
		}
		encodeErr := presentation.EncodePCM16WAV(output, 48000, pcm48)
		closeErr := output.Close()
		if encodeErr != nil {
			fail(fmt.Sprintf("encode WAV: %v", encodeErr))
		}
		if closeErr != nil {
			fail(fmt.Sprintf("close WAV: %v", closeErr))
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

// writePNG 把單張 framebuffer 寫成 PNG，錯誤訊息帶檔名以便回查。
func writePNG(name string, framebuffer []uint32) error {
	output, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("create screenshot %s: %w", name, err)
	}
	encodeErr := presentation.EncodePNG(output, umc6618.Width, umc6618.Height, framebuffer)
	closeErr := output.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode screenshot %s: %w", name, encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close screenshot %s: %w", name, closeErr)
	}
	return nil
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

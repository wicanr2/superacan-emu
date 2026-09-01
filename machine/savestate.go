package machine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/wicanr2/superacan-emu/chip/frc"
	"github.com/wicanr2/superacan-emu/chip/hostdma"
	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/chip/umc6650"
	"github.com/wicanr2/superacan-emu/cpu/m65c02"
	"github.com/wicanr2/superacan-emu/cpu/m68k"
)

// 存檔格式。magic 之後是版本與標頭長度，讀端兩者都強制相符；接著是 IPL 與 ROM 的
// SHA-256 讓存檔綁定媒體身分，最後是 payload 長度與 payload 的 SHA-256。
//
// 這個格式是本專案自有的，與 Bcan 的 ACANRTS 不相容，也不宣稱相容。
const (
	saveStateMagic      = "ACANGOS1"
	saveStateVersion    = 1
	saveStateHeaderSize = 96
)

type saveStateHeader struct {
	Magic       [8]byte
	Version     uint16
	HeaderSize  uint16
	Flags       uint32
	IPLSHA256   [32]byte
	ROMSHA256   [32]byte
	PayloadSize uint64
	_           [8]byte // 保留，讓標頭剛好 96 bytes
}

// SystemState 是整台機器在 frame 邊界的快照。欄位順序即為序列化順序；
// 新增欄位一定要同步提高 saveStateVersion，讀端才不會把舊檔誤讀成新版。
type SystemState struct {
	M68K   m68k.Snapshot
	M65C02 m65c02.Snapshot

	TimelineCycles      uint64
	TimelineCounts      [5]uint64
	TimelineLast        m68k.Phase
	SoundTimelineCycles uint64
	SoundTimelineLast   m65c02.Cycle

	Instructions        uint64
	SoundInstructions   uint64
	IRQAcknowledgements [8]uint64
	SoundReset          bool
	SoundCredit         int64
	SoundIRQ6           bool

	SoundRAM     [65536]byte
	SoundRAMMask uint32
	WorkRAM      [65536]byte
	SRAM         [SRAMSize]byte
	E90B3C       uint16
	Control      uint16
	LowOverlay   bool
	HighOverlay  bool

	SoundIRQEnable uint8
	SoundIRQStatus uint8
	SoundIO        [256]uint8
	SoundPads      [2]uint16
	SoundShiftCtrl uint8
	SoundShiftRegs [2]uint8
	SoundLatched   [2]uint16
	SoundLatch     [2]uint8
	SoundLatchFull [2]bool

	Video   umc6618.Snapshot
	Audio   umc6619.Snapshot
	Lockout umc6650.Snapshot
	FRC     frc.Snapshot
	DMA     hostdma.Snapshot
}

// Snapshot 取出整台機器的狀態。只在 CPU 指令邊界呼叫才有意義；System 的執行迴圈
// 本來就以指令為單位推進，因此 RunFrame／RunInstructions 回傳之後即為安全點。
func (s *System) Snapshot() SystemState {
	bus := s.Bus
	sound := s.SoundBus
	return SystemState{
		M68K:   s.M68K.Snapshot(),
		M65C02: s.M65C02.Snapshot(),

		TimelineCycles:      s.Timeline.Cycles,
		TimelineCounts:      s.Timeline.Counts,
		TimelineLast:        s.Timeline.Last,
		SoundTimelineCycles: s.SoundTimeline.Cycles,
		SoundTimelineLast:   s.SoundTimeline.Last,

		Instructions:        s.Instructions,
		SoundInstructions:   s.SoundInstructions,
		IRQAcknowledgements: s.IRQAcknowledgements,
		SoundReset:          s.soundReset,
		SoundCredit:         s.soundCredit,
		SoundIRQ6:           s.soundIRQ6,

		SoundRAM:     bus.soundRAM,
		SoundRAMMask: bus.soundRAMMask,
		WorkRAM:      bus.workRAM,
		SRAM:         bus.sram,
		E90B3C:       bus.e90b3c,
		Control:      bus.control,
		LowOverlay:   !bus.loOff,
		HighOverlay:  !bus.hiOff,

		SoundIRQEnable: sound.irqEnable,
		SoundIRQStatus: sound.irqStatus,
		SoundIO:        sound.io,
		SoundPads:      sound.pads,
		SoundShiftCtrl: sound.shiftCtrl,
		SoundShiftRegs: sound.shiftRegs,
		SoundLatched:   sound.latched,
		SoundLatch:     sound.latch,
		SoundLatchFull: sound.latchFull,

		Video:   bus.video.Snapshot(),
		Audio:   sound.audio.Snapshot(),
		Lockout: bus.lockout.Snapshot(),
		FRC:     bus.frc.Snapshot(),
		DMA:     bus.dma.Snapshot(),
	}
}

// Restore 一次套用整份狀態。呼叫端必須先完成驗證：本函式假設 state 已經可信，
// 中途不會失敗，因此不會留下半套狀態。
func (s *System) Restore(state SystemState) {
	bus := s.Bus
	sound := s.SoundBus

	s.M68K.Restore(state.M68K)
	s.M65C02.Restore(state.M65C02)

	s.Timeline.Cycles, s.Timeline.Counts, s.Timeline.Last =
		state.TimelineCycles, state.TimelineCounts, state.TimelineLast
	s.SoundTimeline.Cycles, s.SoundTimeline.Last =
		state.SoundTimelineCycles, state.SoundTimelineLast

	s.Instructions, s.SoundInstructions = state.Instructions, state.SoundInstructions
	s.IRQAcknowledgements = state.IRQAcknowledgements
	s.soundReset, s.soundCredit, s.soundIRQ6 = state.SoundReset, state.SoundCredit, state.SoundIRQ6

	bus.soundRAM, bus.soundRAMMask = state.SoundRAM, state.SoundRAMMask
	bus.workRAM, bus.sram = state.WorkRAM, state.SRAM
	bus.e90b3c, bus.control = state.E90B3C, state.Control
	bus.loOff, bus.hiOff = !state.LowOverlay, !state.HighOverlay

	sound.irqEnable, sound.irqStatus, sound.io = state.SoundIRQEnable, state.SoundIRQStatus, state.SoundIO
	sound.pads, sound.shiftCtrl, sound.shiftRegs = state.SoundPads, state.SoundShiftCtrl, state.SoundShiftRegs
	sound.latched, sound.latch, sound.latchFull = state.SoundLatched, state.SoundLatch, state.SoundLatchFull
	sound.ramMask = uint16(state.SoundRAMMask)

	bus.video.Restore(state.Video)
	sound.audio.Restore(state.Audio)
	bus.lockout.Restore(state.Lockout)
	bus.frc.Restore(state.FRC)
	bus.dma.Restore(state.DMA)
}

// SaveState 寫出一份完整存檔。payload 先在記憶體組好再算雜湊，寫出的檔案要嘛
// 完整要嘛不存在。
func (s *System) SaveState(output io.Writer) error {
	var payload bytes.Buffer
	state := s.Snapshot()
	if err := binary.Write(&payload, binary.LittleEndian, &state); err != nil {
		return fmt.Errorf("machine: encode save state: %w", err)
	}
	header := saveStateHeader{
		Version: saveStateVersion, HeaderSize: saveStateHeaderSize,
		IPLSHA256: s.IPLSHA256, ROMSHA256: s.ROMSHA256,
		PayloadSize: uint64(payload.Len()),
	}
	copy(header.Magic[:], saveStateMagic)

	var file bytes.Buffer
	if err := binary.Write(&file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("machine: encode save state header: %w", err)
	}
	if file.Len() != saveStateHeaderSize {
		return fmt.Errorf("machine: save state header is %d bytes, want %d", file.Len(), saveStateHeaderSize)
	}
	digest := sha256.Sum256(payload.Bytes())
	if _, err := file.Write(digest[:]); err != nil {
		return err
	}
	if _, err := file.Write(payload.Bytes()); err != nil {
		return err
	}
	_, err := output.Write(file.Bytes())
	return err
}

// LoadState 讀入並套用存檔。任何一項驗證失敗都在套用之前回傳錯誤，現行狀態不變：
// 版本、標頭長度、媒體身分、payload 長度與 payload 雜湊全部要相符。
func (s *System) LoadState(input io.Reader) error {
	raw, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("machine: read save state: %w", err)
	}
	if len(raw) < saveStateHeaderSize+sha256.Size {
		return fmt.Errorf("machine: save state is %d bytes, shorter than its header", len(raw))
	}
	var header saveStateHeader
	if err := binary.Read(bytes.NewReader(raw[:saveStateHeaderSize]), binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("machine: decode save state header: %w", err)
	}
	if string(header.Magic[:]) != saveStateMagic {
		return fmt.Errorf("machine: save state magic %q is not %q", header.Magic[:], saveStateMagic)
	}
	if header.Version != saveStateVersion {
		return fmt.Errorf("machine: save state version %d, want %d", header.Version, saveStateVersion)
	}
	if header.HeaderSize != saveStateHeaderSize {
		return fmt.Errorf("machine: save state header size %d, want %d", header.HeaderSize, saveStateHeaderSize)
	}
	if header.IPLSHA256 != s.IPLSHA256 {
		return fmt.Errorf("machine: save state belongs to a different IPL")
	}
	if header.ROMSHA256 != s.ROMSHA256 {
		return fmt.Errorf("machine: save state belongs to a different cartridge")
	}

	digest := raw[saveStateHeaderSize : saveStateHeaderSize+sha256.Size]
	payload := raw[saveStateHeaderSize+sha256.Size:]
	if uint64(len(payload)) != header.PayloadSize {
		return fmt.Errorf("machine: save state payload is %d bytes, header says %d", len(payload), header.PayloadSize)
	}
	if actual := sha256.Sum256(payload); !bytes.Equal(actual[:], digest) {
		return fmt.Errorf("machine: save state payload failed its integrity check")
	}

	var state SystemState
	if err := binary.Read(bytes.NewReader(payload), binary.LittleEndian, &state); err != nil {
		return fmt.Errorf("machine: decode save state payload: %w", err)
	}
	s.Restore(state)
	return nil
}

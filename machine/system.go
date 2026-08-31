package machine

import (
	"fmt"

	"github.com/wicanr2/superacan-emu/cpu/m65c02"
	"github.com/wicanr2/superacan-emu/cpu/m68k"
)

// System wires the first production CPU, bus and shared timeline together.
// Additional chips attach to Bus and Timeline without changing the CPU API.
type System struct {
	Bus                 *Bus
	Timeline            *Timeline
	M68K                *m68k.CPU
	M65C02              *m65c02.CPU
	SoundBus            *SoundBus
	SoundTimeline       *SoundTimeline
	Instructions        uint64
	SoundInstructions   uint64
	IRQAcknowledgements [8]uint64
	soundReset          bool
	soundCredit         int64
}

// RunFrame advances the shared hardware timeline until UM6618 completes one
// additional frame. maxInstructions is a fail-closed bound for broken timing
// or software loops; the frontend does not manufacture vblank events.
func (s *System) RunFrame(maxInstructions uint64) (uint64, error) {
	if maxInstructions == 0 {
		return 0, fmt.Errorf("machine: frame instruction bound must be nonzero")
	}
	target := s.Bus.Video().Frame() + 1
	var executed uint64
	for s.Bus.Video().Frame() < target {
		if executed == maxInstructions {
			return executed, fmt.Errorf("machine: frame did not complete within %d instructions", maxInstructions)
		}
		s.M68K.SetInterruptLevel(s.Bus.Video().HighestIRQLevel())
		if _, err := s.M68K.Step(); err != nil {
			return executed, err
		}
		s.Instructions++
		executed++
	}
	return executed, nil
}

func NewSystem(ipl, rom, key []byte) (*System, error) {
	bus, err := NewBus(ipl, rom, key)
	if err != nil {
		return nil, err
	}
	timeline := &Timeline{}
	soundBus := newSoundBus(&bus.soundRAM)
	soundTimeline := &SoundTimeline{}
	system := &System{
		Bus: bus, Timeline: timeline,
		M68K:     m68k.New(bus, timeline),
		M65C02:   m65c02.New(soundBus, soundTimeline),
		SoundBus: soundBus, SoundTimeline: soundTimeline,
		soundReset: true,
	}
	system.M68K.SetInterruptAcknowledge(func(level uint8) {
		system.IRQAcknowledgements[level&7]++
		system.Bus.Video().ClearIRQ(level)
	})
	timeline.OnAdvance = system.advanceDevices
	bus.setControlObserver(system.controlChanged)
	return system, nil
}

func (s *System) Reset() error { return s.M68K.Reset() }

func (s *System) SoundResetAsserted() bool { return s.soundReset }

func (s *System) RunInstructions(count uint64) (m68k.StepResult, error) {
	var result m68k.StepResult
	for i := uint64(0); i < count; i++ {
		s.M68K.SetInterruptLevel(s.Bus.Video().HighestIRQLevel())
		var err error
		result, err = s.M68K.Step()
		if err != nil {
			return result, err
		}
		s.Instructions++
	}
	return result, nil
}

func (s *System) controlChanged(oldValue, newValue uint16) error {
	wasReset := oldValue&1 == 0
	isReset := newValue&1 == 0
	if !wasReset && isReset {
		s.soundReset = true
		s.soundCredit = 0
		return nil
	}
	if wasReset && !isReset {
		s.soundReset = false
		s.soundCredit = 0
		return s.M65C02.Reset()
	}
	return nil
}

func (s *System) advanceDevices(m68kCycles uint8) error {
	if !s.soundReset {
		s.soundCredit += int64(m68kCycles)
		for s.soundCredit >= 6 {
			result, err := s.M65C02.Step()
			if err != nil {
				return err
			}
			s.SoundInstructions++
			s.soundCredit -= int64(result.Cycles) * 3
		}
	}
	s.Bus.Video().AdvanceM68KCycles(m68kCycles)
	return nil
}

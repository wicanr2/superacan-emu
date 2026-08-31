package machine

import "github.com/wicanr2/superacan-emu/cpu/m68k"

// System wires the first production CPU, bus and shared timeline together.
// Additional chips attach to Bus and Timeline without changing the CPU API.
type System struct {
	Bus          *Bus
	Timeline     *Timeline
	M68K         *m68k.CPU
	Instructions uint64
}

func NewSystem(ipl, rom, key []byte) (*System, error) {
	bus, err := NewBus(ipl, rom, key)
	if err != nil {
		return nil, err
	}
	timeline := &Timeline{}
	return &System{
		Bus: bus, Timeline: timeline,
		M68K: m68k.New(bus, timeline),
	}, nil
}

func (s *System) Reset() error { return s.M68K.Reset() }

func (s *System) RunInstructions(count uint64) (m68k.StepResult, error) {
	var result m68k.StepResult
	for i := uint64(0); i < count; i++ {
		var err error
		result, err = s.M68K.Step()
		if err != nil {
			return result, err
		}
		s.Instructions++
	}
	return result, nil
}

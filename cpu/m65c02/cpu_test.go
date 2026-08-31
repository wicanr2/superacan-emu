package m65c02

import "testing"

type testMachine struct {
	memory [65536]uint8
	cycles []Cycle
}

func (m *testMachine) Read8(address uint16) (uint8, error) { return m.memory[address], nil }
func (m *testMachine) Write8(address uint16, value uint8) error {
	m.memory[address] = value
	return nil
}
func (m *testMachine) Advance(cycle Cycle) error {
	m.cycles = append(m.cycles, cycle)
	return nil
}

func TestResetAndSoundDriverPrologue(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0xfffc], machine.memory[0xfffd] = 0x00, 0xf0
	copy(machine.memory[0xf000:], []byte{0x78, 0xa2, 0xff, 0x9a, 0x20, 0xdc, 0xf7})
	cpu := New(machine, machine)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	if state := cpu.State(); state.PC != 0xf000 || state.SP != 0xfd || state.Cycles != 7 {
		t.Fatalf("reset state=%+v", state)
	}
	for range 4 {
		if _, err := cpu.Step(); err != nil {
			t.Fatal(err)
		}
	}
	state := cpu.State()
	if state.PC != 0xf7dc || state.X != 0xff || state.SP != 0xfd {
		t.Fatalf("prologue state=%+v", state)
	}
	if machine.memory[0x01ff] != 0xf0 || machine.memory[0x01fe] != 0x06 {
		t.Fatalf("JSR stack=$%02X%02X", machine.memory[0x01ff], machine.memory[0x01fe])
	}
}

func TestUnknownOpcodeFailsClosedAtAddress(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0x1234] = 0x02
	cpu := New(machine, machine)
	cpu.state.PC = 0x1234
	result, err := cpu.Step()
	if err == nil || result.PCBefore != 0x1234 || result.Opcode != 0x02 || cpu.State().PC != 0x1234 {
		t.Fatalf("result=%+v state=%+v err=%v", result, cpu.State(), err)
	}
}

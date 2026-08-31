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

func TestIRQIsLevelSensitiveAndRTIRestoresExecution(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0x8000] = 0xea
	machine.memory[0x9000] = 0x40
	machine.memory[0xfffe], machine.memory[0xffff] = 0x00, 0x90
	cpu := New(machine, machine)
	cpu.state = State{PC: 0x8000, SP: 0xfd, P: flagDecimal | flagUnused}
	cpu.SetIRQ(true)

	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Interrupt || result.Cycles != 7 || cpu.state.PC != 0x9000 || cpu.state.SP != 0xfa {
		t.Fatalf("IRQ result=%+v state=%+v", result, cpu.state)
	}
	if cpu.state.P&flagInterruptDisable == 0 || cpu.state.P&flagDecimal != 0 {
		t.Fatalf("IRQ flags=$%02X", cpu.state.P)
	}
	if machine.memory[0x01fd] != 0x80 || machine.memory[0x01fc] != 0x00 {
		t.Fatalf("IRQ return address=$%02X%02X", machine.memory[0x01fd], machine.memory[0x01fc])
	}

	cpu.SetIRQ(false)
	result, err = cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if result.Opcode != 0x40 || result.Cycles != 6 || cpu.state.PC != 0x8000 || cpu.state.SP != 0xfd {
		t.Fatalf("RTI result=%+v state=%+v", result, cpu.state)
	}
	if cpu.state.P&flagDecimal == 0 || cpu.state.P&flagInterruptDisable != 0 {
		t.Fatalf("restored flags=$%02X", cpu.state.P)
	}
}

func TestWAIWaitsForIRQAndRespectsInterruptMask(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0x8000] = 0xcb
	machine.memory[0x8001] = 0xea
	cpu := New(machine, machine)
	cpu.state = State{PC: 0x8000, SP: 0xfd, P: flagInterruptDisable | flagUnused}

	result, err := cpu.Step()
	if err != nil || result.Opcode != 0xcb || result.Cycles != 3 {
		t.Fatalf("WAI result=%+v err=%v", result, err)
	}
	result, err = cpu.Step()
	if err != nil || !result.Waiting || result.Cycles != 1 || cpu.state.PC != 0x8001 {
		t.Fatalf("waiting result=%+v state=%+v err=%v", result, cpu.state, err)
	}
	cpu.SetIRQ(true)
	result, err = cpu.Step()
	if err != nil || result.Opcode != 0xea || result.Interrupt || cpu.state.PC != 0x8002 {
		t.Fatalf("masked wake result=%+v state=%+v err=%v", result, cpu.state, err)
	}
}

func TestLSRZeroPage(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0x8000], machine.memory[0x8001] = 0x46, 0x42
	machine.memory[0x0042] = 0x01
	cpu := New(machine, machine)
	cpu.state = State{PC: 0x8000, SP: 0xfd, P: flagNegative | flagUnused}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	if result.Cycles != 5 || machine.memory[0x0042] != 0 || cpu.state.P&flagCarry == 0 || cpu.state.P&flagZero == 0 || cpu.state.P&flagNegative != 0 {
		t.Fatalf("result=%+v state=%+v value=$%02X", result, cpu.state, machine.memory[0x0042])
	}
}

func TestJMPIndirectCrossesPointerPageOn65C02(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0x8000], machine.memory[0x8001], machine.memory[0x8002] = 0x6c, 0xff, 0x12
	machine.memory[0x12ff], machine.memory[0x1300] = 0x34, 0x56
	machine.memory[0x1200] = 0xaa
	cpu := New(machine, machine)
	cpu.state = State{PC: 0x8000, SP: 0xfd, P: flagUnused}
	result, err := cpu.Step()
	if err != nil || result.Cycles != 5 || cpu.state.PC != 0x5634 {
		t.Fatalf("result=%+v state=%+v err=%v", result, cpu.state, err)
	}
}

func TestCompareIndexImmediate(t *testing.T) {
	machine := &testMachine{}
	machine.memory[0x8000], machine.memory[0x8001] = 0xe0, 0x42
	cpu := New(machine, machine)
	cpu.state = State{PC: 0x8000, X: 0x42, P: flagUnused | flagNegative}
	result, err := cpu.Step()
	if err != nil || result.Cycles != 2 || cpu.state.P&flagCarry == 0 || cpu.state.P&flagZero == 0 || cpu.state.P&flagNegative != 0 {
		t.Fatalf("result=%+v state=%+v err=%v", result, cpu.state, err)
	}
}

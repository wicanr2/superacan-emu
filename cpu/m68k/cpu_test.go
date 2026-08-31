package m68k

import (
	"reflect"
	"strings"
	"testing"
)

type eventLog struct{ events []string }

func (l *eventLog) Advance(p Phase) error {
	l.events = append(l.events, "advance:"+phaseName(p.Kind))
	return nil
}

type testBus struct {
	log        *eventLog
	words      map[uint32]uint16
	bytes      map[uint32]uint8
	writes     []wordWrite
	byteWrites []byteWrite
}

type byteWrite struct {
	address uint32
	value   uint8
}

type wordWrite struct {
	address uint32
	value   uint16
}

func (b *testBus) Read8(address uint32) (uint8, error) {
	b.log.events = append(b.log.events, "read8")
	return b.bytes[address], nil
}
func (b *testBus) Read16(address uint32) (uint16, error) {
	b.log.events = append(b.log.events, "read16")
	return b.words[address], nil
}
func (b *testBus) Write8(address uint32, value uint8) error {
	b.log.events = append(b.log.events, "write8")
	b.byteWrites = append(b.byteWrites, byteWrite{address: address, value: value})
	return nil
}
func (b *testBus) Write16(address uint32, value uint16) error {
	b.log.events = append(b.log.events, "write16")
	b.writes = append(b.writes, wordWrite{address: address, value: value})
	b.words[address] = value
	return nil
}

func phaseName(kind PhaseKind) string {
	switch kind {
	case PhaseInternal:
		return "internal"
	case PhaseInstructionFetch:
		return "fetch"
	case PhaseDataRead:
		return "read"
	case PhaseDataWrite:
		return "write"
	default:
		return "other"
	}
}

func TestResetVectorsPrefetchAndPhaseOrder(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{log: log, words: map[uint32]uint16{
		0: 0x00fc, 2: 0xfffe,
		4: 0x0000, 6: 0x0400,
		0x0400: 0x4e71, 0x0402: 0x4e71,
	}}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}

	state := cpu.State()
	if state.A[7] != 0x00fcfffe || state.PC != 0x0400 {
		t.Fatalf("reset vectors: A7=$%08X PC=$%08X", state.A[7], state.PC)
	}
	if state.IRD != 0x4e71 || state.IRC != 0x4e71 || state.Cycles != 40 {
		t.Fatalf("prefetch/cycles: IRD=$%04X IRC=$%04X cycles=%d", state.IRD, state.IRC, state.Cycles)
	}

	want := []string{
		"advance:internal",
		"advance:read", "read16",
		"advance:read", "read16",
		"advance:read", "read16",
		"advance:read", "read16",
		"advance:fetch", "read16",
		"advance:fetch", "read16",
	}
	if !reflect.DeepEqual(log.events, want) {
		t.Fatalf("phase order:\n got %v\nwant %v", log.events, want)
	}
}

func TestNOPAdvancesPrefetchByOneInstruction(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{log: log, words: map[uint32]uint16{
		0: 0, 2: 0x1000,
		4: 0, 6: 0x0400,
		0x0400: 0x4e71, 0x0402: 0x4e71, 0x0404: 0xffff,
	}}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.PCBefore != 0x0400 || result.PCAfter != 0x0402 || result.Cycles != 4 {
		t.Fatalf("result: %+v", result)
	}
	if state.IRD != 0x4e71 || state.IRC != 0xffff || state.Cycles != 44 {
		t.Fatalf("state: %+v", state)
	}
}

func TestAutovectoredInterruptStacksStateAndRefillsQueue(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{log: log, words: map[uint32]uint16{
		0: 0, 2: 0x1000, 4: 0, 6: 0x0400,
		0x0400: 0x4e71, 0x0402: 0x4e71,
		0x0070: 0, 0x0072: 0x0800,
		0x0800: 0x4e73, 0x0802: 0x4e71, 0x0804: 0x4e71,
	}}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	cpu.state.SR = 0x2000
	acknowledged := uint8(0)
	cpu.SetInterruptAcknowledge(func(level uint8) { acknowledged = level })
	cpu.SetInterruptLevel(4)
	result, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state := cpu.State()
	if result.InterruptLevel != 4 || acknowledged != 4 || result.Cycles != 44 {
		t.Fatalf("result=%+v ack=%d", result, acknowledged)
	}
	if state.PC != 0x0800 || state.SR != 0x2400 || state.A[7] != 0x0ffa {
		t.Fatalf("PC=$%06X SR=$%04X SP=$%06X", state.PC, state.SR, state.A[7])
	}
	if bus.words[0x0ffa] != 0x2000 || bus.words[0x0ffc] != 0 || bus.words[0x0ffe] != 0x0400 {
		t.Fatalf("frame SR=$%04X PC=$%04X%04X", bus.words[0x0ffa], bus.words[0x0ffc], bus.words[0x0ffe])
	}
	cpu.SetInterruptLevel(0)
	rte, err := cpu.Step()
	if err != nil {
		t.Fatal(err)
	}
	state = cpu.State()
	if rte.Cycles != 20 || state.PC != 0x0400 || state.SR != 0x2000 || state.A[7] != 0x1000 {
		t.Fatalf("RTE result=%+v PC=$%06X SR=$%04X SP=$%06X", rte, state.PC, state.SR, state.A[7])
	}
}

func TestUnknownOpcodeFailsClosed(t *testing.T) {
	log := &eventLog{}
	bus := &testBus{log: log, words: map[uint32]uint16{
		4: 0, 6: 0x0400,
		0x0400: 0xffff,
	}}
	cpu := New(bus, log)
	if err := cpu.Reset(); err != nil {
		t.Fatal(err)
	}
	before := cpu.State()
	_, err := cpu.Step()
	if err == nil || !strings.Contains(err.Error(), "unimplemented opcode") {
		t.Fatalf("expected unimplemented opcode error, got %v", err)
	}
	if after := cpu.State(); after != before {
		t.Fatalf("unknown opcode mutated state: before=%+v after=%+v", before, after)
	}
}

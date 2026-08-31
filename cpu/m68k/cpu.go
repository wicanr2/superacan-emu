package m68k

import "fmt"

const addressMask uint32 = 0x00ff_ffff

// State contains all currently modelled architectural and prefetch state.
// More exception and pipeline state will be added with the corresponding
// evidence-backed vertical slices.
type State struct {
	D [8]uint32
	A [8]uint32

	PC uint32
	SR uint16

	IRD uint16 // current instruction word
	IRC uint16 // next prefetched word

	Cycles uint64
}

// StepResult describes one complete instruction without hiding its timing.
type StepResult struct {
	PCBefore uint32
	PCAfter  uint32
	Opcode   uint16
	Cycles   uint64
}

// CPU is an independent Motorola 68000 implementation.
type CPU struct {
	bus       Bus
	scheduler Scheduler
	state     State
}

func New(bus Bus, scheduler Scheduler) *CPU {
	if bus == nil {
		panic("m68k: nil bus")
	}
	if scheduler == nil {
		panic("m68k: nil scheduler")
	}
	return &CPU{bus: bus, scheduler: scheduler}
}

func (c *CPU) State() State { return c.state }

// Reset performs the first evidence-backed vertical slice: supervisor state,
// initial SSP/PC vector reads and two-word prefetch. The 40-cycle total is a
// sample-derived starting contract and remains subject to Motorola-spec review.
func (c *CPU) Reset() error {
	c.state = State{SR: 0x2700}

	if err := c.advance(Phase{Kind: PhaseInternal, Cycles: 16}); err != nil {
		return err
	}

	sspHi, err := c.readWord(0, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset SSP high: %w", err)
	}
	sspLo, err := c.readWord(2, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset SSP low: %w", err)
	}
	c.state.A[7] = uint32(sspHi)<<16 | uint32(sspLo)

	pcHi, err := c.readWord(4, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset PC high: %w", err)
	}
	pcLo, err := c.readWord(6, FCSupervisorProgram, PhaseDataRead)
	if err != nil {
		return fmt.Errorf("m68k reset PC low: %w", err)
	}
	c.state.PC = (uint32(pcHi)<<16 | uint32(pcLo)) & addressMask

	c.state.IRD, err = c.readWord(c.state.PC, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return fmt.Errorf("m68k reset first prefetch: %w", err)
	}
	c.state.IRC, err = c.readWord(c.state.PC+2, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return fmt.Errorf("m68k reset second prefetch: %w", err)
	}
	return nil
}

// Step executes one complete instruction. Unknown opcodes fail closed and do
// not silently behave as NOP. NOP is the first opcode vertical slice.
func (c *CPU) Step() (StepResult, error) {
	result := StepResult{PCBefore: c.state.PC, Opcode: c.state.IRD}
	start := c.state.Cycles

	switch c.state.IRD {
	case 0x4e71: // NOP
		if err := c.prefetch(); err != nil {
			return result, fmt.Errorf("m68k NOP prefetch: %w", err)
		}
	default:
		return result, fmt.Errorf("m68k: unimplemented opcode $%04X at $%06X", c.state.IRD, c.state.PC)
	}

	result.PCAfter = c.state.PC
	result.Cycles = c.state.Cycles - start
	return result, nil
}

func (c *CPU) prefetch() error {
	nextAddress := (c.state.PC + 4) & addressMask
	next, err := c.readWord(nextAddress, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	c.state.PC = (c.state.PC + 2) & addressMask
	c.state.IRD = c.state.IRC
	c.state.IRC = next
	return nil
}

func (c *CPU) readWord(address uint32, fc FunctionCode, kind PhaseKind) (uint16, error) {
	address &= addressMask
	if address&1 != 0 {
		return 0, fmt.Errorf("odd word address $%06X", address)
	}
	phase := Phase{Kind: kind, Cycles: 4, Address: address, Width: WidthWord, FC: fc}
	if err := c.advance(phase); err != nil {
		return 0, err
	}
	return c.bus.Read16(address)
}

func (c *CPU) advance(phase Phase) error {
	if err := c.scheduler.Advance(phase); err != nil {
		return err
	}
	c.state.Cycles += uint64(phase.Cycles)
	return nil
}

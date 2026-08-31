// Package m68k implements the Motorola 68000 CPU used by Super A'Can.
//
// The implementation is independent.  Moira and the archived C++ emulator are
// differential oracles only; production code does not translate or wrap them.
package m68k

// PhaseKind identifies a CPU-visible scheduling boundary.
type PhaseKind uint8

const (
	PhaseInternal PhaseKind = iota
	PhaseInstructionFetch
	PhaseDataRead
	PhaseDataWrite
	PhaseInterruptAcknowledge
)

// Width is the width of one bus transaction in bits.
type Width uint8

const (
	WidthNone Width = 0
	WidthByte Width = 8
	WidthWord Width = 16
)

// FunctionCode is the value driven on the 68000 FC2-FC0 pins.
type FunctionCode uint8

const (
	FCUserData          FunctionCode = 1
	FCUserProgram       FunctionCode = 2
	FCSupervisorData    FunctionCode = 5
	FCSupervisorProgram FunctionCode = 6
	FCCPU               FunctionCode = 7
)

// Phase describes time that must elapse before a bus side effect occurs.
// Address and Value are meaningful for bus phases. Cycles uses 68000 clocks.
type Phase struct {
	Kind    PhaseKind
	Cycles  uint8
	Address uint32
	Width   Width
	Write   bool
	Value   uint32
	FC      FunctionCode
}

// Scheduler advances the rest of the machine to a CPU phase boundary.
// Implementations must be deterministic and must complete Advance before the
// CPU performs the corresponding Bus operation.
type Scheduler interface {
	Advance(Phase) error
}

// Bus is the 68000-visible address space. The CPU masks addresses to 24 bits.
// Word access is one transaction; long access will be expressed as two ordered
// word transactions by the CPU.
type Bus interface {
	Read8(address uint32) (uint8, error)
	Read16(address uint32) (uint16, error)
	Write8(address uint32, value uint8) error
	Write16(address uint32, value uint16) error
}

package m68k

// instructionStream consumes extension words from the two-word prefetch queue
// while making every replacement fetch visible to the shared scheduler.
type instructionStream struct {
	cpu         *CPU
	word        uint16
	nextAddress uint32
}

func (c *CPU) newInstructionStream() *instructionStream {
	return &instructionStream{
		cpu:         c,
		word:        c.state.IRC,
		nextAddress: (c.state.PC + 4) & addressMask,
	}
}

func (s *instructionStream) nextWord() (uint16, error) {
	result := s.word
	next, err := s.cpu.readWord(s.nextAddress, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return 0, err
	}
	s.word = next
	s.nextAddress = (s.nextAddress + 2) & addressMask
	return result, nil
}

func (s *instructionStream) finish() error {
	next, err := s.cpu.readWord(s.nextAddress, FCSupervisorProgram, PhaseInstructionFetch)
	if err != nil {
		return err
	}
	s.cpu.state.PC = (s.nextAddress - 2) & addressMask
	s.cpu.state.IRD = s.word
	s.cpu.state.IRC = next
	return nil
}

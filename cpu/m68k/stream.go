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

func (s *instructionStream) nextLong() (uint32, error) {
	hi, err := s.nextWord()
	if err != nil {
		return 0, err
	}
	lo, err := s.nextWord()
	if err != nil {
		return 0, err
	}
	return uint32(hi)<<16 | uint32(lo), nil
}

func (s *instructionStream) nextBriefIndexedAddress(base uint32) (uint32, error) {
	extension, err := s.nextWord()
	if err != nil {
		return 0, err
	}
	return s.cpu.briefIndexedAddress(base, extension), nil
}

func (c *CPU) briefIndexedAddress(base uint32, extension uint16) uint32 {
	// Full extension format and scale factors belong to 68020, not MC68000.
	indexRegister := uint8(extension >> 12 & 7)
	index := c.state.D[indexRegister]
	if extension&0x8000 != 0 {
		index = c.state.A[indexRegister]
	}
	if extension&0x0800 == 0 {
		index = uint32(int32(int16(index)))
	}
	displacement := int32(int8(extension))
	// 保留完整 32 位元：An 是 32 位元暫存器，位址匯流排的 24 位元遮罩由
	// readWord／writeWord 負責，PC 目標則由呼叫端自行遮罩。
	return uint32(int32(base) + int32(index) + displacement)
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

package machine

import "github.com/wicanr2/superacan-emu/cpu/m68k"

// InstructionRecord 是一條已執行（或執行失敗）的 68000 指令。
type InstructionRecord struct {
	Index  uint64
	PC     uint32
	Opcode uint16
	Cycles uint64
}

// InstructionRing 是有界的指令回溯緩衝：只保留最近 N 條，成本固定，
// 不會因為長時間執行而無限成長。預設關閉，開啟後不改變任何排程結果。
type InstructionRing struct {
	records []InstructionRecord
	next    int
	filled  bool
}

func NewInstructionRing(size int) *InstructionRing {
	if size <= 0 {
		return nil
	}
	return &InstructionRing{records: make([]InstructionRecord, size)}
}

func (r *InstructionRing) observe(index uint64, result m68k.StepResult) {
	if r == nil {
		return
	}
	r.records[r.next] = InstructionRecord{
		Index: index, PC: result.PCBefore, Opcode: result.Opcode, Cycles: result.Cycles,
	}
	r.next++
	if r.next == len(r.records) {
		r.next = 0
		r.filled = true
	}
}

// Records 依執行順序回傳保留下來的指令。
func (r *InstructionRing) Records() []InstructionRecord {
	if r == nil {
		return nil
	}
	if !r.filled {
		return append([]InstructionRecord(nil), r.records[:r.next]...)
	}
	ordered := make([]InstructionRecord, 0, len(r.records))
	ordered = append(ordered, r.records[r.next:]...)
	return append(ordered, r.records[:r.next]...)
}

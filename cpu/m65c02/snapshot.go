package m65c02

// Snapshot 是 65C02 在指令邊界的完整可觀察狀態，包含 WAI 造成的停等旗標與
// 兩條中斷輸入的取樣狀態。
type Snapshot struct {
	State      State
	IRQLine    bool
	NMIPending bool
	Waiting    bool
}

func (c *CPU) Snapshot() Snapshot {
	return Snapshot{State: c.state, IRQLine: c.irqLine, NMIPending: c.nmiPending, Waiting: c.waiting}
}

func (c *CPU) Restore(snapshot Snapshot) {
	c.state = snapshot.State
	c.irqLine = snapshot.IRQLine
	c.nmiPending = snapshot.NMIPending
	c.waiting = snapshot.Waiting
}

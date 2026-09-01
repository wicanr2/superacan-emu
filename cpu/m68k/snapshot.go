package m68k

// Snapshot 是 68000 在指令邊界的完整可觀察狀態。stepTrace 是單一指令內的暫存，
// 不跨指令邊界存在，因此不列入；interruptAcknowledge 與 bus、scheduler 一樣是
// 接線而不是狀態。
type Snapshot struct {
	State          State
	InterruptLevel uint8
	Level7Pending  bool
}

func (c *CPU) Snapshot() Snapshot {
	return Snapshot{State: c.state, InterruptLevel: c.interruptLevel, Level7Pending: c.level7Pending}
}

func (c *CPU) Restore(snapshot Snapshot) {
	c.state = snapshot.State
	c.interruptLevel = snapshot.InterruptLevel
	c.level7Pending = snapshot.Level7Pending
	c.stepTrace = nil
}

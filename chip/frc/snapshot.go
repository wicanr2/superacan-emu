package frc

// Snapshot 是 FRC 計時器的完整狀態，含尚未走完的餘數。
type Snapshot struct {
	Control   uint16
	Frequency uint16
	Remaining int64
	Active    bool
	Pending   bool
	Supported bool
}

func (d *Device) Snapshot() Snapshot {
	return Snapshot{
		Control: d.control, Frequency: d.frequency, Remaining: d.remaining,
		Active: d.active, Pending: d.pending, Supported: d.supported,
	}
}

func (d *Device) Restore(s Snapshot) {
	d.control, d.frequency, d.remaining = s.Control, s.Frequency, s.Remaining
	d.active, d.pending, d.supported = s.Active, s.Pending, s.Supported
}

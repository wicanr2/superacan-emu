package umc6650

// Snapshot 是 UMC6650 的 128 byte 內部記憶體與位址閂。
type Snapshot struct {
	Memory  [128]byte
	Address uint8
}

func (d *Device) Snapshot() Snapshot {
	return Snapshot{Memory: d.memory, Address: d.address}
}

func (d *Device) Restore(s Snapshot) {
	d.memory, d.address = s.Memory, s.Address
}

package umc6619

// ChannelSnapshot 是單一 PCM 通道的完整播放狀態，含小數相位。
type ChannelSnapshot struct {
	Pitch       uint16
	Increment   uint32
	Start       uint16
	Length      uint16
	Current     uint16
	End         uint16
	Fraction    uint32
	DMA         uint8
	VolumeLeft  uint8
	VolumeRight uint8
	OneShot     bool
}

// Snapshot 是 UM6619 會影響後續輸出的全部狀態。sampleCount 是統計量，一併保存
// 讓載入後的計數延續而不是歸零。
type Snapshot struct {
	Address        uint8
	Registers      [256]uint8
	Channels       [16]ChannelSnapshot
	Active         uint16
	SampleCycles   uint64
	TimerRemaining int64
	TimerRunning   bool
	TimerIRQ       bool
	DMAIRQ         bool
	SampleCount    uint64
}

func (d *Device) Snapshot() Snapshot {
	s := Snapshot{
		Address: d.address, Registers: d.registers, Active: d.active,
		SampleCycles: d.sampleCycles, TimerRemaining: d.timerRemaining,
		TimerRunning: d.timerRunning, TimerIRQ: d.timerIRQ, DMAIRQ: d.dmaIRQ,
		SampleCount: d.sampleCount,
	}
	for index, c := range d.channels {
		s.Channels[index] = ChannelSnapshot{
			Pitch: c.pitch, Increment: c.increment, Start: c.start, Length: c.length,
			Current: c.current, End: c.end, Fraction: c.fraction, DMA: c.dma,
			VolumeLeft: c.volumeLeft, VolumeRight: c.volumeRight, OneShot: c.oneShot,
		}
	}
	return s
}

func (d *Device) Restore(s Snapshot) {
	d.address, d.registers, d.active = s.Address, s.Registers, s.Active
	d.sampleCycles, d.timerRemaining = s.SampleCycles, s.TimerRemaining
	d.timerRunning, d.timerIRQ, d.dmaIRQ = s.TimerRunning, s.TimerIRQ, s.DMAIRQ
	d.sampleCount = s.SampleCount
	for index, c := range s.Channels {
		d.channels[index] = channel{
			pitch: c.Pitch, increment: c.Increment, start: c.Start, length: c.Length,
			current: c.Current, end: c.End, fraction: c.Fraction, dma: c.DMA,
			volumeLeft: c.VolumeLeft, volumeRight: c.VolumeRight, oneShot: c.OneShot,
		}
	}
}

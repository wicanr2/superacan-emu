package hostdma

// Snapshot 是兩個 DMA 通道的暫存器與觸發計數。busy 只在單次傳輸期間為真，
// 傳輸在指令邊界之內完成，因此不會跨越存檔點。
type Snapshot struct {
	Channels [2]ChannelState
}

func (d *Device) Snapshot() Snapshot {
	return Snapshot{Channels: d.channels}
}

func (d *Device) Restore(s Snapshot) {
	d.channels = s.Channels
}

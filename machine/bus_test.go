package machine

import "testing"

func testMachineBus(t *testing.T) *Bus {
	t.Helper()
	ipl := make([]byte, IPLSize)
	rom := make([]byte, 0x400000)
	key := make([]byte, 16)
	ipl[0], ipl[1], ipl[0x604] = 0xfd, 0x00, 0x4e
	rom[0], rom[1], rom[0x604] = 0x12, 0x34, 0xa5
	b, err := NewBus(ipl, rom, key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOverlayLatchesAreIndependentAndOneWay(t *testing.T) {
	b := testMachineBus(t)
	if low, _ := b.Read16(0); low != 0xfd00 {
		t.Fatalf("low IPL=$%04X", low)
	}
	if high, _ := b.Read8(0xf80604); high != 0x4e {
		t.Fatalf("high IPL=$%02X", high)
	}
	if err := b.Write16(0xe9001c, 0x0002); err != nil {
		t.Fatal(err)
	}
	if low, _ := b.Read16(0); low != 0x1234 || !b.HighOverlayEnabled() {
		t.Fatalf("after low disable: low=$%04X high=%v", low, b.HighOverlayEnabled())
	}
	if err := b.Write16(0xe9001c, 0); err != nil {
		t.Fatal(err)
	}
	if b.LowOverlayEnabled() {
		t.Fatal("low overlay re-enabled after control clear")
	}
	if err := b.Write16(0xe9001c, 0x0008); err != nil {
		t.Fatal(err)
	}
	if high, _ := b.Read8(0xf80604); high != 0xa5 || b.HighOverlayEnabled() {
		t.Fatalf("after high disable: high=$%02X enabled=%v", high, b.HighOverlayEnabled())
	}
}

func TestLockoutWorkRAMMirrorsAndSRAMLanes(t *testing.T) {
	b := testMachineBus(t)
	if err := b.Write8(0xeb0d03, 0x5f); err != nil {
		t.Fatal(err)
	}
	if err := b.Write8(0xeb0d01, 0xa5); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Read8(0xeb0d01); got != 0xa5 {
		t.Fatalf("lockout RAM=$%02X", got)
	}
	if err := b.Write8(0xfc1234, 0x6a); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Read8(0xff1234); got != 0x6a {
		t.Fatalf("Work RAM mirror=$%02X", got)
	}
	_ = b.Write8(0xec0000, 0x11)
	_ = b.Write8(0xec0001, 0x22)
	if even, _ := b.Read8(0xec0000); even != 0xff {
		t.Fatalf("SRAM even lane=$%02X", even)
	}
	if odd, _ := b.Read8(0xec0001); odd != 0x22 {
		t.Fatalf("SRAM odd lane=$%02X", odd)
	}
}

func TestObserverReportsOneCompleteTransactionPerAccess(t *testing.T) {
	b := testMachineBus(t)
	var transactions []Transaction
	b.SetObserver(func(transaction Transaction) { transactions = append(transactions, transaction) })

	if err := b.Write16(0xfc0010, 0x1234); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Read16(0xfc0010); err != nil || got != 0x1234 {
		t.Fatalf("Read16=$%04X err=%v", got, err)
	}
	want := []Transaction{
		{Address: 0xfc0010, Width: 2, Write: true, Value: 0x1234},
		{Address: 0xfc0010, Width: 2, Value: 0x1234},
	}
	if len(transactions) != len(want) {
		t.Fatalf("transactions=%+v", transactions)
	}
	for i := range want {
		if transactions[i] != want[i] {
			t.Fatalf("transaction[%d]=%+v, want %+v", i, transactions[i], want[i])
		}
	}
}

func TestUMC6618WindowsAndWordWriteAreDeviceTransactions(t *testing.T) {
	b := testMachineBus(t)
	if err := b.Write16(0xf00008, 0x0188); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Read16(0xf00008); got != 0x0188 || b.Video().VideoFlags() != 0x0188 {
		t.Fatalf("video flags=$%04X device=$%04X", got, b.Video().VideoFlags())
	}
	if err := b.Write16(0xf0020e, 0x7c1f); err != nil {
		t.Fatal(err)
	}
	if err := b.Write16(0xf44400, 0x1234); err != nil {
		t.Fatal(err)
	}
	if palette, _ := b.Read16(0xf0020e); palette != 0x7c1f {
		t.Fatalf("palette=$%04X", palette)
	}
	if vram, _ := b.Read16(0xf44400); vram != 0x1234 {
		t.Fatalf("VRAM=$%04X", vram)
	}
	if err := b.Write16(0xf0001e, 0x8000); err != nil {
		t.Fatal(err)
	}
	if b.Video().SpriteDMAStarts() != 1 {
		t.Fatalf("word write triggered sprite DMA %d times", b.Video().SpriteDMAStarts())
	}
}

func TestControllerDirectModeAndSoundIOWindow(t *testing.T) {
	b := testMachineBus(t)
	sound := newSoundBus(&b.soundRAM)
	b.attachSound(sound)
	sound.SetPad(0, 0x7fff)
	sound.SetPad(1, 0xbfff)
	if p1, _ := b.Read16(0xe80200); p1 != 0x8000 {
		t.Fatalf("P1 direct=$%04X", p1)
	}
	if p2, _ := b.Read16(0xe80202); p2 != 0x4000 {
		t.Fatalf("P2 direct=$%04X", p2)
	}
	if err := b.Write8(0xe80404, 0xa5); err != nil {
		t.Fatal(err)
	}
	if got, _ := sound.Read8(0x0404); got != 0xa5 {
		t.Fatalf("forwarded latch=$%02X", got)
	}
	if err := b.Write8(0xe9000a, 1); err != nil {
		t.Fatal(err)
	}
	if sound.IRQStatus()&0x20 == 0 {
		t.Fatalf("command source=$%02X", sound.IRQStatus())
	}
}

func TestFRCWordTransactionsConfigureDeviceOnce(t *testing.T) {
	b := testMachineBus(t)
	if err := b.Write16(0xe90016, 2); err != nil {
		t.Fatal(err)
	}
	if err := b.Write16(0xe90014, 0xa201); err != nil {
		t.Fatal(err)
	}
	if control, _ := b.Read16(0xe90014); control != 0xa201 {
		t.Fatalf("control=$%04X", control)
	}
	if frequency, _ := b.Read16(0xe90016); frequency != 2 {
		t.Fatalf("frequency=$%04X", frequency)
	}
	if b.FRC().RemainingCycles() != 1024*2 {
		t.Fatalf("remaining=%d", b.FRC().RemainingCycles())
	}
}

func TestHostDMAUsesAtomicControlAndMachineBusTransactions(t *testing.T) {
	b := testMachineBus(t)
	b.rom[0x10000], b.rom[0x10001], b.rom[0x10002], b.rom[0x10003] = 0x12, 0x34, 0x56, 0x78
	for address, value := range map[uint32]uint16{
		0xe90020: 0x0001, 0xe90022: 0x0000,
		0xe90024: 0x00fc, 0xe90026: 0x0200,
		0xe90028: 0x0001,
	} {
		if err := b.Write16(address, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Write16(0xe9002a, 0x9000); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Read16(0xfc0200); got != 0x1234 {
		t.Fatalf("first word=$%04X", got)
	}
	if got, _ := b.Read16(0xfc0202); got != 0x5678 {
		t.Fatalf("second word=$%04X", got)
	}
	if b.HostDMA().Channel(0).Triggers != 1 {
		t.Fatalf("triggers=%d", b.HostDMA().Channel(0).Triggers)
	}
}

func TestUM6619HostReadPortsExposeSoundRAMLatches(t *testing.T) {
	bus := testMachineBus(t)
	// $E90004/05 讀 sound RAM $040C/$040D；$E9000C/0D 讀 $040A。
	if err := bus.Write8(0xe8040c, 0x12); err != nil {
		t.Fatal(err)
	}
	if err := bus.Write8(0xe8040d, 0x34); err != nil {
		t.Fatal(err)
	}
	if err := bus.Write8(0xe8040a, 0x5a); err != nil {
		t.Fatal(err)
	}
	value, err := bus.Read16(0xe90004)
	if err != nil || value != 0x1234 {
		t.Fatalf("$E90004=$%04X err=%v, want $1234", value, err)
	}
	flag, err := bus.Read8(0xe9000c)
	if err != nil || flag != 0x5a {
		t.Fatalf("$E9000C=$%02X err=%v, want $5A", flag, err)
	}
	high, err := bus.Read8(0xe9000d)
	if err != nil || high != 0x5a {
		t.Fatalf("$E9000D=$%02X err=%v, want $5A", high, err)
	}
}

func TestSoundCycleCounterPortIsReadable(t *testing.T) {
	bus := testMachineBus(t)
	bus.SetSoundCycleSource(func() uint64 { return 0x1_0000 + 0x2345 })
	value, err := bus.Read16(0xe90018)
	if err != nil || value != uint16((0x1_0000+0x2345)%0xffff) {
		t.Fatalf("$E90018=$%04X err=%v", value, err)
	}
}

func TestCartridgeSaveRoundTripsAndRejectsWrongSize(t *testing.T) {
	bus := testMachineBus(t)
	if err := bus.Write8(0xec0001, 0x5a); err != nil {
		t.Fatal(err)
	}
	payload := bus.CartridgeSave()
	if len(payload) != SRAMSize || payload[0] != 0x5a {
		t.Fatalf("save len=%d first=$%02X", len(payload), payload[0])
	}

	fresh := testMachineBus(t)
	if err := fresh.LoadCartridgeSave(payload); err != nil {
		t.Fatal(err)
	}
	value, err := fresh.Read8(0xec0001)
	if err != nil || value != 0x5a {
		t.Fatalf("restored=$%02X err=%v", value, err)
	}
	if err := fresh.LoadCartridgeSave(payload[:10]); err == nil {
		t.Fatal("大小不符必須拒絕")
	}
	if value, _ := fresh.Read8(0xec0001); value != 0x5a {
		t.Fatalf("拒絕載入不得改變現行狀態，讀到 $%02X", value)
	}
}

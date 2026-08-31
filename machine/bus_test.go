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
	if b.FRC().RemainingCycles() != 1024*0x010002 {
		t.Fatalf("remaining=%d", b.FRC().RemainingCycles())
	}
}

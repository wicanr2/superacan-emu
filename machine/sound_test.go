package machine

import (
	"testing"

	"github.com/wicanr2/superacan-emu/cpu/m65c02"
)

func TestSoundBusSharesRAMAndRoutesUMC6619Port(t *testing.T) {
	bus := testMachineBus(t)
	sound := newSoundBus(&bus.soundRAM)
	if err := bus.Write8(0xe81234, 0xa5); err != nil {
		t.Fatal(err)
	}
	if got, _ := sound.Read8(0x1234); got != 0xa5 {
		t.Fatalf("sound shared RAM=$%02X", got)
	}
	_ = sound.Write8(0x0420, 0x14)
	_ = sound.Write8(0x0422, 0xcf)
	if got, _ := sound.Read8(0x0422); got != 0xcf || sound.Audio().Register(0x14) != 0xcf {
		t.Fatalf("UMC6619 data=$%02X register=$%02X", got, sound.Audio().Register(0x14))
	}
	if empty, _ := sound.Read8(0x0404); empty != 0xcd {
		t.Fatalf("empty latch=$%02X", empty)
	}
}

func TestSoundBusRoutesUMC6619IRQAndDedicatedAcknowledge(t *testing.T) {
	bus := testMachineBus(t)
	sound := newSoundBus(&bus.soundRAM)
	_ = sound.Write8(0x0410, 0x80)
	for _, write := range [][2]uint8{{0x11, 0xff}, {0x12, 0xff}, {0x14, 0xc0}} {
		_ = sound.Write8(0x0420, write[0])
		_ = sound.Write8(0x0422, write[1])
	}
	sound.Audio().Advance(10)
	if sound.IRQStatus() != 0x80 || !sound.IRQAsserted() {
		t.Fatalf("status=$%02X asserted=%v", sound.IRQStatus(), sound.IRQAsserted())
	}
	if status, _ := sound.Read8(0x0411); status != 0x80 || sound.IRQStatus() != 0x80 {
		t.Fatal("$0411 did not preserve level-held source")
	}
	_ = sound.Write8(0x0420, 0x14)
	if _, err := sound.Read8(0x0422); err != nil {
		t.Fatal(err)
	}
	if sound.IRQStatus() != 0 || sound.IRQAsserted() {
		t.Fatalf("ack status=$%02X asserted=%v", sound.IRQStatus(), sound.IRQAsserted())
	}
}

func TestSoundTimelineAdvancesNativeSampleDomain(t *testing.T) {
	bus := testMachineBus(t)
	sound := newSoundBus(&bus.soundRAM)
	timeline := &SoundTimeline{OnAdvance: sound.Audio().Advance}
	for range 80 {
		if err := timeline.Advance(m65c02.Cycle{Internal: true}); err != nil {
			t.Fatal(err)
		}
	}
	if timeline.Cycles != 80 || sound.Audio().SampleCount() != 1 {
		t.Fatalf("cycles=%d samples=%d", timeline.Cycles, sound.Audio().SampleCount())
	}
}

func TestControllerShiftRegisterUsesFallingEdgesMSBFirst(t *testing.T) {
	bus := testMachineBus(t)
	sound := newSoundBus(&bus.soundRAM)
	sound.SetPad(0, 0x7fff) // A pressed, all other controls released.
	_ = sound.Write8(0x0407, 0x15)
	_ = sound.Write8(0x0407, 0x14) // latch P1
	_ = sound.Write8(0x0407, 0x10) // shift A bit
	if got, _ := sound.Read8(0x0402); got != 0 {
		t.Fatalf("first shift=$%02X", got)
	}
	_ = sound.Write8(0x0407, 0x14)
	_ = sound.Write8(0x0407, 0x10) // shift B bit
	if got, _ := sound.Read8(0x0402); got != 1 {
		t.Fatalf("second shift=$%02X", got)
	}
	_ = sound.Write8(0x0407, 0x00) // clear P1 and issue probe IRQ
	if got, _ := sound.Read8(0x0402); got != 0 || sound.IRQStatus()&0x08 == 0 {
		t.Fatalf("clear shift=$%02X irq=$%02X", got, sound.IRQStatus())
	}
}

func TestSoundLatchesAndCommandIRQHaveDedicatedAcks(t *testing.T) {
	bus := testMachineBus(t)
	sound := newSoundBus(&bus.soundRAM)
	_ = sound.Write8(0x0410, 0x28)
	sound.WriteFrom68K(0x0404, 0xa5)
	sound.RequestFrom68K()
	if sound.IRQStatus() != 0x28 || !sound.IRQAsserted() {
		t.Fatalf("sources=$%02X asserted=%v", sound.IRQStatus(), sound.IRQAsserted())
	}
	if got, _ := sound.Read8(0x0404); got != 0xa5 || sound.IRQStatus() != 0x20 {
		t.Fatalf("latch=$%02X sources=$%02X", got, sound.IRQStatus())
	}
	if _, _ = sound.Read8(0x040a); sound.IRQStatus() != 0 {
		t.Fatalf("command ack sources=$%02X", sound.IRQStatus())
	}
}

func TestSoundRAMAliasModelsSingle32KDevice(t *testing.T) {
	bus := testMachineBus(t)
	sound := newSoundBus(&bus.soundRAM)
	bus.sound = sound

	// Default 64 KiB model: the two halves are independent storage.
	if err := bus.Write8(0xe88400, 0xa5); err != nil {
		t.Fatal(err)
	}
	if got, _ := bus.Read8(0xe80400); got == 0xa5 {
		t.Fatal("64 KiB model aliased $8400 onto $0400")
	}

	bus.SetSoundRAMAlias(true)
	if !bus.SoundRAMAlias() {
		t.Fatal("alias model was not selected")
	}
	if err := bus.Write8(0xe88400, 0x5a); err != nil {
		t.Fatal(err)
	}
	if got, _ := bus.Read8(0xe80400); got != 0x5a {
		t.Fatalf("aliased 68000 read=$%02X", got)
	}
	if got, _ := sound.Read8(0x8400); got != 0x5a {
		t.Fatalf("aliased W65C02 read=$%02X", got)
	}
	// $0400-$04FF stays decoded as I/O for the sound CPU even when A15 is dropped,
	// which is what makes the physical cell reachable only through $8400.
	if got, _ := sound.Read8(0x0400); got == 0x5a {
		t.Fatal("I/O page was replaced by aliased RAM")
	}
	if err := sound.Write8(0xf123, 0x3c); err != nil {
		t.Fatal(err)
	}
	if got, _ := bus.Read8(0xe87123); got != 0x3c {
		t.Fatalf("aliased sound write not visible at $7123: $%02X", got)
	}
}

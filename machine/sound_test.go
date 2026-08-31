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

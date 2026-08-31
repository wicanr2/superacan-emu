package machine

import "testing"

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

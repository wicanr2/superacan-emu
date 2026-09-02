package frc

import "testing"

func TestModeOneRaisesLevelUntilAcknowledgeAndReschedules(t *testing.T) {
	d := New()
	d.WriteFrequency(4)
	d.WriteControl(0xa201)
	want := mode1MasterTicks * 5 // 週期算 (n+1)；取 10 的倍數以免整數除法截斷
	if !d.Active() || !d.SupportedMode() || d.RemainingMasterTicks() != want {
		t.Fatalf("initial device=%+v", *d)
	}
	d.AdvanceM68KCycles(uint64(want / masterTicksPerM68KCycle))
	if !d.Pending() || d.Active() {
		t.Fatalf("expired device=%+v", *d)
	}
	d.AdvanceM68KCycles(255)
	if !d.Pending() {
		t.Fatal("pending IRQ was not level-held")
	}
	d.Acknowledge()
	if d.Pending() || !d.Active() || d.RemainingMasterTicks() != want {
		t.Fatalf("acknowledged device=%+v", *d)
	}
}

func TestKnownModesAndUnknownModeFailClosed(t *testing.T) {
	d := New()
	d.WriteControl(0xa200)
	if d.RemainingMasterTicks() != oneSecondMasterTicks {
		t.Fatalf("mode 0 ticks=%d", d.RemainingMasterTicks())
	}
	d.WriteFrequency(3)
	d.WriteControl(0xa20f)
	if d.RemainingMasterTicks() != modeFMasterTicks*4 {
		t.Fatalf("mode F ticks=%d", d.RemainingMasterTicks())
	}
	d.WriteControl(0xa202)
	if d.Active() || d.SupportedMode() {
		t.Fatal("unknown mode was scheduled")
	}
	d.WriteControl(0x0201)
	if d.Active() || d.SupportedMode() {
		t.Fatal("disabled prefix was scheduled")
	}
}

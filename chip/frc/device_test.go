package frc

import "testing"

func TestModeOneRaisesLevelUntilAcknowledgeAndReschedules(t *testing.T) {
	d := New()
	d.WriteFrequency(2)
	d.WriteControl(0xa201)
	want := int64(1024 * 0x010002)
	if !d.Active() || !d.SupportedMode() || d.RemainingCycles() != want {
		t.Fatalf("initial device=%+v", *d)
	}
	d.AdvanceM68KCycles(uint64(want))
	if !d.Pending() || d.Active() {
		t.Fatalf("expired device=%+v", *d)
	}
	d.AdvanceM68KCycles(255)
	if !d.Pending() {
		t.Fatal("pending IRQ was not level-held")
	}
	d.Acknowledge()
	if d.Pending() || !d.Active() || d.RemainingCycles() != want {
		t.Fatalf("acknowledged device=%+v", *d)
	}
}

func TestKnownModesAndUnknownModeFailClosed(t *testing.T) {
	d := New()
	d.WriteControl(0xa200)
	if d.RemainingCycles() != OneHzM68KCycles {
		t.Fatalf("mode 0 cycles=%d", d.RemainingCycles())
	}
	d.WriteFrequency(3)
	d.WriteControl(0xa20f)
	if d.RemainingCycles() != 8192*0x0f0003 {
		t.Fatalf("mode F cycles=%d", d.RemainingCycles())
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

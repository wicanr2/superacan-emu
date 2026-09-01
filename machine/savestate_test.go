package machine

import (
	"bytes"
	"testing"
)

func newSaveStateSystem(t *testing.T) *System {
	t.Helper()
	ipl := make([]byte, IPLSize)
	rom := make([]byte, 0x10000)
	key := make([]byte, 16)
	// reset 向量：SSP=$00001000、PC=$00000400，接著一串 NOP。
	ipl[0], ipl[1], ipl[2], ipl[3] = 0x00, 0x00, 0x10, 0x00
	ipl[4], ipl[5], ipl[6], ipl[7] = 0x00, 0x00, 0x04, 0x00
	for offset := 0x400; offset < 0x500; offset += 2 {
		ipl[offset], ipl[offset+1] = 0x4e, 0x71 // NOP
	}
	system, err := NewSystem(ipl, rom, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := system.Reset(); err != nil {
		t.Fatal(err)
	}
	return system
}

func TestSaveStateRoundTripRestoresExactSnapshot(t *testing.T) {
	system := newSaveStateSystem(t)
	if _, err := system.RunInstructions(64); err != nil {
		t.Fatal(err)
	}
	saved := system.Snapshot()

	var file bytes.Buffer
	if err := system.SaveState(&file); err != nil {
		t.Fatal(err)
	}
	if _, err := system.RunInstructions(64); err != nil {
		t.Fatal(err)
	}
	if system.Snapshot() == saved {
		t.Fatal("繼續執行之後狀態必須改變，否則這個測試證明不了任何事")
	}
	if err := system.LoadState(bytes.NewReader(file.Bytes())); err != nil {
		t.Fatal(err)
	}
	if system.Snapshot() != saved {
		t.Fatal("載入之後的狀態與存檔當下不同")
	}
}

func TestSaveStateReplayIsDeterministic(t *testing.T) {
	system := newSaveStateSystem(t)
	if _, err := system.RunInstructions(32); err != nil {
		t.Fatal(err)
	}
	var file bytes.Buffer
	if err := system.SaveState(&file); err != nil {
		t.Fatal(err)
	}
	if _, err := system.RunInstructions(128); err != nil {
		t.Fatal(err)
	}
	continuous := system.Snapshot()

	// 換一台全新的機器載入同一份存檔，再跑同樣多的指令。
	replay := newSaveStateSystem(t)
	if err := replay.LoadState(bytes.NewReader(file.Bytes())); err != nil {
		t.Fatal(err)
	}
	if _, err := replay.RunInstructions(128); err != nil {
		t.Fatal(err)
	}
	if replay.Snapshot() != continuous {
		t.Fatal("由存檔續跑的結果與連續執行不同")
	}
}

func TestLoadStateRejectsBadFilesWithoutChangingState(t *testing.T) {
	system := newSaveStateSystem(t)
	if _, err := system.RunInstructions(16); err != nil {
		t.Fatal(err)
	}
	var file bytes.Buffer
	if err := system.SaveState(&file); err != nil {
		t.Fatal(err)
	}
	if _, err := system.RunInstructions(16); err != nil {
		t.Fatal(err)
	}
	before := system.Snapshot()
	good := file.Bytes()

	corrupted := append([]byte(nil), good...)
	corrupted[len(corrupted)-1] ^= 0xff

	truncated := append([]byte(nil), good[:len(good)-8]...)

	badMagic := append([]byte(nil), good...)
	badMagic[0] = 'X'

	badVersion := append([]byte(nil), good...)
	badVersion[8] = 0x09

	tests := map[string][]byte{
		"payload 損壞": corrupted,
		"截斷":         truncated,
		"magic 不符":   badMagic,
		"版本不符":       badVersion,
	}
	for name, payload := range tests {
		if err := system.LoadState(bytes.NewReader(payload)); err == nil {
			t.Fatalf("%s 必須被拒絕", name)
		}
		if system.Snapshot() != before {
			t.Fatalf("%s 被拒絕之後不得改變現行狀態", name)
		}
	}

	// 不同卡帶身分的存檔也要拒絕。
	other := newSaveStateSystem(t)
	other.ROMSHA256[0] ^= 0xff
	if err := other.LoadState(bytes.NewReader(good)); err == nil {
		t.Fatal("不同卡帶的存檔必須被拒絕")
	}
}

package cheat

import (
	"bytes"
	"strings"
	"testing"
)

func snapshotWith(values map[uint32]uint16) []byte {
	snapshot := make([]byte, WorkRAMSize)
	for address, value := range values {
		offset := address - WorkRAMBase
		snapshot[offset] = byte(value >> 8)
		snapshot[offset+1] = byte(value)
	}
	return snapshot
}

// 「等於 99 → 縮小 → 縮小」要可重現：同一組快照序列必須得到同一組候選。
func TestSearchNarrowsReproducibly(t *testing.T) {
	first := snapshotWith(map[uint32]uint16{0xfc1a20: 99, 0xfc3e88: 99, 0xfc5000: 99, 0xfc7000: 100})
	second := snapshotWith(map[uint32]uint16{0xfc1a20: 99, 0xfc3e88: 98, 0xfc5000: 99, 0xfc7000: 100})
	third := snapshotWith(map[uint32]uint16{0xfc1a20: 99, 0xfc3e88: 98, 0xfc5000: 97, 0xfc7000: 100})

	run := func() []uint32 {
		search := &Search{Width: 16}
		search.New(first, CompareEqual, 99)
		search.Refine(second, CompareEqual, 99)
		search.Refine(third, CompareUnchanged, 0)
		return search.Candidates()
	}
	a, b := run(), run()
	if len(a) != len(b) {
		t.Fatalf("兩次搜尋長度不同：%d / %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("候選 %d 不同：$%06X / $%06X", i, a[i], b[i])
		}
	}
	if len(a) != 1 || a[0] != 0xfc1a20 {
		t.Fatalf("候選 %v，want [$FC1A20]", a)
	}
}

// 候選超過上限時只回上限筆數，而且要回報截斷——使用者要知道自己看到的不是全部。
func TestSearchReportsTruncation(t *testing.T) {
	snapshot := make([]byte, WorkRAMSize) // 全部是 0
	search := &Search{Width: 16}
	search.New(snapshot, CompareEqual, 0)
	if len(search.Candidates()) != MaxCandidates {
		t.Fatalf("候選 %d，want %d", len(search.Candidates()), MaxCandidates)
	}
	if !search.Truncated() {
		t.Fatal("超過上限必須回報截斷")
	}
}

// 越界或寬度不合法的項目不算有效，寫不進去也存不下來。
func TestEntryValidity(t *testing.T) {
	for _, c := range []struct {
		entry Entry
		want  bool
	}{
		{Entry{Address: 0xfc0000, Width: 8}, true},
		{Entry{Address: 0xfcfffe, Width: 16}, true},
		{Entry{Address: 0xfcffff, Width: 16}, false},
		{Entry{Address: 0xfbffff, Width: 8}, false},
		{Entry{Address: 0xfd0000, Width: 8}, false},
		{Entry{Address: 0xf40000, Width: 8}, false},
		{Entry{Address: 0xfc0000, Width: 12}, false},
	} {
		if got := c.entry.Valid(); got != c.want {
			t.Errorf("$%06X/%d：%v，want %v", c.entry.Address, c.entry.Width, got, c.want)
		}
	}
}

// ACANCHT1 往返之後內容相同。
func TestACANRoundTrip(t *testing.T) {
	entries := []Entry{
		{Name: "生命值", Address: 0xfc1a20, Width: 16, Value: 99, Format: FormatDecimal, Locked: true},
		{Name: "金錢", Address: 0xfc3e88, Width: 16, Value: 9999, Format: FormatBCD},
		{Name: "關卡", Address: 0xfc02c4, Width: 8, Value: 3, Format: FormatHex, Locked: true},
	}
	var buffer bytes.Buffer
	if err := Write(&buffer, entries); err != nil {
		t.Fatal(err)
	}
	loaded, warnings, err := Read(&buffer)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if len(loaded) != len(entries) {
		t.Fatalf("讀回 %d 筆", len(loaded))
	}
	for i := range entries {
		if loaded[i] != entries[i] {
			t.Fatalf("第 %d 筆 %+v，want %+v", i, loaded[i], entries[i])
		}
	}
}

// Bcan 的 .cht 匯入：項目數與內容要與來源相同，壞掉的行以警告回報而不是整檔作廢。
func TestBcanImport(t *testing.T) {
	document := strings.Join([]string{
		"; Bcan per-game cheat file",
		"BCAN_CHT_1",
		"Lives\t$FC1A20\t16\t99\tdec",
		"Money\t$FC3E88\t16\t9999\tbcd",
		"Broken\tnot-an-address\t16\t1\tdec",
		"OutOfRange\t$FD0000\t16\t1\tdec",
		"",
	}, "\n")
	entries, warnings, err := ReadBcan(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("讀到 %d 筆：%+v", len(entries), entries)
	}
	if entries[0].Name != "Lives" || entries[0].Address != 0xfc1a20 || entries[0].Value != 99 {
		t.Fatalf("第一筆 %+v", entries[0])
	}
	if entries[1].Format != FormatBCD {
		t.Fatalf("第二筆格式 %v", entries[1].Format)
	}
	if len(warnings) != 2 {
		t.Fatalf("壞掉的兩行要各回報一次，得到 %v", warnings)
	}
}

// 第一欄是位址時要自動改用「位址優先」的順序：Bcan 的逐欄順序沒有直接證據。
func TestBcanImportAcceptsAddressFirst(t *testing.T) {
	document := "BCAN_CHT_1\n$FC1A20\tLives\t16\t99\tdec\n"
	entries, _, err := ReadBcan(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "Lives" || entries[0].Address != 0xfc1a20 {
		t.Fatalf("得到 %+v", entries)
	}
}

// 標頭不符要拒絕整份檔案，不要當成資料讀進來。
func TestWrongHeaderIsRejected(t *testing.T) {
	if _, _, err := Read(strings.NewReader("SOMETHING_ELSE\n")); err == nil {
		t.Fatal("標頭不符必須拒絕")
	}
}

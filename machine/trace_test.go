package machine

import "testing"

func TestParseAddressRangesAndBoundedTrace(t *testing.T) {
	ranges, err := ParseAddressRanges("$e80000-e9001f,0xF00000")
	if err != nil {
		t.Fatal(err)
	}
	trace := TransactionTrace{Ranges: ranges, Limit: 1}
	trace.Observe(Transaction{Address: 0xe80000, Width: 2})
	trace.Observe(Transaction{Address: 0xe90020, Width: 2})
	trace.Observe(Transaction{Address: 0xf00000, Width: 2, Write: true})
	if trace.Matched != 2 || len(trace.Records) != 1 || trace.Records[0].Address != 0xe80000 {
		t.Fatalf("trace=%+v", trace)
	}
}

func TestParseAddressRangesRejectsInvalidInput(t *testing.T) {
	for _, spec := range []string{"e90000-e80000", "1000000", "e80000-", "one"} {
		if _, err := ParseAddressRanges(spec); err == nil {
			t.Errorf("ParseAddressRanges(%q) accepted invalid input", spec)
		}
	}
}

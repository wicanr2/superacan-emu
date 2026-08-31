package machine

import (
	"fmt"
	"strconv"
	"strings"
)

// Transaction is one complete 68000-visible bus operation. Width is in bytes.
type Transaction struct {
	Address uint32
	Width   uint8
	Write   bool
	Value   uint16
}

// AddressRange is an inclusive 24-bit bus filter.
type AddressRange struct {
	First uint32
	Last  uint32
}

func (r AddressRange) Contains(address uint32) bool {
	return address >= r.First && address <= r.Last
}

// ParseAddressRanges accepts comma-separated hexadecimal addresses or
// inclusive ranges, for example "e80000-e9001f,f00000-f5ffff".
func ParseAddressRanges(spec string) ([]AddressRange, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	parts := strings.Split(spec, ",")
	ranges := make([]AddressRange, 0, len(parts))
	for _, part := range parts {
		bounds := strings.Split(strings.TrimSpace(part), "-")
		if len(bounds) > 2 || bounds[0] == "" {
			return nil, fmt.Errorf("machine: invalid address range %q", part)
		}
		first, err := parseAddress(bounds[0])
		if err != nil {
			return nil, fmt.Errorf("machine: invalid address range %q: %w", part, err)
		}
		last := first
		if len(bounds) == 2 {
			if bounds[1] == "" {
				return nil, fmt.Errorf("machine: invalid address range %q", part)
			}
			last, err = parseAddress(bounds[1])
			if err != nil {
				return nil, fmt.Errorf("machine: invalid address range %q: %w", part, err)
			}
		}
		if first > last {
			return nil, fmt.Errorf("machine: descending address range %q", part)
		}
		ranges = append(ranges, AddressRange{First: first, Last: last})
	}
	return ranges, nil
}

func parseAddress(value string) (uint32, error) {
	value = strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "$")
	parsed, err := strconv.ParseUint(value, 16, 24)
	return uint32(parsed), err
}

// TransactionTrace retains at most Limit matching transactions while still
// counting later matches. A zero Limit disables retention but keeps counting.
type TransactionTrace struct {
	Ranges  []AddressRange
	Limit   uint64
	Matched uint64
	Records []Transaction
}

func (t *TransactionTrace) Observe(transaction Transaction) {
	matched := len(t.Ranges) == 0
	for _, addressRange := range t.Ranges {
		if addressRange.Contains(transaction.Address) {
			matched = true
			break
		}
	}
	if !matched {
		return
	}
	t.Matched++
	if uint64(len(t.Records)) < t.Limit {
		t.Records = append(t.Records, transaction)
	}
}

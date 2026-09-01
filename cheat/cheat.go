// Package cheat 是金手指的資料與搜尋引擎。
//
// 它是除錯／作弊工具，不是硬體行為：可寫範圍只有 Work RAM，寫入只發生在 frame
// 邊界，而且啟用期間的指令數、畫面雜湊與音訊雜湊都不得作為硬體證據或回歸基準。
// 這些界線由 session 與 ui 一起維持，本套件只負責「哪些位址、什麼值」。
package cheat

import (
	"encoding/binary"
	"fmt"
)

// Work RAM 的範圍。搜尋與寫入都只在這 64 KiB 之內。
const (
	WorkRAMBase = 0xfc0000
	WorkRAMSize = 0x10000
)

// MaxCandidates 是搜尋候選的上限，與 Bcan 相同。超過時只回前 4096 筆並回報截斷，
// 不靜默丟掉——使用者要知道自己看到的不是全部。
const MaxCandidates = 4096

// MaxEntries 是清單上限，與 Bcan 相同。
const MaxEntries = 1024

// Format 是數值的顯示與解讀方式。
type Format uint8

const (
	FormatDecimal Format = iota
	FormatHex
	FormatBCD
)

// String 是寫進檔案的名稱。
func (f Format) String() string {
	switch f {
	case FormatHex:
		return "hex"
	case FormatBCD:
		return "bcd"
	default:
		return "dec"
	}
}

// ParseFormat 讀檔案裡的格式名稱。
func ParseFormat(text string) (Format, error) {
	switch text {
	case "dec", "decimal", "Normal decimal":
		return FormatDecimal, nil
	case "hex", "hexadecimal":
		return FormatHex, nil
	case "bcd", "BCD":
		return FormatBCD, nil
	default:
		return FormatDecimal, fmt.Errorf("cheat: 未知的格式 %q", text)
	}
}

// Entry 是清單裡的一筆。
type Entry struct {
	Name    string
	Address uint32
	Width   uint8
	Value   uint32
	Format  Format
	Locked  bool
}

// Valid 回報這一筆是否落在 Work RAM 內且寬度合法。越界的項目不寫入也不儲存。
func (e Entry) Valid() bool {
	if e.Width != 8 && e.Width != 16 && e.Width != 32 {
		return false
	}
	bytes := uint64(e.Width) / 8
	last := uint64(e.Address) + bytes - 1
	return uint64(e.Address) >= WorkRAMBase && last <= WorkRAMBase+WorkRAMSize-1
}

// Compare 是搜尋條件。
type Compare uint8

const (
	// CompareEqual 等於指定值。
	CompareEqual Compare = iota
	// CompareNotEqual 不等於指定值。
	CompareNotEqual
	// CompareGreater 大於前一次快照的值。
	CompareGreater
	// CompareLess 小於前一次快照的值。
	CompareLess
	// CompareChanged 與前一次快照不同。
	CompareChanged
	// CompareUnchanged 與前一次快照相同。
	CompareUnchanged
)

// NeedsValue 回報這個條件要不要使用者輸入數值。
func (c Compare) NeedsValue() bool { return c == CompareEqual || c == CompareNotEqual }

// NeedsPrevious 回報這個條件要不要前一次的快照。
func (c Compare) NeedsPrevious() bool {
	return c == CompareGreater || c == CompareLess || c == CompareChanged || c == CompareUnchanged
}

// Search 是一次搜尋工作階段。快照一律在 frame 邊界取，兩次快照之間的比較才有定義。
type Search struct {
	Width      uint8
	candidates []uint32
	previous   []byte
	refines    int
	truncated  bool
	started    bool
}

// Reset 清掉整個搜尋。
func (s *Search) Reset() {
	s.candidates, s.previous = nil, nil
	s.refines, s.truncated, s.started = 0, false, false
}

// Started 回報是否已經做過第一次搜尋。
func (s *Search) Started() bool { return s.started }

// Refines 是已經縮小的次數。
func (s *Search) Refines() int { return s.refines }

// Truncated 回報候選是否超過上限而被截斷。
func (s *Search) Truncated() bool { return s.truncated }

// Candidates 回傳目前的候選位址。
func (s *Search) Candidates() []uint32 { return s.candidates }

// ReadValue 從快照讀出一個值。快照是 Work RAM 的完整副本，位址以 WorkRAMBase 為基準。
func ReadValue(snapshot []byte, address uint32, width uint8) (uint32, bool) {
	offset := int(address - WorkRAMBase)
	size := int(width) / 8
	if offset < 0 || offset+size > len(snapshot) {
		return 0, false
	}
	switch width {
	case 8:
		return uint32(snapshot[offset]), true
	case 16:
		return uint32(binary.BigEndian.Uint16(snapshot[offset:])), true
	case 32:
		return binary.BigEndian.Uint32(snapshot[offset:]), true
	}
	return 0, false
}

// New 開始一次新搜尋。掃描整個 Work RAM，依 width 對齊。
func (s *Search) New(snapshot []byte, compare Compare, value uint32) {
	if s.Width == 0 {
		s.Width = 16
	}
	step := int(s.Width) / 8
	s.candidates = s.candidates[:0]
	s.truncated = false
	total := 0
	for offset := 0; offset+step <= len(snapshot); offset += step {
		address := WorkRAMBase + uint32(offset)
		current, ok := ReadValue(snapshot, address, s.Width)
		if !ok {
			continue
		}
		if !matches(compare, current, current, value) {
			continue
		}
		total++
		if len(s.candidates) < MaxCandidates {
			s.candidates = append(s.candidates, address)
		}
	}
	s.truncated = total > MaxCandidates
	s.previous = append(s.previous[:0], snapshot...)
	s.refines = 0
	s.started = true
}

// Refine 以新的快照縮小候選。沒有先做過 New 時不做任何事。
func (s *Search) Refine(snapshot []byte, compare Compare, value uint32) {
	if !s.started {
		return
	}
	kept := s.candidates[:0]
	for _, address := range s.candidates {
		current, ok := ReadValue(snapshot, address, s.Width)
		if !ok {
			continue
		}
		previous, hadPrevious := ReadValue(s.previous, address, s.Width)
		if !hadPrevious {
			previous = current
		}
		if matches(compare, current, previous, value) {
			kept = append(kept, address)
		}
	}
	s.candidates = kept
	s.truncated = false
	s.previous = append(s.previous[:0], snapshot...)
	s.refines++
}

func matches(compare Compare, current, previous, value uint32) bool {
	switch compare {
	case CompareEqual:
		return current == value
	case CompareNotEqual:
		return current != value
	case CompareGreater:
		return current > previous
	case CompareLess:
		return current < previous
	case CompareChanged:
		return current != previous
	case CompareUnchanged:
		return current == previous
	}
	return false
}

// Previous 回傳上一次快照裡的值，供畫面顯示「前次」。
func (s *Search) Previous(address uint32) (uint32, bool) {
	return ReadValue(s.previous, address, s.Width)
}

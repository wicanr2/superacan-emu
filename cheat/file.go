package cheat

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// 檔案標頭。ACANCHT1 是本專案自有的格式，BCAN_CHT_1 是 Bcan 0.0.8b 的。
const (
	acanHeader = "ACANCHT1"
	bcanHeader = "BCAN_CHT_1"
)

// Warning 是讀檔時跳過的一行。壞掉的一行不會讓整個檔案作廢，但一定要回報，
// 靜默跳過等於把使用者的資料弄丟又不告訴他。
type Warning struct {
	Line   int
	Detail string
}

func (w Warning) String() string { return fmt.Sprintf("第 %d 行：%s", w.Line, w.Detail) }

// Write 以 ACANCHT1 寫出清單。欄位以 tab 分隔，順序固定：
// 名稱、位址、寬度、值、格式、是否鎖定。
func Write(output io.Writer, entries []Entry) error {
	if _, err := fmt.Fprintf(output, "%s\n", acanHeader); err != nil {
		return err
	}
	for _, entry := range entries {
		locked := 0
		if entry.Locked {
			locked = 1
		}
		if _, err := fmt.Fprintf(output, "%s\t$%06X\t%d\t%d\t%s\t%d\n",
			entry.Name, entry.Address, entry.Width, entry.Value, entry.Format, locked); err != nil {
			return err
		}
	}
	return nil
}

// Read 讀 ACANCHT1。
func Read(input io.Reader) ([]Entry, []Warning, error) {
	return readTabular(input, acanHeader, true)
}

// ReadBcan 讀 Bcan 的 `.cht`。
//
// 證據等級：**hypothesis**。從 Bcan.exe 只能確定檔案開頭是註解行
// 「; Bcan per-game cheat file」與標頭「BCAN_CHT_1」，以及欄位清單
// （Name／Address／Width／Value／Format）；逐欄的順序沒有在字串表裡出現，
// 這裡採「名稱、位址、寬度、值、格式」的順序。第一欄看起來像位址時會自動改用
// 「位址優先」的順序，兩種都不成立的行以 Warning 回報並跳過，不猜。
func ReadBcan(input io.Reader) ([]Entry, []Warning, error) {
	return readTabular(input, bcanHeader, false)
}

func readTabular(input io.Reader, header string, hasLocked bool) ([]Entry, []Warning, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var entries []Entry
	var warnings []Warning
	line := 0
	seenHeader := false

	for scanner.Scan() {
		line++
		text := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !seenHeader {
			if trimmed != header {
				return nil, nil, fmt.Errorf("cheat: 檔案標頭是 %q，不是 %q", trimmed, header)
			}
			seenHeader = true
			continue
		}
		entry, err := parseRow(text, hasLocked)
		if err != nil {
			warnings = append(warnings, Warning{Line: line, Detail: err.Error()})
			continue
		}
		if len(entries) >= MaxEntries {
			warnings = append(warnings, Warning{Line: line,
				Detail: fmt.Sprintf("超過清單上限 %d 筆，其餘略過", MaxEntries)})
			break
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, warnings, err
	}
	if !seenHeader {
		return nil, warnings, fmt.Errorf("cheat: 檔案沒有 %q 標頭", header)
	}
	return entries, warnings, nil
}

func parseRow(text string, hasLocked bool) (Entry, error) {
	fields := strings.Split(text, "\t")
	if len(fields) < 5 {
		return Entry{}, fmt.Errorf("欄位只有 %d 個，至少要 5 個", len(fields))
	}
	// 第一欄看起來像位址時改用「位址優先」的順序。
	name, rest := fields[0], fields[1:]
	if _, err := parseAddress(fields[0]); err == nil {
		name, rest = fields[1], append([]string{fields[0]}, fields[2:]...)
	}
	address, err := parseAddress(rest[0])
	if err != nil {
		return Entry{}, err
	}
	width, err := strconv.ParseUint(strings.TrimSpace(rest[1]), 10, 8)
	if err != nil || (width != 8 && width != 16 && width != 32) {
		return Entry{}, fmt.Errorf("寬度 %q 不是 8／16／32", rest[1])
	}
	value, err := strconv.ParseUint(strings.TrimSpace(rest[2]), 10, 32)
	if err != nil {
		return Entry{}, fmt.Errorf("值 %q 不是十進位整數", rest[2])
	}
	format, err := ParseFormat(strings.TrimSpace(rest[3]))
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{
		Name: name, Address: address, Width: uint8(width),
		Value: uint32(value), Format: format,
	}
	if hasLocked && len(rest) > 4 {
		entry.Locked = strings.TrimSpace(rest[4]) == "1"
	}
	if !entry.Valid() {
		return Entry{}, fmt.Errorf("$%06X 寬度 %d 不在 Work RAM 範圍內", entry.Address, entry.Width)
	}
	return entry, nil
}

func parseAddress(text string) (uint32, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "$")
	text = strings.TrimPrefix(text, "0x")
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("位址 %q 不是十六進位", text)
	}
	if value < WorkRAMBase || value > WorkRAMBase+WorkRAMSize-1 {
		return 0, fmt.Errorf("位址 $%X 不在 Work RAM 範圍內", value)
	}
	return uint32(value), nil
}

package session

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/media"
	"github.com/wicanr2/superacan-emu/ui"
)

// Library 掃描卡帶目錄並回答瀏覽器要顯示的事實。掃描結果會快取，
// 因為每一筆都要讀整個檔案算雜湊，逐幀重掃會讓介面停頓。
type Library struct {
	dirs      []string
	recent    []string
	stateRoot string
	saveDir   string

	scanned    bool
	cartridges []ui.CartridgeEntry
}

// NewLibrary 建立卡帶庫。stateRoot 是存檔槽的根目錄，saveDir 是卡帶電池記憶體
// 的目錄；兩者只用來回報「這個卡帶有哪些存檔」，不會在掃描時寫入任何東西。
func NewLibrary(dirs, recent []string, stateRoot, saveDir string) *Library {
	return &Library{dirs: dirs, recent: recent, stateRoot: stateRoot, saveDir: saveDir}
}

// Directory 回報第一個掃描目錄，供畫面標題顯示。
func (l *Library) Directory() string {
	if len(l.dirs) == 0 {
		return ""
	}
	return l.dirs[0]
}

// Rescan 丟掉快取，下一次查詢會重新掃描。
func (l *Library) Rescan() { l.scanned = false }

// Cartridges 回傳所有掃描到的卡帶，依檔名排序。
func (l *Library) Cartridges() []ui.CartridgeEntry {
	if l.scanned {
		return l.cartridges
	}
	var entries []ui.CartridgeEntry
	seen := make(map[string]bool)
	for _, dir := range l.dirs {
		listing, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, item := range listing {
			if item.IsDir() {
				continue
			}
			extension := strings.ToLower(filepath.Ext(item.Name()))
			if extension != ".bin" && extension != ".zip" {
				continue
			}
			path := filepath.Join(dir, item.Name())
			if seen[path] {
				continue
			}
			seen[path] = true
			entries = append(entries, l.describe(path))
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	l.cartridges, l.scanned = entries, true
	return entries
}

// Recent 回傳最近開啟的卡帶。清單只存路徑，開啟前重新驗算雜湊，
// 檔案不在了就標成缺少並停用。
func (l *Library) Recent() []ui.CartridgeEntry {
	entries := make([]ui.CartridgeEntry, 0, len(l.recent))
	for _, path := range l.recent {
		entries = append(entries, l.describe(path))
	}
	return entries
}

func (l *Library) describe(path string) ui.CartridgeEntry {
	entry := ui.CartridgeEntry{
		Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Path: path,
		Kind: strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		entry.Missing = true
		return entry
	}
	entry.Size = int64(len(raw))
	if entry.Kind == "zip" {
		entry.Parts = zipParts(raw)
	}
	// 雜湊算的是解碼後的卡帶影像：雙部分 ZIP 接合之後才是機器看到的內容。
	image, err := media.DecodeCartridge(path, raw)
	if err != nil {
		entry.Missing = true
		return entry
	}
	entry.SHA256 = sha256.Sum256(image.Bytes)
	_, entry.Verified = verifiedCartridges[hex.EncodeToString(entry.SHA256[:])]
	entry.SaveSlots = l.slotsFor(path)
	entry.Battery = l.batteryFor(path)
	return entry
}

func zipParts(raw []byte) []ui.CartridgePart {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil
	}
	parts := make([]ui.CartridgePart, 0, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		parts = append(parts, ui.CartridgePart{Name: file.Name, Size: int64(file.UncompressedSize64)})
	}
	return parts
}

func (l *Library) slotsFor(romPath string) []int {
	if l.stateRoot == "" {
		return nil
	}
	dir := StateDirFor(l.stateRoot, romPath)
	var slots []int
	for slot := 0; slot < ui.SlotCount; slot++ {
		if _, err := os.Stat(filepath.Join(dir, SlotFileName(slot))); err == nil {
			slots = append(slots, slot)
		}
	}
	return slots
}

func (l *Library) batteryFor(romPath string) int64 {
	if l.saveDir == "" {
		return 0
	}
	stat, err := os.Stat(BatteryPathFor(l.saveDir, romPath))
	if err != nil {
		return 0
	}
	return stat.Size()
}

// BatteryPathFor 是「每個卡帶一個電池檔」的預設規則。
func BatteryPathFor(dir, romPath string) string {
	name := strings.TrimSuffix(filepath.Base(romPath), filepath.Ext(romPath))
	return filepath.Join(dir, name+".sav")
}

// DescribeFirmware 讀一份韌體檔案並回報它在畫面上的樣子。讀不到就是未設定，
// 不是錯誤：使用者本來就可能還沒指定。
func DescribeFirmware(kind ui.FirmwareKind, path string) ui.FirmwareEntry {
	entry := ui.FirmwareEntry{Kind: kind, Path: path}
	if path == "" {
		return entry
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return entry
	}
	entry.Size = int64(len(raw))
	entry.SHA256 = sha256.Sum256(raw)
	entry.Loaded = true
	_, entry.Known = verifiedFirmware[hex.EncodeToString(entry.SHA256[:])]
	// IPL 在餵給 machine 之前會做 word swap，但畫面上要顯示的是檔案本身的雜湊，
	// 使用者拿去和自己的檔案對照時才對得起來。
	if kind == ui.FirmwareIPL && entry.Size != machine.IPLSize {
		entry.Loaded = entry.Size > 0
	}
	return entry
}

// FirmwareSet 是四份韌體，實作 ui.FirmwareSource。
type FirmwareSet [ui.FirmwareCount]ui.FirmwareEntry

// FirmwareEntries 讓 FirmwareSet 滿足 ui.FirmwareSource。
func (f FirmwareSet) FirmwareEntries() []ui.FirmwareEntry { return f[:] }

// DescribeFirmwareSet 一次描述四份韌體。
func DescribeFirmwareSet(ipl, key, soundA, soundB string) FirmwareSet {
	return FirmwareSet{
		DescribeFirmware(ui.FirmwareIPL, ipl),
		DescribeFirmware(ui.FirmwareKey, key),
		DescribeFirmware(ui.FirmwareSoundA, soundA),
		DescribeFirmware(ui.FirmwareSoundB, soundB),
	}
}

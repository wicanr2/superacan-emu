package ui

// FirmwareKind 是四份主機韌體。它們由使用者自行提供，本程式不散布。
type FirmwareKind uint8

const (
	FirmwareIPL FirmwareKind = iota
	FirmwareKey
	FirmwareSoundA
	FirmwareSoundB
)

// FirmwareCount 是韌體項目數。
const FirmwareCount = 4

// FirmwareEntry 是一份韌體檔案的現況。
//
// Known 只表示雜湊與 `docs/verify-rom-matrix.md` 記錄的固定輸入相同；不相同時
// 顯示「未列於已驗證清單」而不是錯誤——本專案沒有立場宣稱其他版本的韌體不合法。
type FirmwareEntry struct {
	Kind   FirmwareKind
	Path   string
	Size   int64
	SHA256 [32]byte
	Loaded bool
	Known  bool
}

// CartridgePart 是雙部分卡帶 ZIP 裡的一個成員。
type CartridgePart struct {
	Name string
	Size int64
}

// CartridgeEntry 是卡帶瀏覽器裡的一筆。
//
// 沒有畫面預覽欄位是刻意的：本程式不散布遊戲畫面，瀏覽器也不顯示。
type CartridgeEntry struct {
	Name      string
	Path      string
	Size      int64
	SHA256    [32]byte
	Kind      string
	Parts     []CartridgePart
	Missing   bool
	Verified  bool
	Battery   int64
	SaveSlots []int
}

// Library 是卡帶與最近清單的來源。ui 不做檔案系統存取，資料由入口提供。
type Library interface {
	Directory() string
	Cartridges() []CartridgeEntry
	Recent() []CartridgeEntry
}

// FirmwareSource 提供四份韌體的現況。
type FirmwareSource interface {
	FirmwareEntries() []FirmwareEntry
}

// Dependency 是一個第三方模組。
type Dependency struct {
	Path    string
	Version string
	License string
}

// AboutInfo 是關於畫面要顯示的事實。全部由入口提供，ui 不查詢執行環境。
type AboutInfo struct {
	Version      string
	BuildDate    string
	GoVersion    string
	Platform     string
	CGOEnabled   bool
	Dependencies []Dependency
}

// firmwareLabel 是韌體項目的顯示名稱。
func (u *UI) firmwareLabel(kind FirmwareKind) string {
	switch kind {
	case FirmwareIPL:
		return u.s.FirmwareIPL
	case FirmwareKey:
		return u.s.FirmwareKey
	case FirmwareSoundA:
		return u.s.FirmwareSoundA
	default:
		return u.s.FirmwareSoundB
	}
}

// shortHash 取雜湊的頭尾，中間省略。畫面上要能一眼對出是不是同一份檔案，
// 完整雜湊在診斷畫面才需要。
func shortHash(sum [32]byte) string {
	const hexDigits = "0123456789abcdef"
	if sum == ([32]byte{}) {
		return "—"
	}
	var head, tail []byte
	for i := 0; i < 3; i++ {
		head = append(head, hexDigits[sum[i]>>4], hexDigits[sum[i]&0xf])
	}
	for i := 29; i < 32; i++ {
		tail = append(tail, hexDigits[sum[i]>>4], hexDigits[sum[i]&0xf])
	}
	return string(head) + "…" + string(tail)
}

// groupInt 以三位一撇顯示大小。
func groupInt(value int64) string {
	if value < 0 {
		return "-" + groupInt(-value)
	}
	return group(uint64(value))
}

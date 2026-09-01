package ui

// Event 是前端唯一要產生的東西。前端負責把 X11 keysym、ebiten.Key 或 Android 的
// KeyEvent 翻成這些型別，ui 不認識任何一種平台代碼。
type Event interface{ isEvent() }

// Direction 是導覽方向。
type Direction uint8

const (
	DirUp Direction = iota
	DirDown
	DirLeft
	DirRight
)

// EdgeTarget 是跳到清單端點的目標。
type EdgeTarget uint8

const (
	EdgeHome EdgeTarget = iota
	EdgeEnd
)

// ActionKind 是不帶方向的動作。
type ActionKind uint8

const (
	ActConfirm ActionKind = iota
	ActCancel
	ActMenu
	ActSecondary
	ActDelete
	ActTabPrev
	ActTabNext
)

// EditKind 是文字編輯動作。
type EditKind uint8

const (
	EditBackspace EditKind = iota
	EditCommit
	EditAbort
)

// PointerPhase 是指標或觸控的階段。
type PointerPhase uint8

const (
	PhaseDown PointerPhase = iota
	PhaseMove
	PhaseUp
	PhaseCancel
)

// LifeKind 是系統層的生命週期事件。
type LifeKind uint8

const (
	LifeBack LifeKind = iota
	LifeFocusLost
	LifeFocusGained
	LifeSuspend
	LifeResume
)

// Profile 決定用哪一套度量。
type Profile uint8

const (
	// ProfileCompact 給桌面的鍵鼠與手把。
	ProfileCompact Profile = iota
	// ProfileTouch 給 Android 的手指操作。
	ProfileTouch
)

type (
	// Nav 是方向鍵導覽。Repeat 為 true 表示這是長按產生的重複。
	Nav struct {
		Dir    Direction
		Repeat bool
	}
	// Page 是翻頁，Delta 以頁為單位。
	Page struct{ Delta int }
	// Edge 是跳到清單頭尾。
	Edge struct{ To EdgeTarget }
	// Action 是確認、取消一類的動作。
	Action struct{ Kind ActionKind }
	// Text 是一個輸入字元。
	Text struct{ R rune }
	// Edit 是編輯中的控制動作。
	Edit struct{ Kind EditKind }
	// Pointer 是滑鼠或單一觸控點。座標是表面像素。
	Pointer struct {
		ID    int
		X, Y  int
		Phase PointerPhase
	}
	// LongPress 等同桌面的右鍵。
	LongPress struct {
		ID   int
		X, Y int
	}
	// Wheel 是滾輪，DY 向下為正。
	Wheel struct{ DY int }
	// RawKey 只在綁定畫面接受。Frontend 會原封不動存進設定檔：同一個實體鍵在
	// X11 keysym 與 ebiten.Key 底下是不同數值，設定檔必須記得這組綁定是哪個
	// 前端寫的，否則跨前端讀入會得到錯誤的按鍵。
	RawKey struct {
		Frontend string
		Code     uint32
		Label    string
	}
	// RawPad 是手把按鍵的原始編號，規則同 RawKey。
	RawPad struct {
		Index  int
		Button int
		Label  string
	}
	// Surface 告訴 ui 目前的表面大小、縮放與版面設定檔。
	Surface struct {
		W, H    int
		Scale   int
		Profile Profile
	}
	// Life 是系統層事件。
	Life struct{ Kind LifeKind }
)

func (Nav) isEvent()       {}
func (Page) isEvent()      {}
func (Edge) isEvent()      {}
func (Action) isEvent()    {}
func (Text) isEvent()      {}
func (Edit) isEvent()      {}
func (Pointer) isEvent()   {}
func (LongPress) isEvent() {}
func (Wheel) isEvent()     {}
func (RawKey) isEvent()    {}
func (RawPad) isEvent()    {}
func (Surface) isEvent()   {}
func (Life) isEvent()      {}

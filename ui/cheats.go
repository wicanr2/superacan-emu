package ui

import (
	"fmt"
	"strconv"
)

// CheatCandidate 是搜尋結果的一筆。
type CheatCandidate struct {
	Address  uint32
	Value    uint32
	Previous uint32
}

// CheatEntry 是清單裡的一筆。
type CheatEntry struct {
	Name    string
	Address uint32
	Width   uint8
	Value   uint32
	Format  string
	Locked  bool
}

// CheatState 是介面要顯示的金手指現況，由入口提供。
type CheatState struct {
	Enabled    bool
	LockAll    bool
	Entries    []CheatEntry
	Candidates []CheatCandidate
	Total      int
	Truncated  bool
	Refines    int
	Started    bool
	// Wrote 記錄本工作階段是否曾經寫入 Work RAM。一旦為真就不會變回假：
	// 該工作階段的雜湊已經不能作為硬體證據了。
	Wrote bool
}

// CheatSource 由入口提供。
type CheatSource interface{ Cheats() CheatState }

// CheatCommand 是金手指要求入口做的事。
type CheatCommand uint8

const (
	CheatNewSearch CheatCommand = iota
	CheatRefine
	CheatClearSearch
	CheatAdd
	CheatAddLocked
	CheatToggleLock
	CheatRemove
	CheatSetEnabled
	CheatSetLockAll
	CheatImport
	CheatExport
)

// Cheat 是金手指的 Intent。它只描述意圖，實際的記憶體寫入由入口在 frame 邊界做，
// 而且只走 PokeWorkRAM 那條有範圍檢查的通道。
type Cheat struct {
	Command CheatCommand
	Width   uint8
	Compare uint8
	Value   uint32
	Address uint32
	Index   int
	Flag    bool
	Path    string
}

func (Cheat) isIntent() {}

var (
	cheatWidths    = []string{"8-bit", "16-bit", "32-bit"}
	cheatWidthBits = []uint8{8, 16, 32}
	cheatCompares  = []string{"等於", "不等於", "大於前值", "小於前值", "變動", "未變"}
)

// cheatSearchScreen 是 S6.1。
type cheatSearchScreen struct {
	focus   int
	width   int
	compare int
	value   string
	editing bool
	list    int
}

func (s *cheatSearchScreen) id() string { return "S6.1" }

func (s *cheatSearchScreen) rows(u *UI) []optionRow {
	return []optionRow{
		{kind: optionReadOnly, label: u.s.CheatRange, text: u.s.CheatRangeValue},
		{kind: optionChoice, label: u.s.CheatWidth, choices: cheatWidths, index: &s.width},
		{kind: optionChoice, label: u.s.CheatCompare, choices: cheatCompares, index: &s.compare},
		{kind: optionReadOnly, label: u.s.CheatValue, text: s.valueDisplay(u.s)},
	}
}

func (s *cheatSearchScreen) valueDisplay(strings Strings) string {
	if s.editing {
		return s.value + "▏"
	}
	if s.value == "" {
		return strings.None
	}
	return s.value
}

func (s *cheatSearchScreen) parsedValue() uint32 {
	value, err := strconv.ParseUint(s.value, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(value)
}

func (s *cheatSearchScreen) command(u *UI, command CheatCommand) {
	u.emit(Cheat{
		Command: command,
		Width:   cheatWidthBits[s.width],
		Compare: uint8(s.compare),
		Value:   s.parsedValue(),
	})
}

func (s *cheatSearchScreen) handle(u *UI, ev Event) bool {
	rows := s.rows(u)
	if s.editing {
		switch e := ev.(type) {
		case Text:
			if e.R >= '0' && e.R <= '9' && len(s.value) < 10 {
				s.value += string(e.R)
			}
			return true
		case Edit:
			switch e.Kind {
			case EditBackspace:
				if len(s.value) > 0 {
					s.value = s.value[:len(s.value)-1]
				}
			case EditCommit, EditAbort:
				s.editing = false
			}
			return true
		case Action:
			if e.Kind == ActConfirm || e.Kind == ActCancel {
				s.editing = false
			}
			return true
		}
		return true
	}
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			s.focus = moveFocus(s.focus, len(rows)+3, -1)
			return true
		case DirDown:
			s.focus = moveFocus(s.focus, len(rows)+3, +1)
			return true
		case DirLeft, DirRight:
			if s.focus < len(rows) {
				delta := 1
				if e.Dir == DirLeft {
					delta = -1
				}
				rows[s.focus].adjust(delta)
			}
			return true
		}
	case Action:
		switch e.Kind {
		case ActConfirm:
			switch {
			case s.focus == 3:
				s.editing = true
			case s.focus == len(rows):
				s.command(u, CheatNewSearch)
			case s.focus == len(rows)+1:
				s.command(u, CheatRefine)
			case s.focus == len(rows)+2:
				s.command(u, CheatClearSearch)
			}
			return true
		case ActSecondary:
			// 對目前選到的候選加入清單並鎖定。
			state := u.cheats()
			if s.list < len(state.Candidates) {
				u.emit(Cheat{
					Command: CheatAddLocked,
					Address: state.Candidates[s.list].Address,
					Width:   cheatWidthBits[s.width],
					Value:   state.Candidates[s.list].Value,
				})
			}
			return true
		case ActTabNext, ActTabPrev:
			u.pop()
			u.push(&cheatListScreen{})
			return true
		case ActCancel:
			u.pop()
			return true
		}
	case Page:
		state := u.cheats()
		if len(state.Candidates) > 0 {
			s.list = moveFocus(s.list, len(state.Candidates), e.Delta)
		}
		return true
	}
	return false
}

func (s *cheatSearchScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	state := u.cheats()
	// 標題列不再重複標示：常駐的 CHEAT 標記已經在同一個角落，
	// 兩個講同一件事只會互相蓋掉。
	top, _ := page{title: u.s.CheatTitle, back: true, status: u.s.CheatTruncNote}.draw(u, c)

	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	rows := s.rows(u)
	y := drawOptionRows(u, c, x, top, width, rows, s.focus)
	y += m.Grid

	buttons := []string{u.s.CheatNewSearch, u.s.CheatRefine, u.s.CheatClear}
	buttonW := width / 4
	for index, label := range buttons {
		u.drawButton(c, x+index*(buttonW+m.Grid), y, buttonW, m.RowHeight, label,
			s.focus == len(rows)+index)
	}
	y += m.RowHeight + m.SectionGap

	summary := fmt.Sprintf(u.s.CheatCandidates, state.Total, len(state.Candidates))
	if state.Started {
		summary += fmt.Sprintf(u.s.CheatRefines, state.Refines)
	}
	c.rowText(x, y, m.RowHeight, m.SmallSize, u.theme.TextDim, summary)
	y += m.RowHeight

	for index, candidate := range state.Candidates {
		if y+m.RowHeight > c.height()-m.FooterBar*2 {
			break
		}
		colour := u.focusRow(c, x, y, width, index == s.list)
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.SmallSize, colour,
			fmt.Sprintf("$%06X", candidate.Address))
		c.rowText(x+width/4, y, m.RowHeight, m.SmallSize, colour, fmt.Sprintf("%d", candidate.Value))
		c.rowText(x+width/2, y, m.RowHeight, m.SmallSize, colour,
			fmt.Sprintf(u.s.CheatPrevious, candidate.Previous))
		y += m.RowHeight
	}
}

// cheatListScreen 是 S6.2。
type cheatListScreen struct{ focus int }

func (s *cheatListScreen) id() string { return "S6.2" }

func (s *cheatListScreen) handle(u *UI, ev Event) bool {
	state := u.cheats()
	switch e := ev.(type) {
	case Nav:
		switch e.Dir {
		case DirUp:
			s.focus = moveFocus(s.focus, len(state.Entries), -1)
		case DirDown:
			s.focus = moveFocus(s.focus, len(state.Entries), +1)
		}
		return true
	case Action:
		switch e.Kind {
		case ActConfirm:
			if s.focus < len(state.Entries) {
				u.emit(Cheat{Command: CheatToggleLock, Index: s.focus})
				u.toast(u.s.CheatEvidenceWarning, SeverityWarn)
			}
			return true
		case ActDelete:
			if s.focus < len(state.Entries) {
				u.emit(Cheat{Command: CheatRemove, Index: s.focus})
			}
			return true
		case ActSecondary:
			u.emit(Cheat{Command: CheatSetEnabled, Flag: !state.Enabled})
			return true
		case ActTabNext, ActTabPrev:
			u.pop()
			u.push(&cheatSearchScreen{width: 1})
			return true
		case ActCancel:
			u.pop()
			return true
		}
	}
	return false
}

func (s *cheatListScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	state := u.cheats()
	top, _ := page{title: u.s.CheatTitle, back: true, status: u.s.CheatLockNote}.draw(u, c)

	x := m.PanelPad
	width := c.width() - m.PanelPad*2
	enabled := "[ ] " + u.s.CheatEnable
	if state.Enabled {
		enabled = "[■] " + u.s.CheatEnable
	}
	c.rowText(x, top, m.RowHeight, m.BodySize, u.theme.Text, enabled)
	c.rowText(x+width/3, top, m.RowHeight, m.SmallSize, u.theme.TextDim, u.s.CheatEnableHint)
	y := top + m.RowHeight + m.Grid
	c.rect(x, y, width, 1, u.theme.Border)
	y += 1 + m.Grid

	for _, column := range []struct {
		offset int
		label  string
	}{
		{m.RowPadX, u.s.CheatColumnLock}, {width / 8, u.s.CheatColumnName},
		{width / 2, u.s.CheatColumnAddress}, {width * 5 / 8, u.s.CheatColumnWidth},
		{width * 3 / 4, u.s.CheatColumnValue}, {width * 7 / 8, u.s.CheatColumnFormat},
	} {
		c.rowText(x+column.offset, y, m.RowHeight, m.SmallSize, u.theme.TextDim, column.label)
	}
	y += m.RowHeight

	if len(state.Entries) == 0 {
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, u.theme.TextOff, u.s.CheatEmpty)
		return
	}
	for index, entry := range state.Entries {
		colour := u.focusRow(c, x, y, width, index == s.focus)
		lock := "○"
		if entry.Locked {
			lock = "●"
		}
		c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, colour, lock)
		c.rowText(x+width/8, y, m.RowHeight, m.BodySize, colour, entry.Name)
		c.rowText(x+width/2, y, m.RowHeight, m.SmallSize, colour, fmt.Sprintf("$%06X", entry.Address))
		c.rowText(x+width*5/8, y, m.RowHeight, m.SmallSize, colour, fmt.Sprintf("%d", entry.Width))
		c.rowText(x+width*3/4, y, m.RowHeight, m.SmallSize, colour, fmt.Sprintf("%d", entry.Value))
		c.rowText(x+width*7/8, y, m.RowHeight, m.SmallSize, colour, entry.Format)
		y += m.RowHeight
	}
}

func (u *UI) cheats() CheatState {
	if u.cheat == nil {
		return CheatState{}
	}
	return u.cheat.Cheats()
}

// drawCheatMarker 在畫面角落畫常駐標記。它不受覆蓋層開關影響：
// 一旦這個工作階段寫過 Work RAM，畫面上就必須看得出來。
func (u *UI) drawCheatMarker(c *canvas) {
	state := u.cheats()
	if !state.Enabled && !state.Wrote {
		return
	}
	m := u.metrics
	label := u.s.CheatMarker
	width := c.font.Measure(label, m.SmallSize) + m.RowPadX*2
	c.rect(c.width()-width-m.Grid, m.Grid, width, m.RowHeight, u.theme.Warn)
	c.textCenter(c.width()-width-m.Grid, m.Grid+(m.RowHeight-c.font.Height(m.SmallSize))/2,
		width, m.SmallSize, u.theme.FocusText, label)
}

package session

import (
	"fmt"
	"os"

	"github.com/wicanr2/superacan-emu/cheat"
	"github.com/wicanr2/superacan-emu/ui"
)

// cheatState 是 session 這一側的金手指狀態。它不進 ACANGOS1：金手指是外部工具
// 狀態，不是機器狀態。
type cheatState struct {
	enabled bool
	lockAll bool
	entries []cheat.Entry
	search  cheat.Search
	wrote   bool
	pokes   uint64
}

// Cheats 讓介面顯示現況。
func (s *Session) Cheats() ui.CheatState {
	state := ui.CheatState{
		Enabled:   s.cheat.enabled,
		LockAll:   s.cheat.lockAll,
		Total:     len(s.cheat.search.Candidates()),
		Truncated: s.cheat.search.Truncated(),
		Refines:   s.cheat.search.Refines(),
		Started:   s.cheat.search.Started(),
		Wrote:     s.cheat.wrote,
	}
	for _, entry := range s.cheat.entries {
		state.Entries = append(state.Entries, ui.CheatEntry{
			Name: entry.Name, Address: entry.Address, Width: entry.Width,
			Value: entry.Value, Format: entry.Format.String(), Locked: entry.Locked,
		})
	}
	snapshot := s.workRAMSnapshot()
	candidates := s.cheat.search.Candidates()
	if len(candidates) > cheatDisplayLimit {
		candidates = candidates[:cheatDisplayLimit]
	}
	for _, address := range candidates {
		value, _ := cheat.ReadValue(snapshot, address, s.cheat.search.Width)
		previous, _ := s.cheat.search.Previous(address)
		state.Candidates = append(state.Candidates, ui.CheatCandidate{
			Address: address, Value: value, Previous: previous,
		})
	}
	return state
}

// cheatDisplayLimit 是畫面一次列出的候選數。搜尋本身的上限是 cheat.MaxCandidates，
// 這裡只是不要為了畫面去複製四千筆。
const cheatDisplayLimit = 64

// PokeCount 是本工作階段經由 UI 通道寫入 Work RAM 的次數。回歸測試用它斷言
// 「金手指關閉時這條通道沒有寫入」。
func (s *Session) PokeCount() uint64 { return s.cheat.pokes }

// workRAMSnapshot 在 frame 邊界取 Work RAM 的完整副本。兩次快照之間的比較才有定義，
// 所以搜尋一律用這個函式取值，不在指令中途讀。
func (s *Session) workRAMSnapshot() []byte {
	snapshot := make([]byte, cheat.WorkRAMSize)
	if s.System == nil {
		return snapshot
	}
	for offset := range snapshot {
		value, err := s.System.Bus.Read8(cheat.WorkRAMBase + uint32(offset))
		if err != nil {
			break
		}
		snapshot[offset] = value
	}
	return snapshot
}

// applyCheats 在 RunFrame 之前把鎖定項寫進 Work RAM。寫入只走 poke 那條有範圍
// 檢查的通道，而且只在 frame 邊界，不在指令中途插入。
func (s *Session) applyCheats() {
	if !s.cheat.enabled || s.System == nil {
		return
	}
	for _, entry := range s.cheat.entries {
		if !entry.Locked && !s.cheat.lockAll {
			continue
		}
		if !entry.Valid() {
			continue
		}
		if err := s.poke(ui.PokeWorkRAM{
			Addr: entry.Address, Width: entry.Width / 8, Value: entry.Value,
		}); err != nil {
			s.UI.Fail(err.Error())
			continue
		}
		s.cheat.wrote = true
	}
}

func (s *Session) applyCheat(command ui.Cheat) error {
	switch command.Command {
	case ui.CheatNewSearch:
		s.cheat.search.Width = command.Width
		s.cheat.search.New(s.workRAMSnapshot(), cheat.Compare(command.Compare), command.Value)
	case ui.CheatRefine:
		s.cheat.search.Refine(s.workRAMSnapshot(), cheat.Compare(command.Compare), command.Value)
	case ui.CheatClearSearch:
		s.cheat.search.Reset()
	case ui.CheatAdd, ui.CheatAddLocked:
		return s.addCheat(command)
	case ui.CheatToggleLock:
		if command.Index < len(s.cheat.entries) {
			s.cheat.entries[command.Index].Locked = !s.cheat.entries[command.Index].Locked
		}
	case ui.CheatRemove:
		if command.Index < len(s.cheat.entries) {
			s.cheat.entries = append(s.cheat.entries[:command.Index], s.cheat.entries[command.Index+1:]...)
		}
	case ui.CheatSetEnabled:
		s.cheat.enabled = command.Flag
	case ui.CheatSetLockAll:
		s.cheat.lockAll = command.Flag
	case ui.CheatImport:
		return s.importCheats(command.Path)
	case ui.CheatExport:
		return s.exportCheats(command.Path)
	}
	return nil
}

func (s *Session) addCheat(command ui.Cheat) error {
	if len(s.cheat.entries) >= cheat.MaxEntries {
		return fmt.Errorf("session: 金手指清單已達上限 %d 筆", cheat.MaxEntries)
	}
	entry := cheat.Entry{
		Name:    fmt.Sprintf("$%06X", command.Address),
		Address: command.Address,
		Width:   command.Width,
		Value:   command.Value,
		Locked:  command.Command == ui.CheatAddLocked,
	}
	if !entry.Valid() {
		return fmt.Errorf("session: $%06X 寬度 %d 不在 Work RAM 範圍內", entry.Address, entry.Width)
	}
	s.cheat.entries = append(s.cheat.entries, entry)
	return nil
}

func (s *Session) importCheats(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 先當成本專案的格式讀；標頭不符再試 Bcan 的。
	entries, warnings, err := cheat.Read(file)
	if err != nil {
		if _, seekErr := file.Seek(0, 0); seekErr != nil {
			return seekErr
		}
		entries, warnings, err = cheat.ReadBcan(file)
		if err != nil {
			return err
		}
	}
	s.cheat.entries = entries
	for _, warning := range warnings {
		s.UI.Fail(warning.String())
	}
	return nil
}

func (s *Session) exportCheats(path string) error {
	file, err := os.Create(path + ".tmp")
	if err != nil {
		return err
	}
	if err := cheat.Write(file, s.cheat.entries); err != nil {
		file.Close()
		os.Remove(path + ".tmp")
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(path + ".tmp")
		return err
	}
	return os.Rename(path+".tmp", path)
}

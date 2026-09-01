package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/ui"
)

// SlotFileName 是存檔槽的檔名規則。每個卡帶一個目錄，槽號決定檔名，
// 這樣使用者不必記得哪個檔案屬於哪一格。
func SlotFileName(slot int) string { return fmt.Sprintf("slot%d.acanstate", slot) }

func (s *Session) slotPath(slot int) string {
	return filepath.Join(s.StateDir, SlotFileName(slot))
}

// Slots 讀出十個槽的現況。壞檔不是錯誤：它要以「已拒絕」的樣子出現在畫面上，
// 使用者才知道那一格不能用，而不是按下去才發現。
func (s *Session) Slots() []ui.SlotInfo {
	slots := make([]ui.SlotInfo, ui.SlotCount)
	for slot := range slots {
		slots[slot] = s.slotInfo(slot)
	}
	return slots
}

func (s *Session) slotInfo(slot int) ui.SlotInfo {
	info := ui.SlotInfo{Index: slot}
	if s.StateDir == "" {
		return info
	}
	path := s.slotPath(slot)
	stat, err := os.Stat(path)
	if err != nil {
		return info
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		info.Rejected = true
		info.Reason = err.Error()
		return info
	}
	inspected := machine.InspectSaveState(raw, s.System.IPLSHA256, s.System.ROMSHA256)
	info.Stamp = s.stamp(stat)
	if !inspected.Valid {
		info.Rejected = true
		info.Reason = inspected.Reason
		return info
	}
	info.Present = true
	info.Frame = inspected.Frame
	info.Thumb = thumbnail(inspected.Framebuffer)
	return info
}

// stamp 是槽位上顯示的時間。可由 Session.Stamp 換掉，讓畫面雜湊的測試不受
// 檔案時間影響——時間戳是環境，不是介面行為。
func (s *Session) stamp(stat os.FileInfo) string {
	if s.Stamp != nil {
		return s.Stamp(stat)
	}
	return stat.ModTime().Format("01-02 15:04")
}

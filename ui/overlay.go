package ui

import "fmt"

// overlayScreen 是 S3 遊戲中覆蓋選單。開啟時入口停止呼叫 RunFrame，
// 鍵盤與手把事件全部送給 UI，machine 收到「全部放開」。這不是改寫晶片狀態，
// 是輸入來源的閘門——時間線本來就沒有前進。
type overlayScreen struct{ focus int }

func (s *overlayScreen) id() string { return "S3" }

func (s *overlayScreen) rows(u *UI) []menuRow {
	slot := fmt.Sprintf("%s%d", textSlotPrefix, u.config.Interface.SaveSlot)
	return []menuRow{
		{label: textResume, action: func(u *UI) { u.Close() }},
		{label: textSaveState, value: slot, chevron: true, action: func(u *UI) {
			u.push(&slotsScreen{mode: slotModeSave, focus: u.config.Interface.SaveSlot})
		}},
		{label: textLoadState, value: slot, chevron: true, action: func(u *UI) {
			u.push(&slotsScreen{mode: slotModeLoad, focus: u.config.Interface.SaveSlot})
		}},
		{label: textResetMachine, chevron: true, action: func(u *UI) {
			u.push(&resetScreen{})
		}},
		{label: textCheats, chevron: true, action: func(u *UI) { u.push(&cheatListScreen{}) }},
		{label: textSettings, chevron: true, action: func(u *UI) { u.push(&settingsScreen{}) }},
		{label: textDiagnostics, chevron: true, action: func(u *UI) { u.push(&diagnosticsScreen{}) }},
		{label: textScreenshot, hotkey: textScreenshotHK, action: func(u *UI) {
			u.emit(Capture{Kind: CaptureScreenshot})
			u.toast(textScreenshotSaved, SeverityInfo)
		}},
		{label: captureLabel(u), hotkey: textCaptureHK, value: captureValue(u), action: func(u *UI) {
			if u.diagnostics(nil).Recording {
				u.emit(Capture{Kind: CaptureClipStop})
				u.toast(textCaptureStopped, SeverityInfo)
				return
			}
			u.emit(Capture{Kind: CaptureClipStart})
			u.toast(textCaptureStarted, SeverityInfo)
		}},
		{label: textEjectCart, gapBefore: true, action: func(u *UI) {
			u.emit(UnloadCartridge{})
			u.Close()
		}},
		{label: textQuit, action: func(u *UI) {
			u.modal = &confirm{
				title: textQuitAsk, body: textQuitWhy, accept: textQuit,
				onYes: func(u *UI) { u.emit(Quit{}) },
			}
		}},
	}
}

func (s *overlayScreen) handle(u *UI, ev Event) bool {
	if handleMenu(u, ev, &s.focus, s.rows(u)) {
		return true
	}
	if e, ok := ev.(Action); ok && (e.Kind == ActCancel || e.Kind == ActMenu) {
		u.Close()
		return true
	}
	return false
}

func (s *overlayScreen) draw(u *UI, c *canvas, snap Snapshot) {
	m := u.metrics
	rows := s.rows(u)
	width := 0
	for _, row := range rows {
		w := c.font.Measure(row.label, m.BodySize) + c.font.Measure(row.value, m.BodySize) +
			m.RowPadX*3 + c.font.Measure("›", m.BodySize)
		width = max(width, w)
	}
	width = max(width, c.width()/3)
	titleH, footerH := m.RowHeight+m.Grid, m.RowHeight
	height := titleH + 1 + menuHeight(m, rows) + 1 + footerH
	x := (c.width() - width) / 2
	y := (c.height() - height) / 2

	c.rect(0, 0, c.width(), c.height(), u.theme.Scrim)
	c.rect(x, y, width, height, u.theme.Panel)
	c.border(x, y, width, height, u.theme.Border)

	title := textNoCartridge
	if snap != nil {
		if name, _, _ := snap.Cartridge(); name != "" {
			title = name
		}
	}
	c.rowText(x+m.RowPadX, y, titleH, m.BodySize, u.theme.Text, title)
	if u.paused {
		c.textRight(x+width-m.RowPadX, y+(titleH-c.font.Height(m.BodySize))/2, m.BodySize, u.theme.TextDim, "‖ "+textPaused)
	}
	c.rect(x, y+titleH, width, 1, u.theme.Border)

	end := drawMenuRows(u, c, x, y+titleH+1, width, rows, s.focus)
	c.rect(x, end, width, 1, u.theme.Border)
	c.rowText(x+m.RowPadX, end+1, footerH, m.SmallSize, u.theme.TextDim, statusLine(snap))
}

func captureLabel(u *UI) string {
	if u.diagnostics(nil).Recording {
		return textCaptureStop
	}
	return textCaptureStart
}

func captureValue(u *UI) string {
	facts := u.diagnostics(nil)
	if !facts.Recording {
		return ""
	}
	return fmt.Sprintf(textCaptureFrames, facts.CaptureFrames)
}

func statusLine(snap Snapshot) string {
	if snap == nil {
		return "—"
	}
	m68kCount, _ := snap.Instructions()
	return fmt.Sprintf("frame %s · 68k %s", group(snap.FrameIndex()), group(m68kCount))
}

// group 以三位一撇顯示大數字。
func group(v uint64) string {
	digits := fmt.Sprintf("%d", v)
	head := len(digits) % 3
	if head == 0 {
		head = 3
	}
	out := digits[:head]
	for i := head; i < len(digits); i += 3 {
		out += "," + digits[i:i+3]
	}
	return out
}

// resetScreen 是「重設主機」的子選單。冷開機與軟重設分開，因為兩者對裝置狀態
// 的影響不同，混成一個按鈕會讓使用者無法選擇。
type resetScreen struct{ focus int }

func (s *resetScreen) id() string { return "S3.reset" }

func (s *resetScreen) rows(u *UI) []menuRow {
	return []menuRow{
		{label: textSoftReset, action: func(u *UI) {
			u.emit(Reset{Kind: ResetSoft})
			u.Close()
		}},
		{label: textColdReset, action: func(u *UI) {
			u.emit(Reset{Kind: ResetCold})
			u.Close()
		}},
	}
}

func (s *resetScreen) handle(u *UI, ev Event) bool {
	if handleMenu(u, ev, &s.focus, s.rows(u)) {
		return true
	}
	if e, ok := ev.(Action); ok && e.Kind == ActCancel {
		u.pop()
		return true
	}
	return false
}

func (s *resetScreen) draw(u *UI, c *canvas, snap Snapshot) {
	m := u.metrics
	rows := s.rows(u)
	width := c.width() / 4
	height := m.RowHeight*(len(rows)+1) + 1
	x := (c.width() - width) / 2
	y := (c.height() - height) / 2
	c.rect(0, 0, c.width(), c.height(), u.theme.Scrim)
	c.rect(x, y, width, height, u.theme.Panel)
	c.border(x, y, width, height, u.theme.Border)
	c.rowText(x+m.RowPadX, y, m.RowHeight, m.BodySize, u.theme.Text, textResetMachine)
	c.rect(x, y+m.RowHeight, width, 1, u.theme.Border)
	drawMenuRows(u, c, x, y+m.RowHeight+1, width, rows, s.focus)
}

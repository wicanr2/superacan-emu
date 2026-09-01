package ui

// touchScreen 是 S5.6 觸控版面設定。手掌大小與慣用手的差異無法用單一預設涵蓋，
// 所以不透明度、大小、死區與左右手都要能調。
type touchScreen struct{ focus int }

func (t *touchScreen) id() string { return "S5.6" }

func (t *touchScreen) rows(u *UI) []optionRow {
	touch := &u.config.Input.Touch
	return []optionRow{
		{kind: optionRange, label: u.s.TouchOpacity, value: &touch.Opacity,
			min: 20, max: 100, step: 10, unit: "%", bar: true},
		{kind: optionRange, label: u.s.TouchScale, value: &touch.Scale,
			min: 70, max: 150, step: 10, unit: "%"},
		{kind: optionRange, label: u.s.TouchDeadzone, value: &touch.DPadDeadzone,
			min: 5, max: 40, step: 5, unit: "%", note: u.s.TouchDeadzoneNote},
		{kind: optionToggle, label: u.s.TouchSwapHands, flag: &touch.SwapHands},
		{kind: optionToggle, label: u.s.TouchStickMode, flag: &touch.StickMode,
			disabled: true, reason: u.s.StageStickMode},
		{kind: optionToggle, label: u.s.TouchHaptics, flag: &touch.Haptics},
	}
}

func (t *touchScreen) handle(u *UI, ev Event) bool {
	return handleOptions(u, ev, &t.focus, t.rows(u), func() {
		// 版面參數一改就要重算，否則要等到轉向才生效。
		u.touch.layout = TouchLayout{}
		u.emit(ApplyConfig{Config: u.config})
	})
}

func (t *touchScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	top, _ := page{title: u.s.TouchTitle, back: true, status: u.s.TouchNote}.draw(u, c)
	drawOptionRows(u, c, m.PanelPad, top, c.width()-m.PanelPad*2, t.rows(u), t.focus)
}

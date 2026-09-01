package ui

import "fmt"

// aspectChoices 與 filterChoices 的順序即設定檔存的字串順序。
var (
	aspectValues  = []string{"4:3", "1:1", "stretch"}
	aspectLabels  = []string{"4:3", "1:1 像素", "填滿"}
	filterValues  = []string{"nearest", "scanline25", "scanline50", "scanline75"}
	filterLabels  = []string{"Nearest", "Scanline 25", "Scanline 50", "Scanline 75"}
	bufferChoices = []int{50, 100, 200, 400}
)

func indexOf(values []string, value string) int {
	for index, item := range values {
		if item == value {
			return index
		}
	}
	return 0
}

// videoScreen 是 S5.3 影像設定。
type videoScreen struct {
	focus  int
	aspect int
	filter int
}

func (v *videoScreen) id() string { return "S5.3" }

func (v *videoScreen) sync(u *UI) {
	v.aspect = indexOf(aspectValues, u.config.Video.Aspect)
	v.filter = indexOf(filterValues, u.config.Video.Filter)
}

func (v *videoScreen) rows(u *UI) []optionRow {
	video := &u.config.Video
	return []optionRow{
		{kind: optionRange, label: textVideoScale, value: &video.Scale, min: 1, max: 8, step: 1, unit: "×",
			note: fmt.Sprintf("%d×%d", 320*video.Scale, 240*video.Scale)},
		{kind: optionToggle, label: textVideoInteger, flag: &video.IntegerScale},
		{kind: optionChoice, label: textVideoAspect, choices: aspectLabels, index: &v.aspect},
		{kind: optionChoice, label: textVideoFilter, choices: filterLabels, index: &v.filter},
		{kind: optionToggle, label: textVideoFullscreen, flag: &video.Fullscreen, note: "F11"},
		{kind: optionToggle, label: textVideoFrameBlend, flag: &video.FrameBlend,
			disabled: true, reason: textStageFrameBlend},
		{kind: optionToggle, label: textVideoShowFPS, flag: &video.ShowFPS, note: "F10"},
		{kind: optionToggle, label: textVideoSuppress, flag: &u.config.Interface.SuppressInfoToasts,
			note: textVideoSuppressNote},
	}
}

func (v *videoScreen) handle(u *UI, ev Event) bool {
	rows := v.rows(u)
	return handleOptions(u, ev, &v.focus, rows, func() {
		u.config.Video.Aspect = aspectValues[v.aspect]
		u.config.Video.Filter = filterValues[v.filter]
		u.emit(ApplyConfig{Config: u.config})
	})
}

func (v *videoScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	v.sync(u)
	top, _ := page{title: textVideoTitle, back: true, status: textVideoApertureNote}.draw(u, c)
	drawOptionRows(u, c, m.PanelPad, top, c.width()-m.PanelPad*2, v.rows(u), v.focus)
}

// audioScreen 是 S5.4 音訊設定。
//
// 沒有等化器、沒有各聲道音量、沒有增益調整：UM6619 的混音增益與削波還沒升格成
// 硬體事實，在取得同狀態 oracle 之前提供調整介面，等於邀請使用者用聽感覆蓋
// 未定的硬體行為。
type audioScreen struct {
	focus  int
	buffer int
}

func (a *audioScreen) id() string { return "S5.4" }

func (a *audioScreen) rows(u *UI) []optionRow {
	audio := &u.config.Audio
	sink := audio.Sink
	if sink == "" {
		sink = textAudioNoSink
	}
	stats := u.audioStats()
	return []optionRow{
		{kind: optionRange, label: textAudioVolume, value: &audio.MasterVolume,
			min: 0, max: 100, step: 5, bar: true},
		{kind: optionToggle, label: textAudioMuteFast, flag: &audio.MuteOnFastFwd},
		{kind: optionRange, label: textAudioBuffer, value: &audio.BufferMS,
			min: 50, max: 400, step: 50, unit: " ms", note: textAudioBufferNote},
		{kind: optionReadOnly, label: textAudioSink, text: sink},
		{kind: optionReadOnly, label: textAudioFormat, text: textAudioFormatValue},
		{kind: optionReadOnly, label: textAudioBufferState,
			text: fmt.Sprintf("%s 已用 %d / %d ms",
				meterBar(stats.BufferedMS, 0, max(stats.BufferMS, 1), 16), stats.BufferedMS, stats.BufferMS)},
		{kind: optionReadOnly, label: textAudioUnderrun, text: fmt.Sprintf("%d", stats.Underruns)},
	}
}

func (a *audioScreen) handle(u *UI, ev Event) bool {
	return handleOptions(u, ev, &a.focus, a.rows(u), func() {
		u.emit(SetVolume{Percent: u.config.Audio.MasterVolume})
		u.emit(ApplyConfig{Config: u.config})
	})
}

func (a *audioScreen) draw(u *UI, c *canvas, _ Snapshot) {
	m := u.metrics
	top, _ := page{title: textAudioTitle, back: true, status: textAudioNote}.draw(u, c)
	drawOptionRows(u, c, m.PanelPad, top, c.width()-m.PanelPad*2, a.rows(u), a.focus)
}

// AudioStats 是主機播放端的狀態。這些數字只描述播放，不描述 UM6619。
type AudioStats struct {
	BufferedMS int
	BufferMS   int
	Underruns  uint64
}

// AudioStatsSource 由入口提供。
type AudioStatsSource interface{ AudioStats() AudioStats }

func (u *UI) audioStats() AudioStats {
	if u.audio == nil {
		return AudioStats{BufferMS: u.config.Audio.BufferMS}
	}
	stats := u.audio.AudioStats()
	if stats.BufferMS == 0 {
		stats.BufferMS = u.config.Audio.BufferMS
	}
	return stats
}

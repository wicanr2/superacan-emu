package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wicanr2/superacan-emu/session"
	"github.com/wicanr2/superacan-emu/ui"
)

// uiEvents 是 --ui-script 認得的事件名稱。名稱是介面層的抽象動作，不是按鍵：
// 同一個腳本在任何前端上意義相同。
var uiEvents = map[string]ui.Event{
	"menu":      ui.Action{Kind: ui.ActMenu},
	"confirm":   ui.Action{Kind: ui.ActConfirm},
	"cancel":    ui.Action{Kind: ui.ActCancel},
	"delete":    ui.Action{Kind: ui.ActDelete},
	"tabprev":   ui.Action{Kind: ui.ActTabPrev},
	"tabnext":   ui.Action{Kind: ui.ActTabNext},
	"up":        ui.Nav{Dir: ui.DirUp},
	"down":      ui.Nav{Dir: ui.DirDown},
	"left":      ui.Nav{Dir: ui.DirLeft},
	"right":     ui.Nav{Dir: ui.DirRight},
	"home":      ui.Edge{To: ui.EdgeHome},
	"end":       ui.Edge{To: ui.EdgeEnd},
	"back":      ui.Life{Kind: ui.LifeBack},
	"secondary": ui.Action{Kind: ui.ActSecondary},
}

// parseUIScript 讀 frame:event 對，格式與 --press 一致。
func parseUIScript(spec string) (map[uint64][]ui.Event, error) {
	script := make(map[uint64][]ui.Event)
	if strings.TrimSpace(spec) == "" {
		return script, nil
	}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		frameText, name, found := strings.Cut(entry, ":")
		if !found {
			return nil, fmt.Errorf("ui script entry %q is not frame:event", entry)
		}
		frame, err := strconv.ParseUint(strings.TrimSpace(frameText), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ui script entry %q: %w", entry, err)
		}
		event, ok := uiEvents[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, fmt.Errorf("ui script event %q is not one of %s", name, uiEventNames())
		}
		script[frame] = append(script[frame], event)
	}
	return script, nil
}

func uiEventNames() string {
	names := make([]string, 0, len(uiEvents))
	for name := range uiEvents {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// uiSurface 解析 WxH。
func uiSurface(spec string, profile ui.Profile, scale int) (ui.Surface, error) {
	widthText, heightText, found := strings.Cut(strings.ToLower(spec), "x")
	if !found {
		return ui.Surface{}, fmt.Errorf("ui surface %q is not WxH", spec)
	}
	width, err := strconv.Atoi(widthText)
	if err != nil {
		return ui.Surface{}, err
	}
	height, err := strconv.Atoi(heightText)
	if err != nil {
		return ui.Surface{}, err
	}
	if width <= 0 || height <= 0 {
		return ui.Surface{}, fmt.Errorf("ui surface %q must be positive", spec)
	}
	return ui.Surface{W: width, H: height, Scale: scale, Profile: profile}, nil
}

// composeFrame 產生「遊戲畫面加覆蓋層」的合成圖，供雜湊與 PNG 輸出。
func composeFrame(s *session.Session, surface ui.Surface) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, surface.W, surface.H))
	s.Compose(dst)
	return dst
}

func writeComposedPNG(name string, frame *image.RGBA) error {
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, frame)
}

// frameClock 讓 UI 的時間跟著模擬 frame 走，而不是掛鐘。toast 的存活時間因此
// 在 headless 是可重現的。
func frameClock(frame uint64) time.Duration {
	return time.Duration(frame) * 16667 * time.Microsecond
}

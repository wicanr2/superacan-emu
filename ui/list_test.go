package ui

import "testing"

func TestListWindowKeepsFocusVisible(t *testing.T) {
	cases := []struct {
		name              string
		top, focus, count int
		height, rowHeight int
		wantFirst         int
		wantLast          int
		wantTop           int
	}{
		{"整份放得下", 0, 0, 5, 500, 24, 0, 5, 0},
		{"焦點在下方時往下捲", 0, 12, 17, 240, 24, 3, 13, 3},
		{"焦點在上方時往上捲", 5, 2, 17, 240, 24, 2, 12, 2},
		{"清單縮短時捲動位置跟著收回", 12, 1, 5, 240, 24, 0, 5, 0},
		{"高度不足一列時至少畫一列", 0, 3, 17, 10, 24, 3, 4, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			top := tc.top
			first, last := listWindow(&top, tc.focus, tc.count, tc.height, tc.rowHeight)
			if first != tc.wantFirst || last != tc.wantLast || top != tc.wantTop {
				t.Fatalf("first=%d last=%d top=%d，預期 %d/%d/%d",
					first, last, top, tc.wantFirst, tc.wantLast, tc.wantTop)
			}
			if tc.focus < first || tc.focus >= last {
				t.Fatalf("焦點 %d 落在可視範圍 [%d,%d) 之外", tc.focus, first, last)
			}
		})
	}
}

// 十七個熱鍵在觸控版面放不下一頁。焦點移到最後一個動作時，那一列必須被畫出來——
// 畫不出來的列等於那個動作沒辦法重新指定。
func TestHotkeyScreenScrollsToTheLastAction(t *testing.T) {
	surface := Surface{W: 1280, H: 720, Scale: 1, Profile: ProfileTouch}
	u := New(Options{Surface: surface, Config: DefaultConfig(), Slots: fixedSlots{}})
	u.Update(0)
	u.Open()
	h := &hotkeyScreen{}
	u.push(h)

	// 焦點在清單兩端會繞回，所以按 len-1 次剛好停在最後一列。
	for range len(Hotkeys) - 1 {
		u.Handle(Nav{Dir: DirDown})
	}
	if got, want := h.focus, len(Hotkeys)-1; got != want {
		t.Fatalf("焦點停在第 %d 列，預期最後一列 %d", got, want)
	}
	render(t, "S5.2-scrolled", u, surface)

	m := u.metrics
	first, _ := listWindow(&h.top, h.focus, len(Hotkeys), surface.H-m.FooterBar-m.TitleBar-m.RowHeight, m.RowHeight)
	if first == 0 {
		t.Fatal("清單沒有捲動，最後幾個動作畫不出來")
	}
	if h.focus < h.top {
		t.Fatalf("焦點 %d 在捲動位置 %d 之上", h.focus, h.top)
	}
}

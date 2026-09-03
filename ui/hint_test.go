package ui

import (
	"image"
	"testing"
)

// 提示畫在最外層那一層——覆蓋選單每一頁都有頁尾提示，唯獨遊戲畫面沒有，
// 第一次執行的人因此看不到任何操作線索。這幾條釘住它出現與消失的條件。
func TestMenuHintShowsUntilTheMenuIsOpened(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := newTestUI(surface)
	u.SetDefaultHotkeys(map[string]Binding{"menu": {Frontend: "x11", Code: 1, Label: "F1"}})

	if !drawsMenuHint(u, surface) {
		t.Fatal("剛載入卡帶時沒有畫出開選單的提示")
	}

	u.Open()
	u.Close()
	if drawsMenuHint(u, surface) {
		t.Fatal("開過選單之後提示還在")
	}
}

func TestMenuHintDisappearsOnItsOwn(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := newTestUI(surface)
	u.SetDefaultHotkeys(map[string]Binding{"menu": {Frontend: "x11", Code: 1, Label: "F1"}})

	if !drawsMenuHint(u, surface) {
		t.Fatal("提示一開始就沒出現")
	}
	u.Update(menuHintDuration)
	if drawsMenuHint(u, surface) {
		t.Fatalf("超過 %v 之後提示還在", menuHintDuration)
	}
}

// 觸控版面不畫：那邊螢幕上就有 ☰ 鍵，再疊一行字只是擋畫面。
func TestMenuHintIsDesktopOnly(t *testing.T) {
	surface := Surface{W: 1280, H: 720, Scale: 1, Profile: ProfileTouch}
	u := newTestUI(surface)
	u.SetDefaultHotkeys(map[string]Binding{"menu": {Frontend: "x11", Code: 1, Label: "F1"}})
	if drawsMenuHint(u, surface) {
		t.Fatal("觸控版面不該畫開選單的提示")
	}
}

// 沒有綁定就不提示：寫死一個鍵名在改過鍵位之後會變成假訊息。
func TestMenuHintNeedsABinding(t *testing.T) {
	surface := Surface{W: 960, H: 720, Scale: 1, Profile: ProfileCompact}
	u := newTestUI(surface)
	u.SetDefaultHotkeys(map[string]Binding{})
	if drawsMenuHint(u, surface) {
		t.Fatal("沒有綁定卻仍然提示")
	}
}

// drawsMenuHint 比對「有畫提示」與「跳過提示」兩張圖：左下角那一塊有差就是畫了。
// 直接比像素而不是看旗標，測的才是使用者真的看得到的東西。
//
// menuHintSince／menuHintStarted 是刻意不還原的：它們記的是「提示第一次出現在
// 什麼時候」，還原等於把計時歸零，自動消失那條就永遠測不到。
func drawsMenuHint(u *UI, surface Surface) bool {
	withHint := renderImage(u, surface)

	opened := u.menuOpened
	u.menuOpened = true
	without := renderImage(u, surface)
	u.menuOpened = opened

	corner := image.Rect(0, surface.H-u.metrics.RowHeight-u.metrics.Grid*2, surface.W/2, surface.H)
	for y := corner.Min.Y; y < corner.Max.Y; y++ {
		for x := corner.Min.X; x < corner.Max.X; x++ {
			if withHint.RGBAAt(x, y) != without.RGBAAt(x, y) {
				return true
			}
		}
	}
	return false
}

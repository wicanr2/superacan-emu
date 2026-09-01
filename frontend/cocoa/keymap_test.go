package cocoa

import "testing"

// 鍵碼與鍵名的對照要一一對應。重複的鍵碼表示表裡有兩個名稱指到同一個實體鍵，
// 設定畫面會顯示錯的那一個。
func TestKeyNamesAreUnique(t *testing.T) {
	seen := map[string]uint16{}
	for code, name := range KeyNames {
		if other, clash := seen[name]; clash {
			t.Errorf("%q 同時對到 $%02X 與 $%02X", name, other, code)
		}
		seen[name] = code
	}
}

// 常用鍵的鍵碼要與 Carbon 的 kVK_* 相同。這些值是實體位置，寫錯的話在真機上
// 會按到別的鍵，而且在 Linux 上編譯得過、測不出來。
func TestWellKnownKeyCodes(t *testing.T) {
	for _, c := range []struct {
		name string
		code uint16
		want uint16
	}{
		{"A", KeyA, 0x00}, {"Z", KeyZ, 0x06}, {"X", KeyX, 0x07},
		{"Return", KeyReturn, 0x24}, {"Tab", KeyTab, 0x30},
		{"Escape", KeyEscape, 0x35}, {"RightShift", KeyRightShift, 0x3c},
		{"Left", KeyLeft, 0x7b}, {"Right", KeyRight, 0x7c},
		{"Down", KeyDown, 0x7d}, {"Up", KeyUp, 0x7e},
		{"F1", KeyF1, 0x7a}, {"F5", KeyF5, 0x60}, {"F9", KeyF9, 0x65},
	} {
		if c.code != c.want {
			t.Errorf("%s = $%02X，want $%02X", c.name, c.code, c.want)
		}
	}
}

func TestKeyLabelFallsBackToEmpty(t *testing.T) {
	if KeyLabel(KeyF1) != "F1" {
		t.Fatalf("F1 的名稱是 %q", KeyLabel(KeyF1))
	}
	if KeyLabel(0xffff) != "" {
		t.Fatal("未知鍵碼不該編一個名字出來")
	}
}

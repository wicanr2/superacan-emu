package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/superacan-emu/ui"
)

// 重新綁定之後寫檔再讀回，綁定必須相同——不然「設定」等於沒有設定。
func TestConfigRoundTripKeepsBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)
	config := ui.DefaultConfig()
	config.Input.Players[0].Keyboard["a"] = ui.Binding{Frontend: "x11", Code: 0x7a, Label: "z"}
	config.Input.Hotkeys["menu"] = ui.Binding{Frontend: "x11", Code: 0xffbe, Label: "F1"}
	config.Interface.SaveSlot = 4

	if err := SaveConfig(path, config); err != nil {
		t.Fatal(err)
	}
	loaded, warnings, err := LoadConfig(path)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if loaded.Input.Players[0].Keyboard["a"] != config.Input.Players[0].Keyboard["a"] {
		t.Fatalf("玩家綁定 %+v", loaded.Input.Players[0].Keyboard)
	}
	if loaded.Input.Hotkeys["menu"] != config.Input.Hotkeys["menu"] {
		t.Fatalf("熱鍵 %+v", loaded.Input.Hotkeys)
	}
	if loaded.Interface.SaveSlot != 4 {
		t.Fatalf("存檔槽 %d", loaded.Interface.SaveSlot)
	}
}

// 未知的頂層鍵要保留。只忽略不保留的話，舊版寫一次設定就把新版的欄位刪光。
func TestUnknownKeysSurviveARoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)
	original := []byte(`{
  "config_version": 1,
  "ui": {"save_slot": 2},
  "future_feature": {"nested": [1, 2, 3], "flag": true},
  "another_unknown": "keep me"
}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, _, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Unknown) != 2 {
		t.Fatalf("未知鍵 %v", loaded.Unknown)
	}
	if err := SaveConfig(path, loaded); err != nil {
		t.Fatal(err)
	}

	var document map[string]json.RawMessage
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(written, &document); err != nil {
		t.Fatal(err)
	}
	var future struct {
		Nested []int `json:"nested"`
		Flag   bool  `json:"flag"`
	}
	if err := json.Unmarshal(document["future_feature"], &future); err != nil {
		t.Fatal(err)
	}
	if len(future.Nested) != 3 || !future.Flag {
		t.Fatalf("未知鍵內容被改動：%+v", future)
	}
	if string(document["another_unknown"]) != `"keep me"` {
		t.Fatalf("未知鍵 %s", document["another_unknown"])
	}
}

// 整份無法解析時改名而不是覆寫，使用者才有機會救回自己的綁定。
func TestBrokenConfigIsRenamedNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	broken := []byte(`{"ui": {"save_slot": ]]] this is not json`)
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}

	config, warnings, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("應該記一則 warn，得到 %v", warnings)
	}
	if config.Interface.SaveSlot != ui.DefaultConfig().Interface.SaveSlot {
		t.Fatal("解析失敗時應該用預設值")
	}
	saved, err := os.ReadFile(path + ".bad")
	if err != nil {
		t.Fatalf("原內容必須被保留成 .bad：%v", err)
	}
	if string(saved) != string(broken) {
		t.Fatal(".bad 的內容必須與原檔相同")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("原檔應該已經被改名")
	}
}

// 一個壞欄位不該讓所有設定歸零。
func TestTypeMismatchOnlyResetsThatField(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)
	document := []byte(`{
  "config_version": 1,
  "ui": {"save_slot": 7},
  "video": "this should be an object"
}`)
	if err := os.WriteFile(path, document, 0o644); err != nil {
		t.Fatal(err)
	}
	config, warnings, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0].Field != "video" {
		t.Fatalf("warnings=%v", warnings)
	}
	if config.Interface.SaveSlot != 7 {
		t.Fatalf("其他欄位不該被影響，save_slot=%d", config.Interface.SaveSlot)
	}
	if config.Video.Scale != ui.DefaultConfig().Video.Scale {
		t.Fatalf("壞掉的欄位應該回到預設，scale=%d", config.Video.Scale)
	}
}

// 綁定帶前端識別：別的前端寫的鍵碼不得被硬套。
func TestBindingsAreScopedToTheFrontendThatWroteThem(t *testing.T) {
	binding := ui.Binding{Frontend: "x11", Code: 0x7a, Label: "z"}
	if !binding.Usable("x11") {
		t.Fatal("同一個前端應該可以用")
	}
	if binding.Usable("ebiten") {
		t.Fatal("別的前端的鍵碼不得被套用")
	}
	if (ui.Binding{}).Usable("x11") {
		t.Fatal("空綁定不可用")
	}
}

// 原子寫入：寫檔過程中不得留下半份設定。
func TestSaveConfigIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	if err := SaveConfig(path, ui.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Fatalf("暫存檔沒有清掉：%s", entry.Name())
		}
	}
}

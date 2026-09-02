package ui

import (
	"reflect"
	"testing"
)

// 五種語言的字串表都要完整：少一個 key 會變成畫面上的空白，而空白不會有人注意到。
func TestEveryLanguageHasEveryString(t *testing.T) {
	for _, language := range Languages {
		strings := stringsFor(language)
		value := reflect.ValueOf(strings)
		for index := 0; index < value.NumField(); index++ {
			if value.Field(index).String() == "" {
				t.Errorf("%s 缺少 %s", language, value.Type().Field(index).Name)
			}
		}
	}
}

// 語言表不得有 Strings 沒有的 key，也不得漏掉 Strings 的欄位。
func TestTranslationTableMatchesStruct(t *testing.T) {
	fields := map[string]bool{}
	structType := reflect.TypeOf(Strings{})
	for index := 0; index < structType.NumField(); index++ {
		fields[structType.Field(index).Name] = true
	}
	seen := map[string]bool{}
	for _, row := range translations {
		if !fields[row.key] {
			t.Errorf("語言表有多餘的 key：%s", row.key)
		}
		if seen[row.key] {
			t.Errorf("語言表重複了 key：%s", row.key)
		}
		seen[row.key] = true
	}
	for name := range fields {
		if !seen[name] {
			t.Errorf("語言表漏了 %s", name)
		}
	}
}

// 換語言之後同一畫面必須不同；不同表示字串真的換了，而不是被快取住。
func TestSwitchingLanguageChangesTheScreen(t *testing.T) {
	seen := map[string]Language{}
	for _, language := range Languages {
		config := DefaultConfig()
		config.Interface.Language = string(language)
		u := New(Options{
			Surface: surfaceCases[0].surface, Config: config, Slots: fixedSlots{},
			Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true}, About: fixedAbout,
			AudioStats: fixedAudioStats{}, Diagnostics: fixedDiagnostics{},
		})
		u.Update(0)
		u.Open()
		hash := render(t, "S3-"+string(language), u, surfaceCases[0].surface)
		if other, clash := seen[hash]; clash {
			t.Fatalf("%s 與 %s 的畫面相同", language, other)
		}
		seen[hash] = language
	}
}

// 未知的語言標籤回到繁體中文，而不是變成一片空白。
func TestUnknownLanguageFallsBack(t *testing.T) {
	config := DefaultConfig()
	config.Interface.Language = "kl-GL"
	u := New(Options{Surface: surfaceCases[0].surface, Config: config})
	if u.Language() != LanguageTraditionalChinese {
		t.Fatalf("回退到 %s", u.Language())
	}
	if u.s.Resume == "" {
		t.Fatal("回退之後字串表不得是空的")
	}
}

// 切換語言要寫回設定，否則下次啟動又變回去。
func TestLanguageSwitchWritesConfig(t *testing.T) {
	u := New(Options{Surface: surfaceCases[0].surface, Config: DefaultConfig()})
	u.Open()
	u.push(&languageScreen{})
	u.TakeIntents()
	u.Handle(Action{Kind: ActConfirm}) // 第一項是 English
	if u.Language() != LanguageEnglish {
		t.Fatalf("語言 %s", u.Language())
	}
	if u.config.Interface.Language != string(LanguageEnglish) {
		t.Fatalf("設定裡的語言 %q", u.config.Interface.Language)
	}
	intents := u.TakeIntents()
	if len(intents) != 1 {
		t.Fatalf("要求存檔，得到 %#v", intents)
	}
	if _, ok := intents[0].(ApplyConfig); !ok {
		t.Fatalf("得到 %#v", intents[0])
	}
}

// 翻譯之後的字串比原文長是常態。每個畫面在五種語言下都不能讓文字掉出畫面。
func TestNoScreenOverflowsInAnyLanguage(t *testing.T) {
	screens := map[string]func() screen{
		"S0":   func() screen { return &startScreen{} },
		"S0.1": func() screen { return &firmwareScreen{} },
		"S1":   func() screen { return &browserScreen{} },
		"S3":   func() screen { return &overlayScreen{} },
		"S4":   func() screen { return &slotsScreen{mode: slotModeSave} },
		"S5":   func() screen { return &settingsScreen{} },
		"S5.1": func() screen { return &bindingScreen{} },
		"S5.2": func() screen { return &hotkeyScreen{} },
		"S5.3": func() screen { return &videoScreen{} },
		"S5.4": func() screen { return &audioScreen{} },
		"S5.5": func() screen { return &languageScreen{} },
		"S6.1": func() screen { return &cheatSearchScreen{width: 1} },
		"S6.2": func() screen { return &cheatListScreen{} },
		"S7":   func() screen { return &diagnosticsScreen{} },
		"S8":   func() screen { return &aboutScreen{} },
		"S9":   func() screen { return &haltScreen{} },
	}
	for _, language := range Languages {
		for _, surface := range surfaceCases {
			for name, build := range screens {
				config := DefaultConfig()
				config.Interface.Language = string(language)
				u := New(Options{
					Surface: surface.surface, Config: config, Slots: fixedSlots{},
					Library: fixedLibrary{}, Firmware: fixedFirmware{complete: true},
					About: fixedAbout, AudioStats: fixedAudioStats{},
					Diagnostics: fixedDiagnostics{}, Cheats: fixedCheats{enabled: true},
				})
				u.Update(0)
				// 出廠鍵位也會出現在 S5.2 的鍵位欄，用最長的鍵名壓一次版面。
				u.SetDefaultHotkeys(longestKeyNameDefaults())
				u.SetMode(ModeShell, "停止原因")
				u.stack = []screen{build()}
				render(t, name+"-"+string(language), u, surface.surface)
				limit := surface.surface.W / surface.surface.Scale
				if u.TextRightEdge() > limit {
					t.Errorf("%s / %s / %s：文字畫到 %d，畫面只有 %d 寬",
						name, language, surface.name, u.TextRightEdge(), limit)
				}
			}
		}
	}
}

func TestLanguageScreenRenders(t *testing.T) {
	u := newSettingsUI(t, nil)
	u.Open()
	u.push(&languageScreen{})
	checkHash(t, "S5.5/"+surfaceCases[0].name,
		render(t, "S5.5/"+surfaceCases[0].name, u, surfaceCases[0].surface))
}

// longestKeyNameDefaults 用最長的鍵名填滿每一個熱鍵，讓版面測試量到最壞情況。
func longestKeyNameDefaults() map[string]Binding {
	defaults := make(map[string]Binding, len(Hotkeys))
	for index, action := range Hotkeys {
		defaults[action] = Binding{Frontend: "test", Code: uint32(index + 1), Label: "RightShift"}
	}
	return defaults
}

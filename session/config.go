package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wicanr2/superacan-emu/ui"
)

// ConfigFileName 是設定檔名。
const ConfigFileName = "config.json"

// ConfigPath 回傳這個平台的設定檔路徑。
func ConfigPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "superacan-emu", ConfigFileName), nil
	default:
		// Linux 與 Android 都走 XDG；os.UserConfigDir 已經實作 XDG_CONFIG_HOME
		// 與 ~/.config 的回退。
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "superacan-emu", ConfigFileName), nil
	}
}

// ConfigWarning 是讀取設定時遇到、但不足以讓整份設定失效的問題。
type ConfigWarning struct {
	Field  string
	Detail string
}

func (w ConfigWarning) String() string { return fmt.Sprintf("%s：%s", w.Field, w.Detail) }

// LoadConfig 讀設定檔。
//
// 三條規則各自對應一種失敗：欄位型別不符只讓那個欄位回到預設並記一則 warn，
// 一個壞欄位不該讓所有設定歸零；未知的頂層鍵保留原樣，否則舊版寫一次設定就會把
// 新版的欄位刪光；整份無法解析時把檔案改名成 config.json.bad 再用預設值繼續，
// 使用者才有機會救回自己的綁定。檔案不存在不是錯誤。
func LoadConfig(path string) (ui.Config, []ConfigWarning, error) {
	config := ui.DefaultConfig()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil, nil
	}
	if err != nil {
		return config, nil, err
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		bad := path + ".bad"
		if renameErr := os.Rename(path, bad); renameErr != nil {
			return config, nil, fmt.Errorf("設定檔無法解析且無法改名：%w", renameErr)
		}
		return config, []ConfigWarning{{
			Field:  ConfigFileName,
			Detail: fmt.Sprintf("無法解析，已改名為 %s，本次使用預設值", filepath.Base(bad)),
		}}, nil
	}

	var warnings []ConfigWarning
	targets := configTargets(&config)
	for key, value := range document {
		target, known := targets[key]
		if !known {
			if config.Unknown == nil {
				config.Unknown = map[string]json.RawMessage{}
			}
			config.Unknown[key] = value
			continue
		}
		if err := json.Unmarshal(value, target); err != nil {
			warnings = append(warnings, ConfigWarning{Field: key, Detail: "型別不符，使用預設值"})
		}
	}
	return config, warnings, nil
}

// SaveConfig 原子寫入設定檔：先寫暫存檔再改名，中途失敗不會留下半份設定。
// 未識別的頂層鍵原樣寫回。
func SaveConfig(path string, config ui.Config) error {
	document := make(map[string]json.RawMessage, len(config.Unknown)+16)
	for key, value := range config.Unknown {
		document[key] = value
	}
	for key, target := range configTargets(&config) {
		encoded, err := json.Marshal(target)
		if err != nil {
			return err
		}
		document[key] = encoded
	}
	// map 的鍵在編碼時會排序，所以同一份設定每次寫出的位元組相同。
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// configTargets 把頂層鍵對到欄位指標。逐鍵解碼才能做到「一個壞欄位不影響其他」，
// 直接對整個結構 Unmarshal 會在第一個型別錯誤就放棄整份。
func configTargets(config *ui.Config) map[string]any {
	return map[string]any{
		"config_version": &config.ConfigVersion,
		"firmware":       &config.Firmware,
		"paths":          &config.Paths,
		"recent":         &config.Recent,
		"video":          &config.Video,
		"audio":          &config.Audio,
		"input":          &config.Input,
		"ui":             &config.Interface,
		"diagnostics":    &config.Diagnostics,
		"cheats":         &config.Cheats,
	}
}

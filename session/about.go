package session

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sort"

	"github.com/wicanr2/superacan-emu/ui"
)

// moduleLicenses 是第三方模組的授權。清單由 TestEveryDependencyHasALicense 守著：
// 建置資訊裡出現而這裡沒有的模組會讓測試失敗，所以發行包的授權清單不會漏掉相依。
var moduleLicenses = map[string]string{
	"github.com/hajimehoshi/ebiten/v2":     "Apache-2.0",
	"github.com/hajimehoshi/bitmapfont/v4": "Apache-2.0（字型資料另有六份來源授權，見 docs/ui-font.md）",
	"github.com/jezek/xgb":                 "BSD-3-Clause",
	"github.com/ebitengine/gomobile":       "BSD-3-Clause",
	"github.com/ebitengine/hideconsole":    "Apache-2.0",
	"github.com/ebitengine/oto/v3":         "Apache-2.0",
	"github.com/ebitengine/purego":         "Apache-2.0",
	"github.com/pierrec/lz4/v4":            "BSD-3-Clause",
	"golang.org/x/image":                   "BSD-3-Clause",
	"golang.org/x/sync":                    "BSD-3-Clause",
	"golang.org/x/sys":                     "BSD-3-Clause",
	"golang.org/x/text":                    "BSD-3-Clause",
}

// About 由建置資訊組出關於畫面的內容。第三方清單直接來自 debug.ReadBuildInfo，
// 不是人工維護的副本，所以不會與實際連進去的東西不一致。
func About(version, buildDate string) ui.AboutInfo {
	info := ui.AboutInfo{
		Version:    version,
		BuildDate:  buildDate,
		GoVersion:  runtime.Version(),
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		CGOEnabled: cgoEnabled,
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, module := range build.Deps {
		license, known := moduleLicenses[module.Path]
		if !known {
			license = "未登記授權"
		}
		info.Dependencies = append(info.Dependencies, ui.Dependency{
			Path: module.Path, Version: module.Version, License: license,
		})
	}
	sort.Slice(info.Dependencies, func(i, j int) bool {
		return info.Dependencies[i].Path < info.Dependencies[j].Path
	})
	return info
}

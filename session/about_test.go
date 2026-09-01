package session

import (
	"runtime/debug"
	"testing"
)

// 發行包要附完整的第三方授權清單。這條測試把「清單」綁在實際連進去的模組上：
// 新增相依而忘了登記授權，測試就會紅，而不是等到發行時才發現漏掉。
func TestEveryDependencyHasALicense(t *testing.T) {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("沒有建置資訊")
	}
	for _, module := range build.Deps {
		if _, known := moduleLicenses[module.Path]; !known {
			t.Errorf("%s %s 沒有登記授權", module.Path, module.Version)
		}
	}
}

// 關於畫面顯示的 cgo 狀態必須是這個 binary 的真實情況，不是寫死的字串。
func TestAboutReportsBuildFacts(t *testing.T) {
	info := About("0.1.0", "2026-09-01")
	if info.Version != "0.1.0" || info.BuildDate != "2026-09-01" {
		t.Fatalf("得到 %+v", info)
	}
	if info.GoVersion == "" || info.Platform == "" {
		t.Fatalf("建置資訊不完整：%+v", info)
	}
	if info.CGOEnabled != cgoEnabled {
		t.Fatalf("cgo 狀態 %v，want %v", info.CGOEnabled, cgoEnabled)
	}
	for _, dep := range info.Dependencies {
		if dep.License == "未登記授權" {
			t.Errorf("%s 沒有登記授權", dep.Path)
		}
	}
}
